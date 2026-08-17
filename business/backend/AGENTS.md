# Identity 后端规则

开工前先读仓库根 [`docs/开发规范.md`](../../../docs/开发规范.md)，以及 [`docs/文档目录.md`](./docs/文档目录.md) 与 [`docs/模块开发规范.md`](./docs/模块开发规范.md)。下面是不看手册也不能违反的部分。

## 顺序

1. 先跑 `tools/verify-domain.ps1`。它红着就说明业务事实还没定，**不要开始实现**。
2. 先改机器契约（`module.json` operation、Proto、migration），再写实现。
3. 自内向外：`capability/<name>/model` → `capability/<name>` → `data/<name>` → `application/<surface>` → `app`；跨 capability 编排才进入模块级 `biz`。

## 不可违反

- `capability/<name>/model` 拥有该边界的事实与不变量，`capability/<name>` 拥有用例和仓储端口；模块级 `biz` 只做跨 capability 规则。内层都不依赖框架，`data/<name>` 实现端口。越界会被 `cmd/archcheck` 在编译期拦下。
- surface 之间禁止相互导入。字段一样的契约也要各写一份。
- `module.json` 声明的 `authentication` 与 `requiredPermissions`，必须在 `router` 上挂中间件真正执行。声明而不执行是假边界。
- 每个能力挂自己的 `RequirePermission`，不要在 surface 顶层挂一个了事。
- 只读 `-config` 的一份完整 YAML；没有环境变量、没有代码默认值、缺配置就启动失败。
- 显式注入，禁止全局访问器和 `init()` 注册。
- 所有 I/O 传递请求 context；写入校验版本或旧状态；可重试命令有稳定幂等键。
- 数据库和关键事件用同一事务的 Outbox，消费者用 Inbox 去重。
- Proto 与生成产物不可手改，migration 只追加。
- 迁移写代码前必须在领域文档中建立旧字段与目标字段的语义对照；只允许一对一拼写规范化，禁止 `commercial_id -> merchant_id`、`shop_id -> department_id` 或其他近似映射。
- 旧业务维度、状态机和可观察行为必须由契约、仓储及对照测试显式保留；通用 claims 缺字段时演进主契约，不得在 data/controller 层静默改义。
- 认证、凭据、会话、商户、店铺、员工和组织结构属于同一 Identity 数据库；需要原子保持的一致性必须由一个仓储事务方法完成。
- 套餐权益、客户、配送/物流/售后/投诉、游客风险、客服账号和商户能力治理同属 Identity 数据库，但必须按 capability 表族和仓储端口隔离；跨 capability 写入由模块级用例显式声明同库事务。
- 通知 Provider、短信/邮件模板、通知事件和投递证据属于 Platform；Identity auth 只拥有验证码挑战与核验，通过可靠契约请求投递。
- Platform 只能通过公开契约读取主体/组织上下文或消费有版本的 Outbox 事件；不得从 Platform 数据库反向编辑 Identity 事实。
- Access Identity 与 contribution-scoped Module Capability 均由 Identity 签发，但使用独立密钥和受众；业务模块只能验签，禁止自签。

## 提交前

`tools/verify-fast.ps1` 必须在**干净工作树上第一次就通过**。新能力必须同时有放行用例和拒绝用例，样板见 `internal/identity/app/server_test.go`。改 Manifest / `g.Meta` / contribution 后按仓库根 `docs/命名规范检查.md` 评审。
