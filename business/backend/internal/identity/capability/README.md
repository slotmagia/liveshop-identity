# Identity capability 目录

本目录只定义 `identity` 发布模块内部的业务边界。所有 capability 共用本模块的进程、数据库、Manifest 和发布生命周期；禁止在子目录创建独立 `go.mod`、`module.json`、数据库连接配置或服务入口。

| Capability | 责任 |
|---|---|
| `auth` | 认证、凭据、会话、验证码挑战/核验与上下文切换 |
| `subject` | 全局主体、realm 与旧身份映射 |
| `merchant` | 商户主体与经营关系 |
| `shop` | 店铺、商业隔离映射、结算币种，以及商户可编辑的隐私文档、政策版本与私有应用凭据 |
| `workforce` | 平台操作员、商户员工、主播与组织/店铺范围 |
| `authorization` | 角色、策略、grant、数据范围与有效授权 |
| `subscription` | 套餐、配额、商户权限权益、revision 与命令账本 |
| `customer` | 客户档案、地址、标签、备注与收藏 |
| `fulfillment` | 配送规则、物流、售后与投诉 |
| `risk` | 游客风险事件、状态与版本化风险决策 |
| `customer_service` | 外部/直播客服账号、渠道分配和操作范围 |
| `merchant_governance` | 商户能力摘要、设置治理状态和幂等干预命令 |

短信、邮件、通知事件与投递证据归 Platform notification；对象存储和直播 Provider 也归 Platform。Identity 不建立这些能力的本地副本。

跨 capability 写入必须由模块级用例明确事务边界；任何 capability 不得直接修改另一个 capability 的表。
