# Verification

## 验证矩阵

| 日期 | 范围 | 命令/方式 | 结果 | 备注 |
|---|---|---|---|---|
| 2026-07-12 | 上游锁定 | `git describe --tags --exact-match HEAD`; `git rev-parse HEAD` | 通过 | `v1.0.0-rc.21` / `bde9b2f44887d34ec54799ae191d50f97914359e` |
| 2026-07-12 | License/署名 | 静态核对 `LICENSE`、`NOTICE`、README Section 7 和 AGENTS protected metadata | 通过 | 只证明文件保留，不代表法律审查完成 |
| 2026-07-12 | BlackRain foundation | `git diff --cached --check`; spec 文件静态核对 | 通过 | 不代表服务可运行 |
| 2026-07-12 | 上游 Go tests | `go test ./...` | 部分通过 | 各后端包通过；根包因未生成 `web/classic/dist`，`go:embed` 匹配失败 |
| YYYY-MM-DD | 上游完整 build/test | frontend build + Go + database matrix | 未跑 | 初始化后下一阶段 |
| YYYY-MM-DD | Cloud/Relay contract | token + usage integration tests | 未跑 | 尚无 BlackRain 实现 |
| YYYY-MM-DD | WORK/CODE E2E | 真实授权模型渠道 | 未跑 | 发布门槛 |

## 已验证

- New API release 源码、历史、AGPLv3、NOTICE 和上游开发规则已导入。
- `origin` 与 `upstream` 已分离，当前基线可精确定位。
- 已运行 `go test ./...`；除依赖前端构建产物的根包外，测试包均通过。

## 未验证风险

- 尚未生成 `web/classic/dist`，因此根包的 `go:embed` 校验未通过；尚未运行完整前后端构建矩阵。
- 尚未运行数据库 migration 或 Docker smoke。
- 尚未配置任何生产数据库、Redis、域名、Secret、模型渠道、备份或监控。
- Cloud 企业客户、scoped token、usage 对账和 BlackRain 双引擎 E2E 尚未实现。
- AGPL、模型厂商转售条款、支付、税务、备案、内容安全和日志留存尚未正式审查。
