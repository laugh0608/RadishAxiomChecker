# Checker 文档与结构导航

用途：为维护者按任务定位能力、实现边界与验证入口。本页不承载当前顺位、规范正文、逐日流水或发布状态。

## 默认入口

| 任务 | 入口 |
| --- | --- |
| 当前能力、近期顺位、待验证问题与停止线 | [当前状态](status/current.md) |
| 语义可靠性、同域泛化、证明链与结果表达验收 | [开发计划](development-plan.md) |
| 分支、CI、发布和外部操作边界 | [仓库治理](repository-governance.md) |
| 精确源码输入集合与摘要重放 | [checker.source](checker-source-v0.1.md) |

## 代码职责

| 包 | 职责 |
| --- | --- |
| `strictjson`、`protocol`、`bundle` | 严格字节入口、版本化请求和只读制品闭包 |
| `axiomir` | 独立类型检查、义务定义重建、具体数据模型与有限解释 |
| `axiomevidence` | Evidence 结构、义务一致性、状态、反例、输出、证明支撑与结论 |
| `resourcebudget` | 一次 invocation 的累计逻辑资源与内部时间预算 |
| `checkresult`、`checkercli` | 检查聚合、身份、canonical companion 与唯一 CLI |
| `sourceidentity` | 闭合源码快照身份生成和复核 |
| `checkerbuild`、`checkerartifact` | 受控构建与独立 payload acceptance |
| `payloadarchive`、`distributionaccept`、`payloaddistribution` | 候选归档、分发验收与分发包 |

具体输入、反例和输出重放共享 Checker 自有解释器；不复用生产实现。源码身份覆盖全仓不等于这些包都属于每次运行的可信基，运行时依赖与工具链 / 构建 / 分发职责应分别审阅。

## 专题切片

下列切片描述各自冻结或锁定的检查范围；其中“本切片不做”和当时 payload 进度须结合其上下文阅读，不能当作后续全部实现的统一现状。当前能力以状态页汇总，字节绑定的规范不为同步阶段措辞原地改写。

- 不依赖 `encoding/json` 的字节级严格 JSON / RFC 8785 JCS 外壳；
- `axiom-check-request` `0.1` 与 `axiom-check-bundle-manifest` `0.1` 的闭合解析；
- 只读 bundle 布局、普通文件、声明长度、原始 SHA-256、manifest 覆盖和 request 绑定检查；
- 独立导入且由摘要锁定的 RadishAxiom 合同 fixture。
- [`checker.source` v0.1](checker-source-v0.1.md) 的闭合仓库输入集合、canonical manifest、原始字节 SHA-256 复算和失败关闭门禁。
- [Axiom IR v0.1 严格结构与类型良构切片](axiom-ir-structure-v0.1.md)：对已锁定 keyed-finite-table profile 核对 canonical 字节、闭合 tag、全部 definition domain ID、声明 / 主键、18 个 expression op、projection / group 字段覆盖、节点 table type 关系、节点引用 / DAG / 可达性及完整文档 domain digest。
- [Axiom Evidence v0.1 严格结构与身份切片](axiom-evidence-structure-v0.1.md)：对 28 个锁定 bundle 的实际 profile 核对 canonical 字节、闭合 tag、tool / execution / obligation / trust / uncovered definition domain ID、直接引用索引、完整 Evidence document domain digest，以及 subject 的 IR raw content / document domain 双摘要绑定。
- [Axiom Evidence v0.1 obligation completeness 切片](axiom-evidence-obligation-completeness-v0.1.md)：只凭独立解析的 IR、profile、显式 benchmark execution I/O 边界和 trust 条目重建完整 definition / ID 集合，精确拒绝缺失、多余、expectation 与 anchor 漂移；不读取 `axiom-obligation-set` 制品作为真相源。
- [Axiom Evidence v0.1 state / support 切片](axiom-evidence-state-support-v0.1.md)：核对 expectation 与五态矩阵、execution kind / result、精确 tool role、unknown attempt reason、trusted scope、backend attestation 绑定，以及 checked result 的有限 artifact 闭包。
- [Axiom Evidence v0.1 counterexample world / WF 切片](axiom-evidence-counterexample-worlds-v0.1.md)：保留失败见证的受界值、闭合记录、表和 world，以独立 IR 声明核对类型、范围、容量、主键唯一性与规范顺序，并区分 input-conformance 的预期非 WF 输入。
- [Axiom Evidence v0.1 concrete input / Pre 切片](axiom-evidence-concrete-inputs-v0.1.md)：从 Evidence 与 execution 的精确 `host-input` 边界读取 `axiom-benchmark-data` `0.1`，按 IR 重建完整输入、核对 `WF`、独立求值锁定 assume 子集，并确认 input-conformance 的 `checked` / `failed` 分类和 witness 制品投影。
- [Axiom Evidence v0.1 counterexample target replay 切片](axiom-evidence-counterexample-targets-v0.1.md)：复用同一 concrete value / table / expression 语义，按稳定拓扑顺序解释锁定 5 类 node、18 类 expression 与 2 类 aggregate，核对 proof-failure 的 `WF ∧ Pre`、trace、observed、required field / key、目标违反及 paired-world 公开等价。
- [Axiom Evidence v0.1 concrete output comparison 切片](axiom-evidence-concrete-outputs-v0.1.md)：复用同一 benchmark-data decoder 与有限 IR execution，严格区分 envelope role 和 execution I/O role，独立核对 semantic / host / actual / golden output，并重放 3 个 host-output mismatch entry。
- [Axiom Evidence v0.1 proof support 真值与能力边界切片](axiom-evidence-proof-support-v0.1.md)：重新打开 obligation-set、query、response 与 tool artifact，核对精确 execution / backend / trust 绑定；显式报告空 kernel / certificate 能力、attestation-only、remaining trust 与 missing proof material，不让 `completed`、`unsat` 或 producer tag 自证。
- [Axiom Evidence v0.1 conclusion 确定性重算切片](axiom-evidence-conclusion-v0.1.md)：不读取 producer conclusion、pipeline receipt 或 expected result 作为真相源，按规范优先级从已检查 obligation state 与必需 execution 独立形成 kind / refs，并以 `conclusion-mismatch` 拒绝生产聚合漂移。
- [Independent Check 内存结果聚合切片](independent-result-aggregation-v0.1.md)：将十类真实检查物化为契约 check ID，保守形成 remaining trust 与 missing artifact，并按拒绝、incomplete、允许 trust、无 trust 的唯一顺位形成四态内存结果，同时绑定 Evidence / request 双摘要与 checker source / toolchain / TCB 边界。
- [Independent Check canonical companion 与 invocation failure](canonical-companion-v0.1.md)：对具有显式 checker binary / runtime TCB 身份的内存结果实施唯一 JCS 编码、严格重解析与 result-domain identity 复算，并让外层进程失败只形成绑定 request 的 `not-produced` 记录。
- [Independent Checker invocation 与累计资源边界](invocation-resource-budget-v0.1.md)：用一次调用唯一的累计账本连接前置身份、十类检查和编码前门禁，让 `CHK-DIGEST-01` / `CHK-RESOURCE-01` 分别形成真实 `rejected` / `incomplete`，并让 `CHK-PROCESS-01` 继续只形成外层 `not-produced` failure。
- [Independent Checker CLI v0.1](checker-cli-v0.1.md)：提供唯一的 `check --bundle-root=<canonical-realpath>` 产品入口，严格拒绝参数、stdin 与 realpath 漂移，在读取 bundle 前复算当前 executable SHA-256，并把 linker 注入的 source / 版本、实际 `go1.26.7` toolchain 与 runtime TCB 绑定到既有 invocation / companion 路径。
- [Independent Checker macOS arm64 受控构建与 payload acceptance v0.1](checker-artifact-build-v0.1.md)：复算已接受 Go archive 后用两个隔离 cache / home / tmp 运行精确 `go1.26.7`，只在 binary bytes 一致时形成 artifact / canonical provenance；独立 acceptance 再检查 Mach-O、Go build info、自身份与 normal / rejected / incomplete 三条真实 CLI 路径。
- [Independent Checker payload 候选归档与留存边界 v0.1](checker-payload-retention-v0.1.md)：把 artifact、canonical provenance / acceptance 和 retention manifest 确定性封装为可逐字重建的 USTAR；GitHub Actions artifact 只作最长 90 天候选暂存，不能冒充 durable active runtime store。
- [Independent Checker runtime distribution package v0.1](checker-payload-distribution-v0.1.md)：独立复核内层候选、精确 Go payload 和法律材料后，形成 distribution acceptance，并把候选、acceptance、manifest、checker `LICENSE` 与 Go `LICENSE` / `PATENTS` 确定性封装为闭合外层 USTAR。

## 跨仓规范与事实

上游项目为 [RadishAxiom](https://github.com/laugh0608/RadishAxiom)。其 ADR、语义、IR / Evidence 和机器契约决定公共边界；本仓实现范围与下一步由当前状态导航。网页分支链接只用于阅读，规范 / fixture 的导入仍须绑定精确来源提交和摘要，不能作为可变运行时依赖。

已发布 payload 的源身份与本仓后续文档或代码变更分别追踪。源码清单更新不重绑旧 binary、acceptance、Release 或主仓登记。
