# Identity

`identity` 是 LiveShop 的认证身份与组织目录模块。它统一服务按项目启用的 admin、merch、shop、live surface，不按「商户端」或「总后台端」拆分事实源。

Identity 拥有主体、凭据、Access Identity、登录/refresh session、商户、店铺、员工、组织结构、角色、授权策略、有效授权与 Module Capability；权限定义只来自 Platform Registry 活动目录投影。

开始开发前完成 `backend/docs/domain/` 下的事实、不变量、状态机、事务和外部契约，并将示例 API、Proto 和 migration 替换为真实业务定义。实现规范见 [`backend/docs/模块开发规范.md`](./backend/docs/模块开发规范.md)。

启动前必须填 `backend/configs/module.yaml`：`database.url`、Identity 自有的 `module_capability.private_key` 与 Platform Registry mTLS 客户端配置都没有默认值，缺任一项进程直接拒绝启动。Module Capability 的验签公钥只从同一私钥派生，不存在 Platform/Gateway 第二签发源。

每加一个 `frontend-<surface>`，同时在 `module.json` 补上它的 contribution；骨架只生成了后端 surface 和 `httpRoutes`。

```powershell
./backend/tools/verify-domain.ps1   # 业务契约是否已填写（实现前必须过）
./backend/tools/verify-fast.ps1     # fmt / vet / archcheck / test
```

本地容器启动方式见 [`backend/deploy/README.md`](./backend/deploy/README.md)。当前主实现已交付 `/auth/*`、四个精确 runtime 资源、Identity Directory gRPC、组织/IAM、用户与会话管理、Registry/Subscription 投影和 Module Capability 签发；旧 Platform 授权数据只允许通过有双目标签名回执的一次性交接迁移，不能形成运行时兼容路径。
