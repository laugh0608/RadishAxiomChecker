# Axiom Evidence v0.1 counterexample target replay 切片

本文冻结独立 checker 对锁定 keyed-finite-table proof-failure counterexample 的有限解释与目标重放。调用方必须先通过 bundle、Axiom IR、Axiom Evidence 结构 / 身份、obligation completeness、state / support、counterexample world / `WF` 与 concrete input / `Pre`；生产 replay execution 的 `completed`、solver response、pipeline receipt、Evidence `failed` 和预期独立结果都不是本切片的真相源。

## 单一 concrete 语义

checker 没有为反例另建第二套 runtime value。`VerifyCounterexampleTargets` 复用 concrete input 切片已经使用的 Bool、数学 Int、Text、名义 Enum、闭合 record、有限 table、primary-key 比较和 De Bruijn environment；原 assume evaluator 扩展为 input / output table 均可解析的共享 evaluator。

锁定 12 份唯一 IR 的动态闭包为：

- node：`input`、`filter`、`map`、`lookup_join`、`group`；
- expression：`literal_bool`、`literal_int`、`literal_text`、`literal_enum`、`bound`、`field`、`not`、`and`、`eq`、`le`、`int_add`、`int_sub`、`if`、`match_option`、`forall_rows`、`lookup`、`count_where`、`sum_where`；
- aggregate：`count` 与数学整数 `sum`。

解释器从 parser 已验证并防御性复制的 node DAG 出发，以稳定 DFS 拓扑顺序执行，不依赖 definition 的序列化顺序。filter 与 map 保持源键顺序；lookup join 对每个左行要求恰好一个右匹配；group 按声明 key 分区并将输出重新按 primary key 规范排序。每个 node 完成后重新核对输出 record、字段闭包、值范围、容量、键唯一性与 canonical order。

零匹配 join、非唯一 join、数学结果越界、aggregate 越界、未知动态绑定和 node table `WF` 失败都失败关闭。`ExecutionFailure` 只记录某个具体 node 的有限求值事实；只有后续与同一 failed obligation 的 subject 和 kind 精确绑定时，才能成为目标反例的一部分。

## Trace、observed 与目标绑定

Evidence parser 继续严格解析原有格式，但不再丢弃已经验证过的 trace / observed 内容。当前 proof-failure replay 额外要求：

1. trace 恰好依次绑定当前 IR document domain digest、当前 obligation ID 和 `observation = failed`；
2. observed 必须是 `obligation-failure` 且反向绑定同一 obligation；
3. required field 必须解析到锁定 input / output interface 字段，required key 必须真实出现在 retained world；
4. 每个 proof world 必须是完整 `WF` 输入并独立满足全部 assume / `Pre`；
5. 动态违反必须对应 target kind，不能只因别的执行错误或 Evidence 已声明 `failed` 就接受。

当前实际闭合目标为：

| 场景 | failed target | 独立重放事实 |
| --- | --- | --- |
| `AX-B01 wrong-add` | `contract-guarantee` | DAG 正常执行，锁定 guarantee 为假 |
| `AX-B01 wrong-drop-zero` | `row-coverage` | 目标 filter 丢失 witness 行，且锁定 guarantee 为假 |
| `AX-B02 wrong-constant-tier` | `field-origin` | 目标输出 expression 未读取 required origin，且 guarantee 为假 |
| `AX-B02 wrong-region-join` | `key-cardinality` | 目标 join 对同一左行产生多个右匹配 |
| `AX-B03 wrong-single-group` | `group-conservation` | witness 中多个原始 group value 被目标输出折叠，且 guarantee 为假 |
| `AX-B03 wrong-unit-sum` | `contract-guarantee` | DAG 正常执行，锁定 guarantee 为假 |
| `AX-B04 wrong-sensitive-filter` | `noninterference` | 两个公开等价输入产生不同输出行集合 |
| `AX-B04 wrong-sensitive-priority` | `noninterference` | 两个公开等价输入产生不同公开输出值 |

这里共有 8 个 `prove + failed` target。四个 invalid-input 场景包含 9 个 artifact / interface `input-conformance` failed entry，仍由 `VerifyConcreteInputs` 的同一真实输入分类；`CHK-CONCRETE-01` 的 3 个 host / output failed entry 由后续 [concrete output comparison](axiom-evidence-concrete-outputs-v0.1.md) 独立重放。三类集合不能用场景数、world 数或 result entry 数互相替代。

## 公开等价与输出差异

`paired-input` 必须恰好两个完整 `WF ∧ Pre` world。checker 按 noninterference contract 的 input interface、IR record label 与 canonical primary-key order 对齐行：所有 `public` 字段必须相同，`sensitive` 字段允许不同。随后分别独立执行 DAG，并比较受保护 output interface 的行存在性与公开字段；一个正常结束而另一个产生明确语义 `ExecutionFailure` 也构成差异。资源错误或 checker 内部不一致不能冒充非干扰违反。

## 资源、验证与停止线

解释器按 expression、node、row、join scan 与 aggregate term 消费 `semantic-steps`；input world 与 retained node table 使用确定性 logical-byte 上界并受 `working-memory` 限制。当前仍不是 Go heap 精确计量，`wall-clock`、跨阶段累计预算、内部 `incomplete` result 和外层进程限制尚未形成。

导入的 24 个前置完整场景全部进入该入口：8 个 proof failure 被独立确认，3 个 host / output entry 被显式交给独立 output comparison 入口，其余没有本方法目标。正向执行覆盖四题全部 checked input artifact；负例覆盖 node / predecessor 绑定、projection、join 非唯一、group aggregate、数值范围、trace、observed、required key、paired public input、目标未违反、semantic step 与 logical memory。

`VerifyCounterexampleTargets` 本身不解析或比较 host / golden output；调用方必须继续进入上述 concrete output comparison。该方法不检查 counterexample minimality，不验证 kernel rule、backend attestation 或 certificate，不重算 conclusion，不形成 remaining trust / missing artifact，也不生成 checker binary、`checker.artifact`、CLI 或独立四态 result。

`VerifyCounterexampleTargets` 成功只确认锁定有限 world 确实反驳对应 proof target；动态重放不能升级为 `proved`，不能说明生产输出正确，也不能形成 `accepted` 或六平台结论。
