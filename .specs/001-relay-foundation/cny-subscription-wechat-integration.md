# CNY 余额充值与微信支付闭环

## 决策

- DeepKey 当前账户页只有按量充值档位，没有面向客户的订阅套餐。MeiMei API 使用 `payment_setting.amount_options` 管理 10、20、50、100、200、500、1000、2000 元固定充值档位。
- 客户选择档位后统一调用钱包微信 Native 充值接口。支付成功必须增加 `users.quota`，不得创建 `user_subscriptions`，也不得把充值额度放入订阅账本。
- 微信回调和主动查单都调用幂等钱包入账逻辑，金额、币种、商户身份和用户订单归属必须匹配。
- 已经通过旧套餐通道支付的历史订单，先作废对应有效订阅，再把未消费额度补入钱包；两个动作必须成对核对，避免额度丢失或重复可用。

## 运行时配置

以下 GitHub Secrets/Variables 需要在部署步骤注入运行中的容器。仓库中的 Secrets 不会自动进入已经运行的服务器容器：

- `WECHAT_PAY_APP_ID`
- `WECHAT_PAY_MCH_ID`
- `WECHAT_PAY_MERCHANT_SERIAL_NO`
- `WECHAT_PAY_API_V3_KEY`
- `WECHAT_PAY_NOTIFY_URL`（必须是 `https` 公网地址）
- `WECHAT_PAY_PUBLIC_KEY_ID`
- `WECHAT_PAY_MERCHANT_PRIVATE_KEY_PEM` 或 `WECHAT_PAY_MERCHANT_PRIVATE_KEY_PATH`
- `WECHAT_PAY_PUBLIC_KEY_PEM` 或 `WECHAT_PAY_PUBLIC_KEY_PATH`

PEM Secret 适合 GitHub Actions 或平台环境变量直接注入；文件路径方式适合只读 Secret 卷。应用两种方式都支持，且不记录密钥内容。

生产发布由 `.github/workflows/deploy-production.yml` 在 `production` Environment 中读取上述配置：

- 单行标识和 URL 使用 GitHub Variables，API v3 密钥与 PEM 使用 GitHub Secrets。
- Runner 先验证变量、32 字符 API v3 密钥和 PEM 格式，再通过临时目录上传到生产机。
- 生产机将每套完整配置安装到 `/etc/meimei-api/secrets/wechatpay/releases/<release-id>`；目录同时包含合并后的 `production.env` 和两份 PEM，容器把该版本目录只读挂载到 `/run/secrets/wechatpay`。
- 安装脚本先在同一文件系统内完成复制、权限设置和 PEM 复验，最后通过目录重命名提交版本，不直接覆盖运行中配置；日志不得输出密钥或 PEM 内容。
- 发布清单同时固定镜像 digest、环境文件和 Secret 版本。部署失败或执行 `rollback` 时三者一起恢复到上一份清单，禁止新旧支付配置混用。

## 客户展示规则

- 钱包充值和计费详情中的额度金额统一显示为人民币符号 `¥`，数值按 `1:1` 展示。
- 生产后台 `USDExchangeRate` 设置为 `1`；该运行时选项保存在生产数据库中，不由镜像或部署脚本重置。
- 钱包页只展示固定余额充值档位，使用“余额充值”和“立即充值”表述，不显示套餐、订阅、有效期、套餐额度或购买次数。
- 客户充值只提供微信 Native 扫码支付，不展示余额支付、Stripe、Creem、Waffo Pancake 或 Epay 入口。
- 客户计费详情只查询消费日志（`type=2`）；管理员可查询全部日志类型并按充值、消费、管理、系统、错误、退款和登录类型筛选。

## 验证

- Go：`go test ./model ./controller ./setting`
- 前端：`tsgo -b`、变更文件 `oxlint`、受保护头格式检查、`rsbuild build`
- 部署：`bash -n deploy/production/install-wechat-pay.sh`、`bash -n deploy/production/deploy.sh`、`bash deploy/production/deployment-scripts_test.sh`、production workflow 检查；生产支付密钥版本默认保存在 `$APP_ROOT/.secrets/wechatpay`，无需部署账号获取 `/etc` 写权限
- 支付回调必须在微信商户平台配置为 `WECHAT_PAY_NOTIFY_URL` 的精确地址。
