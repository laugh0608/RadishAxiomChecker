# RadishAxiom Independent Checker (Go)

这是 RadishAxiom 独立 checker 的 Go 实现仓库。它与生产 Rust `raxc` 分仓、分依赖图和分发布流水线，输入只来自版本化协议和按摘要锁定的测试制品。

当前实现以下拒绝优先的小切片：

- 不依赖 `encoding/json` 的字节级严格 JSON / RFC 8785 JCS 外壳；
- `axiom-check-request` `0.1` 与 `axiom-check-bundle-manifest` `0.1` 的闭合解析；
- 只读 bundle 布局、普通文件、声明长度、原始 SHA-256、manifest 覆盖和 request 绑定检查；
- 独立导入且由摘要锁定的 RadishAxiom 合同 fixture。
- [`checker.source` v0.1](docs/checker-source-v0.1.md) 的闭合仓库输入集合、canonical manifest、原始字节 SHA-256 复算和失败关闭门禁。
- [Axiom IR v0.1 严格结构切片](docs/axiom-ir-structure-v0.1.md)：对已锁定 keyed-finite-table profile 核对 canonical 字节、闭合 tag、全部 definition domain ID、声明类型索引、主键良构、节点引用 / DAG / 可达性及完整文档 domain digest。

Git commit / tree 不充当 `checker.source`，仓库也不生成 checker binary 或 `checker.artifact`。Axiom IR 结构与声明良构通过不等于语义接受：当前实现仍不执行表达式、projection / group、节点输入输出关系或首域义务的完整类型 / 语义检查，不解析 Axiom Evidence 语义，不重建 obligation，不检查 certificate，也不生成四态独立结果或产品 CLI。测试通过仅表示这些实现路径被检查，不构成形式证明或跨平台结论。

当前资源实现只收口该 parser 切片实际消费的边界：request/manifest 单文档字节、JSON 容器深度、JSON 成员/元素、严格 JSON token、单 artifact 字节、manifest 唯一 artifact 总字节和流式 SHA-256。`wall-clock` 与逻辑 `working-memory` 字段会被闭合解析，但完整的跨阶段累计计数、内部 `incomplete` 结果形成和外层进程限制仍属于后续切片；这里不会把未实现的计数伪装成已执行。

## 工具链与验证

模块固定 `go 1.26.0` 与 `toolchain go1.26.7`。调用方必须显式提供已验收的 `go1.26.7`，并禁止自动工具链下载：

```sh
GOTOOLCHAIN=local CGO_ENABLED=0 go test ./...
GOTOOLCHAIN=local CGO_ENABLED=0 go vet ./...
```

初始 core 只使用 Go 标准库，因此没有 `go.sum`、vendor 目录、构建时下载或 `cgo`。

源码身份专项门禁为：

```sh
./scripts/check-source-identity.sh
```

有意改变源码输入时运行 `./scripts/update-source-identity.sh` 重放并更新 `source-identity/checker-source-v0.1.jcs`，随后必须审阅 manifest diff。两个脚本都使用本机 `GOTOOLCHAIN=local`，不会自动取得 `go1.26.7`。
