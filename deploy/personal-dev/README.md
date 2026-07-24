# MeiMei API Personal Dev

本目录提供 fpsmeimei 可选的个人 VPS Hybrid 开发方式，不替代仓库根目录已有的完整本地 Docker Compose。没有 Personal Dev VPS 的贡献者继续使用标准本地模式，不需要配置 Tailscale、SSH 或云端资源。

## 两条开发路线

1. **标准本地模式**：Go API、PostgreSQL、Redis 和前端均在开发者本机运行，适合 alleyaaa 等没有 Personal Dev VPS 的贡献者；入口仍是仓库原有的 `make dev-*` 和根目录 Compose。
2. **fpsmeimei Hybrid 模式**：本机 Compose project 同时运行 `web-dev` 与 `tunnel`；个人 VPS 独立运行 Go API、PostgreSQL 和 Valkey。本地不重复运行后端、数据库或缓存。

Hybrid 模式的职责如下：

```text
OrbStack / Docker Desktop: meimei-api-personal-web
  web-dev (127.0.0.1:3002)
       │
       ▼
  tunnel (autossh, 127.0.0.1:3310)
       │ Tailscale + restricted SSH key
       ▼
Personal Dev VPS: 127.0.0.1:3100
  Go API + isolated PostgreSQL + isolated Valkey
```

`tunnel` 使用 `autossh` 和 `restart: unless-stopped`。网络切换、VPS 短暂不可达或 SSH 子进程退出后会自动重连；手动停止 Compose project 后不会自行启动。

## 首台电脑的一次性配置

1. 安装并登录 Tailscale，确认可通过 MagicDNS SSH 到 Personal Dev VPS。
2. 执行 `make personal-dev-init`，检查被忽略的 `.env.personal-dev` 和本地 runtime env。
3. 执行 `make personal-dev-tunnel-bootstrap`。该命令会在 `.personal-dev/` 生成本机专用 tunnel key、固定 VPS host key，并把公钥安装到 VPS。
4. 执行 `make personal-dev-up`，首次同步/启动远端 backend，并构建本地 `web-dev + tunnel` project。
5. 确认 OrbStack 中 `meimei-api-personal-web` 下两个服务均为 healthy，访问 `http://127.0.0.1:3002`。

VPS `authorized_keys` 中的 tunnel key 被限制为只可转发 `127.0.0.1:3100`，禁止 shell、PTY、agent/X11 forwarding 和其他目标端口。Compose 只挂载 `.personal-dev/` 中的专用 key，不挂载具有完整权限的管理 key。

## 日常可视化操作

首次构建完成后，日常不需要命令行：

- 在 OrbStack 展开 `meimei-api-personal-web`，项目级点击启动，会同时启动 `tunnel` 和 `web-dev`。
- 项目级点击停止，会同时停止两个服务；不会停止 VPS backend，也不会删除 volume。
- 不要只启动 `web-dev`，它依赖 healthy 的 `tunnel`。
- 只有 Go 后端代码变化时才运行 `make personal-dev-rebuild`；前端源码由 bind mount 热更新。

维护命令仍保留给 Codex、自动化和故障排查：

```bash
make personal-dev-web-up
make personal-dev-web-status
make personal-dev-web-logs
make personal-dev-doctor
make personal-dev-status
```

`make personal-dev-down` 会同时停止本地 Hybrid project 和远端 backend；日常只想关闭本机前端时，应使用 OrbStack 的项目停止按钮。

## 第二台 Windows 电脑

推荐使用 WSL2 Ubuntu + Docker Desktop WSL integration，并把仓库放在 WSL 文件系统（例如 `~/Projects/goodbyeri-studio/meimei-api`），不要放在 `/mnt/c`，以保证 bind mount 与热更新性能。

一次性步骤：

1. Windows 登录同一 Tailscale tailnet，确认 `tailscale ping dev-fpsmeimei.taildbda00.ts.net` 成功。
2. 在 WSL 中准备 Node、Go、Git、SSH 和可访问 Docker Desktop 的 `docker compose`。
3. 在该电脑单独配置 `.env.personal-dev` 与管理 SSH key 路径；不要复制另一台电脑的 `.personal-dev/tunnel_ed25519`。
4. 运行 `make personal-dev-tunnel-bootstrap`，为 Windows 主机生成独立受限 key。
5. 运行 `make personal-dev-web-up` 完成首次镜像构建。之后只在 Docker Desktop 的 `meimei-api-personal-web` 项目上点击启动/停止。

首次配置时应先验证容器能通过 Tailscale 到达 VPS 的 `100.x.x.x:22`。如果 Windows 宿主机能连接但容器不能连接，应检查 Docker Desktop/WSL 的 Tailscale 路由；不要为此开放 VPS 公网 SSH。

两台电脑可以同时运行各自的本地 `web-dev + tunnel`，因为端口位于不同宿主机。不要让两台电脑同时执行 `personal-dev-sync` 或 `personal-dev-rebuild`，它们共享同一个远端 source/backend，后执行者会覆盖远端开发源码。

每个 VPS 项目必须使用独立的远端目录、Compose project、network、volumes、数据库、缓存、端口和 runtime env。不得复用其他项目或 production 的数据与 Secret。
