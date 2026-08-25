# Axiom Evidence v0.1 state / support 切片

本文冻结独立 checker 对锁定 keyed-finite-table Evidence profile 的五态、execution、tool role、attempt、trust、support 与有限 artifact 闭合关系。输入必须先通过 bundle、Axiom IR、Axiom Evidence 结构 / 身份和 obligation completeness；生产 conclusion、pipeline receipt 中的成功标记和预期独立结果都不是本切片的真相源。

后续已经增加 [counterexample world / WF](axiom-evidence-counterexample-worlds-v0.1.md) 与 [concrete input / Pre](axiom-evidence-concrete-inputs-v0.1.md) 检查；本文的“不重放”只界定 `VerifyStateSupport` 自身，不能用来跳过这些后续入口。非输入目标违反和 host / golden output 真值仍未实现。

## 保留模型与固定顺序

Evidence parser 不再在结构检查后丢弃结果关系，而是保留：

- obligation 的 expectation、kind、subject 与 result kind；
- `assumptions`、`artifacts`、`attempts`、`reason`、`execution`、`trust` 与 proof support；
- execution 的 kind、tool、result kind / code，以及完整、已规范排序的 input / output role；
- tool 的完整 role 集合和 trust category。

`VerifyStateSupport` 先按 execution ID 排序核对全部 execution 的 tool role，再按 obligation ID 排序核对每项状态。map 只用于摘要查找，不决定首个错误或输出顺序。任何关系漂移统一形成稳定 `invalid-state-support`，不以 warning、alias 或 fallback 继续。

## Execution kind 与 tool role

当前闭合映射为：

| execution kind | 必需 tool role |
| --- | --- |
| `normalize` | `ir-normalizer` |
| `generate-obligations` | `obligation-generator` |
| `prove` | `prover` |
| `check-certificate` | `certificate-checker` |
| `check-fixture` | `fixture-checker` |
| `execute-host` | `host-executor` |
| `compare-output` | `output-comparator` |
| `replay-counterexample` | `counterexample-replayer` |

锁定 bundle generator 先前让 `fixture-checker` 执行 `replay-counterexample`，但没有声明 `counterexample-replayer`。主仓已修正 tool definition、沿 tool → execution / trust → Evidence → request / manifest / expected result 重算摘要，并在生成期按本表拒绝漂移；checker 没有接受旧角色 alias。

## Expectation 与五态

允许矩阵严格为：

- `prove`：`proved` 完成，`failed` / `unknown` 阻断；禁止 `checked` / `trusted`；
- `check`：`checked` 完成，`failed` / `unknown` 阻断；禁止 `proved` / `trusted`；
- `trust`：`trusted` 完成，`unknown` 阻断；禁止 `proved` / `checked` / `failed`。

`trusted` 只允许完成 `trust-boundary`，result trust 必须与 obligation subject scope 为同一 ID，且 subject category 与 trust definition category 相同。它不能替代核心 prove / check，也不能解除 failed / unknown。

## Proved 与 unknown

锁定 `kernel-replay` 与 `backend-attestation` 都必须绑定 `kind: prove`、`result: completed`、tool 具有 `prover` role 的 execution；当前 profile 的该 execution 恰好有一个 `obligation-set` 和 `query` input，以及一个 `response` output。

`backend-attestation` 的 support query / response 必须分别等于 execution role 所绑定的 artifact；support trust 必须解析到 `proof-backend`，并同时出现在该 result 的有序 assumptions。此处只确认绑定与显式剩余信任，不验证后端响应或把 attestation 升级为独立 proof。`kernel-replay` 也只确认生产记录关系，尚未重演任何 kernel rule。

`unknown` 必须有非空、按 execution ID 排序的 attempts。当前映射包括 timeout → `timeout`、resource-exhausted → `resource-exhausted`、backend-unavailable → `unavailable`、unsupported / incomplete-certificate → `unsupported`、indeterminate → `completed`、operational-error → `error`；attempt execution kind 还必须对应目标 prove 或具体 check。后续成功不能删除这些历史 attempt。

## Checked、failed 与 artifact 边界

`checked` 只接受完成的动态 execution，并按义务类别固定：

- `ir-structure` → `check-fixture`，artifacts 等于该 execution 的完整 I/O 集合；
- `input-conformance` → `check-fixture`，artifacts 等于全部 `host-input`；
- `host-conformance` → `execute-host`，artifacts 等于 `host-input` 与 `host-output`，不把 target module 冒充被检查的数据；
- `output-conformance` → `compare-output`，artifacts 等于 `actual-output` 与 `golden-output` 的唯一集合。

artifact subject 必须出现在 result artifacts。缺失、多余或未知 artifact 都拒绝。

`failed` 必须绑定完成的 replay 或 comparison：prove 义务与 input conformance 使用 `replay-counterexample`，host / output conformance 使用 `compare-output`。本切片只检查 execution 类别、完成状态与显式 host / actual / golden artifact 边界；counterexample world / `WF` 与 input-conformance 真值由上述后续入口另行检查，非输入目标违反和具体输出差异真值仍留给后续 replay。

## 场景与负例

- 28 个导入 bundle 中，3 个继续在 artifact / digest / resource 前置层拒绝，`chk-obligation-01` 继续在 obligation completeness 拒绝；
- 其余 24 个场景逐项通过 state / support 闭合，覆盖 proved、checked、unknown、failed、trusted、kernel replay、backend attestation、timeout、unavailable、host execution、output comparison 与 counterexample replay；
- 合成负例覆盖 expectation / state 误用、缺失 support、错误 tool role、未完成 prove execution、attempt reason 漂移、attestation trust 遗漏、response 漂移、checked artifact 闭包遗漏、trusted scope 漂移和 failed execution kind 漂移；
- 同一有效文档重复验证 100 次保持相同成功结果。

## 停止线

`VerifyStateSupport` 本身不验证 kernel rule、certificate 或 backend attestation 真值，不解释 obligation-set / query 定理，不重放 counterexample、输入、host 或 golden output，不判断 witness 最小性，不重算 conclusion，不应用 assurance policy，也不生成 remaining trust、missing artifact、checker binary、`checker.artifact`、CLI 或独立四态 result。调用方仍须按顺序进入已经实现的 world / `WF` 与 concrete input / `Pre` 检查。

`VerifyStateSupport` 成功只表示锁定 Evidence 的状态与引用关系闭合，不能升级为 `checked`、`proved`、`accepted` 或六平台结论。
