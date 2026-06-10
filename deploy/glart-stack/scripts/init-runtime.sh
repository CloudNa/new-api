#!/usr/bin/env sh
set -eu

STACK_DIR="${1:-$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)}"
ENV_FILE="$STACK_DIR/.env"

if [ ! -f "$ENV_FILE" ]; then
  cp "$STACK_DIR/.env.example" "$ENV_FILE"
fi

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

for dir in \
  runtime/new-api/data \
  runtime/new-api/logs \
  runtime/redis \
  runtime/gpt-load \
  runtime/cpa-manager-plus \
  runtime/cliproxyapi/auths \
  runtime/cliproxyapi/logs \
  runtime/caddy/data \
  runtime/caddy/config
do
  mkdir -p "$STACK_DIR/$dir"
done

CONFIG_PATH="$STACK_DIR/runtime/cliproxyapi/config.yaml"
if [ ! -f "$CONFIG_PATH" ]; then
  sed \
    -e "s|\${CLIPROXYAPI_MANAGEMENT_KEY}|${CLIPROXYAPI_MANAGEMENT_KEY:-}|g" \
    -e "s|\${CLIPROXYAPI_API_KEY}|${CLIPROXYAPI_API_KEY:-}|g" \
    "$STACK_DIR/cliproxyapi.config.example.yaml" > "$CONFIG_PATH"
  chmod 600 "$CONFIG_PATH"
fi

chmod 700 \
  "$STACK_DIR/runtime/cpa-manager-plus" \
  "$STACK_DIR/runtime/cliproxyapi/auths" \
  "$STACK_DIR/runtime/cliproxyapi/logs"

printf '%s\n' "Runtime initialized at $STACK_DIR"
printf '%s\n' "Edit $ENV_FILE before first deploy, then rerun this script if you need to regenerate config.yaml."
