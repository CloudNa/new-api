#!/usr/bin/env sh
set -eu

ROOT_DIR="${GLART_STACK_ROOT:-/opt/glart-api/app}"
COMPOSE_DIR="${GLART_STACK_COMPOSE_DIR:-$ROOT_DIR/deploy/glart-stack}"
COMPOSE_FILE="${GLART_STACK_COMPOSE_FILE:-compose.yml}"

cd "$ROOT_DIR"

git fetch --prune origin
branch="$(git rev-parse --abbrev-ref HEAD)"
git pull --ff-only origin "$branch"

cd "$COMPOSE_DIR"

docker compose -f "$COMPOSE_FILE" pull gpt-load cliproxyapi caddy redis
docker compose -f "$COMPOSE_FILE" build new-api glart-stack-updater
docker compose -f "$COMPOSE_FILE" up -d --remove-orphans

docker compose -f "$COMPOSE_FILE" ps
