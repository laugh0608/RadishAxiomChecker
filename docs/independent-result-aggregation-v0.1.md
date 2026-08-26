# Independent Check 内存结果聚合切片

本文冻结 checker 对已经进入结果层的 keyed-finite-table bundle 所形成的内存级 check、remaining trust、missing artifact、身份边界与四态结果。规范真相源仍是 RadishAxiom 主仓库按摘要锁定的 Independent Check Contract v0.1 与 ADR 0008；本实现不读取 `expected-result.jcs` 来决定实际结果，也不生成 canonical companion document。

入口为 `checkresult.EvaluateVerifiedBundle`。它只接受已经由 `bundle.Verify` 严格解析 request / manifest、验证全部现存 blob 并形成缺失 blob 清单的输入；随后重新打开 Evidence 与 IR，调用 checker 自有 parser、义务重建、状态检查、有限重放、proof support 审计与 production conclusion 重算。Evidence / IR 无法形成结构化 Document 时不会伪造四态结果，仍由更早的调用层保留真实失败。

## Check 物化与身份

结果层恰好形成以下十类 check：

1. `strict-parse`；
2. `identity`；
3. `subject`；
4. `obligation-reconstruction`；
5. `state-support`；
6. `counterexample-replay`；
7. `concrete-check-replay`；
8. `proof-support`；
9. `conclusion-recompute`；
10. `isolation-report`。

每项 definition 只接受 Independent Check Contract v0.1 的闭合 kind、outcome、code 与 ref kind；code 与 ref 由 checker 排序，ref 按 `(kind, digest)` 唯一化。check ID 精确按 `SHA-256("axiom-independent-check-v0.1:check" || NUL || canonical_definition)` 形成，并以主仓冻结的严格 Evidence 拒绝 fixture 黄金 ID 交叉验证。`expected-result.jcs` 不进入生产 API，也不能提供实际 check outcome、trust、missing 或结果 kind。

`passed` 表示当前 profile 声明的检查范围已经重算；`trusted` 表示绑定成立但结果仍保留允许的 trust；`incomplete` 表示缺少 artifact、能力、材料或内部资源；`rejected` 表示已经发现确定矛盾。资源与 artifact 不可得不会降级为默认成功，其他领域错误不会被误写成 incomplete。

## Remaining trust 与 proof 边界

`Document.TrustInventory` 按 ID 返回 Evidence 的全部 trust 条目及类别。当前切片采取保守边界：严格解析、独立语义与具体重放不会自动删除 producer 声明的 trust；所有条目都进入 `remaining_trust`。请求未明确允许的任一类别使 `state-support` 与总结果保持 `incomplete`，不会把 Evidence 本身改写为错误。

`ProofSupportCheck` 的逐项 finding、`IndependentlyVerified`、attestation 与 `MissingProofMaterial` 原样保留在 `Evaluation` 中：

- `certificate-required` 下决定 `satisfied` 的 attestation 仍为 `incomplete`；
- `attestation-allowed` 只形成 `trusted`，并保留精确 `proof-backend` trust；
- 对已经由独立 failure / input / host-output replay 决定的非 `satisfied` production conclusion，未参与该优先分支的 producer proof 缺口仍保留在 proof 审计中，但不会反向把正确报告的 `violated`、`input_rejected` 或 `implementation_inconsistent` 改写为程序结论；
- 当前 kernel / certificate 独立能力仍为空，attestation 永远不会增加 `IndependentlyVerified`。

## Missing artifact 与四态顺位

manifest 列出但目录缺失的 blob 由 `bundle.Verify` 形成排序唯一的 `MissingArtifacts`，现存 blob 仍逐一验证普通文件、声明长度和 SHA-256。缺失 artifact 使 identity check 与总结果为 `incomplete`；digest、长度、symlink、路径或未列出 blob 漂移仍是确定拒绝。`ReadArtifact` 对缺失 blob继续失败关闭，不从网络、缓存或相邻目录补齐。

`Aggregate` 只使用实际 check outcome、missing 与 trust，按唯一顺位形成：

1. 任一 `rejected` check → `rejected`；
2. 否则任一 `incomplete`、missing artifact 或不允许的 trust → `incomplete`；
3. 否则 remaining trust 非空 → `accepted-with-trust`；
4. 否则全部十类必需检查完成 → `accepted`。

非 `accepted` 结果保留决定它的 check ID refs；已有拒绝优先于同时存在的 incomplete、missing 与 trust，但后者仍保留在内存结果中。`accepted` / `accepted-with-trust` 接受的是 Evidence 对自身 production conclusion 的忠实表达，不等于候选程序为 `satisfied`、真实意图正确或形式证明完成。

## Companion 身份边界

内存结果同时绑定：

- Evidence raw content digest 与 document domain digest；
- request raw content digest 与 `axiom-independent-check-v0.1:request` document domain digest；
- 当前 checker source digest、实际调用方提供的 Go toolchain 与实现版本；
- `canonicalization`、`checker-core`、`cryptographic-primitive`、`rule-interpreter` 四个必需 TCB source / version，未来实际使用 certificate checker 时可增加对应条目。

缺少 checker source / toolchain / version、任一必需 TCB 或非拒绝结果所需的 Evidence / request domain identity时，不形成规范内存结果。这里明确绑定的是 source，而不是尚未构建的 checker binary；不得把 source digest 写成 `checker.artifact`。

## 锁定语料与停止线

25 个实际进入结果层且具有独立 expected companion 的场景由真实 bundle / Evidence / IR findings 形成：22 个 `accepted-with-trust`、2 个 `incomplete`、1 个 `rejected`。remaining trust 与 missing artifact 清单逐项匹配版本化锁定记录；正确报告 `violated`、`input_rejected` 与 `implementation_inconsistent` 的场景均可被独立接受。`CHK-PROOF-01` 的 12 项 attestation 在 `certificate-required` 下保持 incomplete，`CHK-PROOF-02` 在 `attestation-allowed` 下保持 trust，独立证明数均为 0。四态优先级、check ID、缺失 TCB / check、不可用 document identity、缺失 artifact、不允许 trust 与 100 次确定性重复均有独立测试。

`chk-digest-01`、`chk-resource-01` 仍在进入此 API 前失败关闭；`chk-process-01` 描述外层进程终止，调用方不得据此制造 checker result。下一切片才能决定这些前置失败如何进入完整 invocation / canonical companion 边界。

本切片不生成 canonical result JCS，不实现 result parser、产品 CLI、checker binary 或 `checker.artifact`，不累计完整跨阶段资源，不实现内部 wall-clock 中断，不检查 minimality，不增加 kernel / certificate 能力，不执行 solver、Node、生产 compiler 或 adapter，也不产生六平台结果。
