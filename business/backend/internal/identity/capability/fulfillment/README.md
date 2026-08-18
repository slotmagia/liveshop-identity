# fulfillment

负责配送规则、物流、售后与投诉。投诉工单绑定 `merchant_id + shop_id`；`target_id` 是 Trade/Live/Catalog 引用，不是外键。旧 revision 不得覆盖新审核。

本 capability 不保存退款结果、库存回补或客服聊天。对 Trade 退款只通过后续公开幂等命令，本切片不发 Outbox。

领域契约先于实现，见：

- [`docs/domain/complaints/事实.md`](../../../../docs/domain/complaints/事实.md)
- [`docs/domain/complaints/不变量.md`](../../../../docs/domain/complaints/不变量.md)
- [`docs/domain/complaints/状态机.md`](../../../../docs/domain/complaints/状态机.md)
- [`docs/domain/complaints/事务.md`](../../../../docs/domain/complaints/事务.md)
- [`docs/domain/complaints/外部契约.md`](../../../../docs/domain/complaints/外部契约.md)

商户后台「投诉管理」本切片提供当前店铺列表、详情与审核。Shop 提交投诉不在本边界内。

商户后台「售后工单」本切片提供当前店铺列表、详情、审核与确认退货。Shop 申请、Trade 退款回写和 Catalog 回补不在本边界内。

- [`docs/domain/aftersales/事实.md`](../../../../docs/domain/aftersales/事实.md)
- [`docs/domain/aftersales/不变量.md`](../../../../docs/domain/aftersales/不变量.md)
- [`docs/domain/aftersales/状态机.md`](../../../../docs/domain/aftersales/状态机.md)
- [`docs/domain/aftersales/事务.md`](../../../../docs/domain/aftersales/事务.md)
- [`docs/domain/aftersales/外部契约.md`](../../../../docs/domain/aftersales/外部契约.md)

商户后台「发货/配送」本切片提供当前店铺配送规则与发货预设。物流发货单与轨迹见「物流发货」。

- [`docs/domain/shipping-delivery/事实.md`](../../../../docs/domain/shipping-delivery/事实.md)
- [`docs/domain/shipping-delivery/不变量.md`](../../../../docs/domain/shipping-delivery/不变量.md)
- [`docs/domain/shipping-delivery/状态机.md`](../../../../docs/domain/shipping-delivery/状态机.md)
- [`docs/domain/shipping-delivery/事务.md`](../../../../docs/domain/shipping-delivery/事务.md)
- [`docs/domain/shipping-delivery/外部契约.md`](../../../../docs/domain/shipping-delivery/外部契约.md)

商户后台「物流发货」本切片提供当前店铺发货单列表、详情、发货、追加轨迹与确认收货。Shop 查询/确认和 Trade 订单履约轴不在本边界内。

- [`docs/domain/shipments/事实.md`](../../../../docs/domain/shipments/事实.md)
- [`docs/domain/shipments/不变量.md`](../../../../docs/domain/shipments/不变量.md)
- [`docs/domain/shipments/状态机.md`](../../../../docs/domain/shipments/状态机.md)
- [`docs/domain/shipments/事务.md`](../../../../docs/domain/shipments/事务.md)
- [`docs/domain/shipments/外部契约.md`](../../../../docs/domain/shipments/外部契约.md)

- `model/`：该 capability 的事实、不变量、状态机和领域错误。
- `repository.go`：按聚合或事务能力声明仓储端口与用例。
- `../../data/fulfillment/`：生产仓储适配器。

本目录不是独立服务；公开 HTTP/gRPC/事件契约仍由 identity 模块统一发布。
