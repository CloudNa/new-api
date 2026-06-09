# Glart clean stack

This stack deploys the clean composition line:

- `new-api` is the only public gateway at `https://api.glart.cn`.
- `gpt-load` stays private on the Docker network and is reachable from root users through `/gl`.
- `CLIProxyAPI` stays private on the Docker network and is reachable from root users through `/cpa`.
- Caddy is the only service publishing host ports `80` and `443`.

Do not commit files under `runtime/` or `.env`; they hold database files, logs, auth state, OAuth credentials, and management keys.

## First deploy

1. Copy `.env.example` to `.env` and replace every `change-me-*` value with a long random secret.
2. Create runtime files:

   ```powershell
   ./scripts/init-runtime.ps1
   ```

   On Linux servers:

   ```bash
   sh ./scripts/init-runtime.sh
   ```

3. Confirm `runtime/cliproxyapi/config.yaml` was generated from the matching `.env` values.
4. Start the stack:

   ```bash
   docker compose -f compose.yml up -d --build
   ```

## New API upstream channels

Use these internal base URLs when adding channels in `new-api`:

- GPT-Load OpenAI-compatible group: `http://gpt-load:3001/proxy/openai`
- GPT-Load Gemini group: `http://gpt-load:3001/proxy/gemini`
- GPT-Load Anthropic group: `http://gpt-load:3001/proxy/anthropic`
- CLIProxyAPI OpenAI-compatible endpoint: `http://cliproxyapi:8317`

Use a `proxy-test` group for initial validation before moving channels to default production groups.

## One-click updates

The dashboard system maintenance page calls the private `glart-stack-updater` service. It supports a combined stack update and separate component updates for `new-api`, `GPT-Load`, and `CLIProxyAPI`. Each component gets its own backup and can be rolled back independently from the same page.

The combined update runs:

```bash
backup new-api runtime and current image
git pull --ff-only
run custom source-retention checks
go test ./service ./controller ./model ./router ./relay ./pkg/billingexpr ./setting/billing_setting ./pkg/profit ./pkg/promptcompress -count=1
docker compose build new-api
docker compose up -d new-api redis gpt-load cliproxyapi caddy glart-stack-updater
smoke new-api and optional proxy-test chat
backup GPT-Load runtime and current image
docker compose pull gpt-load && docker compose up -d --no-deps gpt-load
smoke GPT-Load
backup CLIProxyAPI runtime and current image
docker compose pull cliproxyapi && docker compose up -d --no-deps cliproxyapi
smoke CLIProxyAPI
docker compose build/up glart-stack-updater
```

This keeps the Glart bridge and deployment scripts in the Git branch, so future upstream `new-api` updates do not overwrite the local sidecar integration.

Single component updates use the same backup and smoke path for only that service. Rollback is intentionally component-scoped: choose `new-api`, `GPT-Load`, or `CLIProxyAPI`; `all` rollback is disabled so a sidecar can be reverted without touching the gateway or the other sidecar.

Backups are written under `runtime/backups/<component>/` by default. They include the relevant runtime path, `.env`, Compose/Caddy files, a rollback image tag, and a small manifest with the pre-update Git commit and Compose status. Keep this directory private and never commit it.

Optional proxy-test chat smoke can be enabled by setting `GLART_SMOKE_MODEL` and `GLART_SMOKE_API_KEY` in the private server `.env`. Leave them empty to skip that smoke safely.

For the fuller V2 profit/compression rollout check, run the standalone smoke script from the server checkout. It reads credentials only from environment variables and never writes them to disk:

```bash
GLART_BASE_URL=https://api.glart.cn \
GLART_ROOT_USER_ID=1 \
GLART_ROOT_ACCESS_TOKEN='root-access-token' \
GLART_API_KEY='proxy-test-api-key' \
GLART_SMOKE_MODEL=gpt-5.5 \
sh deploy/glart-stack/scripts/v2-smoke.sh
```

You can use `GLART_ROOT_COOKIE` instead of `GLART_ROOT_ACCESS_TOKEN` when running from a browser session, but `GLART_ROOT_USER_ID` is still required by root-only APIs. Set `GLART_SMOKE_STREAM=1` and `GLART_SMOKE_LONG_CONTEXT=1` when you want the script to include stream and long-context chat smoke. Set `GLART_V2_SMOKE_REQUIRE_ROOT=1` or `GLART_V2_SMOKE_REQUIRE_CHAT=1` to turn missing credentials into a hard failure.

Set `GLART_STACK_RUN_TESTS=0` or `GLART_STACK_SKIP_TESTS=1` only for emergency updates where the test container cannot run. Normal one-click updates should keep tests enabled.

The Go test stage uses `GLART_STACK_GO_TEST_IMAGE`, defaulting to `golang:1.26.1` because it includes `git`. The updater marks the mounted `/workspace` repository as a Git safe directory before running tests, which avoids dubious-ownership failures when Docker mounts the host checkout into the test container.

Set `GLART_STACK_HOST_ROOT` in the private server `.env` to the absolute checkout path on the Docker host, for example `/opt/glart-api/app`. The updater mounts the repository at that same absolute path inside its container so Docker bind mounts resolve against the real host checkout instead of a container-only path.
