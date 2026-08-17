# shop

负责店铺、商业隔离映射、结算币种，以及商户可编辑的店铺客户隐私文档与政策版本。

- `model/`：该 capability 的事实、不变量、状态机和领域错误。
- `repository.go`：按聚合或事务能力声明仓储端口。
- `usecase.go`：声明用例边界和模块内部编排入口。
- `privacy.go`：店铺隐私 GET/保存用例；平台叠加层由 merch 逻辑读取 merchant governance，不进入本仓储。
- `policy.go`：店铺政策列表、保存新版本和发布草稿；平台叠加层仍由 merchant governance 拥有。
- `app.go`：店铺私有应用列表、创建、轮换密钥和启停；平台叠加层仍由 merchant governance 拥有。
- 商户后台店铺生命周期：分页目录、当前会话店铺读取、新建、编辑名称/子域名、启停和关闭；写命令仅 `MERCHANT_OWNER`。
- `../../data/shop/`：生产仓储适配器的目标目录。

本目录不是独立服务；公开 HTTP/gRPC/事件契约仍由 identity 模块统一发布。
