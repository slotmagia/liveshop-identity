# Platform 授权事实一次性迁入 Identity 与 Subscription

> 历史迁移说明：本文记录 Subscription 仍独立部署时的双回执方案。现行运行边界中 Subscription 已是 Identity 内部 capability；新迁移应在 Identity 单一数据库和单一导入账本内完成，不再创建独立 Subscription 回执或网络同步路径。

运行时只有 Identity IAM 主路径和 Subscription 权益事实源，不存在 Platform 回读或双写。Platform 先冻结一个全量 envelope；Identity 导入 IAM 子集并签发 receipt，Subscription 导入 entitlement 子集并签发独立 receipt。Platform 只有持久验证两份都绑定相同 export digest、rowCount 和 importId 的回执后才能删除旧表。

## 前置条件

1. 停止旧 Platform IAM 写入。
2. Platform Registry 已激活全部目标模块。
3. Identity 已完成 migration 004，且 Registry 投影已同步。导入引用的每个权限码必须存在于当前活动投影，否则整个事务回滚。
4. 单独生成 Ed25519 migration receipt key。它不是 Access Identity 或 Module Capability 的运行时密钥。

## 导入

```powershell
/app/identity-authorization-import `
  -dsn "liveshop:***@tcp(identity-db:3306)/liveshop_identity?parseTime=true&loc=UTC" `
  -input /handoff/platform-authorization.json `
  -import-id cutover-2026-08-14 `
  -identity-instance identity-prod-cn `
  -receipt-output /handoff/identity-receipt.json `
  -receipt-key-id identity-migration-2026-01 `
  -receipt-private-key '<base64url-ed25519-seed>'
```

同一 digest 重试返回同一 durable receipt 内容；同一 import ID 使用不同 digest 会被拒绝。Identity 只导入 domain、role、role permission、scope 与 subject grant，明确不写 `identity_entitlement_projection`。旧 entitlement rows 必须由 Subscription importer 从同一个 envelope 导入；Subscription receipt 还携带 `targetImportedRowCount` 和 `targetProjectionDigest`。旧 v1 department IAM 或 delegation 表非空时失败关闭，因为它们无法无损映射到最终领域模型。

把 Identity 与 Subscription 两份 receipt、对应公钥、target instance/schema 交给 Platform `authorizationexport -finalize`。任一回执缺失、签名错误、target 不符或未绑定同一 export 时不得删除源表。

## 全新本地环境

全新环境没有旧 Platform IAM，不应伪造空导出。等 Identity Registry 投影 ready 后运行固定、显式权限清单：

```powershell
/app/identity-authorization-import `
  -dsn "liveshop:liveshop-local@tcp(identity-db:3306)/liveshop_identity?parseTime=true&loc=UTC" `
  -bootstrap-local `
  -bootstrap-id liveshop-local-v1
```

该命令只建立显式平台角色/grant 与 merchant 授权域，merchant entitlement revision 初始为零。商户 `2001` 的权限必须先由 Subscription `permissionadmin` 写入唯一事实源，再由 Identity 同步器投影；投影缺失或超过独立 max staleness 时 readiness 和授权签发失败关闭。
