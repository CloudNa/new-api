from __future__ import annotations

import json
import os
import subprocess
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.request import Request, urlopen


HOST = os.environ.get("GLART_STACK_UPDATER_HOST", "0.0.0.0")
PORT = int(os.environ.get("GLART_STACK_UPDATER_PORT", "8787"))
TOKEN = os.environ.get("GLART_STACK_UPDATER_TOKEN", "")
ROOT = Path(os.environ.get("GLART_STACK_ROOT", "/workspace"))
COMPOSE_DIR = Path(os.environ.get("GLART_STACK_COMPOSE_DIR", str(ROOT / "deploy/glart-stack")))
COMPOSE_FILE = os.environ.get("GLART_STACK_COMPOSE_FILE", "compose.yml")
SCRIPT = ROOT / "deploy/glart-stack/scripts/update-glart-stack.sh"
LOG_LIMIT = 200

state_lock = threading.Lock()
state = {
    "running": False,
    "last_exit": None,
    "started_at": "",
    "finished_at": "",
    "message": "idle",
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


def precheck() -> dict:
    checks = []

    def add(name: str, ok: bool, message: str = "") -> None:
        checks.append({"name": name, "ok": ok, "message": message})

    add("updater token", bool(TOKEN), "configured" if TOKEN else "GLART_STACK_UPDATER_TOKEN is empty")
    add("stack root", ROOT.exists(), str(ROOT))
    add("compose dir", COMPOSE_DIR.exists(), str(COMPOSE_DIR))
    add("compose file", (COMPOSE_DIR / COMPOSE_FILE).exists(), str(COMPOSE_DIR / COMPOSE_FILE))
    add("update script", SCRIPT.exists(), str(SCRIPT))

    ok, out = run_command(["git", "status", "--short", "--branch"], ROOT)
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
    }
    try:
        result["new_api_healthy"] = http_ok("http://new-api:3000/api/status")
        result["gpt_load_healthy"] = http_ok("http://gpt-load:3001/health")
        result["cliproxyapi_ready"] = http_ok("http://cliproxyapi:8317/management.html")
        result["ok"] = all(
            [
                result["new_api_healthy"],
                result["gpt_load_healthy"],
                result["cliproxyapi_ready"],
            ]
        )
        result["message"] = "smoke passed" if result["ok"] else "smoke failed"
    except Exception as exc:
        result["error"] = str(exc)
        result["message"] = "smoke failed"
    return result


def http_ok(url: str) -> bool:
    req = Request(url, headers={"User-Agent": "glart-stack-updater"})
    with urlopen(req, timeout=10) as resp:
        return 200 <= resp.status < 400


def run_update() -> None:
    set_state(
        running=True,
        started_at=now(),
        finished_at="",
        last_exit=None,
        message="update running",
        log_tail=[],
    )
    append_log(f"[{now()}] starting glart stack update")
    env = os.environ.copy()
    process = subprocess.Popen(
        ["sh", str(SCRIPT)],
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
    message = "update completed" if exit_code == 0 else f"update failed with exit code {exit_code}"
    append_log(f"[{now()}] {message}")
    set_state(running=False, last_exit=exit_code, finished_at=now(), message=message)


class Handler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        if not self.ensure_auth():
            return
        if self.path == "/status":
            self.write_json(snapshot())
            return
        if self.path == "/precheck":
            self.write_json(precheck())
            return
        self.write_json({"message": "not found"}, 404)

    def do_POST(self) -> None:
        if not self.ensure_auth():
            return
        if self.path == "/smoke":
            self.write_json(smoke())
            return
        if self.path == "/update":
            with state_lock:
                if state["running"]:
                    self.write_json(snapshot(), 409)
                    return
                state["running"] = True
            threading.Thread(target=run_update, daemon=True).start()
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
