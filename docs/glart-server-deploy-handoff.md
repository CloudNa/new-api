# Glart new-api 服务器部署交接

最后核对时间：2026-06-15  
用途：让新的 Codex 会话或其他维护者可以立刻理解 `api.glart.cn` 的服务器连接方式、new-api 部署结构、更新/回滚流程，以及其他相关项目如何接入 new-api。

## 一句话结论

- 公网主入口是 `https://api.glart.cn`，公网流量经 EdgeOne 到 Oracle Cloud，再由 Caddy 转发到 `new-api`。
- `new-api` 是唯一公网 API 网关，负责用户、密钥、计费、渠道、日志、收益托管、压缩和 sidecar bridge。
- GPT-Load、CLIProxyAPI、CPA Manager Plus、Redis、updater 都只在 Docker 内网 `glart-internal` 中运行，不直接暴露管理端口。
- 服务器项目路径是 `/opt/glart-api/app`，当前代码分支是 `codex/clean-newapi-gptload-cpa`，服务器 `origin` 指向 `https://github.com/CloudNa/new-api.git`。
- 更新 new-api 使用服务器内置脚本：`sh deploy/glart-stack/scripts/update-glart-stack.sh update new-api`。

## 连接服务器

服务器信息：

```text
Host: 144.24.60.180
User: ubuntu
Project path: /opt/glart-api/app
Hostname: glart-api
```

在当前 Windows 工作机上，默认 SSH 可能不会自动选到 Oracle key。请显式指定 key：

```powershell
ssh -i "$env:USERPROFILE\.ssh\oracle-glart-20260504.key" -o IdentitiesOnly=yes ubuntu@144.24.60.180
```

快速只读检查：

```powershell
ssh -i "$env:USERPROFILE\.ssh\oracle-glart-20260504.key" -o IdentitiesOnly=yes ubuntu@144.24.60.180 "cd /opt/glart-api/app && hostname && git branch --show-current && git rev-parse --short HEAD && docker compose -f deploy/glart-stack/compose.yml ps"
```

不要把私钥内容、`.env`、API key、OAuth state、cookies、订阅 URL、auth 文件写入聊天、文档或 Git。

## 当前代码和远端

本地工作区：

```text
D:\shuziren\Glart api
```

本地当前分支：

```text
codex/clean-newapi-gptload-cpa
```

重要远端：

```text
cloudna  https://github.com/CloudNa/new-api.git
origin   https://github.com/QuantumNous/new-api.git
server-staging ssh://ubuntu@144.24.60.180/opt/glart-api/app/.git
```

服务器仓库当前配置：

```text
/opt/glart-api/app
branch: codex/clean-newapi-gptload-cpa
origin: https://github.com/CloudNa/new-api.git
```

也就是说，常规发布流程是：

1. 本地提交到 `codex/clean-newapi-gptload-cpa`。
2. 推送到 `cloudna/codex/clean-newapi-gptload-cpa`。
3. 服务器执行更新脚本，脚本会从服务器当前分支的 `origin` 拉取同名分支。

## GitHub 代理

当前本地 Git 代理配置为：

```text
http.proxy=http://127.0.0.1:3067
https.proxy=http://127.0.0.1:3067
http.https://github.com.proxy=http://127.0.0.1:3067
https.https://github.com.proxy=http://127.0.0.1:3067
```

如果本地推送 GitHub 失败，先确认 `127.0.0.1:3067` 代理正在运行。不要随便切换为系统默认网络，之前 GitHub Desktop 可用但 Codex shell 不一定自动共享同一代理。

## 部署目录结构

核心目录：

```text
/opt/glart-api/app
  deploy/glart-stack/compose.yml
  deploy/glart-stack/Caddyfile
  deploy/glart-stack/.env                 # 有真实密钥，禁止输出/提交
  deploy/glart-stack/scripts/update-glart-stack.sh
  deploy/glart-stack/scripts/v2-smoke.sh
  deploy/glart-stack/runtime/             # 数据、日志、备份、auth state，禁止提交
```

主要 runtime 路径：

```text
deploy/glart-stack/runtime/new-api/data       # new-api SQLite 数据库等
deploy/glart-stack/runtime/new-api/logs       # new-api 日志
deploy/glart-stack/runtime/redis              # Redis 数据
deploy/glart-stack/runtime/gpt-load           # GPT-Load 数据
deploy/glart-stack/runtime/cliproxyapi        # CLIProxyAPI config/auths/logs
deploy/glart-stack/runtime/cpa-manager-plus   # CPA Manager Plus 数据
deploy/glart-stack/runtime/caddy              # Caddy 证书和配置状态
deploy/glart-stack/runtime/backups            # 更新前自动备份
```

## Docker 服务

当前 Compose 服务：

```text
new-api              glart-new-api             内网 3000
redis                glart-redis               内网 6379
gpt-load             glart-gpt-load            内网 3001
cliproxyapi          glart-cliproxyapi         内网 8317
cpa-manager-plus     glart-cpa-manager-plus    内网 18317
caddy                glart-caddy               公网 80/443
glart-stack-updater  glart-stack-updater       内网 8787
```

所有服务都在 Docker network：

```text
glart-internal
```

只有 Caddy 绑定公网端口 `80` 和 `443`。其他管理端口只 `expose` 给 Docker 内网，不要加 `ports` 暴露到公网。

## 公网流量路径

```text
用户 / API 客户端
  -> https://api.glart.cn
  -> Tencent EdgeOne
  -> Oracle Cloud 144.24.60.180:443
  -> Caddy
  -> new-api:3000
```

Caddy 对 API/SSE 路径做了特殊处理：

```text
/v1/*
/v1beta/*
/pg/*
/mj/*
/suno/*
/kling/v1/*
/jimeng/*
```

这些路径转发到 `new-api:3000` 时：

- `flush_interval -1`
- 下发 `X-Accel-Buffering: no`
- 下发 no-store/no-cache
- 移除 `Alt-Svc`

目的：降低 SSE/流式响应被缓冲、压缩或 HTTP/3 干扰的概率。

## new-api 如何部署

`new-api` 在服务器上不是直接拉官方镜像，而是从当前仓库源码构建本地镜像：

```yaml
new-api:
  build:
    context: ../..
    dockerfile: Dockerfile
  image: glart/new-api:${NEW_API_VERSION:-clean}
```

当前线上容器使用的镜像 tag 来自 `.env` 中的 `NEW_API_VERSION`，`/api/status` 中可看到类似：

```text
version: glart-clean-a987d2e1
```

注意：这个版本号是自定义镜像标识，不等同于官方 new-api 上游 tag。官方上游基线和更新检查由系统维护里的组合 updater/上游合并逻辑维护。

## 发布流程

本地提交并推送：

```powershell
cd "D:\shuziren\Glart api"
git status --short
git add -- <files>
git commit -m "<message>"
git push cloudna codex/clean-newapi-gptload-cpa
```

服务器更新 new-api：

```powershell
ssh -i "$env:USERPROFILE\.ssh\oracle-glart-20260504.key" -o IdentitiesOnly=yes ubuntu@144.24.60.180 "cd /opt/glart-api/app && sh deploy/glart-stack/scripts/update-glart-stack.sh update new-api"
```

脚本会自动做这些事：

1. `docker compose config --quiet`
2. 为目标组件创建备份到 `deploy/glart-stack/runtime/backups/<component>/...`
3. `git fetch --prune origin`
4. `git pull --ff-only origin <当前分支>`
5. 运行 Go 单测子集
6. `docker compose build new-api`
7. `docker compose up -d --no-deps new-api`
8. 运行 new-api、GPT-Load、CLIProxyAPI、CPA Manager Plus smoke
9. 如果配置了 `GLART_SMOKE_API_KEY` 和 `GLART_SMOKE_MODEL`，会额外跑 `proxy-test` chat smoke

当前脚本文件在服务器上可能没有执行位，所以推荐始终用 `sh deploy/glart-stack/scripts/update-glart-stack.sh ...` 调用。

## 单独更新 sidecar

每个组件可以单独更新：

```bash
cd /opt/glart-api/app
sh deploy/glart-stack/scripts/update-glart-stack.sh update new-api
sh deploy/glart-stack/scripts/update-glart-stack.sh update gpt-load
sh deploy/glart-stack/scripts/update-glart-stack.sh update cliproxyapi
sh deploy/glart-stack/scripts/update-glart-stack.sh update cpa-manager-plus
```

不建议日常使用 `update all`，除非明确要同时更新所有组件。单组件更新更方便定位问题和单独回滚。

## 回滚

查看备份：

```bash
cd /opt/glart-api/app
find deploy/glart-stack/runtime/backups -maxdepth 2 -mindepth 2 -type d | sort | tail -30
```

回滚最近一次目标组件镜像：

```bash
cd /opt/glart-api/app
sh deploy/glart-stack/scripts/update-glart-stack.sh rollback new-api
```

回滚指定备份：

```bash
cd /opt/glart-api/app
sh deploy/glart-stack/scripts/update-glart-stack.sh rollback new-api 20260615-221251-e1262498
```

默认回滚只恢复容器镜像，不恢复 runtime 数据。若必须恢复 runtime，需要显式设置：

```bash
GLART_STACK_RESTORE_RUNTIME_ON_ROLLBACK=1 sh deploy/glart-stack/scripts/update-glart-stack.sh rollback new-api <backup_id>
```

谨慎使用 runtime 恢复，因为它会覆盖数据库、配置或 auth state。除非明确知道影响范围，否则不要恢复 runtime。

重要限制：当前 rollback 主要恢复镜像和可选 runtime，不会自动把 Git 工作树 reset 回旧提交。如果需要源码也回退，应另行执行 Git revert/reset，并重新构建。

## 常见更新故障

### `Permission denied` 执行更新脚本

现象：

```text
bash: deploy/glart-stack/scripts/update-glart-stack.sh: Permission denied
```

处理：用 `sh` 调用脚本：

```bash
sh deploy/glart-stack/scripts/update-glart-stack.sh update new-api
```

### `insufficient permission for adding an object`

现象：

```text
error: insufficient permission for adding an object to repository database .git/objects
fatal: failed to write object
```

原因：之前某些 Git 操作用 `sudo/root` 跑过，导致 `.git` 或代码文件 owner 变成 `root:root`。

只修复代码仓库 ownership，排除 runtime 和 node_modules：

```bash
cd /opt/glart-api/app
sudo chown -R ubuntu:ubuntu .git
sudo find . \
  -path ./deploy/glart-stack/runtime -prune -o \
  -path ./web/default/node_modules -prune -o \
  -path ./web/classic/node_modules -prune -o \
  ! -user ubuntu -exec chown ubuntu:ubuntu {} +
git status --short
```

不要对整个 `/opt/glart-api/app` 无脑 `chown -R`，以免误伤 runtime 中由容器用户管理的文件权限。

### Docker 内 `fatal: detected dubious ownership in repository at '/workspace'`

测试容器内可以使用：

```bash
git config --global --add safe.directory /workspace
```

更新脚本的 Go 测试命令已经包含这个处理。

## 发布后健康检查

服务器检查：

```bash
cd /opt/glart-api/app
git rev-parse --short HEAD
git status --short
docker compose -f deploy/glart-stack/compose.yml ps
```

公网检查：

```powershell
Invoke-WebRequest -UseBasicParsing -Uri "https://api.glart.cn/api/status" -TimeoutSec 30 | Select-Object -ExpandProperty Content
```

页面路由检查：

```powershell
$r = Invoke-WebRequest -UseBasicParsing -Uri "https://api.glart.cn/profit-managed" -TimeoutSec 30
"Status=$($r.StatusCode) Length=$($r.Content.Length)"
```

API/SSE 路径检查时优先看：

- Caddy 是否仍对 `/v1/*` 等 relay 路径禁用缓冲。
- EdgeOne 是否没有对 API 路径启用缓存、页面优化、压缩或可能影响 SSE 的智能处理。
- new-api 日志中的 `stream_status` / `/api/stream/diagnostics`。

## sidecar 入口和权限

当前 bridge 路径：

```text
/gl          -> GPT-Load，root 登录后访问
/cpa         -> CPA Manager Plus，root 登录后访问
/cpa-native  -> CLIProxyAPI 原生管理页，root 登录后访问
```

顶部导航目前对 root 显示：

```text
GPT-Load        /gl
CPA Manager Plus /cpa
```

`/cpa-native` 仍可用于直接进入 CLIProxyAPI 原生页，但当前主入口已换成 CPA Manager Plus。

这些入口由 `middleware.RootSessionAuth()` 保护。不要直接把 GPT-Load、CLIProxyAPI、CPA Manager Plus 管理端口暴露公网。

## 其他项目如何使用 new-api

外部项目作为 API 客户端调用：

```text
Base URL: https://api.glart.cn/v1
Auth: Authorization: Bearer <new-api 中创建的 API key>
Format: OpenAI-compatible
```

示例：

```bash
curl https://api.glart.cn/v1/chat/completions \
  -H "Authorization: Bearer <NEW_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.5",
    "messages": [{"role": "user", "content": "hello"}],
    "stream": false
  }'
```

不要把真实 API key 写入仓库或文档。API key 应在 new-api 面板中创建、轮换和禁用。

如果新项目部署在同一个 Docker stack 内，建议加入同一个内网：

```yaml
services:
  your-service:
    image: your/image:latest
    restart: unless-stopped
    environment:
      NEW_API_BASE_URL: http://new-api:3000/v1
      NEW_API_KEY: ${YOUR_SERVICE_NEW_API_KEY}
    expose:
      - "8080"
    networks:
      - glart-internal

networks:
  glart-internal:
    external: true
    name: glart-internal
```

内网服务调用 new-api 时可用：

```text
http://new-api:3000/v1
```

公网或其他服务器调用时用：

```text
https://api.glart.cn/v1
```

原则：

- 用户、密钥、计费、日志、渠道、收益和风控统一走 new-api。
- 新项目不要绕过 new-api 直接调用 sidecar，除非只是内部管理工具。
- 新项目需要可视化管理端时，优先做 new-api root-only bridge，不直接开放管理端口。
- 新项目如果要成为 new-api 上游，优先提供 OpenAI-compatible `/v1` 接口，然后在 new-api 渠道里配置内部地址。

## 如何部署新的相关 sidecar

推荐步骤：

1. 在 `deploy/glart-stack/compose.yml` 新增服务。
2. 服务只加入 `glart-internal`。
3. 只写 `expose`，不要写公网 `ports`。
4. 数据放到 `deploy/glart-stack/runtime/<service-name>`。
5. 密钥放到 `deploy/glart-stack/.env`，不要提交。
6. 如果需要从 new-api 页面进入，新增 root-only bridge：
   - `controller/sidecar_proxy.go`
   - `router/web-router.go`
   - 前端顶部导航或侧边栏入口
7. 如果服务需要被 new-api 作为上游使用，在 new-api 渠道中配置内网地址，例如 `http://your-service:<port>/v1`。
8. 更新 `deploy/glart-stack/scripts/update-glart-stack.sh`，让该服务可以单独 update/rollback/smoke。

新增服务时不要改 protected project identity，包括 new-api、QuantumNous、包名、许可证、上游归属等。

## 当前自定义功能边界

当前分支包含这些自定义层，更新上游时要保留：

- 组合 updater 和按组件更新/回滚。
- GPT-Load、CLIProxyAPI、CPA Manager Plus 内网 sidecar。
- new-api root-only sidecar bridge。
- GPT-Load/CPA Manager Plus 顶部入口。
- 收益托管侧边栏入口和 Overview 概览。
- 收益中心、压缩、成本档案、输出策略、风险记录、响应缓存等 V2 能力。
- SSE 诊断和 stream 状态增强。

上游合并或一键更新时不要直接删除这些自定义层。若必须改核心 relay/计费链路，应保持最小 hook，并跑更新脚本自带测试和 smoke。

## 安全规则

禁止提交或输出：

- `deploy/glart-stack/.env`
- `deploy/glart-stack/runtime/**`
- API key
- OAuth state
- cookies
- subscription URL
- CLIProxyAPI auth files
- GPT-Load auth/encryption key
- CPA Manager Plus admin/management key
- Redis password

排查问题时可以输出字段名、路径名、状态和脱敏后的短 hash，但不要输出真实值。

## 常用命令速查

本地前端验证：

```powershell
cd "D:\shuziren\Glart api\web\default"
.\node_modules\.bin\eslint.exe "src/hooks/use-sidebar-data.ts"
.\node_modules\.bin\tsc.exe -b --pretty false
.\node_modules\.bin\rsbuild.exe build
```

本地后端测试示例：

```powershell
cd "D:\shuziren\Glart api"
go test ./service ./controller ./model ./router ./relay ./pkg/billingexpr ./setting/billing_setting ./pkg/profit ./pkg/promptcompress -count=1
```

服务器状态：

```bash
cd /opt/glart-api/app
docker compose -f deploy/glart-stack/compose.yml ps
docker logs --tail 120 glart-new-api
```

服务器更新：

```bash
cd /opt/glart-api/app
sh deploy/glart-stack/scripts/update-glart-stack.sh update new-api
```

服务器回滚：

```bash
cd /opt/glart-api/app
sh deploy/glart-stack/scripts/update-glart-stack.sh rollback new-api
```

公网状态：

```bash
curl -fsS https://api.glart.cn/api/status
```

