from __future__ import annotations

import json
import os
import subprocess
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.error import URLError
from urllib.parse import parse_qs, urlparse
from urllib.request import Request, urlopen


HOST = os.environ.get("GLART_STACK_UPDATER_HOST", "0.0.0.0")
PORT = int(os.environ.get("GLART_STACK_UPDATER_PORT", "8787"))
TOKEN = os.environ.get("GLART_STACK_UPDATER_TOKEN", "")
ROOT = Path(os.environ.get("GLART_STACK_ROOT", "/workspace"))
COMPOSE_DIR = Path(os.environ.get("GLART_STACK_COMPOSE_DIR", str(ROOT / "deploy/glart-stack")))
COMPOSE_FILE = os.environ.get("GLART_STACK_COMPOSE_FILE", "compose.yml")
SCRIPT = ROOT / "deploy/glart-stack/scripts/update-glart-stack.sh"
LOG_LIMIT = 200
BACKUP_DIR = Path(os.environ.get("GLART_STACK_BACKUP_DIR", str(COMPOSE_DIR / "runtime/backups")))
VALID_UPDATE_COMPONENTS = {"all", "new-api", "gpt-load", "cliproxyapi"}
VALID_ROLLBACK_COMPONENTS = {"new-api", "gpt-load", "cliproxyapi"}

state_lock = threading.Lock()
state = {
    "running": False,
    "last_exit": None,
    "started_at": "",
    "finished_at": "",
    "message": "idle",
    "current_action": "",
    "current_component": "",
    "current_backup_id": "",
    "log_tail": [],
}


def now() -> str:
    return time.strftime("%Y-%m-%d %H:%M:%S %z")


def append_log(line: str) -> None:
    with state_lock:
        state["log_tail"].append(line.rstrip())
        state["log_tail"] = state["log_tail"][-LOG_LIMIT:]


def snapshot() -> dict:
    with state_lock:
        data = dict(state)
        data["log_tail"] = list(state["log_tail"])
        data["enabled"] = bool(TOKEN)
        return data


def set_state(**kwargs) -> None:
    with state_lock:
        state.update(kwargs)


def authorized(handler: BaseHTTPRequestHandler) -> bool:
    if not TOKEN:
        return False
    expected = f"Bearer {TOKEN}"
    return handler.headers.get("Authorization", "") == expected


def run_command(cmd: list[str], cwd: Path, timeout: int = 30) -> tuple[bool, str]:
    try:
        completed = subprocess.run(
            cmd,
            cwd=str(cwd),
            text=True,
            capture_output=True,
            timeout=timeout,
            check=False,
        )
        output = (completed.stdout + completed.stderr).strip()
        return completed.returncode == 0, output
    except Exception as exc:
        return False, str(exc)


def parse_json_body(handler: BaseHTTPRequestHandler) -> dict:
    length = int(handler.headers.get("Content-Length", "0") or "0")
    if length <= 0:
        return {}
    body = handler.rfile.read(length)
    if not body.strip():
        return {}
    try:
        parsed = json.loads(body.decode("utf-8"))
    except json.JSONDecodeError as exc:
        raise ValueError(f"invalid JSON body: {exc}") from exc
    if not isinstance(parsed, dict):
        raise ValueError("JSON body must be an object")
    return parsed


def normalize_component(value: object, valid_components: set[str], default: str) -> str:
    component = str(value or default).strip().lower()
    if component not in valid_components:
        allowed = ", ".join(sorted(valid_components))
        raise ValueError(f"invalid component '{component}', allowed: {allowed}")
    return component


def list_backups(component: str) -> dict:
    component = normalize_component(component, VALID_ROLLBACK_COMPONENTS, "new-api")
    component_dir = BACKUP_DIR / component
    backups = []
    if component_dir.exists():
        for path in sorted(component_dir.iterdir(), key=lambda item: item.name, reverse=True):
            if not path.is_dir():
                continue
            manifest_path = path / "manifest.env"
            manifest = read_manifest(manifest_path)
            backups.append(
                {
                    "id": path.name,
                    "component": manifest.get("COMPONENT", component),
                    "created_at": manifest.get("CREATED_AT", ""),
                    "before_commit": manifest.get("BEFORE_COMMIT", ""),
                    "service_image": manifest.get("SERVICE_IMAGE", ""),
                    "rollback_tag": manifest.get("ROLLBACK_TAG", ""),
                    "runtime_path": manifest.get("RUNTIME_PATH", ""),
                }
            )
    return {
        "enabled": bool(TOKEN),
        "component": component,
        "backups": backups,
    }


def read_manifest(path: Path) -> dict[str, str]:
    manifest = {}
    if not path.exists():
        return manifest
    for raw_line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        manifest[key.strip()] = value.strip()
    return manifest


def precheck() -> dict:
    checks = []

    def add(name: str, ok: bool, message: str = "") -> None:
        checks.append({"name": name, "ok": ok, "message": message})

    add("updater token", bool(TOKEN), "configured" if TOKEN else "GLART_STACK_UPDATER_TOKEN is empty")
    add("stack root", ROOT.exists(), str(ROOT))
    add("compose dir", COMPOSE_DIR.exists(), str(COMPOSE_DIR))
    add("compose file", (COMPOSE_DIR / COMPOSE_FILE).exists(), str(COMPOSE_DIR / COMPOSE_FILE))
    add("update script", SCRIPT.exists(), str(SCRIPT))
    add("env file", (COMPOSE_DIR / ".env").exists(), str(COMPOSE_DIR / ".env"))
    add("docker socket", Path("/var/run/docker.sock").exists(), "/var/run/docker.sock")

    try:
        BACKUP_DIR.mkdir(parents=True, exist_ok=True)
        probe = BACKUP_DIR / ".write-test"
        probe.write_text(now(), encoding="utf-8")
        probe.unlink(missing_ok=True)
        add("backup dir writable", True, str(BACKUP_DIR))
    except Exception as exc:
        add("backup dir writable", False, f"{BACKUP_DIR}: {exc}")

    custom_files = [
        ROOT / "controller/sidecar_proxy.go",
        ROOT / "router/web-router.go",
        ROOT / "web/default/src/hooks/use-top-nav-links.ts",
        ROOT / "controller/system_update.go",
    ]
    missing_custom = [str(path) for path in custom_files if not path.exists()]
    add("custom bridge sources", not missing_custom, ", ".join(missing_custom) if missing_custom else "present")
    add("response cache sources", response_cache_sources_ok(), "present" if response_cache_sources_ok() else "missing response cache hooks")
    add("output policy sources", output_policy_sources_ok(), "present" if output_policy_sources_ok() else "missing output policy hooks")
    add("profit risk sources", profit_risk_sources_ok(), "present" if profit_risk_sources_ok() else "missing profit risk hooks")
    add("OmniRoute parity sources", omniroute_parity_sources_ok(), "present" if omniroute_parity_sources_ok() else "missing OmniRoute parity guards")

    if SCRIPT.exists():
        ok, out = run_command(["sh", "-n", str(SCRIPT)], ROOT)
        add("update script syntax", ok, out)

    ok, out = run_command(["git", "-c", f"safe.directory={ROOT}", "status", "--short", "--branch"], ROOT)
    add("git status", ok, out.splitlines()[0] if out else "")

    ok, out = run_command(["docker", "compose", "-f", COMPOSE_FILE, "config", "--quiet"], COMPOSE_DIR, 60)
    add("docker compose config", ok, out)

    all_ok = all(item["ok"] for item in checks)
    return {
        "enabled": bool(TOKEN),
        "ok": all_ok,
        "message": "precheck passed" if all_ok else "precheck failed",
        "checked_at": now(),
        "checks": checks,
    }


def smoke() -> dict:
    result = {
        "enabled": bool(TOKEN),
        "ok": False,
        "message": "",
        "checked_at": now(),
        "status": None,
        "content_type": "",
        "new_api_healthy": False,
        "gpt_load_healthy": False,
        "cliproxyapi_ready": False,
        "sidecar_bridge_sources_ok": False,
        "response_cache_sources_ok": False,
        "output_policy_sources_ok": False,
        "profit_risk_sources_ok": False,
        "omniroute_parity_sources_ok": False,
        "proxy_test_chat_checked": False,
        "proxy_test_chat_ok": False,
        "proxy_test_chat_skipped": False,
    }
    try:
        result["new_api_healthy"], result["status"], result["content_type"] = http_status_ok("http://new-api:3000/api/status")
        result["gpt_load_healthy"] = http_ok("http://gpt-load:3001/health")
        result["cliproxyapi_ready"] = http_ok("http://cliproxyapi:8317/management.html")
        result["sidecar_bridge_sources_ok"] = custom_bridge_sources_ok()
        result["response_cache_sources_ok"] = response_cache_sources_ok()
        result["output_policy_sources_ok"] = output_policy_sources_ok()
        result["profit_risk_sources_ok"] = profit_risk_sources_ok()
        result["omniroute_parity_sources_ok"] = omniroute_parity_sources_ok()
        result["proxy_test_chat_checked"], result["proxy_test_chat_ok"], result["proxy_test_chat_skipped"] = proxy_test_chat_smoke()
        result["ok"] = all(
            [
                result["new_api_healthy"],
                result["gpt_load_healthy"],
                result["cliproxyapi_ready"],
                result["sidecar_bridge_sources_ok"],
                result["response_cache_sources_ok"],
                result["output_policy_sources_ok"],
                result["profit_risk_sources_ok"],
                result["omniroute_parity_sources_ok"],
                result["proxy_test_chat_ok"] or result["proxy_test_chat_skipped"],
            ]
        )
        result["message"] = "smoke passed" if result["ok"] else "smoke failed"
    except Exception as exc:
        result["error"] = str(exc)
        result["message"] = "smoke failed"
    return result


def http_ok(url: str) -> bool:
    ok, _, _ = http_status_ok(url)
    return ok


def http_status_ok(url: str) -> tuple[bool, int | None, str]:
    req = Request(url, headers={"User-Agent": "glart-stack-updater"})
    with urlopen(req, timeout=10) as resp:
        return 200 <= resp.status < 400, resp.status, resp.headers.get("Content-Type", "")


def custom_bridge_sources_ok() -> bool:
    try:
        web_router = (ROOT / "router/web-router.go").read_text(encoding="utf-8")
        top_nav = (ROOT / "web/default/src/hooks/use-top-nav-links.ts").read_text(encoding="utf-8")
        controller = (ROOT / "controller/sidecar_proxy.go").read_text(encoding="utf-8")
        return all(
            [
                '"/gl"' in web_router,
                '"/cpa"' in web_router,
                "GPT-Load" in top_nav,
                "CLIProxyAPI" in top_nav,
                "GPT_LOAD_INTERNAL_URL" in controller,
                "CLIPROXYAPI_INTERNAL_URL" in controller,
            ]
        )
    except Exception:
        return False


def response_cache_sources_ok() -> bool:
    try:
        profit_settings = (ROOT / "pkg/profit/settings.go").read_text(encoding="utf-8")
        response_cache = (ROOT / "pkg/profit/response_cache.go").read_text(encoding="utf-8")
        response_cache_service = (ROOT / "service/response_cache.go").read_text(encoding="utf-8")
        compatible_handler = (ROOT / "relay/compatible_handler.go").read_text(encoding="utf-8")
        text_quota = (ROOT / "service/text_quota.go").read_text(encoding="utf-8")
        observation = (ROOT / "pkg/profit/observation.go").read_text(encoding="utf-8")
        profit_center = (ROOT / "web/default/src/features/system-settings/maintenance/profit-center-section.tsx").read_text(
            encoding="utf-8"
        )
        return all(
            [
                "ResponseCacheRule" in profit_settings,
                "BuildResponseCacheDecision" in response_cache,
                "PutResponseCache" in response_cache,
                "PrepareResponseCacheForRelay" in response_cache_service,
                "ServeResponseCacheHit" in response_cache_service,
                "StoreResponseCacheForRelay" in compatible_handler,
                "ProfitResponseCacheObservationFromContext" in text_quota,
                "response_cache_saved_usd" in observation,
                "ResponseCacheModeSelect" in profit_center,
                "profit-response-cache-rules-json" in profit_center,
            ]
        )
    except Exception:
        return False


def output_policy_sources_ok() -> bool:
    try:
        output_policy = (ROOT / "pkg/profit/output_policy.go").read_text(encoding="utf-8")
        output_policy_service = (ROOT / "service/output_policy.go").read_text(encoding="utf-8")
        compatible_handler = (ROOT / "relay/compatible_handler.go").read_text(encoding="utf-8")
        text_quota = (ROOT / "service/text_quota.go").read_text(encoding="utf-8")
        observation = (ROOT / "pkg/profit/observation.go").read_text(encoding="utf-8")
        api_types = (ROOT / "web/default/src/features/system-settings/api.ts").read_text(encoding="utf-8")
        return all(
            [
                "BuildOutputPolicyRequestDecision" in output_policy,
                "ApplyOutputPolicyForRelay" in output_policy_service,
                "ProfitOutputPolicyDecisionFromContext" in output_policy_service,
                "ApplyOutputPolicyForRelay" in compatible_handler,
                "MergeOutputPolicyCompletion" in text_quota,
                "output_policy_requested_max_tokens" in observation,
                "output_policy_applied_max_tokens" in observation,
                "output_policy_enforced_limit_exceeded" in observation,
                "output_policy_requested_max_tokens" in api_types,
                "output_policy_applied_max_tokens" in api_types,
                "output_policy_enforced_limit_exceeded" in api_types,
            ]
        )
    except Exception:
        return False


def profit_risk_sources_ok() -> bool:
    try:
        settings = (ROOT / "pkg/profit/settings.go").read_text(encoding="utf-8")
        risk = (ROOT / "pkg/profit/risk.go").read_text(encoding="utf-8")
        risk_service = (ROOT / "service/profit_risk_enforcement.go").read_text(encoding="utf-8")
        relay = (ROOT / "controller/relay.go").read_text(encoding="utf-8")
        model_profit = (ROOT / "model/profit.go").read_text(encoding="utf-8")
        api_types = (ROOT / "web/default/src/features/system-settings/api.ts").read_text(encoding="utf-8")
        profit_center = (ROOT / "web/default/src/features/system-settings/maintenance/profit-center-section.tsx").read_text(encoding="utf-8")
        return all(
            [
                "validRiskMode" in settings,
                "RiskEnforcement != ModeEnforce" in risk,
                "EnforceProfitRiskBeforeRelay" in risk_service,
                "EnforceProfitRiskBeforeRelay" in relay,
                "profit_risk_live_enforced_count" in model_profit,
                "profit_risk_live_enforced_count" in api_types,
                "风险真实拦截" in profit_center,
            ]
        )
    except Exception:
        return False


def omniroute_parity_sources_ok() -> bool:
    try:
        attribution = (ROOT / "pkg/promptcompress/attribution.go").read_text(encoding="utf-8")
        embed = (ROOT / "pkg/promptcompress/omniroute_embed.go").read_text(encoding="utf-8")
        manifest = (ROOT / "pkg/promptcompress/omniroute_manifest_test.go").read_text(encoding="utf-8")
        behavior = (ROOT / "pkg/promptcompress/omniroute_behavior_test.go").read_text(encoding="utf-8")
        return all(
            [
                'OmniRouteParityCommit = "630baa6"' in attribution,
                "omniroute/caveman_rules/_schema.json" in embed,
                "TestOmniRouteVendoredRuleBlobParity" in manifest,
                "omniRouteExpectedBlobSHA" in manifest,
                "gitBlobSHA" in manifest,
                "TestOmniRouteOfficialRTKSmartTruncateBehavior" in behavior,
                "TestOmniRouteOfficialCavemanEngineBehavior" in behavior,
                "TestOmniRouteOfficialStackedPipelineBehavior" in behavior,
                "7a3f82d887ff96f9208aa3e17e7e4ac94d9107c4" in manifest,
                "606cc22457d857bac0bc50567dbaca30e30b52c0" in manifest,
            ]
        )
    except Exception:
        return False


def proxy_test_chat_smoke() -> tuple[bool, bool, bool]:
    api_key = os.environ.get("GLART_SMOKE_API_KEY", "").strip()
    model = os.environ.get("GLART_SMOKE_MODEL", "").strip()
    if not api_key or not model:
        return False, False, True
    payload = json.dumps(
        {
            "model": model,
            "messages": [{"role": "user", "content": "Return exactly: ok"}],
            "max_tokens": 8,
        }
    ).encode("utf-8")
    req = Request(
        "http://new-api:3000/v1/chat/completions",
        data=payload,
        headers={
            "User-Agent": "glart-stack-updater",
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        },
        method="POST",
    )
    try:
        with urlopen(req, timeout=60) as resp:
            return True, 200 <= resp.status < 400, False
    except URLError:
        return True, False, False


def run_operation(
    action: str,
    component: str,
    backup_id: str = "latest",
    restore_runtime: bool = False,
) -> None:
    set_state(
        running=True,
        started_at=now(),
        finished_at="",
        last_exit=None,
        message=f"{action} {component} running",
        current_action=action,
        current_component=component,
        current_backup_id=backup_id,
        log_tail=[],
    )
    append_log(f"[{now()}] starting glart stack {action} {component}")
    env = os.environ.copy()
    if restore_runtime:
        env["GLART_STACK_RESTORE_RUNTIME_ON_ROLLBACK"] = "1"
    process = subprocess.Popen(
        ["sh", str(SCRIPT), action, component, backup_id],
        cwd=str(ROOT),
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        env=env,
    )
    assert process.stdout is not None
    for line in process.stdout:
        append_log(line)
    exit_code = process.wait()
    message = (
        f"{action} {component} completed"
        if exit_code == 0
        else f"{action} {component} failed with exit code {exit_code}"
    )
    append_log(f"[{now()}] {message}")
    set_state(running=False, last_exit=exit_code, finished_at=now(), message=message)


class Handler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        if not self.ensure_auth():
            return
        parsed = urlparse(self.path)
        if parsed.path == "/status":
            self.write_json(snapshot())
            return
        if parsed.path == "/precheck":
            self.write_json(precheck())
            return
        if parsed.path == "/backups":
            params = parse_qs(parsed.query)
            try:
                component = normalize_component(
                    params.get("component", ["new-api"])[0],
                    VALID_ROLLBACK_COMPONENTS,
                    "new-api",
                )
                self.write_json(list_backups(component))
            except ValueError as exc:
                self.write_json({"message": str(exc)}, 400)
            return
        self.write_json({"message": "not found"}, 404)

    def do_POST(self) -> None:
        if not self.ensure_auth():
            return
        parsed = urlparse(self.path)
        if parsed.path == "/smoke":
            self.write_json(smoke())
            return
        if parsed.path == "/update":
            try:
                body = parse_json_body(self)
                component = normalize_component(
                    body.get("component"),
                    VALID_UPDATE_COMPONENTS,
                    "all",
                )
            except ValueError as exc:
                self.write_json({"message": str(exc)}, 400)
                return
            with state_lock:
                if state["running"]:
                    self.write_json(snapshot(), 409)
                    return
                state["running"] = True
            threading.Thread(
                target=run_operation,
                args=("update", component, "latest", False),
                daemon=True,
            ).start()
            time.sleep(0.2)
            self.write_json(snapshot())
            return
        if parsed.path == "/rollback":
            try:
                body = parse_json_body(self)
                component = normalize_component(
                    body.get("component"),
                    VALID_ROLLBACK_COMPONENTS,
                    "new-api",
                )
                backup_id = str(body.get("backup_id") or "latest").strip() or "latest"
                restore_runtime = bool(body.get("restore_runtime", False))
            except ValueError as exc:
                self.write_json({"message": str(exc)}, 400)
                return
            with state_lock:
                if state["running"]:
                    self.write_json(snapshot(), 409)
                    return
                state["running"] = True
            threading.Thread(
                target=run_operation,
                args=("rollback", component, backup_id, restore_runtime),
                daemon=True,
            ).start()
            time.sleep(0.2)
            self.write_json(snapshot())
            return
        self.write_json({"message": "not found"}, 404)

    def ensure_auth(self) -> bool:
        if authorized(self):
            return True
        self.write_json({"message": "unauthorized"}, 401)
        return False

    def write_json(self, payload: dict, status: int = 200) -> None:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format: str, *args) -> None:
        return


if __name__ == "__main__":
    server = ThreadingHTTPServer((HOST, PORT), Handler)
    print(f"glart stack updater listening on {HOST}:{PORT}")
    server.serve_forever()
