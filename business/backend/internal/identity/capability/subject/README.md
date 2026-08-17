# subject

负责全局主体、realm 与旧身份映射。

- `model/`：该 capability 的事实、不变量、状态机和领域错误。
- `repository.go`：按聚合或事务能力声明仓储端口。
- `usecase.go`：声明用例边界和模块内部编排入口。
- `../../data/subject/`：生产仓储适配器的目标目录。

本目录不是独立服务；公开 HTTP/gRPC/事件契约仍由 identity 模块统一发布。
