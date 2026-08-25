# Axiom Evidence v0.1 concrete output comparison 切片

本文冻结独立 checker 对锁定 keyed-finite-table host / golden output 的重建、IR 独立执行与动态关系核对。调用方必须先通过 bundle、Axiom IR、Axiom Evidence 结构 / 身份、obligation completeness、state / support、counterexample world / `WF`、concrete input / `Pre` 和 proof-failure target replay；生产 target、host execution 的 `completed`、output comparator、Evidence result 与预期独立结果都不是本切片的真相源。

后续已经增加 [proof support 真值与能力边界](axiom-evidence-proof-support-v0.1.md)；本文的输出比较成功只界定 `VerifyConcreteOutputs` 自身，不能用来跳过 `InspectProofSupports` 或升级任何 `proved`。

## Artifact、envelope 与 execution role

`VerifyConcreteOutputs` 只通过调用方提供的受约束 artifact reader 重开 Evidence 已声明的 `axiom-benchmark-data` `0.1`，并再次核对 raw SHA-256。decoder 与 concrete input 共用同一严格 JSON、benchmark ID、scalar、record 和 table 路径，不复制 output codec。

锁定语料的 output artifact envelope 统一使用 `role = golden-output`。这是数据格式角色，不决定制品在某次 execution 中的身份；checker 另行严格核对：

- `execute-host`：恰好一个 `host-input`、一个 `target-module` 和一个 `host-output`；
- `compare-output`：恰好一个 `actual-output`、一个 `golden-output`，且没有 output；
- 同一内容摘要可在不同 execution 中承担 host / actual / golden role，但不能省略、交换或猜测 role；
- output artifact 必须包含恰好完整的 IR output interface 集。

每个 output row 继续按 IR record type 解码，检查闭合字段、Bool / Int / Text / 名义 Enum、数学整数范围、容量、主键唯一性和 canonical primary-key order。world 相等比较不会丢弃 interface、record type、field name、value kind、enum identity 或数组顺序。

## 三方独立比较

checker 从相关 `execute-host` 读取完整 `host-input`，要求 role 为 `input`，并由 `Document.Execute` 在 `WF ∧ Pre` 上独立解释有限 IR。生产 target module 只保留为 execution 边界成员，不被执行或信任。

每条关系保持三个来源：

1. checker 自身 IR execution 的 semantic output；
2. `execute-host` 声明产生的 host / actual output artifact；
3. `compare-output` 绑定的 golden output artifact。

checked host-conformance 要求 semantic output 与 host output 完全相等；checked output-conformance 要求 actual 与 golden 相等，且 actual 的全部相关 host producer 也与 checker semantic output 相等。unknown output obligation 只计为未决定，不读取或升级为 checked / failed。

锁定 24 个前置完整场景形成 9 条相关 host execution：四题正确候选的 8 条 execution 全部满足 semantic = host，`CHK-CONCRETE-01` 的 1 条故障 execution 满足 semantic = golden ≠ actual。正确候选另形成 7 条 checked comparison；内容去重后共有 8 个 output artifact。

## Failed mismatch 绑定

`CHK-CONCRETE-01` 的一条不等 comparison 同时支撑 3 个 failed entry：output interface、golden artifact 与 host artifact。每个 entry 必须满足：

- counterexample observed 恰好是 `host-output-mismatch`，actual / expected 分别等于 comparison 的 `actual-output` / `golden-output`；
- trace 恰好为当前 obligation ID 和 `observation = failed`，不能引用兄弟 entry；
- host-conformance artifact subject 必须是 actual，output-conformance artifact subject 必须是 golden，interface subject 必须解析到完整 golden world；
- 恰好一个 input-world witness 必须投影自 actual 的某个 completed host producer；
- 该 producer 的 checker semantic output 必须等于 golden 且不等于 actual。

因此 Evidence 已声明 `failed`、两个 artifact raw bytes 不同或 comparator `completed` 都不能单独成立为 mismatch。三条 entry 由同一真实不等 comparison 分别按自身 subject 和 trace 重放，不把 entry 数误写成 execution 数。

## 资源、验证与停止线

input / output JSON 继续消费 artifact byte、depth、item 与 parser step；每个 concrete data envelope 受确定性 logical-byte 上限约束，IR execution 继续按 input、retained node table 和 semantic step 计数。资源耗尽保持 `resource-limit`，raw bytes 漂移保持 `digest-mismatch`，关系漂移使用 `concrete-check-mismatch` 或 `counterexample-invalid`，不能降级为普通不相等。

负例覆盖 output format / role、execute / compare I/O、checked host 对调、checked golden 替换、failed actual / expected、trace、subject、witness、checked / failed 混用、raw digest、logical memory 与 semantic step；24 个前置完整场景全部进入入口，unknown 没有被升级。

本切片不执行生产 Node target，不检查 counterexample minimality，不验证 kernel rule、backend attestation 或 certificate，不重算 conclusion，不形成 remaining trust / missing artifact，也不生成 checker binary、`checker.artifact`、CLI 或独立四态 result。调用方仍须继续执行 proof support 审计并显式消费其能力与材料分类。

`VerifyConcreteOutputs` 成功只确认锁定具体 input / output artifact 与当前有限 IR 的动态关系；它不能升级任何 `proved`，不能证明其他输入上的实现等价，也不能形成 `accepted` 或六平台结论。
