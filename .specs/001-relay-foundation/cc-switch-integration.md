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
- 深链保留 `resource=provider`、`app`、`name`、`endpoint`、`apiKey`、`homepage` 和 `enabled=true`。
- 客户端名称允许用户修改；空名称提交时回退到当前应用的默认名称。
- 当前只开放 Codex 和 Claude，不展示 Gemini。

## 安全边界

- 真实 API 密钥仅在用户主动点击后写入本机协议 URL，不写入日志、文档或持久化前端状态。
- 服务端仍以用户 ID、Token ID 和请求日志进行计费归属；CC Switch 中的名称和模型配置不作为计费身份。
