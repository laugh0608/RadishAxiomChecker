# Axiom Evidence v0.1 proof support 真值与能力边界切片

本文冻结独立 checker 对锁定 keyed-finite-table Evidence profile 中 `proved` 支撑材料的审计边界。输入必须先通过 bundle、Axiom IR、Axiom Evidence 结构 / 身份、obligation completeness 和 state / support；生产 conclusion、`kind: proved`、`kind: kernel-replay`、prove execution 的 `completed` 以及 cvc5 status 都不是本切片的证明真相源。

入口为 `Document.InspectProofSupports`。返回的 `ProofSupportCheck` 是审计摘要，不是独立四态 result；它把 artifact 检查、attestation 事实、剩余 trust、策略满足情况和缺失 proof material 分开保留。

## 已锁定能力

`CurrentProofSupportCapabilities` 显式返回 checker 源码快照实际接受的能力集合：

- `KernelRuleProfiles`: 空；
- `CertificateProfiles`: 空。

这个空集合是必须消费的能力边界，不是待 producer tag 填充的默认值。Evidence 声称 `kernel-replay` 不会扩展 checker 能力；锁定 profile 也没有 certificate artifact、certificate format、rule step 或独立 certificate checker invocation 可供重放。因此本阶段的 `IndependentlyVerified` 恒为 `0`，任何非零值都必须伴随未来明确版本化的能力、材料、规则覆盖与测试变更。

## Artifact 与 execution 绑定

每项 `proved` 先重新核对 state / support 关系，再按 obligation ID 稳定排序处理。共享 execution 只解析一次。完成的 prove execution 必须恰好绑定：

- 一个 `axiom-obligation-set` `0.1` input；
- 一个 `axiom-smtlib2-qf-uflia-query` `0.1` input；
- 一个 `cvc5-response` `1.3.4` output；
- 一个具有 `prover` role 的精确 tool ID 与 tool artifact。

所有上述 artifact 都通过调用方提供的 `bundle.Verified.ReadArtifact` 重新打开并复算 SHA-256；不能依赖初次 bundle 遍历后的缓存成功。格式、版本、role、摘要或 tool 身份漂移失败关闭。

生产 `axiom-obligation-set` 仍不是 obligation completeness 的真相源。本切片只在独立 obligation completeness 已通过后解析它，并要求其 IR raw / document 双摘要、semantics、profile、全部 definition、domain ID、排序和基数与已经检查的 Evidence obligation 集合完全相同。这样可确认目标 obligation 位于 execution 绑定的集合中，但不会反向让生产 obligation-set 决定期待集合。

## Query 与 response 的有限检查

当前 query inspector 只接受单次、闭合的 ASCII SMT envelope：首命令为 `(set-logic QF_UFLIA)`，末命令为唯一 `(check-sat)`，中间顶层命令限于 `declare-const`、`declare-fun` 和 `assert`，括号必须平衡，至少有一个 assertion，且不接受 comment、quoted symbol、string 或其他未实现词法形式。

这只确认格式与声明的 logic，不解释 SMT term，也不从独立重建的 obligation 生成定理。锁定 query 是生产 fixture 查询，未携带可由 checker 验证的“该 SMT 定理等于每项 obligation”证明；因此每项 finding 的 `TargetInObligationSet` 可以为 true，而 `QueryTheoremVerified` 必须保持 false。

response 只接受一个精确 status frame：`unsat\n`、`sat\n` 或 `unknown\n`。status 被记录但不会自证：

- `backend-attestation` 附着于 `proved` 时必须为 `unsat`，同时 trust 必须以 `kind: tool` 精确 scope 到该 execution 的 prover tool；
- `kernel-replay` 即使同时出现 `unsat` 也没有可重放 proof material；wrong 场景中共享 response 为 `sat` 时，其余 producer `proved` claim 同样不会被升级。

## 分类与策略

| Evidence support | request `proof_support` | 独立覆盖 | 本切片结果 |
| --- | --- | --- | --- |
| `kernel-replay` | 任一锁定值 | 无 | `missing-proof-material`，reason 为 `kernel-replay-material-unavailable` |
| `backend-attestation` | `attestation-allowed` | 只确认 attestation | `attestation-only`，proof support 子策略满足，保留精确 `proof-backend` trust |
| `backend-attestation` | `certificate-required` | 只确认 attestation | `attestation-only` 且缺材料，reason 为 `certificate-required-attestation-only` |

`ProofPolicySatisfied` 计数只表示 request 的 proof-support 子策略，不判断全部 allowed trust category、其他 Evidence trust 或最终 assurance policy。`RemainingTrust` 明确保留 backend trust；attestation 永远不计入 `IndependentlyVerified`。

## 锁定语料结果

28 个导入场景中，digest / resource 两个场景继续在前置层拒绝，`chk-bundle-01` 保留缺失 proof artifact 并进入 `incomplete` 结果层，`chk-obligation-01` 继续在 obligation completeness 拒绝；其余 24 个完整链路场景完成 proof support 审计：

- 共 213 项 producer `proved` claim；
- 148 项 `kernel-replay` 全部因没有可检查 proof material 而失败关闭；
- 65 项 backend attestation 的 query / response / execution / tool / trust 绑定得到确认，但独立证明数仍为 0；
- 其中 53 项满足 `attestation-allowed` proof-support 子策略；`CHK-PROOF-01` 的 12 项在 `certificate-required` 下明确缺少 proof material；
- 合计 160 项 `MissingProofMaterial`，不会因 execution `completed` 或 status 文本而消失。

合成负例覆盖 obligation-set format / IR subject 漂移、query logic / status-command 漂移、response status 漂移、attestation trust scope 错绑、`proved` execution 绑定漂移、artifact digest 漂移、缺失 reader、逻辑内存和 semantic-step 限制；另以 wrong 场景确认 `sat` response 下的 kernel claim 仍保持材料不足。相同输入重复 100 次产生相同排序与分类。

## 资源与停止线

每个唯一 proof / tool artifact 的原始字节计入确定性逻辑内存和 semantic-step 预算，单 artifact 继续受 request `artifact-bytes` / JSON 限制；摘要在解析前重算。`wall-clock` 的 checker 内部累计中断仍未实现，边界与仓库其他切片一致。

本方法不执行 cvc5、Node.js、生产 compiler 或 adapter，不解释 SMT term，不重放 kernel rule，不检查 certificate，不证明 query theorem 与独立 obligation 等价，也不重算 Evidence conclusion。调用方必须继续进入 [production conclusion 确定性重算](axiom-evidence-conclusion-v0.1.md) 与 [Independent Check 内存结果聚合](independent-result-aggregation-v0.1.md)，不能用 proof audit 成功或 producer `satisfied` 跳过后续层；canonical companion、checker binary、`checker.artifact` 与 CLI 仍未生成。

`InspectProofSupports` 成功只表示审计过程本身完整结束；调用方必须读取 `IndependentlyVerified`、`AttestationsConfirmed`、`MissingProofMaterial`、`RemainingTrust` 和逐项 finding，不能把 `error == nil` 当成所有 `proved` 已经成立。后续 `VerifyConclusion` 只验证生产 Evidence 是否忠实聚合其五态，不消费或抹除这些独立 proof 分类。
