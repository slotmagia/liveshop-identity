# Identity 工程规则

- 本模块线协议在兄弟仓 `liveshop-protocol/identity/`（独立 Go module）。本目录 `business` 只放实现与前端贡献。`business` 依赖该 protocol 子目录，反向依赖禁止。
- 本仓库是 Identity 发布模块，拥有 `identity/subscription/customer/fulfillment/risk/customer_service/merchant_governance` capability 的事实、同一个数据库和公开契约；`protocol` 只放这些本模块 capability 的协议，禁止放入其他发布模块的 Proto。
- 开发前读取仓库根 `docs/开发规范.md`、`backend/AGENTS.md`、`backend/docs/文档目录.md` 和 `backend/docs/domain/`。分层总册就是 `docs/开发规范.md`；本仓库操作手册是 `backend/docs/模块开发规范.md`。
- 依赖方向固定为 `application/<surface>` → `capability/<name>`/`biz` → 仓储端口 ← `data/<name>`。目录结构与分层依赖的唯一事实源是本模块 `backend/internal/identity/capability/README.md`。
- `application` 下只允许 `admin`/`merch`/`shop`/`live`，且必须与本仓库的 `frontend-<surface>` 及 `module.json` 的 contribution 一一对应；没有页面贡献就不建目录。骨架先生成后端 surface 与对应的 `httpRoutes`，`contributions` 留空——**加 `frontend-<surface>` 时必须同时补上它的 contribution**，否则 surface 是有后端无入口的悬空目录，应当删掉。
- `module.json` 每个 operation 声明的 `authentication` 与 `requiredPermissions`，必须在对应 `router` 上真正挂了中间件执行。清单是对外契约，声明了却不执行比不声明更危险。
- 改 `module.json`、HTTP `g.Meta`、contribution 或前端 API 路径后，按仓库根 [`docs/命名规范检查.md`](../docs/命名规范检查.md) 做命名检查。
- 禁止导入其他模块 `internal`、源码、DAO 或数据库。
- HTTP 使用 GoFrame，跨模块同步调用使用公开 gRPC，可靠异步流程使用 Outbox/Inbox。
- 修改共享 Manifest、Proto、migration 或组合根前登记单写者。
- 一致性敏感变更必须先明确事实源、不变量、状态机和事务失败窗口。
- Identity 是认证身份与组织目录的唯一事实源：用户主体、凭据、Access Identity、refresh session、商户、店铺、员工、组织单元和成员/店铺关系均在本模块维护。
- Identity 同时拥有角色、角色策略、Subject 授权、数据范围、授权修订、权益投影和有效授权计算；权限定义只消费 Platform Registry 活动目录的版本化只读投影，禁止自创权限码。
- Subscription 权益、Customer、Fulfillment（配送、物流、售后、投诉）、Visitor Risk、Customer Service 和 Merchant Governance 都是 Identity 内部 capability，不得再建立独立服务、数据库、Manifest 或网络投影。
- 短信、邮件、通知事件及投递证据属于 Platform notification；Identity auth 只创建和核验验证码挑战，并以稳定 delivery key 请求 Platform 投递。客服账号、游客风险决策和商户设置治理不得放入 Live 或 Platform。
- Identity 是 contribution-scoped Module Capability 的唯一签发方；Platform 只拥有模块 Registry 与活动 Manifest。`/auth/*` 和 `/runtime/v1/*` 四个显式端点（其中签发资源固定为 `POST /runtime/v1/module-sessions`）是 Gateway 系统边界，不写入 Module Manifest；不得另建兼容签发路由。
- 组织单元与店铺是不同事实；禁止把 `shop_id` 或 `commercial_id` 存入 organization/department 字段。

## 领域语义保持

- 迁移旧项目是架构重构，不是业务重定义；模块按业务能力命名，`admin`、`merch`、`shop`、`live` 只是按需启用的交付 surface。
- 迁移前必须记录旧字段、关联、状态和行为到新模型的逐项对照；只允许一对一拼写规范化，禁止按字段名称猜测或近似映射。
- LiveShop 中 `appid`、`merchant_id`、`commercial_id`、`shop_id` 是不同业务概念；禁止 `commercial_id -> merchant_id`、`shop_id -> department_id`。
- 当前公共 claims 或契约表达不了旧语义时，演进唯一主契约并同步迁移调用方，不在模块内建立隐式转换、fallback 或新旧双路径。
- 旧项目的可观察业务行为必须有语义对照测试；有意变更必须独立形成业务决策和数据迁移方案。
