# CC Switch 客户端对接

## 目标

- 用户从 API 密钥列表将当前真实密钥导入本机 CC Switch。
- 默认选择 Codex，提供商名称默认为 `meimei-codex`。
- 切换为 Claude 时，提供商名称自动改为 `meimei-claude`。
- 不要求用户选择模型，也不向 CC Switch 深链传递模型字段；密钥所属分组内可用模型由 Relay 统一控制。

## 协议约定

- 使用 `ccswitch://v1/import` 自定义协议直接唤起本机 CC Switch。
- Codex 的 `endpoint` 为 Relay 服务地址加 `/v1`。
- Claude 的 `endpoint` 为 Relay 服务地址。
- Relay 服务地址仅接受 HTTP(S)；构造深链时移除查询、片段和末尾斜杠，已有 `/v1` 时不重复追加。
- 深链保留 `resource=provider`、`app`、`name`、`endpoint`、`apiKey`、`homepage` 和 `enabled=true`。
- 客户端名称允许用户修改；空名称提交时回退到当前应用的默认名称。
- 当前只开放 Codex 和 Claude，不展示 Gemini。
- 发起协议唤起后不把“已发起”当作导入成功；页面保留对话框，提供安装入口和重试操作。

## 模型契约与发布门槛

- 不传递 `model` 仅表示由本机客户端使用自身默认模型，不代表 Relay 会为请求自动选择模型。
- 当前验收版 CC Switch 在 Codex 未传 `model` 时使用 `gpt-5-codex`；Claude 未传模型时由 Claude Code 选择默认模型。
- 最低支持与当前验收版本为 CC Switch `v3.18.0`；生产发布前必须用该版本的真实 Codex/Claude 客户端、真实 Token 分组和真实渠道验证默认模型已被放行或稳定映射。
- CC Switch 或上游客户端变更默认模型时，必须重新执行该 E2E，不得仅依赖 URL 单元测试放行。

## 安全边界

- 真实 API 密钥仅在用户主动点击后写入本机协议 URL，不写入日志、文档或持久化前端状态。
- 协议构造或唤起失败时不记录完整深链或异常对象，避免 `apiKey` 进入前端日志和会话回放。
- 服务端仍以用户 ID、Token ID 和请求日志进行计费归属；CC Switch 中的名称和模型配置不作为计费身份。
