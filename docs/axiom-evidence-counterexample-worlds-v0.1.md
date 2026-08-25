# Axiom Evidence v0.1 counterexample world / WF 切片

本文冻结独立 checker 对锁定 keyed-finite-table Evidence profile 的第一段 concrete-data 语义检查。输入必须先通过 bundle、Axiom IR、Axiom Evidence 结构 / 身份、obligation completeness 与 state / support；生产 replay execution 的 `completed`、counterexample 中缓存的观察和预期 checker result 都不是本切片的真相源。

后续完整输入制品与 assume / `Pre` 检查已经在 [concrete input / Pre 切片](axiom-evidence-concrete-inputs-v0.1.md) 实现；本文的停止线仍用于界定 `VerifyCounterexampleWorlds` 自身，不因后续能力增加而被改写。

## 独立保留模型

Axiom IR parser 现在为后续 concrete replay 独立保留：

- enum 成员集合及声明顺序；
- record 的闭合字段、值类型与 `public` / `sensitive` 标签；
- table 的 record type、容量和有序 primary key；
- input / output interface 到 table type 的精确映射；
- canonical assume contract ID 集合，但尚不求值其 expression。

Evidence parser 不再在结构检查后丢弃 failed counterexample，而是保留 kind、preconditions、world、table、record type、字段与值。值 profile 闭合为 `bool`、`int`、`text`、`enum`；锁定 bundle 实际出现 `int`、`text`、`enum`，`bool` 由 checker 自有合成测试覆盖。没有复用生产 codec、解释器或 fixture helper。

## World 锚定与 WF

`VerifyCounterexampleWorlds` 按 obligation ID 稳定排序，只处理 `failed` result，并依次检查：

1. `paired-input` 恰好两个 world，其余当前 kind 恰好一个；
2. 每个 table name 必须解析到 IR input interface，table 与 field 数组保持严格规范顺序；
3. row 的 `record_type`、字段闭包和值 kind 必须相对接口 table 的 IR 声明重算；
4. `Int` 使用数学整数核对声明上下界，enum 同时核对名义类型、成员与声明顺序；
5. 行数不超过 table capacity，主键字段完整、类型可键、键唯一，行按 `false < true`、数学整数、enum 声明顺序、Unicode scalar 文本及复合键逐项比较的顺序严格递增；
6. proof counterexample 的 preconditions 必须恰好等于 IR assume contract ID 集合；其他 counterexample 的每个 precondition 也必须解析到该集合。

world projection 可以只保留见证实际提交的 input table，但提交的每张表和完整 row 都必须接受上述检查。是否允许省略某张输入表不会被解释为该表满足 `Pre`；后续 replay 必须结合绑定的 host-input artifact 重建完整输入。

## Input-conformance 的分层

核心 prove 以及 host / output failure 的 counterexample 必须是已锚定且 `WF` 的输入 world；否则稳定拒绝为 `counterexample-invalid`。

`input-conformance` 的职责正是见证具体输入不满足模式、范围、容量、键、外键或 `Pre`，因此它可以是“接口已锚定但非 WF”的 world。当前切片不会因为重复键或范围失败本身拒绝这种 Evidence，也不会反向宣称它已经证明目标 input-conformance 义务失败。后续必须求值全部适用 assume / `Pre`，并确认至少一个目标条件确实失败。

## 场景与负例

- 28 个导入 bundle 中，3 个继续在 artifact / digest / resource 前置层拒绝，`chk-obligation-01` 继续在 obligation completeness 拒绝；
- 其余 24 个场景继续通过 state / support 和本 world / WF 边界；其中 20 个 failed counterexample 实例覆盖 `single-row`、`row-pair`、`missing-key`、`group` 与 `paired-input`；
- 四个 invalid-input 场景分别保留 `Pre`、外键或重复主键失败，不被粗暴要求为 WF；
- 合成负例覆盖 world cardinality、未知 interface、整数越界、未知 enum member、字段遗漏、record type 漂移、重复键、缺失 assume 集合和非 assume precondition；
- 独立 Axiom IR 数据测试覆盖 `bool`、`int`、`text`、`enum`、容量、枚举主键顺序、重复键与范围；同一有效 world 重复验证 100 次结果稳定。

## 停止线

本切片不解析或核对 `axiom-benchmark-data` / `axiom-host-data` artifact，不求值 assume / `Pre`，不执行 5 类 node 或 18 类 expression，不确认 observed obligation、trace、公开等价、输出差异或 counterexample minimality，不验证 proof / attestation / certificate，不重算 conclusion，也不生成独立四态 result。

`VerifyCounterexampleWorlds` 成功只表示失败见证的有限 world 在当前声明层闭合，或被正确保留为 input-conformance 的待判定非 WF 输入；不能升级为“反例有效”、`failed` 已独立确认、`accepted` 或六平台结论。
