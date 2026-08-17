# Identity 线协议

本目录是 `identity` 对外发布的唯一线协议单元，独立 Go module `github.com/lvtuopen-ai/liveshop-identity/protocol`，与 `../business` 平级。

- 只放本模块拥有的契约：`proto/identity/v1` 与生成客户端 `gen/go/identity/v1`。
- 禁止放入其他模块的 Proto；需要调用别人时依赖对方发布的 protocol 模块。
- 依赖方向单向：`business` 依赖本模块，本模块绝不依赖 `business`。
- 生成文件禁止手改；修改 Proto 后重新生成并运行 buf lint 与相对发布基线的 breaking 检查。
