# Axiom Evidence v0.1 production conclusion 确定性重算切片

本文冻结独立 checker 对锁定 keyed-finite-table Evidence profile 中生产 `conclusion` 的重算与精确比对边界。输入必须先通过 bundle、Axiom IR、Axiom Evidence 结构 / 身份、obligation completeness、state / support、counterexample world / `WF`、concrete input / `Pre`、proof-failure target replay、concrete output comparison 与 proof support 审计。生产 conclusion、pipeline receipt、expected independent result 和 proof support 的 `error == nil` 都不是本切片的聚合真相源。

入口为 `Document.VerifyConclusion`。返回的 `ConclusionCheck` 只包含 checker 自身重算的 production conclusion kind 与带 `obligation` / `execution` 域的决定性 refs；它不是 `accepted`、`accepted-with-trust`、`incomplete` 或 `rejected` 独立结果。

## 真相源与优先级

checker 只从已经保留并由前置层检查的 obligation definition、expectation、五态 result 和必需 execution 完成状态形成结论。producer 的 kind / refs 只在独立形成结果后用于精确比较；pipeline receipt、bundle expected result、生产 obligation-set 与 proof finding 都不能反向决定 production conclusion。

聚合优先级固定为：

1. 更早的 subject 检查已经确定结构拒绝时为 `structure_rejected`；当前 `axiom-ir` Document 只有在结构解析成功后才能进入 `VerifyConclusion`，因此该分支在内部规则中闭合，但不会用无法形成有效 Document 的前置拒绝伪造锁定正例。
2. 任一 `input-conformance` 为 `failed` 时为 `input_rejected`。
3. 全部非 host / output 核心义务满足各自 expectation，且任一 `host-conformance` / `output-conformance` 为 `failed` 时为 `implementation_inconsistent`。
4. 其余任一 `prove` 或 `check` 义务为 `failed` 时为 `violated`。
5. 任一义务未达到 `prove → proved`、`check → checked`、`trust → trusted`，或这些已完成状态绑定的必需 execution 未完成时为 `inconclusive`。
6. 只有全部 expectation 满足、必需 execution 完成且没有上述阻断项时才为 `satisfied`。

已经重放的 failure 优先于并存 unknown；unknown obligation 与 attempt 仍保留在原 Evidence。未被当前 obligation result 要求的历史失败 execution 不参与结论，避免把保留的旧 attempt 误当成当前阻断项。

## 决定性 refs 与精确比较

checker 先为每个 ref 解析独立域，再按 `(kind, value)` 形成稳定集合：

- `input_rejected` 只引用全部 failed input-conformance obligation；
- `implementation_inconsistent` 只引用全部 failed host / output obligation；
- `violated` 引用该优先级下全部 failed obligation；
- `inconclusive` 引用全部阻断 obligation；若一个 otherwise-complete 状态绑定的必需 execution 未完成，则引用该 execution；
- `satisfied` 的 refs 必须为空；
- `structure_rejected` 的 ref 只能由更早 subject 检查提供，不能从 producer conclusion 抄录。

Evidence parser 继续先按公共格式要求拒绝悬空、歧义、重复或非规范 ID 顺序。`VerifyConclusion` 随后比较独立 kind、ref 基数、ref 域、值与稳定顺序；任一遗漏、多余、错类、错值、优先级或内部顺序漂移均以 Independent Check Contract 已冻结的 `conclusion-mismatch` 失败关闭，不降级为 warning 或 fallback。

## Proof、trust 与独立结果隔离

production `satisfied` 只说明 producer 的 obligation 五态满足 Evidence v0.1 聚合规则，不表示独立 proof 已成立、remaining trust 为空或真实意图得到证明。`VerifyConclusion` 不读取 `ProofSupportCheck` 来改写 production kind，也不会删除其中的 `MissingProofMaterial` / `RemainingTrust`。

锁定 24 个前置完整场景中仍有 213 个 producer `proved` claim、0 个独立 proof、65 个 attestation 和 160 个 missing proof material；其中 producer conclusion 为 `satisfied` 的场景仍必须在独立结果中保留这些事实。后续已经由 [Independent Check 内存结果聚合切片](independent-result-aggregation-v0.1.md) 综合 conclusion 自洽性、proof 能力、remaining trust、缺失 artifact 与 request assurance policy；该层仍不会让 production conclusion 决定独立结果。

## 锁定场景与负例

24 个前置完整场景全部由 checker 自身重算并与 producer 精确匹配，分布为：

- 7 个 `satisfied`；
- 4 个 `input_rejected`；
- 8 个 `violated`；
- 4 个 `inconclusive`；
- 1 个 `implementation_inconsistent`。

`structure_rejected` 保持规则闭合但不计入有效 Document 正例。合成负例覆盖 kind、ref 缺失 / 多余、ref 域 / 值、顺序、优先级和必需 execution 完成状态漂移；failure 与 unknown 并存仍稳定选择 failure。相同输入重复 100 次产生相同 kind 与 refs。

## 资源与停止线

obligation 遍历、ref 构造与必需 execution 核对受 request `working-memory` 与 `semantic-steps` 预算约束；零预算或超限以 `resource-limit` 失败关闭。该计数只覆盖本方法，结果层现在可以把明确的局部资源错误物化为 `incomplete`，但完整跨阶段累计资源与 `wall-clock` 内部中断仍属于后续切片。

本方法本身不检查 counterexample minimality，不执行 cvc5、Node.js、生产 compiler 或 adapter，不重放 kernel rule，不检查 certificate，不把 backend attestation 升级为独立 proof，也不修改 Axiom Evidence / Independent Check Contract 公共格式。完整 assurance policy、remaining trust / missing artifact 与四态由后续 [结果层](independent-result-aggregation-v0.1.md) 显式消费，再由 [canonical companion codec](canonical-companion-v0.1.md) 编码已经形成的结果；本方法自身不生成 checker binary、`checker.artifact`、CLI 或 companion。

`VerifyConclusion` 成功只确认生产 Evidence 忠实表达其自身结论；一份正确报告 `violated`、`input_rejected` 或 `implementation_inconsistent` 的 Evidence 同样可以通过本层。调用方不得把该成功升级为 `accepted`、程序正确或六平台结论。
