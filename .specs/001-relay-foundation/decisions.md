# Decisions

## 2026-07-12：Relay 基于锁定 New API release

- 决策：初始 `main` 基于 New API `v1.0.0-rc.21`，commit `bde9b2f44887d34ec54799ae191d50f97914359e`；保留 `upstream` remote，只按评估过的 tag/commit 升级。
- 原因：直接获得渠道、协议、token、计量和运营能力，同时避免跟随每日 HEAD 导致不可复现发布。
- 替代方案：只引用官方 Docker 镜像；从零自研；跟随上游 main。
- 后续复查条件：每次升级必须重跑 License、migration、billing、protocol 和 BlackRain 三方 E2E。

## 2026-07-12：接受并履行 AGPL 与 Section 7

- 决策：Relay 公开运营并提供对应修改源码，保留 New API / QuantumNous 署名、原项目链接、LICENSE、NOTICE 和受保护元数据。
- 原因：Relay 本身就是公开独立产品，AGPL 不是该项目的商业阻塞；私有 Desktop/Cloud 仅通过 HTTP 合同使用服务。
- 替代方案：购买商业许可；使用 Apache/MIT 网关；Relay 私有闭源。
- 后续复查条件：上线前法律复核实际部署、修改和源码提供方式；License 变化时停止升级并重审。

## 2026-07-12：Cloud 是 Relay 企业客户

- 决策：Cloud 使用管理面企业凭据申请 scoped model token；Desktop 使用数据面 token；Relay 不直接依赖 Supabase 或 Cloud 私有表。
- 原因：三个产品独立运营、独立故障域和独立账本，避免把 BlackRain 身份模型写死在公开中转站。
- 替代方案：Relay 直接验证 Supabase JWT；共享数据库；Cloud 代理模型正文。
- 后续复查条件：实现前冻结管理 API、token、usage 和撤销合同。
