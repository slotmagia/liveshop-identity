# auth

负责认证、凭据、会话、登录验证码挑战/核验与上下文切换。

- `model/`：该 capability 的事实、不变量、状态机和领域错误。
- `repository.go`：按聚合或事务能力声明仓储端口。
- `usecase.go`：声明用例边界和模块内部编排入口。
- `../../data/auth/`：生产仓储适配器的目标目录。
- `../../infra/notification/`：Platform `Dispatch` 客户端；本 capability 只通过 `Notifier` 端口调用，不拥有投递证据。

登录验证码事件 `identity.auth.otp.requested` 由 Shop Manifest 操作 `identity.shop.login.otp.create` 声明。Identity 先提交挑战，再 SYNC 调用 Platform；Platform 不知道验证码是否正确。游客升级为顾客不在本切片。

本目录不是独立服务；公开 HTTP/gRPC/事件契约仍由 identity 模块统一发布。
