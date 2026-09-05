# Checker 当前状态与能力边界

更新日期：2026-09-05

用途：供协作者读取当前能力、近期顺位和待复核问题。专题入口见[文档索引](../README.md)，可验收目标见[开发计划](../development-plan.md)。本页不是新语义或公共格式，也不记录某次 payload 的可执行授权。

## 阶段判断

当前实现是围绕 AX-B01–AX-B04 锁定语料形成的独立语义与证据检查器。它能从 IR 独立重建义务并解释有限 world，已超过纯格式校验；但尚未完成全首域支持、一般义务目标的充分反例归因或独立正向证明链。

本次能力盘点基于 `f960603` 的实现与专题；本轮仅更新文档和源码身份，没有新增语义能力或复跑完整动态矩阵。下列历史计数不是本次重新验证的结果。

## 能力矩阵

| 层面 | 已实现的范围 | 尚不能据此声明 |
| --- | --- | --- |
| 字节与身份 | 严格 JSON / JCS、request / manifest、文件类型 / 路径 / 长度 / 摘要、制品闭包和 subject 绑定 | 制品中的业务或证明声明为真 |
| IR 与义务 | 独立类型与 DAG 检查，从 IR / profile / execution 边界重建义务 definition / ID 并核对 completeness | 所有 IR v0.1 构造均被支持，或义务已被证明 |
| 有限语义 | `bool`、`int`、`text`、`enum`，5 类 node、18 类 expression、`count` / `sum`，输入 `WF ∧ Pre` 和具体输出比较 | `fixed`、一般可声明 `option` / `record`、全输入性质或资源外推；精确列表见 [IR profile](../axiom-ir-structure-v0.1.md) |
| 反例 | 锁定 8 个 proof-failure target 的有限重放、paired-world 非干扰检查，以及输入 / 宿主输出失败检查 | 一般目标归因、反例 minimality 或普遍证明；归因待复核见下文 |
| 正向支撑 | obligation-set / query envelope / response / execution / tool / trust 的绑定审计 | SMT term 与义务等价、kernel rule replay 或 certificate 检查；独立证明能力集合为空 |
| 结论与 CLI | production conclusion 重算、十类检查、四态 companion、累计预算与自身 artifact 身份绑定 | `accepted` 等于程序 `satisfied`，或退出码 `0` 等于证据被接受 |
| 资源与交付 | 内部 wall-clock / semantic steps / logical bytes，受控构建、payload acceptance 与分发包实现 | Go heap 精确上界、OS hard isolation、产品 qualification 或当前 HEAD 已有发布 artifact |

具体数据解码仍按输入接口名称映射四题 benchmark ID，见 [benchmark_data.go](../../internal/axiomir/benchmark_data.go)。同域新题、接口改名和节点新组合尚未形成独立泛化验收，不能从已有运算支持直接推导任意同域程序可用。

锁定语料共有 28 个 bundle，IR 入口覆盖 12 份唯一 IR。历史证明材料审计中的 213 个 producer `proved` claim 分为 65 个 attestation-only 和 148 个缺少可检查材料的 kernel claim，独立证明数为 0；原 25 个进入结果层且具有 expected companion 的场景为 22 个 `accepted-with-trust`、2 个 `incomplete`、1 个 `rejected`。不同入口的场景、义务与 world 数不可混用，依据见[证明支撑](../axiom-evidence-proof-support-v0.1.md)和[结果聚合](../independent-result-aggregation-v0.1.md)。

## 待复核：反例目标归因

状态：静态审阅疑点，尚未通过完整 bundle 复现；没有据此宣布既有结果失效。

[ReplayFailureTarget](../../internal/axiomir/target_replay.go) 对 `contract-guarantee` 检查指定契约，对 `key-cardinality` 匹配指定节点和失败种类；但 `row-coverage`、`field-origin`、`group-conservation` 将局部特征与 `anyGuaranteeFalse` 组合。行数减少、字段引用缺失或字段值合并，加上其他保证失败，是否足以反驳所声明义务，需要按规范逐项核实。

待构造的区分例是：过滤器正确移除无需保留的行，后续金额计算出错；Evidence 却把失败指向过滤器。测试须保留合法 IR、`WF ∧ Pre`、完整身份和 trace 绑定，确认能抵达目标判断。字段来源还需核对限定接口和传递依赖；分组需核对目标前驱、分组键和聚合守恒，不能只依靠全输入同名字段的值数差异。

具体规范若不足以定义目标谓词，先形成跨仓语义决策；既有窄场景已通过不等于一般归因已被证明，也不能以收窄叙述替代必要的实现修复。复现与关闭标准见开发计划。

## 近期顺位

1. 核实目标归因：构造“别的义务失败、声明目标成立”的反例负例，先复现再定性，必要时补规范和实现。
2. 验收同域可复用能力：选取独立合成新题、接口改名与已有节点组合，明确 benchmark 与语义输入的版本化边界。
3. 打通首条正向独立证明链：选择一个小规则集，完整连接义务、逻辑编码和可检查证明，并保留剩余信任。
4. 完善结果解释：让消费者同时看清程序结论、证据接受结果、逐义务覆盖、材料缺口与外层隔离依据。

测试与能力矩阵随各切片更新；资源和分发工作优先回答真实负载与来源问题。上述是 Checker 语义线顺位，不改变主仓 runtime / launcher 的现行前置和授权。

## 停止线与验证

- 不因本文扩大 parser tag、义务规则、certificate 支持集合或 assurance policy；上游 Evidence 漂移按其 ADR 0009 和版本化迁移处理。
- `accepted` / `accepted-with-trust` 接受的是 Evidence 对程序结论的表达；决定性失败可能先于其他证明缺口，不能宣称全部 proof claim 已验证。
- `isolation-report` 的内部结果不能替代 launcher 实施与观测 OS 隔离的证据。内部逻辑预算与外层硬限制分别验收。
- 当前源码与已发布 payload 按精确身份区分。修改本文会改变 `checker.source`，不会改写已有 binary、provenance、acceptance 或主仓 inactive 登记。
- 分仓独立、标准库限制、工具链和外部动作边界继续遵循根入口与治理文档。新增规划不授权下载、运行产品 payload、发布或激活。

文档检查至少包括链接、文本、`git diff --check`、仓库门禁与源码身份重放。正式 Go 质量门禁要求精确 `go1.26.7`；其他版本的检查只作标注版本的补充。命令见[根入口](../../README.md)。
