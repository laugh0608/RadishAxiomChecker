# RadishAxiom Independent Checker (Go)

这是 RadishAxiom 独立 checker 的 Go 实现仓库。它与生产 Rust `raxc` 分仓、分依赖图和分发布流水线，输入只来自版本化协议和按摘要锁定的测试制品。

当前实现以下拒绝优先的小切片：

- 不依赖 `encoding/json` 的字节级严格 JSON / RFC 8785 JCS 外壳；
- `axiom-check-request` `0.1` 与 `axiom-check-bundle-manifest` `0.1` 的闭合解析；
- 只读 bundle 布局、普通文件、声明长度、原始 SHA-256、manifest 覆盖和 request 绑定检查；
- 独立导入且由摘要锁定的 RadishAxiom 合同 fixture。
- [`checker.source` v0.1](docs/checker-source-v0.1.md) 的闭合仓库输入集合、canonical manifest、原始字节 SHA-256 复算和失败关闭门禁。
- [Axiom IR v0.1 严格结构与类型良构切片](docs/axiom-ir-structure-v0.1.md)：对已锁定 keyed-finite-table profile 核对 canonical 字节、闭合 tag、全部 definition domain ID、声明 / 主键、18 个 expression op、projection / group 字段覆盖、节点 table type 关系、节点引用 / DAG / 可达性及完整文档 domain digest。
- [Axiom Evidence v0.1 严格结构与身份切片](docs/axiom-evidence-structure-v0.1.md)：对 28 个锁定 bundle 的实际 profile 核对 canonical 字节、闭合 tag、tool / execution / obligation / trust / uncovered definition domain ID、直接引用索引、完整 Evidence document domain digest，以及 subject 的 IR raw content / document domain 双摘要绑定。
- [Axiom Evidence v0.1 obligation completeness 切片](docs/axiom-evidence-obligation-completeness-v0.1.md)：只凭独立解析的 IR、profile、显式 benchmark execution I/O 边界和 trust 条目重建完整 definition / ID 集合，精确拒绝缺失、多余、expectation 与 anchor 漂移；不读取 `axiom-obligation-set` 制品作为真相源。
- [Axiom Evidence v0.1 state / support 切片](docs/axiom-evidence-state-support-v0.1.md)：核对 expectation 与五态矩阵、execution kind / result、精确 tool role、unknown attempt reason、trusted scope、backend attestation 绑定，以及 checked result 的有限 artifact 闭包。
- [Axiom Evidence v0.1 counterexample world / WF 切片](docs/axiom-evidence-counterexample-worlds-v0.1.md)：保留失败见证的受界值、闭合记录、表和 world，以独立 IR 声明核对类型、范围、容量、主键唯一性与规范顺序，并区分 input-conformance 的预期非 WF 输入。

Git commit / tree 不充当 `checker.source`，仓库也不生成 checker binary 或 `checker.artifact`。Axiom IR、Evidence 结构、obligation completeness、state / support 与 counterexample world / WF 闭合通过不等于语义接受：当前实现不求值 `Pre`、不执行 IR、不确认目标义务确已违反，也不重放 kernel rule 或 concrete artifact，不验证 backend attestation 真值，不重算 conclusion，不检查 certificate，不生成四态独立结果或产品 CLI。测试通过仅表示这些实现路径被检查，不构成形式证明或跨平台结论。

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
