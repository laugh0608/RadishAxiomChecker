# Axiom Evidence v0.1 obligation completeness 切片

本文冻结独立 checker 对锁定 keyed-finite-table IR / Evidence profile 的义务定义与集合完整性比较边界。规范来源是按摘要锁定的首域语义、Axiom IR v0.1、Axiom Evidence v0.1 与 ADR 0008；生产 `axiom-obligation-set` artifact、生产生成器代码、schema、缓存和“已经检查”布尔值均不是输入真相。

## 固定处理顺序

1. bundle、Axiom IR 与 Axiom Evidence 必须先分别通过既有原始摘要、严格结构、definition / document 身份、引用和 subject 双摘要绑定；
2. Axiom IR parser 保留按规范 ID 顺序解析的 node / contract definition、输入接口、输出接口与输出记录字段，只构造结果无关的静态义务模型；
3. Evidence parser 保留 obligation profile、每项不可信 obligation definition、execution I/O role 与 trust category；
4. `VerifyObligationCompleteness` 从 IR 和显式 profile / boundary 独立重建 definition，用 `axiom-evidence-v0.1:obligation` 域、`NUL` 与 canonical definition bytes 重算 ID；
5. 预期与 Evidence 按 ID 精确比较，缺失、多余、错误 expectation、kind、anchor、path 或重复定义均以 `obligation-mismatch` 拒绝。

map 只用于摘要集合查找。IR entry 顺序来自已经核对的 canonical ID 顺序；接口、字段、trust ID 与 benchmark artifact 在重建前显式排序；重复运行必须产生相同 definition / ID 集合。

## IR 独立生成位置

当前锁定 profile 从 IR 恰好生成：

- subject document 的一个 `ir-structure` 和 program 的一个 `effect-empty`；
- 每个非 input node 的 `totality` 与 `key-cardinality`；
- filter、map、lookup_join 与 group 的 `row-coverage`；group 另有 `group-conservation`；
- node / contract definition 中每个 `int_add`、`int_sub`、`count_where`、`sum_where` 的规范 path `numeric-range`；
- group 的每个 count / sum aggregate 的 `numeric-range`；
- 每个命名 output record field 的 `field-origin`；
- 每个 guarantee formula 的 `contract-guarantee`，以及每个 noninterference contract 的 `noninterference`。

group 同时生成覆盖与守恒义务，依据首域语义的“行覆盖 / 分组守恒”区分、Axiom IR 的 AX-B03 映射及锁定 Evidence profile；两项不能合并。主仓 ADR 0009 已冻结 v0.1 规范原始字节不变、后续 v0.2 显式修正义务位置表并重新绑定摘要的迁移边界；本实现只声明支持当前锁定 profile。

## Profile 与显式 benchmark boundary

verification 与 benchmark profile 都为 Evidence 中每项显式 trust 生成一个 `(category, trust ID)` 的 `trust-boundary`。这保证 trust entry 与 obligation 一一对应，但本切片尚不判断 trust 是否覆盖所有真实依赖；该语义属于后续 state / support 与 artifact graph 边界。

benchmark profile 另外生成：

- 每个 IR input interface 的 `input-conformance`；
- execution I/O 中每个唯一 `host-input` artifact 的 `input-conformance`；
- 每个唯一 `host-output` artifact 的 `host-conformance`；
- 每个唯一 `golden-output` artifact 的 `output-conformance`；
- 出现 golden-output boundary 时，每个 IR output interface 的 `output-conformance`。

这些 role 只用于确定“必须存在何种 definition”的显式边界。本切片本身不判断 role 是否配给正确 execution kind、tool role、result、support 或 artifact format；调用方必须另行执行 [state / support 切片](axiom-evidence-state-support-v0.1.md)，后者仍不会提前冒充 concrete replay。

## 锁定场景与负例

- 28 个 bundle 中，`chk-bundle-01`、`chk-digest-01`、`chk-resource-01` 继续在前置 bundle / identity / resource 层拒绝；
- 其余 25 个身份有效 Evidence 中，24 个 definition / ID 集合精确匹配；
- `chk-obligation-01` 保持结构和文档身份有效，但因缺少一个规范 `numeric-range` 义务而得到 `obligation-mismatch`；
- 局部负例覆盖缺失、多余、expectation、path、anchor、同 anchor 冲突 expectation、非规范 obligation 顺序和重复遍历稳定性。

测试使用锁定的 12 份唯一 IR，不读取它们旁边的 `axiom-obligation-set` blob 来生成期待集合。该 blob 仍可作为 Evidence 引用 artifact 存在，但不能影响完整性判断。

## 停止线

本切片只判断 obligation definition 与集合完整性。它不：

- 判断 `proved`、`checked`、`unknown`、`failed`、`trusted` 与 expectation 是否正确配对；
- 判断 execution kind、tool role、execution result、support、assumption 或 trust 的完整语义；
- 重放 counterexample、具体输入、host output、golden comparison、certificate 或 backend attestation；
- 重算 conclusion、应用 assurance policy、形成 remaining trust 或独立四态 result；
- 读取生产 obligation set、执行 solver / Node / `raxc`、生成 binary、`checker.artifact` 或 CLI。

`VerifyObligationCompleteness` 成功不能升级为 `checked`、`proved`、`accepted` 或六平台结论。
