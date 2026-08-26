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
- [Axiom Evidence v0.1 concrete input / Pre 切片](docs/axiom-evidence-concrete-inputs-v0.1.md)：从 Evidence 与 execution 的精确 `host-input` 边界读取 `axiom-benchmark-data` `0.1`，按 IR 重建完整输入、核对 `WF`、独立求值锁定 assume 子集，并确认 input-conformance 的 `checked` / `failed` 分类和 witness 制品投影。
- [Axiom Evidence v0.1 counterexample target replay 切片](docs/axiom-evidence-counterexample-targets-v0.1.md)：复用同一 concrete value / table / expression 语义，按稳定拓扑顺序解释锁定 5 类 node、18 类 expression 与 2 类 aggregate，核对 proof-failure 的 `WF ∧ Pre`、trace、observed、required field / key、目标违反及 paired-world 公开等价。
- [Axiom Evidence v0.1 concrete output comparison 切片](docs/axiom-evidence-concrete-outputs-v0.1.md)：复用同一 benchmark-data decoder 与有限 IR execution，严格区分 envelope role 和 execution I/O role，独立核对 semantic / host / actual / golden output，并重放 3 个 host-output mismatch entry。
- [Axiom Evidence v0.1 proof support 真值与能力边界切片](docs/axiom-evidence-proof-support-v0.1.md)：重新打开 obligation-set、query、response 与 tool artifact，核对精确 execution / backend / trust 绑定；显式报告空 kernel / certificate 能力、attestation-only、remaining trust 与 missing proof material，不让 `completed`、`unsat` 或 producer tag 自证。
- [Axiom Evidence v0.1 conclusion 确定性重算切片](docs/axiom-evidence-conclusion-v0.1.md)：不读取 producer conclusion、pipeline receipt 或 expected result 作为真相源，按规范优先级从已检查 obligation state 与必需 execution 独立形成 kind / refs，并以 `conclusion-mismatch` 拒绝生产聚合漂移。
- [Independent Check 内存结果聚合切片](docs/independent-result-aggregation-v0.1.md)：将十类真实检查物化为契约 check ID，保守形成 remaining trust 与 missing artifact，并按拒绝、incomplete、允许 trust、无 trust 的唯一顺位形成四态内存结果，同时绑定 Evidence / request 双摘要与 checker source / toolchain / TCB 边界。
- [Independent Check canonical companion 与 invocation failure](docs/canonical-companion-v0.1.md)：对具有显式 checker binary / runtime TCB 身份的内存结果实施唯一 JCS 编码、严格重解析与 result-domain identity 复算，并让外层进程失败只形成绑定 request 的 `not-produced` 记录。

Git commit / tree 不充当 `checker.source`，仓库也不生成 checker binary 或 `checker.artifact`。Axiom IR、Evidence 结构、obligation completeness、state / support、counterexample world / WF、concrete input / Pre、proof-failure target replay、concrete output comparison、proof support 审计、production conclusion 重算、四态内存结果与 canonical codec 闭合通过不等于完整语义接受：有限解释只确认锁定具体 world / artifact、query envelope、status、attestation、生产聚合与 assurance policy 关系，不证明其他输入或 query theorem；当前实现不重放 kernel rule，不把 backend attestation 升级为独立 proof，不检查 minimality，不检查 certificate，也没有实际 checker binary 身份可形成正式运行 companion 或产品 CLI。测试通过仅表示这些实现路径被检查，不构成形式证明或跨平台结论。

当前资源实现收口实际消费的 request / manifest / artifact 字节、JSON 深度 / item / step、bundle 总字节、流式 SHA-256、concrete input / output、proof support artifact、conclusion ref 与 DAG table 的确定性逻辑字节，以及 assume、node、row、lookup、aggregate、contract replay、output comparison、proof envelope inspection 与 conclusion traversal step；明确的局部资源失败可进入内存 `incomplete`。`wall-clock` 尚未形成 checker 内部中断机制，完整跨阶段累计计数和外层进程限制仍属于后续切片；这里不会把未实现的计数伪装成已执行。

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
