#!/usr/bin/env sh
set -eu

ROOT_DIR="${GLART_STACK_ROOT:-/opt/glart-api/app}"
COMPOSE_DIR="${GLART_STACK_COMPOSE_DIR:-$ROOT_DIR/deploy/glart-stack}"
COMPOSE_FILE="${GLART_STACK_COMPOSE_FILE:-compose.yml}"
BACKUP_ROOT="${GLART_STACK_BACKUP_DIR:-$COMPOSE_DIR/runtime/backups}"
RUN_TESTS="${GLART_STACK_RUN_TESTS:-1}"
GO_TEST_IMAGE="${GLART_STACK_GO_TEST_IMAGE:-golang:1.26.1}"
ACTION="${1:-${GLART_STACK_ACTION:-update}}"
COMPONENT="${2:-${GLART_STACK_COMPONENT:-all}}"
BACKUP_ID="${3:-${GLART_STACK_BACKUP_ID:-latest}}"
RESTORE_RUNTIME_ON_ROLLBACK="${GLART_STACK_RESTORE_RUNTIME_ON_ROLLBACK:-0}"
STAGE="init"
GIT_UPDATED=0

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S %z')" "$*"
}

fail() {
  log "failed stage: $STAGE"
  exit 1
}

trap 'code=$?; if [ "$code" -ne 0 ]; then log "failed stage: $STAGE exit=$code"; fi' EXIT

run() {
  log "+ $*"
  "$@"
}

compose() {
  docker compose -f "$COMPOSE_DIR/$COMPOSE_FILE" "$@"
}

git_root() {
  git -c "safe.directory=$ROOT_DIR" -C "$ROOT_DIR" "$@"
}

require_file() {
  if [ ! -f "$1" ]; then
    log "missing required file: $1"
    fail
  fi
}

load_env() {
  require_file "$COMPOSE_DIR/.env"
  set -a
  # shellcheck disable=SC1090
  . "$COMPOSE_DIR/.env"
  set +a
}

normalize_component() {
  case "$1" in
    all|new-api|gpt-load|cliproxyapi) printf '%s' "$1" ;;
    *) log "invalid component: $1"; fail ;;
  esac
}

component_service() {
  case "$1" in
    new-api) printf '%s' "new-api" ;;
    gpt-load) printf '%s' "gpt-load" ;;
    cliproxyapi) printf '%s' "cliproxyapi" ;;
    *) printf '%s' "" ;;
  esac
}

component_container() {
  case "$1" in
    new-api) printf '%s' "glart-new-api" ;;
    gpt-load) printf '%s' "glart-gpt-load" ;;
    cliproxyapi) printf '%s' "glart-cliproxyapi" ;;
    *) printf '%s' "" ;;
  esac
}

component_image() {
  case "$1" in
    new-api) printf 'glart/new-api:%s' "${NEW_API_VERSION:-clean}" ;;
    gpt-load) printf '%s' "ghcr.io/tbphp/gpt-load:latest" ;;
    cliproxyapi) printf '%s' "eceasy/cli-proxy-api:latest" ;;
    *) printf '%s' "" ;;
  esac
}

component_runtime_path() {
  case "$1" in
    new-api) printf '%s' "runtime/new-api" ;;
    gpt-load) printf '%s' "runtime/gpt-load" ;;
    cliproxyapi) printf '%s' "runtime/cliproxyapi" ;;
    all) printf '%s' "runtime" ;;
    *) printf '%s' "" ;;
  esac
}

precheck_common() {
  STAGE="precheck"
  require_file "$COMPOSE_DIR/.env"
  require_file "$COMPOSE_DIR/$COMPOSE_FILE"
  require_file "$ROOT_DIR/controller/sidecar_proxy.go"
  require_file "$ROOT_DIR/router/web-router.go"
  require_file "$ROOT_DIR/web/default/src/hooks/use-top-nav-links.ts"
  require_file "$ROOT_DIR/pkg/profit/response_cache.go"
  require_file "$ROOT_DIR/service/response_cache.go"
  grep -q '"/gl"' "$ROOT_DIR/router/web-router.go" || fail
  grep -q '"/cpa"' "$ROOT_DIR/router/web-router.go" || fail
  grep -q "GPT-Load" "$ROOT_DIR/web/default/src/hooks/use-top-nav-links.ts" || fail
  grep -q "CLIProxyAPI" "$ROOT_DIR/web/default/src/hooks/use-top-nav-links.ts" || fail
  run docker compose -f "$COMPOSE_DIR/$COMPOSE_FILE" config --quiet
}

create_backup() {
  component="$(normalize_component "$1")"
  STAGE="backup-$component"
  backup_stamp="$(date '+%Y%m%d-%H%M%S')"
  before_commit="$(git_root rev-parse HEAD 2>/dev/null || printf 'unknown')"
  before_short_commit="$(git_root rev-parse --short HEAD 2>/dev/null || printf 'unknown')"
  backup_dir="$BACKUP_ROOT/$component/$backup_stamp-$before_short_commit"
  rollback_tag="glart/rollback-$component:$backup_stamp"
  container="$(component_container "$component")"
  service_image="$(component_image "$component")"
  runtime_path="$(component_runtime_path "$component")"

  mkdir -p "$backup_dir"
  log "writing $component backup to $backup_dir"

  current_image_id=""
  if [ -n "$container" ]; then
    current_image_id="$(docker inspect -f '{{.Image}}' "$container" 2>/dev/null || true)"
  fi
  if [ -n "$current_image_id" ]; then
    docker tag "$current_image_id" "$rollback_tag" 2>/dev/null || true
  fi

  git_root rev-parse HEAD > "$backup_dir/before_commit.txt" 2>/dev/null || true
  git_root status --short --branch > "$backup_dir/git_status.txt" 2>/dev/null || true
  compose ps > "$backup_dir/compose_ps_before.txt" 2>/dev/null || true

  if [ -n "$runtime_path" ] && [ -e "$COMPOSE_DIR/$runtime_path" ]; then
    tar --exclude='runtime/backups' -czf "$backup_dir/runtime.tar.gz" -C "$COMPOSE_DIR" .env "$runtime_path" Caddyfile compose.yml
  else
    tar --exclude='runtime/backups' -czf "$backup_dir/runtime.tar.gz" -C "$COMPOSE_DIR" .env Caddyfile compose.yml
  fi

  {
    printf 'COMPONENT=%s\n' "$component"
    printf 'CREATED_AT=%s\n' "$backup_stamp"
    printf 'BEFORE_COMMIT=%s\n' "$before_commit"
    printf 'SERVICE_IMAGE=%s\n' "$service_image"
    printf 'CURRENT_IMAGE_ID=%s\n' "$current_image_id"
    printf 'ROLLBACK_TAG=%s\n' "$rollback_tag"
    printf 'RUNTIME_PATH=%s\n' "$runtime_path"
  } > "$backup_dir/manifest.env"

  log "$component backup completed: $backup_dir"
}

latest_backup_dir() {
  component="$(normalize_component "$1")"
  if [ "$BACKUP_ID" = "latest" ] || [ -z "$BACKUP_ID" ]; then
    ls -1dt "$BACKUP_ROOT/$component"/* 2>/dev/null | head -n 1
    return
  fi
  printf '%s/%s/%s\n' "$BACKUP_ROOT" "$component" "$BACKUP_ID"
}

git_update_and_tests() {
  if [ "$GIT_UPDATED" = "1" ]; then
    return
  fi

  STAGE="git-update"
  run git_root fetch --prune origin
  branch="$(git_root rev-parse --abbrev-ref HEAD)"
  run git_root pull --ff-only origin "$branch"
  GIT_UPDATED=1

  STAGE="custom-smoke-source"
  require_file "$ROOT_DIR/controller/sidecar_proxy.go"
  require_file "$ROOT_DIR/router/web-router.go"
  require_file "$ROOT_DIR/web/default/src/hooks/use-top-nav-links.ts"
  grep -q '"/gl"' "$ROOT_DIR/router/web-router.go" || fail
  grep -q '"/cpa"' "$ROOT_DIR/router/web-router.go" || fail
  grep -q "GPT-Load" "$ROOT_DIR/web/default/src/hooks/use-top-nav-links.ts" || fail
  grep -q "CLIProxyAPI" "$ROOT_DIR/web/default/src/hooks/use-top-nav-links.ts" || fail
  grep -q "GetSystemUpdateStatus" "$ROOT_DIR/controller/system_update.go" || fail
  grep -q 'apiRouter.Group("/profit")' "$ROOT_DIR/router/api-router.go" || fail
  grep -q "GetProfitAnalytics" "$ROOT_DIR/controller/profit.go" || fail
  grep -q "UpdateProfitCostProfiles" "$ROOT_DIR/controller/profit.go" || fail
  grep -q "PreviewProfitRoute" "$ROOT_DIR/controller/profit.go" || fail
  grep -q "CostProfilesOptionKey" "$ROOT_DIR/pkg/profit/settings.go" || fail
  grep -q "PreviewRoute" "$ROOT_DIR/pkg/profit/cost.go" || fail
  grep -q 'apiRouter.Group("/settings/compression")' "$ROOT_DIR/router/api-router.go" || fail
  grep -q "PreviewCompression" "$ROOT_DIR/controller/compression.go" || fail
  require_file "$ROOT_DIR/pkg/promptcompress/compressor.go"
  grep -q "ApplyPromptCompressionForRelay" "$ROOT_DIR/relay/compatible_handler.go" || fail
  grep -q "PromptCompressionStats" "$ROOT_DIR/relay/common/relay_info.go" || fail
  grep -q "compression_rules_version" "$ROOT_DIR/pkg/profit/observation.go" || fail
  grep -q "ProfitCenterSection" "$ROOT_DIR/web/default/src/features/system-settings/operations/section-registry.tsx" || fail
  require_file "$ROOT_DIR/web/default/src/features/system-settings/maintenance/profit-center-section.tsx"
  grep -q "PromptCompressionSection" "$ROOT_DIR/web/default/src/features/system-settings/operations/section-registry.tsx" || fail
  require_file "$ROOT_DIR/web/default/src/features/system-settings/maintenance/prompt-compression-section.tsx"
  grep -q "ResponseCacheRule" "$ROOT_DIR/pkg/profit/settings.go" || fail
  grep -q "BuildResponseCacheDecision" "$ROOT_DIR/pkg/profit/response_cache.go" || fail
  grep -q "PutResponseCache" "$ROOT_DIR/pkg/profit/response_cache.go" || fail
  grep -q "PrepareResponseCacheForRelay" "$ROOT_DIR/service/response_cache.go" || fail
  grep -q "ServeResponseCacheHit" "$ROOT_DIR/service/response_cache.go" || fail
  grep -q "StoreResponseCacheForRelay" "$ROOT_DIR/relay/compatible_handler.go" || fail
  grep -q "ProfitResponseCacheObservationFromContext" "$ROOT_DIR/service/text_quota.go" || fail
  grep -q "response_cache_saved_usd" "$ROOT_DIR/pkg/profit/observation.go" || fail
  grep -q "ResponseCacheModeSelect" "$ROOT_DIR/web/default/src/features/system-settings/maintenance/profit-center-section.tsx" || fail
  grep -q "profit-response-cache-rules-json" "$ROOT_DIR/web/default/src/features/system-settings/maintenance/profit-center-section.tsx" || fail

  if [ "$RUN_TESTS" != "0" ] && [ "${GLART_STACK_SKIP_TESTS:-0}" != "1" ]; then
    STAGE="go-test"
    mkdir -p "$COMPOSE_DIR/runtime/cache/go-build" "$COMPOSE_DIR/runtime/cache/gomod"
    run docker run --rm \
      -v "$ROOT_DIR:/workspace" \
      -v "$COMPOSE_DIR/runtime/cache/go-build:/tmp/go-cache" \
      -v "$COMPOSE_DIR/runtime/cache/gomod:/tmp/gomodcache" \
      -w /workspace \
      -e GOCACHE=/tmp/go-cache \
      -e GOMODCACHE=/tmp/gomodcache \
      "$GO_TEST_IMAGE" \
      sh -c 'git config --global --add safe.directory /workspace && go test ./service ./controller ./model ./router ./relay ./pkg/billingexpr ./setting/billing_setting ./pkg/profit ./pkg/promptcompress -count=1'
  else
    log "go test stage skipped by configuration"
  fi
}

retry_smoke() {
  label="$1"
  attempts="$2"
  delay_seconds="$3"
  shift 3

  attempt=1
  while [ "$attempt" -le "$attempts" ]; do
    if "$@"; then
      log "$label smoke passed on attempt $attempt/$attempts"
      return 0
    fi
    if [ "$attempt" -lt "$attempts" ]; then
      log "$label smoke not ready (attempt $attempt/$attempts); retrying in ${delay_seconds}s"
      sleep "$delay_seconds"
    fi
    attempt=$((attempt + 1))
  done

  log "$label smoke failed after $attempts attempts"
  return 1
}

smoke_new_api() {
  STAGE="smoke-new-api"
  retry_smoke "new-api" 40 3 sh -c "docker exec glart-new-api wget -q -O - http://localhost:3000/api/status | grep -q '\"success\"[[:space:]]*:[[:space:]]*true'"
}

smoke_gpt_load() {
  STAGE="smoke-gpt-load"
  retry_smoke "gpt-load" 30 2 docker exec glart-gpt-load wget -q --spider -T 10 -O /dev/null http://localhost:3001/health
}

smoke_cliproxyapi() {
  STAGE="smoke-cliproxyapi"
  retry_smoke "cliproxyapi" 30 2 docker exec glart-cliproxyapi wget -q --spider -T 10 -O /dev/null http://localhost:8317/management.html
}

optional_proxy_test_chat_smoke() {
  if [ -n "${GLART_SMOKE_API_KEY:-}" ] && [ -n "${GLART_SMOKE_MODEL:-}" ]; then
    STAGE="proxy-test-chat-smoke"
    log "running optional proxy-test chat smoke with configured model"
    run docker exec glart-new-api wget -q -O - \
      --header="Authorization: Bearer $GLART_SMOKE_API_KEY" \
      --header="Content-Type: application/json" \
      --post-data="{\"model\":\"$GLART_SMOKE_MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"Return exactly: ok\"}],\"max_tokens\":8}" \
      http://localhost:3000/v1/chat/completions
  else
    log "optional proxy-test chat smoke skipped: GLART_SMOKE_API_KEY or GLART_SMOKE_MODEL is not configured"
  fi
}

rollback_after_failed_update() {
  component="$(normalize_component "$1")"
  STAGE="auto-rollback-$component"
  log "$component post-update smoke failed; attempting automatic image rollback"
  BACKUP_ID="latest"
  RESTORE_RUNTIME_ON_ROLLBACK="0"
  rollback_component "$component"
  STAGE="post-update-smoke-$component"
  fail
}

update_new_api() {
  create_backup new-api
  git_update_and_tests
  STAGE="build-new-api"
  run compose build new-api
  STAGE="up-new-api"
  run compose up -d --remove-orphans new-api redis gpt-load cliproxyapi caddy glart-stack-updater
  if ! smoke_new_api; then
    rollback_after_failed_update new-api
  fi
  if ! optional_proxy_test_chat_smoke; then
    rollback_after_failed_update new-api
  fi
}

update_gpt_load() {
  create_backup gpt-load
  STAGE="pull-gpt-load"
  run compose pull gpt-load
  STAGE="up-gpt-load"
  run compose up -d --no-deps gpt-load
  if ! smoke_gpt_load; then
    rollback_after_failed_update gpt-load
  fi
}

update_cliproxyapi() {
  create_backup cliproxyapi
  STAGE="pull-cliproxyapi"
  run compose pull cliproxyapi
  STAGE="up-cliproxyapi"
  run compose up -d --no-deps cliproxyapi
  if ! smoke_cliproxyapi; then
    rollback_after_failed_update cliproxyapi
  fi
}

update_updater() {
  STAGE="build-updater"
  run compose build glart-stack-updater
  STAGE="up-updater"
  run compose up -d --no-deps glart-stack-updater || log "updater self-refresh will apply on next run"
}

update_all() {
  update_new_api
  update_gpt_load
  update_cliproxyapi
  update_updater
}

rollback_component() {
  component="$(normalize_component "$1")"
  if [ "$component" = "all" ]; then
    log "rollback all is intentionally disabled; rollback each component explicitly"
    fail
  fi

  backup_dir="$(latest_backup_dir "$component")"
  if [ ! -d "$backup_dir" ]; then
    log "backup not found for $component: $backup_dir"
    fail
  fi
  manifest="$backup_dir/manifest.env"
  require_file "$manifest"

  # shellcheck disable=SC1090
  . "$manifest"
  service="$(component_service "$component")"
  service_image="$(component_image "$component")"
  rollback_image="${ROLLBACK_TAG:-}"
  if [ -z "$rollback_image" ]; then
    rollback_image="${CURRENT_IMAGE_ID:-}"
  fi
  if [ -z "$rollback_image" ]; then
    log "backup has no rollback image reference: $backup_dir"
    fail
  fi

  STAGE="rollback-image-$component"
  run docker image inspect "$rollback_image"
  run docker tag "$rollback_image" "$service_image"

  if [ "$RESTORE_RUNTIME_ON_ROLLBACK" = "1" ]; then
    STAGE="rollback-runtime-$component"
    require_file "$backup_dir/runtime.tar.gz"
    log "restoring runtime archive for $component from $backup_dir"
    run tar -xzf "$backup_dir/runtime.tar.gz" -C "$COMPOSE_DIR"
  else
    log "runtime restore skipped for $component; set GLART_STACK_RESTORE_RUNTIME_ON_ROLLBACK=1 to restore runtime archive"
  fi

  STAGE="rollback-up-$component"
  run compose up -d --no-deps "$service"
  case "$component" in
    new-api) smoke_new_api ;;
    gpt-load) smoke_gpt_load ;;
    cliproxyapi) smoke_cliproxyapi ;;
  esac
  log "$component rollback completed from $backup_dir"
}

main() {
  cd "$ROOT_DIR"
  load_env
  precheck_common
  COMPONENT="$(normalize_component "$COMPONENT")"
  case "$ACTION:$COMPONENT" in
    update:all) update_all ;;
    update:new-api) update_new_api ;;
    update:gpt-load) update_gpt_load ;;
    update:cliproxyapi) update_cliproxyapi ;;
    rollback:new-api) rollback_component new-api ;;
    rollback:gpt-load) rollback_component gpt-load ;;
    rollback:cliproxyapi) rollback_component cliproxyapi ;;
    *) log "unsupported action/component: $ACTION $COMPONENT"; fail ;;
  esac
  STAGE="done"
  log "glart stack $ACTION $COMPONENT completed successfully"
}

main "$@"
