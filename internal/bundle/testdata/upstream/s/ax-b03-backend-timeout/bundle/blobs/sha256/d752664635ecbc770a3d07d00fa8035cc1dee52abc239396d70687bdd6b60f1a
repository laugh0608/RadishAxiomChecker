# ADR 0008：独立 checker 的实现语言、制品交换与隔离边界

日期：2026-08-22

状态：Accepted

用途：比较并冻结首个独立 Evidence checker 的实现语言、仓库与进程隔离、离线制品交换、检查结果协议和失败边界，完成首个受控实现切片之前的架构决策闭环。

读者：独立 checker、编译器、IR、Evidence、验证后端、基准、构建、发布与安全审阅维护者，以及复核项目可信计算基的协作者。

不包含：checker 源码或仓库创建、精确 CLI 参数与退出码、Alethe / CPC 最终选择、certificate checker 实现、依赖安装、Rust / Go workspace、发布签名、远程 artifact 服务、`.rax` 表面语法或 Agent 实验 execution lock。

## 背景与判定方法

[Axiom IR v0.1](../ir/axiom-ir-v0.md)要求严格 canonical JSON、内容寻址、类型与语义重算；[Axiom Evidence v0.1](../evidence/axiom-evidence-v0.md)要求独立 checker 从 IR 重建完整义务、重放反例与具体检查、区分 certificate 和 `backend-attestation`、重算生产结论，并在 Evidence 文档之外输出独立结果。[ADR 0004](0004-raxc-production-implementation-language.md)选择 Rust 作为生产 `raxc` 宿主，但明确禁止 checker 复用生产规范化、类型、义务、Evidence 聚合或后端适配代码；[ADR 0007](0007-first-verification-first-compilation-pipeline.md)进一步冻结可离线重放的内容寻址生产制品 DAG。

独立性不是“换一种语言”就自动成立。checker 必须同时避免以下同源失败：

- 生产与 checker 共用 parser、normalizer、类型检查器、义务生成器、解释器、JCS helper 或证书 adapter，使同一个缺陷同时产生并接受错误 Evidence；
- checker 通过库调用、插件或 FFI 嵌入 `raxc`、cvc5 或 Node.js，把独立复核退化为生产路径自检；
- artifact resolver 从 PATH、网络、原工作目录或可变缓存补齐缺失字节，使“离线复核”无法重现实际输入；
- checker 把缺失 artifact、不支持的 certificate 或资源耗尽误报为 Evidence 错误，或把生产 `satisfied` 原样抄成独立接受；
- 独立结果被写回 Evidence，形成自引用或由生产 producer 冒充 checker 身份。

语言比较对象沿用生产宿主决策中已经评估的 Go、OCaml、F# 与 Rust。评分为 1–5，5 表示更适合首个独立 checker；“可信基与依赖”及“维护成本”中 5 表示边界更小或成本更低。共同缺陷隔离、闭合模型、严格字节处理、可信基与四题复核是门槛，权重为 2。评分是基于现行规范和官方资料的设计判断，不是尚未执行的实现 spike 或性能基准。

| 维度 | 权重 | Go | OCaml | F# | Rust |
| --- | ---: | ---: | ---: | ---: | ---: |
| 与生产 Rust 的共同缺陷隔离 | 2 | 5 | 5 | 5 | 2 |
| 闭合数据与状态建模 | 2 | 3 | 5 | 5 | 5 |
| 严格字节、数学整数与规范化 | 2 | 4 | 3 | 4 | 5 |
| 小可信基与低依赖下限 | 2 | 5 | 3 | 3 | 4 |
| 跨平台离线分发 | 1 | 5 | 3 | 3 | 4 |
| 子进程、资源与摘要能力 | 1 | 5 | 3 | 5 | 5 |
| 长期维护成本 | 1 | 5 | 3 | 3 | 4 |
| AX-B01 至 AX-B04 独立复核能力 | 2 | 4 | 5 | 5 | 5 |
| 加权合计 |  | **57** | 51 | 55 | 55 |

Go 的优势是不同于生产 Rust 的工具链与标准库实现、无 VM 的单文件分发、标准库任意精度整数、SHA-256、UTF-8 和子进程能力，以及较低的依赖下限。限制同样明确：Go 没有闭合代数数据类型或穷尽模式匹配，map 迭代顺序不受保证；`encoding/json` v1 默认允许重复成员、大小写不敏感匹配并替换非法 UTF-8，不能直接承担本项目的严格入口。checker 必须实现自己的字节级 JSON / JCS profile、显式 tagged union 和默认拒绝分支，规范路径也不得依赖 map 迭代。[Go 语言规范](https://go.dev/ref/spec)、[`encoding/json` 兼容行为](https://pkg.go.dev/encoding/json)、[`math/big`](https://pkg.go.dev/math/big)

OCaml 与 F# 的 variant / discriminated union 更适合闭合模型，也与生产 Rust 隔离；但首版跨平台原生分发、运行时或原生工具链、包生态与依赖审计面更大。Rust 的闭合模型、规范化和分发能力最强，但与生产端共享语言、Cargo 生态和常见实现习惯，会增加意外复用与共同缺陷风险。语言差异本身不是证明，但在代码、仓库和协议已经隔离后仍是有价值的第二层缓解。

## 决策

首个独立 checker 选择 **Go 1.26 语言基线与 `go1.26.7` 精确工具链**，稳定实现 profile 为 `keyed-finite-table-independent-check` `0.1`。

截至本决策日，Go 1.27.0 刚于 2026-08-19 发布；Go 1.26.7 是仍受支持的前一主版本最新补丁。首个实现选择已经历一个发布周期的 1.26 语言线，不自动采用三天前发布的新主版本。[Go release history 与支持政策](https://go.dev/doc/devel/release)

首次创建 checker module 时必须使用 `go 1.26.0` 与 `toolchain go1.26.7`，构建设置 `GOTOOLCHAIN=local`、`CGO_ENABLED=0` 并提交 `go.mod`。初始 checker core 只允许 Go 标准库，不为不存在的外部 module 创建空 `go.sum` 或 vendor 占位；以后若批准外部 module，必须同时提交由 Go 工具维护的 `go.sum` 和经审阅的 vendor 源。新增 module、代码生成器、C 代码、`cgo`、动态库或 build-time 下载必须单独说明必要性、来源、精确版本、许可证、维护状态和可信基影响。Go 本体采用 BSD 风格许可证，分发时保留其许可要求。[Go modules](https://go.dev/ref/mod)、[Go license](https://go.dev/LICENSE)、[跨平台与无 cgo 构建](https://go.dev/doc/install/source)

该选择不授权现在安装 Go、创建 module 或实现 checker。`go1.26.7` 是首个实现基线，不是浮动的 `latest`；补丁升级必须重跑规范字节、负向拒绝和跨平台矩阵，主版本或语言更换必须重新评估本 ADR。

## 独立性与可信计算基

### 仓库、构建与依赖隔离

checker 源码必须位于与生产 `raxc` **不同的 Git 仓库、不同的依赖图和不同的发布流水线**。本仓库保留规范、ADR、格式契约与跨实现语料；未来 checker 仓库只以精确摘要导入规范发布包和测试制品，不通过 submodule、相对路径、Go `replace`、Cargo workspace 或源码复制引用生产实现。

以下边界必须持续成立：

1. checker 不 import、链接、生成或复制 `raxc` 的 parser、normalizer、类型 / 效果检查、义务生成、SMT 编码、反例重放、宿主 codec、Evidence 聚合或 pipeline receipt 实现；
2. checker 不依赖生产 crate、生产生成代码、生产内部 schema、生产构建缓存或生产测试 helper；
3. 生产端集成测试只按二进制摘要和版本化协议调用 checker，不能把 checker 作为 Rust library 或构建脚本；
4. checker 的源码锁定、依赖审查、构建产物和测试结果独立形成；生产构建成功不能替代 checker 构建或复核成功；
5. 同一维护者可以参与两个仓库，但不得据此声称组织或人员独立；本 ADR 只冻结可审计的技术隔离。

### 允许共享与禁止共享

允许双方共享的只有：

- 已接受且具有精确版本 / 摘要的首域语义、Axiom IR、Axiom Evidence、ADR 与外部标准；
- 版本化的 canonical 正例、负例、黄金摘要、基准 fixture 和公共格式样例；
- 经正式提升为规范材料的 tag、domain separator、格式注册表和机器 schema；checker 仍须独立执行语义规则，不能只因 schema 接受就接受 Evidence；
- 生产输出的 canonical IR、Evidence、query、certificate、counterexample、host data、receipt 和工具身份，均只作为待检查的不可信输入。

禁止共享生产源码、库、生成 parser、义务清单、运行时 helper、序列化器、缓存或“已经检查过”的中间布尔结果。复制后改包名仍属于复用；把生产生成的 obligation set 当成规范输入也属于复用。

### 进程与能力隔离

checker 是一次请求一个独立进程的离线 CLI。生产端只能通过规范请求、只读 bundle、stdout / 输出文件和退出状态与其交换；禁止 in-process API、动态插件、共享内存、daemon session、FFI 或回调生产代码。

checker core 不启动 `raxc`、cvc5、Node.js 或生产 adapter。后端 `backend-attestation` 只检查 query、response、execution、tool 与 trust 的绑定，不重新调用后端并声称独立证明。未来 certificate support 若需要专用 checker，必须使用另一个按摘要固定的子进程和版本化协议；不得动态加载，也不得复用生产 proof parser。该子进程的工具身份、规则集和剩余可信边界进入独立结果。

checker 仍信任自身 Go 编译产物、严格 JSON / JCS、SHA-256、语义规则解释、数学整数与操作系统进程边界。使用 certificate checker 时还信任其实现与规则覆盖。`accepted` 只表示相对于这些明确边界完成复核，不能宣传为零信任或真实意图正确。

## 检查请求与离线 bundle

### 规范 JSON profile

`axiom-check-request` `0.1`、`axiom-check-bundle-manifest` `0.1` 与 `axiom-independent-check-result` `0.1` 都使用 Axiom IR / Evidence 已冻结的 JSON profile：I-JSON、RFC 8785 JCS、UTF-8、禁止重复成员、JSON number 与 `null`，数量使用规范十进制字符串，未知字段 / tag / 版本严格拒绝，规范机器字节无 BOM、空白或末尾换行。

所有普通 artifact content digest 直接对原始字节计算 SHA-256。请求、bundle manifest 和独立结果的域摘要对完整文档抽象值使用 JCS；单项检查的 ID 对不含 `id` 的 definition 使用 JCS。两者都采用 Axiom IR 相同公式 `SHA-256(UTF8(domain) || NUL || JCS(value))`，固定域为：

| 对象 | `domain` |
| --- | --- |
| 检查请求 | `axiom-independent-check-v0.1:request` |
| bundle manifest | `axiom-independent-check-v0.1:bundle-manifest` |
| 单项独立检查 | `axiom-independent-check-v0.1:check` |
| 完整独立结果 | `axiom-independent-check-v0.1:result` |

摘要不写回自身文档。独立结果必须同时绑定 Evidence 原始 content digest 和可重算时的 Evidence 文档域摘要，不能互相替代。

### 请求

检查请求恰好包含：

```json
{
  "assurance_policy": {
    "allowed_trust_categories": ["<按字典序排序的 Evidence v0.1 trust category>"],
    "proof_support": "certificate-required"
  },
  "bundle_manifest": "sha256:<manifest 规范字节 content digest>",
  "checker_profile": {
    "name": "keyed-finite-table-independent-check",
    "version": "0.1"
  },
  "evidence": "sha256:<Evidence 原始规范字节 content digest>",
  "limits": [
    { "name": "artifact-bytes", "unit": "byte", "value": "<正数>" },
    { "name": "bundle-bytes", "unit": "byte", "value": "<正数>" },
    { "name": "collection-items", "unit": "item", "value": "<正数>" },
    { "name": "json-depth", "unit": "level", "value": "<正数>" },
    { "name": "semantic-steps", "unit": "step", "value": "<正数>" },
    { "name": "wall-clock", "unit": "millisecond", "value": "<正数>" },
    { "name": "working-memory", "unit": "byte", "value": "<正数>" }
  ],
  "request_version": "0.1"
}
```

`proof_support` 只有 `certificate-required` 与 `attestation-allowed`。前者要求每项 `proved` support 能由 checker 直接 kernel replay 或通过受支持 certificate 完整检查；遇到 `backend-attestation` 必须 `incomplete`。后者允许后端声明在绑定完整时保留为 `proof-backend` trust，并使完成的结果最多为 `accepted-with-trust`。

`allowed_trust_categories` 是调用方明确接受保留的 Evidence v0.1 trust 类别，不存在隐式“全部允许”。未被 checker 重放消除、又不在该列表中的 trust 使结果为 `incomplete`；它不使 Evidence 自身变成错误。

每个 limit 恰好包含 `name`、`unit` 与规范十进制字符串 `value`。`limits` 必须按 `(name, unit)` 排序并恰好包含示例中的七项正数限制。没有默认值、无限值或环境继承。确定性 parser / replay step budget 由 profile 计数；操作系统施加的硬时限或内存限制由伴随运行记录保存，外层直接杀死进程时不得伪造 checker 结果。

### Bundle manifest 与目录布局

manifest 恰好包含 `bundle_version: "0.1"` 和按 `content_digest` 排序且唯一的 `artifacts`。每个 artifact descriptor 恰好包含 Evidence v0.1 的 `byte_length`、`content_digest`、`format`、`format_version`，以及按字典序排序的非空 `roles`。v0.1 role 只有：

- `evidence`：恰好一个，与 request 的 Evidence digest 一致；
- `subject`：Evidence subject 引用的 canonical IR 或 rejected candidate；
- `evidence-artifact`：Evidence 顶层 artifact 清单引用的字节；
- `normative-spec`：Evidence 绑定的语义与格式规范字节，仅用于核对摘要和审计，不能在运行时改写 checker 支持规则。

一个 digest 可以具有多个 role，但 descriptor 只能出现一次。manifest 必须覆盖 Evidence 与完成当前 assurance policy 所需的全部引用；缺少字节可形成 `incomplete`，不能联网或从生产缓存补齐后继续冒充原 bundle。

首版交换载体是普通只读目录，不是 ZIP、tar、OCI image 或远程对象服务：

```text
request.jcs
manifest.jcs
blobs/
  sha256/
    <64 个小写十六进制字符>
```

每个 blob 是不经换行、Unicode、压缩或权限元数据重写的原始字节。resolver 只接受 manifest 列出的普通 blob，拒绝 symlink、目录穿越、大小写变体、缩写 digest、重复 digest、未列出 blob 和同一路径的类型变化；读取后先检查 byte length 与 SHA-256，再交给格式解析器。绝对路径、URL、凭据、PATH 搜索、网络、当前目录 fallback 和可变 cache key 都不进入解析。

目录布局只是离线传输 envelope；规范身份来自 request、manifest 和 blob 字节。外层可以压缩或复制目录，但解包不是检查的一部分，解包后必须恢复完全相同的规范文件和 blob。checker 结果写到输入目录之外，不能修改 bundle。

## 独立检查行为

checker 必须按固定顺序完成以下检查类别；后续检查可以因前置拒绝或资源不足停止，但不能覆盖已形成的拒绝或不完整事实：

1. `strict-parse`：严格解析 request、manifest 和 Evidence，拒绝未知版本、字段、tag、重复成员、非法 UTF-8 与非规范字节；
2. `identity`：重算请求、manifest、Evidence、IR、条目和可得 artifact 摘要，检查长度与引用唯一性；
3. `subject`：使用 checker 自己的 IR parser、normalizer、类型 / 效果与引用规则检查合法 IR 或结构拒绝 subject；
4. `obligation-reconstruction`：从 canonical IR、语义与 Evidence profile 独立重建完整义务 definition / ID，精确比较遗漏、多余、anchor、expectation 和依赖；
5. `state-support`：检查五种状态、execution、tool role、attempt、trust / uncovered 和 support 的闭合关系；
6. `counterexample-replay`：以 checker 自己的有限表解释器重算 `WF`、`Pre`、目标违反、公开等价、trace 与见证最小性声明；
7. `concrete-check-replay`：严格解码具体输入、实际 / 黄金输出，独立解释 IR 的有限 world 并重做 input、host 与 output conformance；不执行生产 Node module；
8. `proof-support`：对 `kernel-replay` 使用 checker 自有规则；对 certificate 检查证书结论与独立重建义务一致；对 attestation 只确认绑定并保留 trust；
9. `conclusion-recompute`：按 Evidence v0.1 优先级重算 `structure_rejected`、`input_rejected`、`implementation_inconsistent`、`violated`、`inconclusive` 或 `satisfied`，拒绝生产结论漂移；
10. `isolation-report`：报告实际 checker、工具链、certificate checker 和规则 / 摘要实现的可信边界，不把缺少组织独立误写成技术证明。

生产 obligation set、pipeline receipt、cvc5 query 和 producer conclusion 都只是待核对线索。receipt 可以定位 artifact，不能替代从 IR 重建义务；生产 query 与 certificate 的定理若不等于 checker 独立重建的目标，即使 certificate 对生产 query 有效也必须拒绝对应 support。

## 独立结果协议

独立结果是 Evidence 之外的 canonical companion document，不能成为 Evidence 成员、不能修改 Evidence digest，也不能使用生产 `producer` tool identity。顶层恰好包含：

```json
{
  "checker": {
    "artifact": "sha256:<当前平台 checker binary>",
    "name": "radishaxiom-independent-checker-go",
    "source": "sha256:<不可变源码快照>",
    "toolchain": "go1.26.7",
    "version": "<精确实现版本>"
  },
  "checks": [],
  "evidence": {
    "content_digest": "sha256:<Evidence 原始字节>",
    "document_digest": { "kind": "available", "value": "sha256:<Evidence 文档域摘要>" }
  },
  "missing_artifacts": [],
  "remaining_trust": [],
  "request": {
    "content_digest": "sha256:<检查请求原始字节>",
    "document_digest": { "kind": "available", "value": "sha256:<检查请求文档域摘要>" }
  },
  "result": { "kind": "accepted" },
  "result_version": "0.1",
  "tcb": []
}
```

Evidence 或 request 无法形成规范文档摘要时，各自的 `document_digest` 精确为 `{ "kind": "unavailable" }`；只有 `rejected` 允许该变体。这样可把存在但不规范的原始输入绑定到拒绝结果，同时不为它伪造规范文档摘要。`missing_artifacts` 按 digest 排序且唯一；`remaining_trust` 按 Evidence trust ID 排序且唯一。

每项 `checks` 条目包含 `id` 和 `definition`。definition 恰好包含上述十种 `kind` 之一、`outcome`、按字典序排序的稳定 `codes` 与按 `(kind, ref)` 排序的 `refs`；ID 使用 `axiom-independent-check-v0.1:check` 域重算。`outcome` 只有：

- `passed`：该项已按 checker 能力完整重算；
- `trusted`：绑定成立，但结论仍依赖显式允许的 trust；
- `incomplete`：没有发现矛盾，但缺少 artifact、能力、受支持规则或资源；
- `rejected`：已经发现格式、摘要、引用、语义重放或结论不一致。

`codes` 来自 profile 的闭合集合，不能以自由文本决定结果。诊断可以另行绑定结果摘要，但不得包含凭据、绝对路径、环境变量值、PID 或真实数据。

`tcb` 条目按 `(category, artifact)` 排序，至少报告 `checker-core`、`canonicalization`、`cryptographic-primitive`、`rule-interpreter`，实际使用时另有 `certificate-checker`；每项绑定实现 artifact 和精确版本。相同源码在不同平台生成的 binary digest 可以不同，不能伪装成同一 artifact。

顶层 `result` 按以下优先级唯一聚合：

1. 任一确定的 `rejected` check 产生 `{ "kind": "rejected", "refs": [...] }`；已有矛盾优先于同时存在的缺失 artifact，但二者都保留；
2. 没有拒绝，但存在 `incomplete`、缺失 artifact、资源未完成、certificate 不支持、`certificate-required` 遇到 attestation，或剩余 trust 不在 policy 允许列表时，产生 `{ "kind": "incomplete", "refs": [...] }`；
3. 所有必需检查完成且剩余 trust 非空，并全部被 request 明确允许时，产生 `{ "kind": "accepted-with-trust", "refs": [...] }`；
4. 所有必需检查完成且 `remaining_trust` 为空时，才产生 `{ "kind": "accepted" }`。

`accepted` / `accepted-with-trust` 接受的是 **Evidence 对其自身生产结论的忠实表达**，不是把程序结论改成 `satisfied`。一份正确报告 `violated`、`structure_rejected` 或 `implementation_inconsistent` 的 Evidence 同样可以被独立接受。反过来，`rejected` 表示检查输入或 Evidence 不一致，不自动证明候选程序正确或错误。

checker 进程在形成规范结果前崩溃、被外层杀死、无法读取 request 或无法确认自身工具身份时，不得由调用方伪造 `incomplete` 结果；只能形成绑定 invocation 的非证明性运行失败记录。checker 自己在显式内部 budget 内检测到资源不足时，可以输出 `incomplete` 并引用真实 check。

## Certificate、attestation 与 trust 分级

- `kernel-replay` 只有在 checker profile 明确支持该规则并直接重演后才是 `passed`；未知规则为 `incomplete`。
- `certificate` 必须绑定原始 certificate、格式版本、独立重建的义务、实际 certificate checker 与检查结果。hole、trust step、未知规则、未检查 side condition、定理不一致或复用生产 parser 都不能成为完整 support。
- `backend-attestation` 永远不升级为独立 proof。`attestation-allowed` 只能产生带 `proof-backend` 及相关 generator trust 的 `accepted-with-trust`；`certificate-required` 下为 `incomplete`。
- checker 对当前 Evidence 的独立规范化、义务重建和具体重放可以把相应生产 generator / decoder trust 标为已缓解，但不能删除 `specification-intent`、`sensitivity-classification`、`input-origin` 等无法由字节复核的真实假设。
- checker 自身的 JCS、SHA-256、规则与 certificate checker 属于 `tcb`，不伪装成 Evidence trust 已经消失。

本 ADR 不冻结 Alethe、CPC、Ethos 或其他 certificate 格式。机器契约可以列出零个受支持 certificate profile；在首种格式及完整规则覆盖通过独立审阅前，`certificate-required` 路径按真实能力保持 `incomplete`。

## 资源、恶意输入与失败关闭

1. 先验证 manifest 声明长度和单 artifact / bundle 总上限，再分配或解析；流式计算摘要，拒绝路径别名和符号链接。
2. JSON 深度、数组 / object 成员数、字符串字节、数学整数位数、IR 节点、表容量、world 行数、trace、义务和 certificate step 都受 request 限制；资源边界不能通过 panic 或整数溢出绕过。
3. Go `encoding/json` 不能直接把不可信字节 unmarshal 到领域 struct；必须先由 checker 自有严格词法 / 结构层拒绝重复成员、非法 UTF-8、大小写别名、JSON number、`null`、未知字段和尾随内容，再构造闭合领域值。
4. 规范顺序只能来自排序后的 slice 或显式比较器；map 只可用于查找，序列化、ID、诊断 refs、义务和结果不能依赖 map 迭代。
5. 数学整数使用受封装的 `math/big.Int`，不得浅拷贝可变值或落入宿主 `int` 溢出；长度转宿主索引前先检查范围。
6. panic、I/O 错误、certificate checker 崩溃、unsupported、timeout 和资源耗尽失败关闭；不调用生产工具、不重试到另一个后端、不联网下载，也不把后续成功覆盖旧 check。
7. 首批 checker 只处理合成基准制品。真实秘密、凭据、生产路径和未脱敏数据不得进入 bundle、诊断、fixture 或提交。

## 跨平台分发与可复核构建

首批目标矩阵为 Linux、macOS、Windows 的 `amd64` 与 `arm64`，使用 `CGO_ENABLED=0` 的平台原生单可执行文件。交叉编译成功只证明产生了文件；每个宣称支持的目标必须原生运行相同正例、负例和资源失败语料。

构建必须固定 `go1.26.7`、`GOTOOLCHAIN=local` 与 `-trimpath`，禁止构建时网络、VCS 漂移、时间生成字段和宿主绝对路径进入规范结果；若以后批准外部 module，正式构建另须使用 `-mod=vendor`。首版不宣称不同 OS / architecture 的二进制逐字节相同；每个平台 artifact 分别记录摘要、Go toolchain 来源、源码摘要和许可证清单。相同 request / bundle 在各平台的 check kind、outcome、codes、Evidence 结论、missing artifact 和 remaining trust 必须一致；仅 checker binary identity 与平台运行伴随记录可以不同。

## 必需验证矩阵

进入 checker 实现前，版本化语料至少物化以下独立入口：

| ID | 输入 / 变化 | 预期独立结果 |
| --- | --- | --- |
| `CHK-CAN-01` | request / manifest / Evidence 含空白、重复成员、非法 UTF-8 或非规范顺序 | `rejected` |
| `CHK-DIGEST-01` | Evidence、IR、artifact 或 checker 引用保留旧摘要后篡改字节 | `rejected` |
| `CHK-BUNDLE-01` | 缺少一个被引用 artifact | `incomplete`，不得联网补齐 |
| `CHK-BUNDLE-02` | symlink、路径穿越、大小写摘要或未列出文件 | `rejected` |
| `CHK-IR-01` | 生产 normalizer 接受未知字段、错误类型或非空效果 | checker 独立拒绝 |
| `CHK-OBLIGATION-01` | 省略或伪造一个范围、守恒或非干扰义务 | `rejected` |
| `CHK-STATUS-01` | 用 `checked` / `trusted` 完成 `prove` 或洗白 `unknown` | `rejected` |
| `CHK-COUNTEREXAMPLE-01` | model 转换的 world 不满足 `WF` / `Pre` 或不违反目标 | `rejected` |
| `CHK-CONCRETE-01` | 生产 Node 输出与 checker 解释结果或黄金结果不同 | 接受正确的 `implementation_inconsistent` Evidence，错误聚合则 `rejected` |
| `CHK-PROOF-01` | `certificate-required` 遇到 attestation、未知规则或不完整 certificate | `incomplete` |
| `CHK-PROOF-02` | `attestation-allowed` 且绑定完整 | 最多 `accepted-with-trust`，保留 backend trust |
| `CHK-CONCLUSION-01` | 生产 conclusion 与独立重算不一致 | `rejected` |
| `CHK-ISOLATION-01` | checker import / 调用生产实现或读取生产 cache | 构建 / 集成门禁失败，不产生合格独立结果 |
| `CHK-RESOURCE-01` | 合法但达到 depth、bytes、steps 或 memory 限制 | `incomplete`；panic / crash 不算结果 |
| `CHK-PLATFORM-01` | 同一 bundle 在六个目标运行 | 规范检查结果一致，工具 artifact 分别绑定 |
| `CHK-PROCESS-01` | checker 被外层杀死或输出截断 | 仅运行失败记录，不伪造四态结果 |

矩阵必须覆盖 AX-B01 至 AX-B04 的正确候选、八个错误候选、invalid input、backend timeout、host mismatch、artifact 篡改、义务遗漏和 trust policy 差异。只验证 happy path 或命令退出零不满足独立性要求。

## 进入受控实现的验收条件

本 ADR 被接受不授权现在创建 checker 仓库、Go module、Cargo workspace、parser、解释器、certificate adapter 或 runner。首个实现变更前仍须同时满足：

1. `axiom-check-request`、`axiom-check-bundle-manifest`、`axiom-independent-check-result` 和 check code registry 形成机器可读 schema、canonical 正例、逐字段负例和黄金摘要；
2. ADR 0004–0007 要求的精确 Rust、cvc5、Node、adapter、pipeline artifact、policy、limits、依赖与许可证身份同本 ADR 的 Go 工具链一起形成可审阅实现就绪清单；
3. 允许共享的规范发布包与禁止共享的生产实现清单物化，未来 checker 仓库可以只凭规范包摘要和公开语料构建测试；
4. AX-B01 至 AX-B04 的 IR / Evidence / artifact 离线 bundle 与本节负向矩阵具有唯一预期结果，缺失 artifact 和进程失败不会冒充 Evidence 状态；
5. checker core 的领域模型、严格 JSON / JCS、SHA-256、义务重建和有限表解释器具有独立设计说明，未复制生产模块或生成输出；
6. 首批平台、资源计数、外层进程限制、构建来源和 tool identity 规则精确固定；交叉编译不替代原生验证；
7. certificate 能力矩阵准确声明“支持、含 trust step 或不支持”，没有格式时允许 `incomplete`，不得用 backend attestation 冒充证书；
8. 项目所有者审阅并单独授权首个小而完整的实现切片；该授权不自动包含远程仓库创建、依赖安装、发布、push 或 Agent 模型调用。

上述材料通过后，首个受控切片应优先覆盖 request / bundle 严格解析、摘要解析和一组拒绝 fixture；不得一次性展开完整编译器、checker、solver、Node 与 Agent runner。

## 风险与重新评估

- Go 缺少穷尽模式匹配，新增 tag 时可能漏掉拒绝分支；所有 union 使用未导出构造、显式 tag switch、未知默认拒绝和逐 tag 负例，不能用反射式通用 map 传播领域值。
- 自行实现严格 JSON / JCS 会增加 checker core；直接使用宽松标准库会更危险。以 RFC 8785、规范黄金字节、重复成员 / Unicode / 排序负例和第三方测试期交叉实现缩小风险，但交叉实现不能成为运行时 fallback。
- 不同语言仍可能共同误读同一份过弱规范；规范反例、独立义务重建、差分 fixture 和人工审阅仍是必要边界。
- 分离仓库增加版本协调成本；只通过版本化规范包、摘要和离线 bundle 协调，不使用浮动分支或源码链接降低成本。
- 单文件静态分发不等于可复现构建或无操作系统信任；每平台记录实际 artifact，不能把 `-trimpath` 写成字节可复现证明。
- 独立具体解释器可以检查有限 world，但不能证明全部输入；它用于重放 `failed` / `checked`，不能把动态执行升级为 `proved`。

出现以下任一事实时，以新 ADR 重新比较 Go、OCaml、F#、Rust、Lean kernel 或其他路径，而不是共享生产实现或增加静默 fallback：

- 严格 JSON / JCS、闭合领域模型或义务重建在 Go 中需要无法审计的依赖、反射或不受控代码生成，导致 checker 不再小于生产路径；
- `CGO_ENABLED=0` 无法承载必需 certificate 能力，且外部子进程协议无法保持定理绑定与资源隔离；
- 六个平台无法运行相同语料并得到相同语义结果，根因是 Go 运行时 / 标准库而非局部缺陷；
- 分离仓库与规范包仍无法阻止 checker 依赖生产生成器，或共同缺陷在四题纵向切片中持续漏检；
- Lean 或另一小内核路径能以更小、可审计且可分发的可信基覆盖同一 certificate 和反例边界。

修改 checker 语言、允许复用生产语义代码、合并仓库或依赖图、改为 in-process / daemon / FFI、允许网络 resolver 或路径 fallback、把 attestation 当独立 proof、把 `incomplete` / `rejected` 混同、把独立结果写入 Evidence，必须以新 ADR 替代本决策。只增加暴露既有规则错误的负向 fixture 不改变格式版本；改变 request、manifest、result 字段、check kind / outcome、聚合优先级或域摘要必须提升相应 v0 minor 并显式迁移。

## 机器契约与当前物化范围

request、bundle manifest、result、check code registry、严格 Evidence 拒绝 bundle 与首批结构负例已经物化为 [Independent Check Contract v0.1](../../contracts/independent-check-v0.1/README.md)，由依赖为零的生成入口复算并纳入仓库门禁。该契约只覆盖 ASCII 结构 fixture、摘要、顺序、身份与四态聚合拒绝；它不表示完整 Evidence 正例、义务重建、语义解释、certificate 或跨平台 checker 已经实现，也不放宽本 ADR 的进入实现条件。

Go `go1.26.7` 的 source、六平台官方归档与 publisher 摘要，以及生产 Rust、cvc5、Node 和相关 profile 的共同实现就绪身份，已经登记到 [Toolchain & Adapter Identity Registry v0.1](../../contracts/toolchain-adapters-v0.1/README.md)。所有 payload、包内容和许可证仍保持未验收；清单也不表示未来 checker binary 已经构建或具备六平台运行证据。
