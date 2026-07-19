from __future__ import annotations

import json
import os
import re
import shlex
import shutil
import subprocess
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.error import URLError
from urllib.parse import parse_qs, urlparse
from urllib.request import Request, urlopen


def env_flag(name: str, default: bool) -> bool:
    value = os.environ.get(name)
    if value is None or value.strip() == "":
        return default
    return value.strip().lower() in {"1", "true", "yes", "on"}


HOST = os.environ.get("GLART_STACK_UPDATER_HOST", "0.0.0.0")
PORT = int(os.environ.get("GLART_STACK_UPDATER_PORT", "8787"))
TOKEN = os.environ.get("GLART_STACK_UPDATER_TOKEN", "")
ROOT = Path(os.environ.get("GLART_STACK_ROOT", "/workspace"))
COMPOSE_DIR = Path(os.environ.get("GLART_STACK_COMPOSE_DIR", str(ROOT / "deploy/glart-stack")))
COMPOSE_FILE = os.environ.get("GLART_STACK_COMPOSE_FILE", "compose.yml")
SCRIPT = ROOT / "deploy/glart-stack/scripts/update-glart-stack.sh"
LOG_LIMIT = 200
BACKUP_DIR = Path(os.environ.get("GLART_STACK_BACKUP_DIR", str(COMPOSE_DIR / "runtime/backups")))
UPSTREAM_URL = os.environ.get("GLART_STACK_UPSTREAM_URL", "https://github.com/QuantumNous/new-api.git")
UPSTREAM_BRANCH = os.environ.get("GLART_STACK_UPSTREAM_BRANCH", "main")
UPSTREAM_REF = os.environ.get("GLART_STACK_UPSTREAM_REF", f"refs/heads/{UPSTREAM_BRANCH}")
UPSTREAM_TRACKING_REF = os.environ.get(
    "GLART_STACK_UPSTREAM_TRACKING_REF",
    f"refs/remotes/glart-upstream/{UPSTREAM_BRANCH}",
)
UPSTREAM_CHECK_TTL_SECONDS = int(os.environ.get("GLART_STACK_UPSTREAM_CHECK_TTL_SECONDS", "300"))
UPSTREAM_CHECK_TIMEOUT_SECONDS = int(os.environ.get("GLART_STACK_UPSTREAM_CHECK_TIMEOUT_SECONDS", "12"))
CUSTOM_REMOTE = os.environ.get("GLART_STACK_CUSTOM_REMOTE", "origin").strip() or "origin"
CUSTOM_BRANCH = os.environ.get("GLART_STACK_CUSTOM_BRANCH", "").strip()
STAGING_ROOT = Path(os.environ.get("GLART_STACK_STAGING_ROOT", str(COMPOSE_DIR / "runtime/staging")))
STAGING_WORKTREE = Path(
    os.environ.get(
        "GLART_STACK_UPSTREAM_MERGE_DIR",
        str(STAGING_ROOT / "new-api-upstream-merge"),
    )
)
UPSTREAM_MERGE_RUN_TESTS = env_flag("GLART_STACK_UPSTREAM_MERGE_RUN_TESTS", True)
UPSTREAM_MERGE_BUILD_IMAGE = env_flag("GLART_STACK_UPSTREAM_MERGE_BUILD_IMAGE", True)
UPSTREAM_MERGE_PUSH = env_flag("GLART_STACK_UPSTREAM_MERGE_PUSH", True)
GO_TEST_IMAGE = os.environ.get("GLART_STACK_GO_TEST_IMAGE", "golang:1.26.1")
VALID_UPDATE_COMPONENTS = {"all", "new-api", "gpt-load", "cliproxyapi", "cpa-manager-plus"}
VALID_ROLLBACK_COMPONENTS = {"new-api", "gpt-load", "cliproxyapi", "cpa-manager-plus"}
CLEANUP_KEEP_ROLLBACK_IMAGES = int(os.environ.get("GLART_STACK_CLEANUP_KEEP_ROLLBACK_IMAGES", "5"))
CLEANUP_KEEP_BACKUPS = int(os.environ.get("GLART_STACK_CLEANUP_KEEP_BACKUPS", "5"))
CLEANUP_BUILD_CACHE_MAX_AGE_HOURS = int(
    os.environ.get("GLART_STACK_CLEANUP_BUILD_CACHE_MAX_AGE_HOURS", "168")
)
CLEANUP_MIN_KEEP = 1
CLEANUP_MAX_KEEP = 20
CLEANUP_MIN_CACHE_AGE_HOURS = 24
CLEANUP_MAX_CACHE_AGE_HOURS = 24 * 90
ROLLBACK_TAG_PATTERN = re.compile(r"^\d{8}-\d{6}$")
ROLLBACK_IMAGE_REPOSITORIES = {
    f"glart/rollback-{component}": component for component in VALID_ROLLBACK_COMPONENTS
}

state_lock = threading.Lock()
upstream_cache_lock = threading.Lock()
upstream_cache = {
    "expires_at": 0.0,
    "data": None,
}
state = {
    "running": False,
    "last_exit": None,
    "started_at": "",
    "finished_at": "",
    "message": "idle",
    "current_action": "",
    "current_component": "",
    "current_backup_id": "",
    "current_staging_dir": "",
    "last_cleanup": None,
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
    data["upstream"] = upstream_status()
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


def format_cmd(cmd: list[str]) -> str:
    text = shlex.join([str(part) for part in cmd])
    return re.sub(r"(https?://)([^/\s'\"@]+)@", r"\1***@", text)


def run_logged(
    cmd: list[str],
    cwd: Path,
    timeout: int = 30,
    env: dict[str, str] | None = None,
) -> None:
    append_log(f"[{now()}] + cd {cwd} && {format_cmd(cmd)}")
    try:
        completed = subprocess.run(
            cmd,
            cwd=str(cwd),
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            timeout=timeout,
            check=False,
            env=env,
        )
    except subprocess.TimeoutExpired as exc:
        output = exc.stdout or ""
        for line in str(output).splitlines():
            append_log(line)
        raise TimeoutError(f"command timed out after {timeout}s: {format_cmd(cmd)}") from exc
    for line in completed.stdout.splitlines():
        append_log(line)
    if completed.returncode != 0:
        raise RuntimeError(f"command failed with exit code {completed.returncode}: {format_cmd(cmd)}")


def run_git(args: list[str], timeout: int = 30) -> tuple[bool, str]:
    return run_command(["git", "-c", f"safe.directory={ROOT}", *args], ROOT, timeout)


def run_git_in(cwd: Path, args: list[str], timeout: int = 30) -> tuple[bool, str]:
    return run_command(["git", "-c", f"safe.directory={cwd}", *args], cwd, timeout)


def git_text_in(cwd: Path, args: list[str], timeout: int = 30) -> str:
    ok, out = run_git_in(cwd, args, timeout)
    if not ok:
        raise RuntimeError(out or f"git {' '.join(args)} failed")
    return out.strip()


def run_git_logged(cwd: Path, args: list[str], timeout: int = 30) -> None:
    run_logged(["git", "-c", f"safe.directory={cwd}", *args], cwd, timeout)


def git_text(args: list[str], timeout: int = 30) -> str:
    ok, out = run_git(args, timeout)
    if not ok:
        raise RuntimeError(out or f"git {' '.join(args)} failed")
    return out.strip()


def git_count(args: list[str], timeout: int = 30) -> int:
    value = git_text(args, timeout)
    return int(value or "0")


def commit_info(ref: str) -> dict:
    commit = git_text(["rev-parse", ref])
    short_commit = git_text(["rev-parse", "--short=8", ref])
    version = git_text(["describe", "--tags", "--always", "--abbrev=8", ref])
    tag_ok, tag_out = run_git(["describe", "--tags", "--abbrev=0", ref])
    meta = git_text(["log", "-1", "--format=%ci%x00%s", ref])
    date, _, subject = meta.partition("\x00")
    return {
        "ref": ref,
        "commit": commit,
        "short_commit": short_commit,
        "version": version,
        "tag": tag_out.strip() if tag_ok else "",
        "date": date.strip(),
        "subject": subject.strip(),
    }


def upstream_status(force: bool = False) -> dict:
    if not UPSTREAM_URL:
        return {
            "enabled": False,
            "source_url": "",
            "branch": UPSTREAM_BRANCH,
            "tracking_ref": UPSTREAM_TRACKING_REF,
            "checked_at": now(),
            "needs_update": False,
            "error": "GLART_STACK_UPSTREAM_URL is empty",
        }

    now_ts = time.time()
    with upstream_cache_lock:
        cached = upstream_cache.get("data")
        if not force and cached and float(upstream_cache.get("expires_at", 0)) > now_ts:
            data = dict(cached)
            data["cached"] = True
            return data

    data = {
        "enabled": True,
        "source_url": UPSTREAM_URL,
        "branch": UPSTREAM_BRANCH,
        "tracking_ref": UPSTREAM_TRACKING_REF,
        "checked_at": now(),
        "needs_update": False,
        "upstream_commits_since_baseline": 0,
        "custom_commits_since_baseline": 0,
        "current": None,
        "baseline": None,
        "latest": None,
        "error": "",
        "cached": False,
    }
    try:
        data["current"] = commit_info("HEAD")
        fetch_refspec = f"+{UPSTREAM_REF}:{UPSTREAM_TRACKING_REF}"
        ok, out = run_git(
            ["fetch", "--quiet", "--no-tags", UPSTREAM_URL, fetch_refspec],
            UPSTREAM_CHECK_TIMEOUT_SECONDS,
        )
        if not ok:
            raise RuntimeError(out or "failed to fetch official upstream")
        data["latest"] = commit_info(UPSTREAM_TRACKING_REF)
        baseline = git_text(["merge-base", "HEAD", UPSTREAM_TRACKING_REF])
        data["baseline"] = commit_info(baseline)
        upstream_count = git_count(["rev-list", "--count", f"{baseline}..{UPSTREAM_TRACKING_REF}"])
        custom_count = git_count(["rev-list", "--count", f"{baseline}..HEAD"])
        data["upstream_commits_since_baseline"] = upstream_count
        data["custom_commits_since_baseline"] = custom_count
        data["needs_update"] = upstream_count > 0
    except Exception as exc:
        data["error"] = str(exc)

    with upstream_cache_lock:
        upstream_cache["data"] = dict(data)
        upstream_cache["expires_at"] = now_ts + max(UPSTREAM_CHECK_TTL_SECONDS, 30)
    return data


def clear_upstream_cache() -> None:
    with upstream_cache_lock:
        upstream_cache["data"] = None
        upstream_cache["expires_at"] = 0.0


def resolved(path: Path) -> Path:
    return path.expanduser().resolve()


def is_relative_to(path: Path, parent: Path) -> bool:
    try:
        path.relative_to(parent)
        return True
    except ValueError:
        return False


def ensure_staging_path(path: Path) -> Path:
    staging_root = resolved(STAGING_ROOT)
    target = resolved(path)
    if target == resolved(ROOT):
        raise RuntimeError("refusing to use production root as staging directory")
    if not is_relative_to(target, staging_root):
        raise RuntimeError(f"staging directory must stay under {staging_root}: {target}")
    staging_root.mkdir(parents=True, exist_ok=True)
    return target


def safe_branch_name(branch: str) -> str:
    cleaned = re.sub(r"[^A-Za-z0-9._-]+", "-", branch.strip())
    cleaned = cleaned.strip(".-")
    return (cleaned or "current")[:80]


def current_branch_name() -> str:
    if CUSTOM_BRANCH:
        return CUSTOM_BRANCH
    branch = git_text(["rev-parse", "--abbrev-ref", "HEAD"])
    if branch == "HEAD":
        raise RuntimeError("production repository is detached; set GLART_STACK_CUSTOM_BRANCH")
    return branch


def ensure_production_clean_for_staging() -> None:
    status = git_text(["status", "--porcelain"], 30)
    if status.strip():
        append_log(status)
        raise RuntimeError("production repository has uncommitted changes; refusing staging merge")


def copy_private_env_to_staging(staging_dir: Path) -> None:
    source = COMPOSE_DIR / ".env"
    target_dir = staging_dir / "deploy/glart-stack"
    target = target_dir / ".env"
    if not source.exists():
        raise RuntimeError(f"missing private env file: {source}")
    target_dir.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, target)
    target.chmod(0o600)


def restore_staging_owner(staging_dir: Path) -> None:
    if os.name == "nt":
        return
    try:
        owner = ROOT.stat()
        uid = owner.st_uid
        gid = owner.st_gid
        paths = [staging_dir]
        ok, git_dir = run_git_in(staging_dir, ["rev-parse", "--git-dir"], 30)
        if ok and git_dir.strip():
            paths.append((staging_dir / git_dir.strip()).resolve())
        ok, common_dir = run_git_in(staging_dir, ["rev-parse", "--git-common-dir"], 30)
        if ok and common_dir.strip():
            paths.append((staging_dir / common_dir.strip()).resolve())
        for target in paths:
            if target.exists():
                shutil.chown(target, user=uid, group=gid)
                if target.is_dir():
                    for child in target.rglob("*"):
                        shutil.chown(child, user=uid, group=gid)
    except Exception as exc:
        append_log(f"[{now()}] unable to restore staging ownership: {exc}")


def remove_existing_staging_worktree(staging_dir: Path) -> None:
    if not staging_dir.exists():
        return
    ok, out = run_git(["worktree", "remove", "--force", str(staging_dir)], 120)
    if ok:
        return
    append_log(out)
    target = ensure_staging_path(staging_dir)
    if target.exists():
        shutil.rmtree(target)
    run_git_logged(ROOT, ["worktree", "prune"], 120)


def run_staging_go_tests(staging_dir: Path) -> None:
    cache_dir = STAGING_ROOT / "cache"
    go_build_cache = cache_dir / "go-build"
    go_mod_cache = cache_dir / "gomod"
    go_build_cache.mkdir(parents=True, exist_ok=True)
    go_mod_cache.mkdir(parents=True, exist_ok=True)
    run_logged(
        [
            "docker",
            "run",
            "--rm",
            "-v",
            f"{staging_dir}:/workspace",
            "-v",
            f"{go_build_cache}:/tmp/go-cache",
            "-v",
            f"{go_mod_cache}:/tmp/gomodcache",
            "-w",
            "/workspace",
            "-e",
            "GOCACHE=/tmp/go-cache",
            "-e",
            "GOMODCACHE=/tmp/gomodcache",
            GO_TEST_IMAGE,
            "sh",
            "-c",
            "git config --global --add safe.directory /workspace && go test ./service ./controller ./model ./router ./relay ./pkg/billingexpr ./setting/billing_setting ./pkg/profit ./pkg/promptcompress -count=1",
        ],
        staging_dir,
        1800,
    )


def run_staging_checks(staging_dir: Path, branch: str) -> None:
    compose_dir = staging_dir / "deploy/glart-stack"
    run_logged(["python", "-m", "py_compile", "deploy/glart-stack/updater/server.py"], staging_dir, 60)
    run_logged(["sh", "-n", "deploy/glart-stack/scripts/update-glart-stack.sh"], staging_dir, 60)
    run_logged(["sh", "-n", "deploy/glart-stack/scripts/v2-smoke.sh"], staging_dir, 60)
    run_logged(["docker", "compose", "-f", COMPOSE_FILE, "config", "--quiet"], compose_dir, 120)
    if UPSTREAM_MERGE_RUN_TESTS:
        run_staging_go_tests(staging_dir)
    else:
        append_log(f"[{now()}] staging go test skipped by GLART_STACK_UPSTREAM_MERGE_RUN_TESTS=0")
    if UPSTREAM_MERGE_BUILD_IMAGE:
        image_tag = f"glart/new-api:upstream-merge-{safe_branch_name(branch)}"
        run_logged(["docker", "build", "-t", image_tag, "-f", "Dockerfile", "."], staging_dir, 2400)
        append_log(f"[{now()}] staging image built: {image_tag}")
    else:
        append_log(f"[{now()}] staging image build skipped by GLART_STACK_UPSTREAM_MERGE_BUILD_IMAGE=0")


def prepare_upstream_merge_operation() -> None:
    branch = ""
    staging_dir = STAGING_WORKTREE
    set_state(
        running=True,
        started_at=now(),
        finished_at="",
        last_exit=None,
        message="prepare upstream merge running",
        current_action="prepare-upstream-merge",
        current_component="new-api",
        current_backup_id="",
        current_staging_dir=str(STAGING_WORKTREE),
        log_tail=[],
    )
    try:
        staging_dir = ensure_staging_path(STAGING_WORKTREE)
        set_state(current_staging_dir=str(staging_dir))
        append_log(f"[{now()}] preparing official upstream merge in staging")
        append_log(f"[{now()}] production root: {ROOT}")
        append_log(f"[{now()}] staging worktree: {staging_dir}")
        branch = current_branch_name()
        staging_branch = f"glart-upstream-merge-{safe_branch_name(branch)}"
        append_log(f"[{now()}] custom branch: {branch}")
        ensure_production_clean_for_staging()
        run_git_logged(
            ROOT,
            [
                "fetch",
                "--prune",
                CUSTOM_REMOTE,
                f"+refs/heads/{branch}:refs/remotes/{CUSTOM_REMOTE}/{branch}",
            ],
            300,
        )
        run_git_logged(ROOT, ["fetch", "--tags", UPSTREAM_URL, f"+{UPSTREAM_REF}:{UPSTREAM_TRACKING_REF}"], 600)
        remove_existing_staging_worktree(staging_dir)
        run_git_logged(
            ROOT,
            [
                "worktree",
                "add",
                "-B",
                staging_branch,
                str(staging_dir),
                f"{CUSTOM_REMOTE}/{branch}",
            ],
            300,
        )
        restore_staging_owner(staging_dir)
        run_git_logged(staging_dir, ["config", "user.name", "Glart Stack Updater"], 30)
        run_git_logged(staging_dir, ["config", "user.email", "stack-updater@glart.local"], 30)
        run_git_logged(staging_dir, ["merge", "--no-edit", UPSTREAM_TRACKING_REF], 900)
        copy_private_env_to_staging(staging_dir)
        run_staging_checks(staging_dir, branch)
        merged_commit = git_text_in(staging_dir, ["rev-parse", "--short=12", "HEAD"])
        if UPSTREAM_MERGE_PUSH:
            run_git_logged(staging_dir, ["push", CUSTOM_REMOTE, f"HEAD:{branch}"], 600)
            append_log(f"[{now()}] pushed merged commit {merged_commit} to {CUSTOM_REMOTE}/{branch}")
        else:
            append_log(f"[{now()}] merged commit {merged_commit} is ready in staging; push skipped")
        restore_staging_owner(staging_dir)
        clear_upstream_cache()
        message = f"prepare upstream merge completed: {merged_commit}"
        append_log(f"[{now()}] {message}")
        set_state(running=False, last_exit=0, finished_at=now(), message=message)
    except Exception as exc:
        restore_staging_owner(staging_dir)
        append_log(f"[{now()}] prepare upstream merge failed: {exc}")
        if staging_dir.exists():
            ok, out = run_git_in(staging_dir, ["status", "--short"], 30)
            if ok and out.strip():
                append_log("[staging git status]")
                append_log(out)
        set_state(
            running=False,
            last_exit=1,
            finished_at=now(),
            message=f"prepare upstream merge failed: {exc}",
        )


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


def bounded_int(value: object, default: int, minimum: int, maximum: int, name: str) -> int:
    if value is None or value == "":
        value = default
    try:
        parsed = int(value)
    except (TypeError, ValueError) as exc:
        raise ValueError(f"{name} must be an integer") from exc
    if parsed < minimum or parsed > maximum:
        raise ValueError(f"{name} must be between {minimum} and {maximum}")
    return parsed


def cleanup_policy(payload: dict | None = None) -> dict:
    payload = payload or {}

    def boolean(name: str, default: bool) -> bool:
        value = payload.get(name, default)
        if isinstance(value, bool):
            return value
        if isinstance(value, str) and value.strip().lower() in {"true", "false"}:
            return value.strip().lower() == "true"
        raise ValueError(f"{name} must be a boolean")

    return {
        "keep_rollback_images": bounded_int(
            payload.get("keep_rollback_images"),
            CLEANUP_KEEP_ROLLBACK_IMAGES,
            CLEANUP_MIN_KEEP,
            CLEANUP_MAX_KEEP,
            "keep_rollback_images",
        ),
        "keep_backups": bounded_int(
            payload.get("keep_backups"),
            CLEANUP_KEEP_BACKUPS,
            CLEANUP_MIN_KEEP,
            CLEANUP_MAX_KEEP,
            "keep_backups",
        ),
        "build_cache_max_age_hours": bounded_int(
            payload.get("build_cache_max_age_hours"),
            CLEANUP_BUILD_CACHE_MAX_AGE_HOURS,
            CLEANUP_MIN_CACHE_AGE_HOURS,
            CLEANUP_MAX_CACHE_AGE_HOURS,
            "build_cache_max_age_hours",
        ),
        "prune_dangling_images": boolean("prune_dangling_images", True),
        "prune_build_cache": boolean("prune_build_cache", True),
    }


def directory_size(path: Path) -> int:
    total = 0
    if not path.exists():
        return total
    for child in path.rglob("*"):
        try:
            if child.is_file():
                total += child.stat().st_size
        except OSError:
            continue
    return total


def docker_json_lines(args: list[str]) -> list[dict]:
    ok, output = run_command(["docker", *args], ROOT, 60)
    if not ok:
        append_log(f"[{now()}] docker query failed: {output}")
        return []
    rows = []
    for line in output.splitlines():
        try:
            value = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(value, dict):
            rows.append(value)
    return rows


def docker_system_df() -> dict:
    rows = docker_json_lines(["system", "df", "--format", "{{json .}}"])
    result = {}
    for row in rows:
        row_type = str(row.get("Type") or "").lower().replace(" ", "_")
        if not row_type:
            continue
        result[row_type] = {
            "total": row.get("TotalCount", "0"),
            "active": row.get("Active", "0"),
            "size": row.get("Size", "0B"),
            "reclaimable": row.get("Reclaimable", "0B"),
        }
    return result


def rollback_image_candidates(keep: int) -> list[dict]:
    rows = docker_json_lines(["image", "ls", "--format", "{{json .}}"])
    grouped: dict[str, list[dict]] = {component: [] for component in VALID_ROLLBACK_COMPONENTS}
    for row in rows:
        repository = str(row.get("Repository") or "")
        tag = str(row.get("Tag") or "")
        component = ROLLBACK_IMAGE_REPOSITORIES.get(repository)
        if not component or not ROLLBACK_TAG_PATTERN.fullmatch(tag):
            continue
        grouped[component].append(
            {
                "component": component,
                "repository": repository,
                "tag": tag,
                "reference": f"{repository}:{tag}",
                "created_at": str(row.get("CreatedAt") or ""),
            }
        )

    candidates = []
    for images in grouped.values():
        images.sort(key=lambda item: (item["tag"], item["created_at"]), reverse=True)
        candidates.extend(images[keep:])
    return candidates


def backup_candidates(keep: int) -> list[dict]:
    candidates = []
    for component in sorted(VALID_ROLLBACK_COMPONENTS):
        component_dir = BACKUP_DIR / component
        if not component_dir.exists():
            continue
        entries = [
            path
            for path in component_dir.iterdir()
            if path.is_dir()
            and not path.is_symlink()
            and ROLLBACK_TAG_PATTERN.fullmatch(path.name)
        ]
        entries.sort(key=lambda item: item.name, reverse=True)
        for path in entries[keep:]:
            candidates.append(
                {
                    "component": component,
                    "id": path.name,
                    "path": path,
                    "bytes": directory_size(path),
                }
            )
    return candidates


def disk_usage() -> dict:
    usage = shutil.disk_usage(ROOT)
    used = usage.total - usage.free
    return {
        "total_bytes": usage.total,
        "used_bytes": used,
        "free_bytes": usage.free,
        "used_percent": round((used / usage.total) * 100, 2) if usage.total else 0,
    }


def cleanup_plan(policy: dict) -> dict:
    rollback = rollback_image_candidates(policy["keep_rollback_images"])
    backups = backup_candidates(policy["keep_backups"])
    return {
        "rollback_images": rollback,
        "backups": backups,
    }


def cleanup_preview(policy: dict | None = None) -> dict:
    policy = policy or cleanup_policy()
    plan = cleanup_plan(policy)
    rollback_by_component = {
        component: sum(1 for item in plan["rollback_images"] if item["component"] == component)
        for component in VALID_ROLLBACK_COMPONENTS
    }
    backup_by_component = {
        component: sum(1 for item in plan["backups"] if item["component"] == component)
        for component in VALID_ROLLBACK_COMPONENTS
    }
    backup_bytes = sum(item["bytes"] for item in plan["backups"])
    return {
        "enabled": bool(TOKEN),
        "checked_at": now(),
        "policy": policy,
        "disk": disk_usage(),
        "docker": docker_system_df(),
        "rollback_images": {
            "candidate_count": len(plan["rollback_images"]),
            "by_component": rollback_by_component,
        },
        "backups": {
            "candidate_count": len(plan["backups"]),
            "estimated_bytes": backup_bytes,
            "by_component": backup_by_component,
        },
        "estimated_backup_reclaimable_bytes": backup_bytes,
    }


def cleanup_command(cmd: list[str], timeout: int = 120) -> tuple[bool, str]:
    append_log(f"[{now()}] + {format_cmd(cmd)}")
    ok, output = run_command(cmd, ROOT, timeout)
    for line in output.splitlines():
        append_log(line)
    return ok, output


def run_cleanup_operation(policy: dict) -> None:
    set_state(
        running=True,
        started_at=now(),
        finished_at="",
        last_exit=None,
        message="cleanup running",
        current_action="cleanup",
        current_component="",
        current_backup_id="",
        current_staging_dir="",
        log_tail=[],
    )
    try:
        errors = []
        before = cleanup_preview(policy)
        plan = cleanup_plan(policy)
        removed_rollback_images = 0
        removed_backups = 0

        for item in plan["rollback_images"]:
            ok, output = cleanup_command(["docker", "image", "rm", item["reference"]])
            if ok:
                removed_rollback_images += 1
            else:
                errors.append(f"rollback image {item['reference']}: {output}")

        for item in plan["backups"]:
            try:
                target = resolved(item["path"])
                backup_root = resolved(BACKUP_DIR)
                if not is_relative_to(target, backup_root) or target == backup_root:
                    raise RuntimeError("backup path escaped backup root")
                shutil.rmtree(target)
                removed_backups += 1
            except Exception as exc:
                errors.append(f"backup {item['component']}/{item['id']}: {exc}")

        dangling_pruned = False
        if policy["prune_dangling_images"]:
            ok, output = cleanup_command(["docker", "image", "prune", "--force"])
            dangling_pruned = ok
            if not ok:
                errors.append(f"dangling images: {output}")

        build_cache_pruned = False
        if policy["prune_build_cache"]:
            ok, output = cleanup_command(
                [
                    "docker",
                    "builder",
                    "prune",
                    "--force",
                    "--filter",
                    f"until={policy['build_cache_max_age_hours']}h",
                ],
                300,
            )
            build_cache_pruned = ok
            if not ok:
                errors.append(f"build cache: {output}")

        after = cleanup_preview(policy)
        result = {
            "completed_at": now(),
            "removed_rollback_images": removed_rollback_images,
            "removed_backups": removed_backups,
            "dangling_images_pruned": dangling_pruned,
            "build_cache_pruned": build_cache_pruned,
            "free_bytes_before": before["disk"]["free_bytes"],
            "free_bytes_after": after["disk"]["free_bytes"],
            "freed_bytes": max(0, after["disk"]["free_bytes"] - before["disk"]["free_bytes"]),
            "errors": errors,
            "preview": after,
        }
        set_state(
            running=False,
            last_exit=1 if errors else 0,
            finished_at=now(),
            message="cleanup completed" if not errors else "cleanup completed with errors",
            last_cleanup=result,
        )
    except Exception as exc:
        append_log(f"[{now()}] cleanup failed: {exc}")
        set_state(
            running=False,
            last_exit=1,
            finished_at=now(),
            message=f"cleanup failed: {exc}",
            last_cleanup={
                "completed_at": now(),
                "removed_rollback_images": 0,
                "removed_backups": 0,
                "dangling_images_pruned": False,
                "build_cache_pruned": False,
                "free_bytes_before": 0,
                "free_bytes_after": 0,
                "freed_bytes": 0,
                "errors": [str(exc)],
            },
        )


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
    add("staging root configured", bool(STAGING_ROOT), str(STAGING_ROOT))
    add("custom remote configured", bool(CUSTOM_REMOTE), CUSTOM_REMOTE)

    try:
        BACKUP_DIR.mkdir(parents=True, exist_ok=True)
        probe = BACKUP_DIR / ".write-test"
        probe.write_text(now(), encoding="utf-8")
        probe.unlink(missing_ok=True)
        add("backup dir writable", True, str(BACKUP_DIR))
    except Exception as exc:
        add("backup dir writable", False, f"{BACKUP_DIR}: {exc}")

    try:
        staging_dir = ensure_staging_path(STAGING_WORKTREE)
        probe_dir = resolved(STAGING_ROOT)
        probe_dir.mkdir(parents=True, exist_ok=True)
        probe = probe_dir / ".write-test"
        probe.write_text(now(), encoding="utf-8")
        probe.unlink(missing_ok=True)
        add("staging dir writable", True, str(staging_dir))
    except Exception as exc:
        add("staging dir writable", False, f"{STAGING_WORKTREE}: {exc}")

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
    add("V2 online smoke script", v2_smoke_script_ok(), "present" if v2_smoke_script_ok() else "missing V2 smoke checks")
    add(
        "upstream merge staging sources",
        upstream_merge_sources_ok(),
        "present" if upstream_merge_sources_ok() else "missing upstream merge staging hooks",
    )

    if SCRIPT.exists():
        ok, out = run_command(["sh", "-n", str(SCRIPT)], ROOT)
        add("update script syntax", ok, out)
    v2_smoke_sh = ROOT / "deploy/glart-stack/scripts/v2-smoke.sh"
    if v2_smoke_sh.exists():
        ok, out = run_command(["sh", "-n", str(v2_smoke_sh)], ROOT)
        add("V2 smoke wrapper syntax", ok, out)

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
        "cpa_manager_plus_ready": False,
        "sidecar_bridge_sources_ok": False,
        "response_cache_sources_ok": False,
        "output_policy_sources_ok": False,
        "profit_risk_sources_ok": False,
        "omniroute_parity_sources_ok": False,
        "v2_smoke_script_ok": False,
        "proxy_test_chat_checked": False,
        "proxy_test_chat_ok": False,
        "proxy_test_chat_skipped": False,
    }
    try:
        result["new_api_healthy"], result["status"], result["content_type"] = http_status_ok("http://new-api:3000/api/status")
        result["gpt_load_healthy"] = http_ok("http://gpt-load:3001/health")
        result["cliproxyapi_ready"] = http_ok("http://cliproxyapi:8317/management.html")
        result["cpa_manager_plus_ready"] = http_ok("http://cpa-manager-plus:18317/health") and http_ok("http://cpa-manager-plus:18317/usage-service/info")
        result["sidecar_bridge_sources_ok"] = custom_bridge_sources_ok()
        result["response_cache_sources_ok"] = response_cache_sources_ok()
        result["output_policy_sources_ok"] = output_policy_sources_ok()
        result["profit_risk_sources_ok"] = profit_risk_sources_ok()
        result["omniroute_parity_sources_ok"] = omniroute_parity_sources_ok()
        result["v2_smoke_script_ok"] = v2_smoke_script_ok()
        result["proxy_test_chat_checked"], result["proxy_test_chat_ok"], result["proxy_test_chat_skipped"] = proxy_test_chat_smoke()
        result["ok"] = all(
            [
                result["new_api_healthy"],
                result["gpt_load_healthy"],
                result["cliproxyapi_ready"],
                result["cpa_manager_plus_ready"],
                result["sidecar_bridge_sources_ok"],
                result["response_cache_sources_ok"],
                result["output_policy_sources_ok"],
                result["profit_risk_sources_ok"],
                result["omniroute_parity_sources_ok"],
                result["v2_smoke_script_ok"],
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
                '"/cpa-native"' in web_router,
                "GPT-Load" in top_nav,
                "CPA Manager Plus" in top_nav,
                "GPT_LOAD_INTERNAL_URL" in controller,
                "CLIPROXYAPI_INTERNAL_URL" in controller,
                "CPA_MANAGER_PLUS_INTERNAL_URL" in controller,
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


def v2_smoke_script_ok() -> bool:
    try:
        script = (ROOT / "deploy/glart-stack/scripts/v2-smoke.py").read_text(encoding="utf-8")
        wrapper = (ROOT / "deploy/glart-stack/scripts/v2-smoke.sh").read_text(encoding="utf-8")
        return all(
            [
                "GLART_BASE_URL" in script,
                "GLART_API_KEY" in script,
                "GLART_ROOT_ACCESS_TOKEN" in script,
                "GLART_ROOT_COOKIE" in script,
                "OmniRoute-style stacked compression preview" in script,
                "profit event diagnostic fields" in script,
                "profit analytics aggregation fields" in script,
                'exec "$PYTHON_BIN" "$SCRIPT_DIR/v2-smoke.py"' in wrapper,
            ]
        )
    except Exception:
        return False


def upstream_merge_sources_ok() -> bool:
    try:
        updater = (ROOT / "deploy/glart-stack/updater/server.py").read_text(encoding="utf-8")
        controller = (ROOT / "controller/system_update.go").read_text(encoding="utf-8")
        router = (ROOT / "router/api-router.go").read_text(encoding="utf-8")
        api_types = (ROOT / "web/default/src/features/system-settings/api.ts").read_text(encoding="utf-8")
        maintenance = (ROOT / "web/default/src/features/system-settings/maintenance/update-checker-section.tsx").read_text(
            encoding="utf-8"
        )
        return all(
            [
                "prepare_upstream_merge" in router,
                "PrepareUpstreamMerge" in controller,
                "prepareSystemUpstreamMerge" in api_types,
                "prepare-upstream-merge" in updater,
                "准备上游合并" in maintenance,
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
        current_staging_dir="",
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
        if parsed.path == "/cleanup/preview":
            params = parse_qs(parsed.query)
            payload = {
                key: values[-1]
                for key, values in params.items()
                if values
            }
            try:
                self.write_json(cleanup_preview(cleanup_policy(payload)))
            except ValueError as exc:
                self.write_json({"message": str(exc)}, 400)
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
        if parsed.path == "/cleanup":
            try:
                policy = cleanup_policy(parse_json_body(self))
            except ValueError as exc:
                self.write_json({"message": str(exc)}, 400)
                return
            with state_lock:
                if state["running"]:
                    self.write_json(snapshot(), 409)
                    return
                state["running"] = True
            threading.Thread(
                target=run_cleanup_operation,
                args=(policy,),
                daemon=True,
            ).start()
            time.sleep(0.2)
            self.write_json(snapshot())
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
        if parsed.path == "/prepare-upstream-merge":
            with state_lock:
                if state["running"]:
                    self.write_json(snapshot(), 409)
                    return
                state["running"] = True
            threading.Thread(
                target=prepare_upstream_merge_operation,
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
