# risk

负责游客风险事件、评分、状态与版本化风险决策。风险事实绑定 `merchant_id + shop_id` 与游客标识；旧 revision 不得覆盖新决策。

本 capability 不接收全局埋点，也不保存 Platform telemetry 原始事件。对 Trade 等消费者只发布稳定、版本化的风险决策。

领域契约先于实现，见：

- [`docs/domain/risk-events/事实.md`](../../../../docs/domain/risk-events/事实.md)
- [`docs/domain/risk-events/不变量.md`](../../../../docs/domain/risk-events/不变量.md)
- [`docs/domain/risk-events/状态机.md`](../../../../docs/domain/risk-events/状态机.md)
- [`docs/domain/risk-events/事务.md`](../../../../docs/domain/risk-events/事务.md)
- [`docs/domain/risk-events/外部契约.md`](../../../../docs/domain/risk-events/外部契约.md)

商户后台「风控记录」本切片只提供当前店铺只读列表。Live 行为信号写入与总后台游客风控页不在本边界内。
