# customer_service

负责外部及直播渠道客服账号、店铺分配和人员操作范围。客服账号不是登录凭据。

当前 Admin 主契约位于 `/admin/identity/customer-accounts`：列表查询的 `merchant_id`/`shop_id` 可空（表示全部），写入必须携带明确 `merchant_id + shop_id`。服务端从 `identity_shop` 解析范围，禁止浏览器拼接四元组。写入通过 Module Capability 当前授权复核、乐观版本和命令幂等账本；认证级 step-up 仍由 Identity 会话能力统一建设，不能在本 capability 自造 token 或 header。
