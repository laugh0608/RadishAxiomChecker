# RadishAxiom Independent Checker (Go)

这是 RadishAxiom 独立 checker 的 Go 实现仓库。它与生产 Rust `raxc` 分仓、分依赖图和分发布流水线，输入只来自版本化协议和按摘要锁定的测试制品。

当前只实现第一个拒绝优先的小切片：

- 不依赖 `encoding/json` 的字节级严格 JSON / RFC 8785 JCS 外壳；
- `axiom-check-request` `0.1` 与 `axiom-check-bundle-manifest` `0.1` 的闭合解析；
- 只读 bundle 布局、普通文件、声明长度、原始 SHA-256、manifest 覆盖和 request 绑定检查；
- 独立导入且由摘要锁定的 RadishAxiom 合同 fixture。

本切片不解析 Axiom IR 或 Axiom Evidence 语义，不重建 obligation，不检查 certificate，也不生成四态独立结果。测试通过仅表示这些实现路径被检查，不构成形式证明或跨平台结论。

当前资源实现只收口该 parser 切片实际消费的边界：request/manifest 单文档字节、JSON 容器深度、JSON 成员/元素、严格 JSON token、单 artifact 字节、manifest 唯一 artifact 总字节和流式 SHA-256。`wall-clock` 与逻辑 `working-memory` 字段会被闭合解析，但完整的跨阶段累计计数、内部 `incomplete` 结果形成和外层进程限制仍属于后续切片；这里不会把未实现的计数伪装成已执行。

## 工具链与验证

模块固定 `go 1.26.0` 与 `toolchain go1.26.7`。调用方必须显式提供已验收的 `go1.26.7`，并禁止自动工具链下载：

```sh
GOTOOLCHAIN=local CGO_ENABLED=0 go test ./...
GOTOOLCHAIN=local CGO_ENABLED=0 go vet ./...
```

初始 core 只使用 Go 标准库，因此没有 `go.sum`、vendor 目录、构建时下载或 `cgo`。
