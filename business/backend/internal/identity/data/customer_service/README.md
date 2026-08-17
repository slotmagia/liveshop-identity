# customer_service data adapter

实现 `identity_customer_service_account` 与 `identity_customer_service_command` 的 MySQL 适配器。商户/店铺范围校验、条件写入和命令结果在同一 `REPEATABLE READ` 事务内完成；不提供内存 fallback。
