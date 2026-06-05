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

The dashboard system maintenance page calls the private `glart-stack-updater` service. The updater runs:

```bash
backup runtime/.env into runtime/backups/
git pull --ff-only
go test ./service ./controller ./relay ./pkg/billingexpr ./setting/billing_setting -count=1
docker compose pull gpt-load cliproxyapi caddy redis
docker compose build new-api glart-stack-updater
docker compose up -d --remove-orphans new-api gpt-load cliproxyapi caddy redis
smoke new-api, GPT-Load, CLIProxyAPI, and Glart bridge source retention
```

This keeps the Glart bridge and deployment scripts in the Git branch, so future upstream `new-api` updates do not overwrite the local sidecar integration.

Backups are written under `runtime/backups/` by default. They include `.env`, runtime databases, auth state, Caddy data, and a small manifest with the pre-update Git commit and Compose status. Keep this directory private and never commit it.

Optional proxy-test chat smoke can be enabled by setting `GLART_SMOKE_MODEL` and `GLART_SMOKE_API_KEY` in the private server `.env`. Leave them empty to skip that smoke safely.

Set `GLART_STACK_RUN_TESTS=0` or `GLART_STACK_SKIP_TESTS=1` only for emergency updates where the test container cannot run. Normal one-click updates should keep tests enabled.
