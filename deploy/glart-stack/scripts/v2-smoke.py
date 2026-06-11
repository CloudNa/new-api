#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import sys
import time
from dataclasses import dataclass
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen


@dataclass
class Check:
    name: str
    ok: bool
    message: str = ""
    skipped: bool = False


BASE_URL = os.environ.get("GLART_BASE_URL", "https://api.glart.cn").rstrip("/")
API_KEY = os.environ.get("GLART_API_KEY", "").strip()
MODEL = os.environ.get("GLART_SMOKE_MODEL", "gpt-5.5").strip() or "gpt-5.5"
ROOT_USER_ID = os.environ.get("GLART_ROOT_USER_ID", "").strip()
ROOT_ACCESS_TOKEN = os.environ.get("GLART_ROOT_ACCESS_TOKEN", "").strip()
ROOT_COOKIE = os.environ.get("GLART_ROOT_COOKIE", "").strip()
REQUIRE_ROOT = os.environ.get("GLART_V2_SMOKE_REQUIRE_ROOT", "0").strip() == "1"
REQUIRE_CHAT = os.environ.get("GLART_V2_SMOKE_REQUIRE_CHAT", "0").strip() == "1"
RUN_STREAM = os.environ.get("GLART_SMOKE_STREAM", "0").strip() == "1"
RUN_LONG_CONTEXT = os.environ.get("GLART_SMOKE_LONG_CONTEXT", "0").strip() == "1"
PROFIT_EVENT_WAIT_SECONDS = int(os.environ.get("GLART_PROFIT_EVENT_WAIT_SECONDS", "20") or "20")

checks: list[Check] = []


def add_check(name: str, ok: bool, message: str = "", skipped: bool = False) -> None:
    checks.append(Check(name=name, ok=ok, message=message, skipped=skipped))
    label = "SKIP" if skipped else "OK" if ok else "FAIL"
    suffix = f" - {message}" if message else ""
    print(f"[{label}] {name}{suffix}")


def has_root_auth() -> bool:
    return bool(ROOT_USER_ID and (ROOT_ACCESS_TOKEN or ROOT_COOKIE))


def root_headers() -> dict[str, str]:
    headers: dict[str, str] = {}
    if ROOT_ACCESS_TOKEN:
        headers["Authorization"] = ROOT_ACCESS_TOKEN
    if ROOT_COOKIE:
        headers["Cookie"] = ROOT_COOKIE
    if ROOT_USER_ID:
        headers["New-Api-User"] = ROOT_USER_ID
    return headers


def request_json(
    method: str,
    path: str,
    payload: Any | None = None,
    headers: dict[str, str] | None = None,
    timeout: int = 30,
) -> tuple[int, Any, str]:
    body: bytes | None = None
    request_headers = {"User-Agent": "glart-v2-smoke"}
    if headers:
        request_headers.update(headers)
    if payload is not None:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        request_headers["Content-Type"] = "application/json"
    req = Request(BASE_URL + path, data=body, headers=request_headers, method=method)
    try:
        with urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            text = raw.decode("utf-8", errors="replace")
            try:
                parsed = json.loads(text) if text.strip() else None
            except json.JSONDecodeError:
                parsed = None
            return resp.status, parsed, text
    except HTTPError as exc:
        raw = exc.read()
        text = raw.decode("utf-8", errors="replace")
        try:
            parsed = json.loads(text) if text.strip() else None
        except json.JSONDecodeError:
            parsed = None
        return exc.code, parsed, text
    except URLError as exc:
        return 0, None, str(exc.reason)


def api_success(payload: Any) -> bool:
    return isinstance(payload, dict) and payload.get("success") is True


def api_data(payload: Any) -> Any:
    if isinstance(payload, dict):
        return payload.get("data")
    return None


def skip_or_fail(name: str, reason: str, required: bool) -> None:
    add_check(name, False, reason, skipped=not required)


def smoke_status() -> None:
    status, payload, _ = request_json("GET", "/api/status", timeout=20)
    add_check("/api/status", status == 200 and api_success(payload), f"HTTP {status}")


def smoke_system_update() -> None:
    if not has_root_auth():
        skip_or_fail("root updater precheck/smoke", "missing GLART_ROOT_USER_ID plus root token or cookie", REQUIRE_ROOT)
        return

    status, payload, _ = request_json("GET", "/api/system_update/precheck", headers=root_headers(), timeout=30)
    data = api_data(payload)
    ok = status == 200 and api_success(payload) and isinstance(data, dict) and data.get("ok") is True
    add_check("updater precheck", ok, f"HTTP {status}; {data.get('message', '') if isinstance(data, dict) else ''}")

    status, payload, _ = request_json("POST", "/api/system_update/smoke", {}, headers=root_headers(), timeout=150)
    data = api_data(payload)
    ok = status == 200 and api_success(payload) and isinstance(data, dict) and data.get("ok") is True
    add_check("updater smoke", ok, f"HTTP {status}; {data.get('message', '') if isinstance(data, dict) else ''}")


def compression_sample_text() -> str:
    repeated = "\n".join(
        f"node_modules/.cache/rsbuild/chunk-{i % 12}.js warning: duplicate source map entry ignored"
        for i in range(900)
    )
    return (
        "Command: bun run build\n"
        "Exit code: 0\n"
        "The following build output is intentionally repetitive for V2 smoke.\n"
        + repeated
        + "\nDo not remove this final sentence; it verifies the compressed text remains non-empty.\n"
    )


def smoke_compression() -> None:
    if not has_root_auth():
        skip_or_fail("compression preview", "missing GLART_ROOT_USER_ID plus root token or cookie", REQUIRE_ROOT)
        return

    payload = {"mode": "stacked", "text": compression_sample_text()}
    status, response, _ = request_json("POST", "/api/compression/preview", payload, headers=root_headers(), timeout=60)
    data = api_data(response)
    stats = data.get("stats") if isinstance(data, dict) else {}
    ok = (
        status == 200
        and api_success(response)
        and isinstance(data, dict)
        and data.get("compressed") is True
        and isinstance(stats, dict)
        and stats.get("omniroute_compatible_mode") == "stacked"
        and int(stats.get("compression_saved_tokens") or 0) > 0
    )
    message = ""
    if isinstance(stats, dict):
        message = (
            f"mode={stats.get('omniroute_compatible_mode')}; "
            f"saved={stats.get('compression_saved_tokens')}"
        )
    add_check("OmniRoute-style stacked compression preview", ok, message or f"HTTP {status}")

    status, response, _ = request_json("GET", "/api/context/rtk/filters", headers=root_headers(), timeout=30)
    data = api_data(response)
    filters = data.get("filters") if isinstance(data, dict) else None
    attribution = data.get("attribution") if isinstance(data, dict) else ""
    ok = status == 200 and api_success(response) and isinstance(filters, list) and "OmniRoute" in str(attribution)
    add_check("RTK filter catalog attribution", ok, f"filters={len(filters) if isinstance(filters, list) else 0}")


def chat_payload(stream: bool = False, long_context: bool = False) -> dict[str, Any]:
    content = "Return exactly: ok"
    if long_context:
        context = "\n".join(f"line {i}: cost route smoke context repeat" for i in range(1400))
        content = f"{context}\n\nReturn exactly: ok"
    payload: dict[str, Any] = {
        "model": MODEL,
        "messages": [{"role": "user", "content": content}],
        "max_tokens": 16,
    }
    if stream:
        payload["stream"] = True
    return payload


def smoke_chat() -> bool:
    if not API_KEY:
        skip_or_fail("proxy-test chat", "missing GLART_API_KEY", REQUIRE_CHAT)
        return False
    headers = {"Authorization": "Bearer " + API_KEY}
    status, payload, text = request_json("POST", "/v1/chat/completions", chat_payload(), headers=headers, timeout=120)
    ok = status == 200 and isinstance(payload, dict) and bool(payload.get("choices"))
    add_check(f"{MODEL} chat completion", ok, f"HTTP {status}")

    if RUN_STREAM:
        status, _, text = request_json("POST", "/v1/chat/completions", chat_payload(stream=True), headers=headers, timeout=120)
        ok_stream = status == 200 and ("data:" in text or "[DONE]" in text)
        add_check(f"{MODEL} stream chat completion", ok_stream, f"HTTP {status}")

    if RUN_LONG_CONTEXT:
        status, payload, _ = request_json(
            "POST",
            "/v1/chat/completions",
            chat_payload(long_context=True),
            headers=headers,
            timeout=180,
        )
        ok_long = status == 200 and isinstance(payload, dict) and bool(payload.get("choices"))
        add_check(f"{MODEL} long-context chat completion", ok_long, f"HTTP {status}")

    return ok


def event_has_required_fields(event: dict[str, Any]) -> bool:
    required = [
        "profit_cost_status",
        "billable_prompt_tokens",
        "upstream_actual_prompt_tokens",
        "estimated_revenue_usd",
        "gross_margin_usd",
        "compression_saved_tokens",
        "profit_route_mode",
        "profit_route_candidate_count",
        "output_policy_mode",
        "output_policy_completion_tokens",
        "profit_risk_mode",
    ]
    return all(key in event for key in required)


def smoke_profit_logs(chat_ran: bool) -> None:
    if not has_root_auth():
        skip_or_fail("profit events/analytics", "missing GLART_ROOT_USER_ID plus root token or cookie", REQUIRE_ROOT)
        return
    if not chat_ran and REQUIRE_CHAT:
        add_check("profit events after chat", False, "chat did not run")
        return

    deadline = time.time() + max(0, PROFIT_EVENT_WAIT_SECONDS)
    events: list[dict[str, Any]] = []
    while True:
        query = urlencode({"group": "proxy-test", "p": "1", "page_size": "20"})
        status, response, _ = request_json("GET", f"/api/profit/events?{query}", headers=root_headers(), timeout=30)
        data = api_data(response)
        if status == 200 and api_success(response) and isinstance(data, dict):
            items = data.get("items")
            if isinstance(items, list):
                events = [item for item in items if isinstance(item, dict)]
                if events and any(event_has_required_fields(event) for event in events):
                    break
        if time.time() >= deadline:
            break
        time.sleep(2)

    matching = [event for event in events if event.get("model_name") == MODEL]
    candidate = matching[0] if matching else (events[0] if events else {})
    ok = bool(candidate) and event_has_required_fields(candidate)
    add_check(
        "profit event diagnostic fields",
        ok,
        f"events={len(events)}; model={candidate.get('model_name', '') if candidate else ''}",
    )

    query = urlencode({"group": "proxy-test"})
    status, response, _ = request_json("GET", f"/api/profit/analytics?{query}", headers=root_headers(), timeout=30)
    data = api_data(response)
    required = [
        "request_count",
        "compression_saved_tokens",
        "output_policy_observed_count",
        "profit_guardrail_action",
        "profit_risk_observed_count",
        "profit_retry_attempt_count",
    ]
    ok = status == 200 and api_success(response) and isinstance(data, dict) and all(key in data for key in required)
    add_check("profit analytics aggregation fields", ok, f"HTTP {status}")


def main() -> int:
    print(f"Glart V2 smoke target: {BASE_URL}")
    print(f"Smoke model: {MODEL}")
    smoke_status()
    smoke_system_update()
    smoke_compression()
    chat_ran = smoke_chat()
    smoke_profit_logs(chat_ran)

    failed = [check for check in checks if not check.ok and not check.skipped]
    print("")
    print(f"Checks: {len(checks)} total, {len(failed)} failed, {sum(1 for item in checks if item.skipped)} skipped")
    if failed:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
