# Axiom Evidence v0.1：证据模型与独立检查边界

状态：Accepted

更新日期：2026-08-20

用途：定义首个目标领域的 Axiom Evidence v0.1 数据模型、验证状态、义务身份、反例、信任清单、结论聚合、规范化和独立检查边界，使验证结果可导出、重放并拒绝不一致声明。

读者：验证器与 Evidence 生成器设计者、独立检查器实现者、基准维护者和工具链评估者。

不包含：`.rax` 表面语法、验证后端选择、具体证明证书格式、独立检查器实现语言、命令行界面、制品打包容器、数字签名、远程证明服务或生产数据接入。

## 规范地位与绑定

本文是 Axiom Evidence `0.1` 的规范设计。它服从：

- [ADR 0002：首个目标领域与基准任务](../adr/0002-first-target-domain-and-benchmarks.md)的最小纵向闭环与四个基准；
- [有键有限表转换语义](../semantics/keyed-finite-table-semantics.md)的正确性、失败、反例和信任边界；
- [Axiom IR v0.1](../ir/axiom-ir-v0.md)的规范化、内容寻址和拒绝规则；
- [ADR 0003：版本标识与兼容性分层](../adr/0003-version-identities-and-compatibility-layers.md)的独立格式版本原则。

Axiom Evidence 只能证明或检查相对于精确语义、精确 Axiom IR 和明确任务输入成立的结论。它不证明任务规范表达了真实意图，不证明输入来自真实世界的事实正确，也不把工具自述升级为独立证明。

本文投影的语义文件 UTF-8 SHA-256 摘要为：

`sha256:6b18d65eefa439956db8eebe1f4ce90e08b4def4abf7c718c2605e7528598d0d`

有效 Evidence 必须记录该精确摘要；不得使用 `latest`、分支名或可变 URL。

## 设计结论

1. Axiom Evidence v0.1 是闭合、严格版本化的 canonical JSON 文档，规范字节使用 RFC 8785 JCS。
2. Evidence 文档绑定规范 Axiom IR 字节、IR 文档域摘要、语义摘要、工具和所有被引用外部制品；摘要不证明内容正确。
3. 验证义务的声明与结果分离。义务 ID 只由 Evidence 版本域、类别、期望状态和目标决定，不能因 profile 或求解结果改变。
4. `proved`、`checked`、`unknown`、`failed` 和 `trusted` 是不可互换的状态；义务期望决定哪些状态可以完成该义务。
5. 独立检查器必须从 IR 与义务 profile 重新生成义务集合，不能只检查生产报告列出的条目，否则遗漏义务会被误判为成功。
6. `failed` 必须绑定可重放反例；“最小反例”和“已缩减见证”必须区分。
7. 信任假设与未覆盖性质分别建模。前者是结论依赖的外部声明，后者是 Evidence 明确不作出的声明。
8. Evidence 内的 `conclusion` 是按本文规则计算的生产结论；独立检查结果必须存在于 Evidence 之外并绑定 Evidence 文档摘要，不能让报告自证。
9. 首版允许求解后端结果在显式信任边界下支撑 `proved`，但独立检查器必须准确报告它只检查了绑定与声明，没有检查证明真值。
10. 时间戳、主机路径、随机 invocation ID、自由文本日志和性能指标不进入核心 Evidence；基准实验可用摘要绑定的伴随记录保存这些非确定信息。

## 顶层文档

规范文档是一个恰好包含以下成员的 JSON object。下列 `<...>` 只表示结构，不是有效制品：

```json
{
  "artifacts": [],
  "conclusion": { "kind": "<conclusion>", "refs": [] },
  "digest_algorithm": "sha-256",
  "evidence_version": "0.1",
  "executions": [],
  "format": "axiom-evidence",
  "obligation_profile": {
    "name": "<profile>",
    "version": "0.1"
  },
  "obligations": [],
  "producer": "sha256:<tool-id>",
  "subject": { "kind": "<subject>" },
  "tools": [],
  "trust": [],
  "uncovered": []
}
```

| 成员 | 含义 | v0.1 规则 |
| --- | --- | --- |
| `format` | 格式族 | 必须精确为 `axiom-evidence` |
| `evidence_version` | Evidence 数据模型与规范化版本 | 必须精确为 `0.1` |
| `digest_algorithm` | 全部 v0.1 内容摘要算法 | 必须精确为 `sha-256` |
| `subject` | 被验证或被结构拒绝的 IR | 使用本文定义的闭合 tagged union |
| `obligation_profile` | 完整义务生成和结论规则 | 只能使用本文的两个 v0.1 profile |
| `artifacts` | 外部字节制品的摘要清单 | 按 `content_digest` 排序且唯一 |
| `tools` | 生产者、验证器、后端、执行器和检查器身份 | 按 `id` 排序且唯一 |
| `producer` | 生成本文档的 tool ID | 必须解析到含 `evidence-producer` role 的条目 |
| `executions` | 证明、检查、执行或比较尝试 | 按 `id` 排序且唯一 |
| `obligations` | 完整义务集合与每项结果 | 按 `id` 排序且唯一 |
| `trust` | 结论实际依赖的外部假设 | 按 `id` 排序且唯一 |
| `uncovered` | Evidence 明确不覆盖的性质 | 按 `id` 排序且唯一 |
| `conclusion` | 对全部必需义务的确定性聚合 | 必须由独立检查器重算 |

顶层与所有嵌套 object 都是闭合的。未知成员、缺失成员、重复成员、未知 tag 或用 `null` 代替空数组必须拒绝。

## JSON、规范字节与内容标识

Evidence 使用与 Axiom IR v0.1 相同的基础 JSON profile：

- 输入必须满足 I-JSON / JCS 的 Unicode、重复成员和字符串要求；
- 禁止 JSON number 与 `null`；
- 布尔量使用 JSON boolean；
- 数量、字节长度、资源限额、索引和其他整数使用规范十进制字符串；
- 名称按 Unicode scalar value 精确比较，不做 NFC、大小写折叠或 locale 处理；
- 规范机器字节是 JCS UTF-8 字节，无 BOM、额外空白或末尾换行；
- pretty 投影使用两个空格、LF 和一个末尾换行，重新规范化后必须恢复相同机器字节。

所有内容标识写作 `sha256:<64 个小写十六进制字符>`。条目 ID 按以下公式计算：

`id = "sha256:" + hex_lower(SHA-256(UTF8(domain) || NUL || JCS(definition)))`

固定域为：

| 对象 | `domain` |
| --- | --- |
| 工具 | `axiom-evidence-v0.1:tool` |
| 执行记录 | `axiom-evidence-v0.1:execution` |
| 验证义务 | `axiom-evidence-v0.1:obligation` |
| 信任项 | `axiom-evidence-v0.1:trust` |
| 未覆盖项 | `axiom-evidence-v0.1:uncovered` |
| 完整 Evidence | `axiom-evidence-v0.1:document` |

工具、执行、义务、信任和未覆盖条目统一包含 `id` 与 `definition`；`id` 不进入自身 definition。义务的 `result` 也不进入 definition，因此同一义务得到不同结果时 ID 保持不变，完整 Evidence 文档摘要则会变化。

完整 Evidence 的文档域摘要不写回文档。独立检查结果、外部清单和基准记录必须记录该摘要，避免自引用。

## Subject 与外部制品

### 合法 IR subject

能够通过 Axiom IR v0.1 严格规范检查的 subject 为：

```json
{
  "ir_artifact": "sha256:<规范 IR 原始字节摘要>",
  "ir_document_digest": "sha256:<Axiom IR 文档域摘要>",
  "ir_version": "0.1",
  "kind": "axiom-ir",
  "semantics": {
    "name": "keyed-finite-table-semantics",
    "sha256": "<精确语义摘要>"
  }
}
```

`ir_artifact` 是对规范 IR 机器字节直接计算的 SHA-256，用于外部制品解析；`ir_document_digest` 按 Axiom IR 的文档域分离规则计算。两者用途不同，检查器必须分别重算，不能互相代替。

### 被拒绝 IR subject

无法形成合法规范 IR 时仍可生成结构拒绝 Evidence：

```json
{
  "artifact": "sha256:<候选原始字节摘要>",
  "kind": "rejected-ir"
}
```

此变体只能使用结构 profile、结构检查义务与 `structure_rejected` 结论。它不得填入伪造的 `ir_version`、语义摘要或 IR 文档域摘要。

### Artifact 描述

每个外部制品描述恰好包含：

```json
{
  "byte_length": "123",
  "content_digest": "sha256:<原始字节摘要>",
  "format": "<稳定格式标识>",
  "format_version": "<精确版本>"
}
```

`content_digest` 对解析器实际消费的原始字节计算，不做换行、Unicode 或归档重写。相同 digest 只能出现一次。`format` / `format_version` 可以标识后续基准数据、证明证书或工具制品；Evidence 解析器可以保留未知格式，但依赖未知格式的重放或证明检查必须报告 `incomplete`，不能假设兼容。

所有 artifact 引用必须在顶层清单中恰好解析一次；未被引用的 artifact 必须拒绝，防止无关字节改变 Evidence 摘要。

Evidence 不记录文件路径、URL、凭据或获取 fallback。调用方通过 digest resolver 提供字节；缺失字节不会使已有摘要变成错误声明，但会阻止依赖它的独立重放。

## 工具与执行记录

### 工具身份

工具 definition 包含：

- `artifact`：工具二进制、脚本包或可重建源制品的 content digest；
- `name`：稳定工具名；
- `roles`：按字典序排序的非空集合；
- `version`：精确版本或不可变修订标识，不能是 `latest`、分支名或版本范围。

v0.1 role 集合为：

- `evidence-producer`；
- `ir-normalizer`；
- `obligation-generator`；
- `prover`；
- `certificate-checker`；
- `fixture-checker`；
- `host-executor`；
- `output-comparator`；
- `counterexample-replayer`。

一个工具可以具有多个 role，但生产路径与独立检查路径不得只用同一个实现自证。若二者共享库、规则生成器或后端，必须形成相应信任项。

### 执行记录

执行 definition 包含：

- `inputs` / `outputs`：按 `(role, artifact)` 排序的制品引用；
- `kind`：`normalize`、`generate-obligations`、`prove`、`check-certificate`、`check-fixture`、`execute-host`、`compare-output` 或 `replay-counterexample`；
- `limits`：按 `(name, unit)` 排序的显式资源限额，可以为空；
- `result`：执行级结果；
- `tool`：执行该步骤的 tool ID。

执行级 `result.kind` 为：

- `completed`；
- `timeout`；
- `resource-exhausted`；
- `unavailable`；
- `unsupported`；
- `error`。

`completed` 精确表示为 `{ "kind": "completed" }`；其余结果恰好包含 `kind` 与稳定、非自由文本的 `code`。`error` 不得被重试或 fallback 静默改写为完成。耗时、token、主机名、PID、绝对路径和日志属于伴随实验记录；它们不是证明支持。

## 义务 profile 与完整性

### Profile

v0.1 提供两个精确 profile：

| 名称 | 用途 | 必需范围 |
| --- | --- | --- |
| `keyed-finite-table-verification` | 对合法 IR 的静态验证 | IR 结构、全部内建语义义务、全部契约与信任边界 |
| `keyed-finite-table-benchmark` | ADR 0002 的完整纵向闭环 | verification 全部内容，加具体输入、宿主执行、输出与黄金比较 |

两者的 `version` 都是 `0.1`。`rejected-ir` subject 只执行 verification profile 的结构拒绝分支。

### 义务种类与期望

义务 definition 包含：

- `expectation`：`prove`、`check` 或 `trust`；
- `kind`：下表中的精确类别；
- `subject`：文档、声明、节点、契约、接口、字段、表达式路径或外部制品 anchor。

| `kind` | 典型 subject | `expectation` |
| --- | --- | --- |
| `ir-structure` | 合法或被拒绝的 IR 文档 | `check` |
| `input-conformance` | 输入 port 与具体输入制品 | `check` |
| `totality` | 表达式、节点或程序 | `prove` |
| `effect-empty` | 程序与节点 | `prove` |
| `key-cardinality` | 表节点、join 或输出 | `prove` |
| `numeric-range` | 算术表达式或聚合 | `prove` |
| `row-coverage` | filter、join、group 或输出保证 | `prove` |
| `group-conservation` | group 节点与聚合字段 | `prove` |
| `field-origin` | 输出字段、控制依赖或保证 | `prove` |
| `noninterference` | 非干扰 contract | `prove` |
| `contract-guarantee` | guarantee contract | `prove` |
| `host-conformance` | IR、具体输入与宿主输出 | `check` |
| `output-conformance` | 实际输出与模式 / 黄金制品 | `check` |
| `trust-boundary` | 一项实际外部假设 | `trust` |

Anchor 使用闭合 tagged union。节点与契约 anchor 必须包含内容 ID；嵌套表达式使用相对于 definition 的路径数组，每个路径元素是成员名或规范非负索引字符串。接口与字段 anchor 使用精确名称；制品 anchor 使用 content digest。路径无法解析或解析后类别不匹配必须拒绝。

### v0.1 义务生成位置

profile 对以下位置恰好各生成一项对应义务，生产者不得自行合并：

| IR / 运行位置 | 必需义务 |
| --- | --- |
| subject 文档 | 一个 `ir-structure`；合法 IR 另有一个程序级 `effect-empty` |
| 每个非 input 转换节点 | 一个 `totality` 和一个 `key-cardinality` |
| 每个 `int_add` / `int_sub` / `fixed_add` / `fixed_sub` 表达式 | 一个以规范表达式路径定位的 `numeric-range` |
| 每个 `count` / `sum` 聚合 | 一个 `numeric-range`；group 节点另有一个 `group-conservation` |
| 每个 filter、map 或 lookup_join 节点 | 一个 `row-coverage` |
| 每个命名输出的每个字段 | 一个 `field-origin` |
| 每个 guarantee contract | 一个 `contract-guarantee` |
| 每个 noninterference contract | 一个 `noninterference` |
| 每项实际外部信任 | 一个 `trust-boundary` |
| benchmark profile 的每个具体 input port / 制品 | 一个 `input-conformance` |
| benchmark profile 的每次宿主输出 | 一个 `host-conformance`；每个模式 / 黄金比较另有一个 `output-conformance` |

嵌套表达式以其所在 node / contract ID 和从 definition 根开始的规范路径区分；相同子式出现在不同路径时是不同义务。相同 `(kind, expectation, subject)` 只能出现一次。类型、绑定、字段闭合等无法形成合法 IR 的问题归入 `ir-structure`，不伪造核心证明义务。

### 完整义务集合

独立检查器必须按 profile 从 subject IR 重新生成义务 definition 与 ID，并与 Evidence 中的集合精确比较：

1. 首先生成 IR 结构与 subject 绑定检查；结构失败时只允许结构拒绝分支。
2. 遍历所有类型、表达式、转换节点、输出和契约，为每个现行语义要求的总性、效果、键、基数、范围、覆盖、分组、来源和非干扰位置生成义务。
3. 由 IR 内建规则产生的义务不能因为 contract 未重复声明而省略；contract 义务也不能因为存在内建检查而省略。
4. verification profile 要求对每项实际依赖的规范、标签、解码、后端、生成器或密码学边界生成 trust obligation。
5. benchmark profile 还要为每个具体输入、宿主执行、输出模式与黄金比较生成 check obligation，并列出宿主和数据来源边界。
6. 多出的未知义务、缺少义务、ID 不匹配、重复义务或错误 expectation 都必须拒绝整份 Evidence。

生产义务生成器可以优化遍历，但独立检查器必须依据规范重建，而不是导入生产生成器的结果作为真相源。

## 五种状态

每项义务恰好有一个 `result`。状态与 expectation 的允许关系为：

| expectation | 完成状态 | 阻断状态 | 禁止状态 |
| --- | --- | --- | --- |
| `prove` | `proved` | `failed`、`unknown` | `checked`、`trusted` |
| `check` | `checked` | `failed`、`unknown` | `proved`、`trusted` |
| `trust` | `trusted` | `unknown` | `proved`、`checked`、`failed` |

### `proved`

`proved` 表示：在 IR 的 `WF` / `Pre` 与 result 列出的 trust assumptions 成立时，生产验证路径声称该性质对全部有效输入成立。它必须使用以下 support 之一：

- `kernel-replay`：独立检查器可从 IR 和受支持规则直接重演；
- `certificate`：绑定证明证书制品、证书 checker、检查 execution 和格式版本；
- `backend-attestation`：绑定规范查询、后端响应、prove execution 与 proof-backend trust 项。

`backend-attestation` 可以透明地支撑生产侧 `proved`，但独立检查器只能报告“绑定成立、后端受信任”，不能声称已经检查证明。若发布或基准策略要求可检查证书，此 support 会使独立结果为 `incomplete`。

`proved` result 恰好包含 `assumptions`、`kind` 与 `support`。`assumptions` 是按 ID 排序的实际 trust 集合。support 变体为：

```json
{ "execution": "sha256:<execution-id>", "kind": "kernel-replay" }
```

```json
{
  "artifact": "sha256:<certificate-artifact>",
  "execution": "sha256:<prove-execution-id>",
  "kind": "certificate"
}
```

```json
{
  "execution": "sha256:<prove-execution-id>",
  "kind": "backend-attestation",
  "query": "sha256:<query-artifact>",
  "response": "sha256:<response-artifact>",
  "trust": "sha256:<proof-backend-trust-id>"
}
```

certificate 的格式和版本从 artifact 描述读取；backend support 的 trust 必须同时出现在 assumptions。

### `checked`

`checked` 只表示对明确列出的有限制品或一次可重放执行完成动态检查。它必须引用 `check-fixture`、`execute-host`、`compare-output` 或 `replay-counterexample` execution 以及所有输入 / 输出制品。它不能证明未枚举输入上的性质，也不能完成 `prove` expectation。

`checked` result 恰好包含按 ID 排序的 `artifacts`、按 trust ID 排序的 `assumptions`、一个 `execution` 和 `kind: "checked"`。

### `unknown`

`unknown` 不作真或假的结论，必须包含非空 attempts 和以下原因之一：

- `timeout`；
- `resource-exhausted`；
- `backend-unavailable`；
- `unsupported`；
- `incomplete-certificate`；
- `indeterminate`；
- `operational-error`。

attempt 引用的 execution 结果必须与原因一致。资源耗尽、工具错误或证书缺失不得改写为 `failed`，重复执行成功也不能删除已发生的 attempt；新的 Evidence 可以记录新的完整结果并产生新文档摘要。

`unknown` result 恰好包含按 execution ID 排序的非空 `attempts`、`kind: "unknown"` 与 `reason`。

### `failed`

`failed` 表示存在具体反例或有限检查差异。它必须引用一个符合下一节的 counterexample，以及一次完成的 counterexample replay 或具体比较 execution。无法重放的求解器 model 不能单独支撑 `failed`；它只能作为待缩减候选，最终无法重放时应为 `unknown`。

`failed` result 恰好包含按 trust ID 排序的 `assumptions`、`counterexample`、一个 `execution` 和 `kind: "failed"`。

### `trusted`

`trusted` 只允许用于 `trust-boundary` 义务，并且必须引用一项 trust ID。它表示 Evidence 的其他结论依赖该声明，但核心没有证明它。将核心 `prove` / `check` 义务改写成 trust obligation、用 trust 解除 `failed` / `unknown`，或把未知后端结果标成 trusted，都必须拒绝。

`trusted` result 恰好为 `{ "kind": "trusted", "trust": "sha256:<trust-id>" }`。

## 反例与见证

### Counterexample 共同结构

counterexample 是闭合 object，包含：

- `kind`：`structure-path`、`single-row`、`row-pair`、`missing-key`、`group` 或 `paired-input`；
- `minimality`：最小性声明；
- `preconditions`：适用于该 world 且已满足的 IR assume contract ID 完整集合，按 ID 排序；
- `trace`：从 subject 到观察结果的非空路径；
- `worlds`：零个、一个或两个规范输入世界；
- `observed`：实际值、故障、行集合或依赖结果。

`structure-path` 不含输入 world；普通数据见证恰好一个 world；非干扰 `paired-input` 恰好两个 world，且检查器必须重算公开等价与输出差异。

### 输入 world 与值

每个 world 包含按 input port 名排序的表；表内行按 IR 主键的规范值顺序排列且键唯一。每行必须恰好符合对应闭合记录类型，不能只提交省略前置条件所需字段的“相关片段”。Evidence 的 trace 可以单独标出真正相关的键与字段。

值采用闭合 tagged union：

- `bool`：JSON boolean；
- `int`：规范有符号十进制字符串；
- `fixed`：`scale` 与 `coefficient` 的规范十进制字符串；
- `text`：Unicode string；
- `enum`：枚举类型 ID 与成员；
- `none` / `some`：显式 Option；
- `record`：记录类型 ID 和按字段名排序的完整字段数组。

检查器必须依据 IR 类型重算值类型、范围、字段闭合、键、全部 applicable assume 和前置条件。counterexample 中缓存的类型判断、公开等价或输出差异不能作为第二真相源。

### Trace

trace step 只能是文档、候选字节范围、节点、表达式 anchor、contract、input/output port、表行键、字段、数学操作、分组或观察结果的已知 tag。候选字节范围用规范非负 offset / length 定位无法解析或非规范的原始输入；其他每一步必须从前一步解析到 IR、world 或重放结果。自由文本解释可以存在于摘要绑定的伴随诊断中，但不能替代机器路径。

### 最小性

`minimality.kind` 为：

- `proved-minimal`：工具已经证明不存在按声明顺序更小的有效见证；
- `reduced`：工具执行了确定性缩减，但没有证明全局最小；
- `unreduced`：只保证见证有效。

v0.1 的缩减顺序标识固定为 `axiom-witness-order-v0.1`。同一种 counterexample 先比较总行数，再比较完整字段值的 JCS 总字节数，最后比较整个 worlds 的 JCS 字节字典序。删除或改写后仍必须满足 `WF`、`Pre` 和失败义务所需的公开等价条件。

`proved-minimal` 必须绑定能够复核最小性的 proof support；仅运行贪心删除、求解器首次返回 model 或得到较短 JCS 不能使用该标签。首批基准允许使用 `reduced`，但必须准确显示。

## 信任与未覆盖项

### Trust

trust definition 包含：

- `category`：信任类别；
- `claim`：非空、稳定的具体声明；
- `mitigations`：按 `(kind, ref)` 排序的证书、交叉实现、重放或人工审阅引用；
- `scope`：IR、artifact、tool、execution、字段标签或外部规范 anchor。

v0.1 category 为：

- `specification-intent`；
- `sensitivity-classification`；
- `input-origin`；
- `decoder-normalizer`；
- `proof-backend`；
- `production-generator`；
- `host-runtime`；
- `cryptographic-primitive`。

mitigation 只说明信任面被怎样缩小，不能删除原 trust。若 certificate 已由独立 checker 完整检查，可以把后端 soundness 从该义务的 assumptions 中移除，但证书 checker、规则和密码学仍应在适用处列出。

### Uncovered

uncovered definition 包含 `category`、`scope` 和稳定 `statement`。v0.1 category 为：

- `real-world-intent`；
- `source-truth-completeness`；
- `resource-performance`；
- `timing-memory-log-side-channel`；
- `host-fidelity-for-all-inputs`；
- `legal-regulatory-compliance`；
- `long-term-archival-authenticity`。

trust 表示“结论依赖该声明为真”，uncovered 表示“本文档不对该性质作结论”。二者不能合并成一个风险字符串，也不能因为一项性质未覆盖就把相关核心失败隐藏起来。

## 结论聚合

`conclusion.kind` 为：

- `satisfied`；
- `structure_rejected`；
- `input_rejected`；
- `violated`；
- `inconclusive`；
- `implementation_inconsistent`。

`refs` 必须按 ID 排序，精确列出决定该结论的义务或 execution。独立检查器按以下优先级重算：

1. 被拒绝 subject 的结构检查为 `failed`，结论是 `structure_rejected`。
2. 合法 IR 的任一 input-conformance 为 `failed`，结论是 `input_rejected`；不评价程序对有效输入的正确性。
3. 核心义务均为 `proved`，但 host/output check 为 `failed`，结论是 `implementation_inconsistent`。
4. 任一其他 `prove` 或 `check` 义务为 `failed`，结论是 `violated`。
5. 任一必需义务为 `unknown`、期望状态未满足、必需 execution 未完成或 profile 制品缺失，结论是 `inconclusive`。
6. 只有 profile 的全部 `prove` 为 `proved`、全部 `check` 为 `checked`、全部 `trust` 为 `trusted`，且没有上述阻断项时，结论才是 `satisfied`。

若同时有已重放的失败和未知尝试，失败结论优先，但未知 attempts 仍保留。`satisfied` 从不表示“无信任”或“符合真实意图”；UI 和摘要必须同时展示 trust 与 uncovered 数量及类别。

## 独立检查边界

独立 Evidence 检查器必须小于生产编译、验证和报告路径，并至少完成：

1. 严格解析 Evidence v0.1，拒绝未知字段、tag、版本和非规范字节；
2. 重算全部条目 ID、Evidence 文档摘要和可获得 artifact content digest；
3. 严格检查或拒绝 subject IR，重算 IR 文档域摘要与语义绑定；
4. 按 profile 独立生成完整义务集合，检查遗漏、多余、期望、依赖和 anchor；
5. 检查五种状态与 support 的结构、工具 role、execution 结果和 trust 引用；
6. 重放可获得的 concrete check 与 failed counterexample；
7. 检查受支持 kernel rule 或 certificate；对 backend attestation 明确保留 proof-backend trust；
8. 重算 conclusion，并拒绝生产结论不一致；
9. 输出绑定 Evidence 文档摘要、checker 身份、每项检查级别、剩余 trust 和缺失 artifact 的独立结果。

独立结果至少区分：

- `accepted`：所有必需结构、重放与当前策略要求的 proof support 已独立检查；
- `accepted-with-trust`：Evidence 自洽且可重放，但仍有明确列出的 backend 或其他 trust；
- `incomplete`：Evidence 结构自洽，但缺少 artifact、规则或受支持证书，无法完成要求级别；
- `rejected`：格式、摘要、义务完整性、状态、反例、结论或引用不一致。

独立结果不是 Evidence 的成员，也不能复用生产 `producer` 身份。检查器实现、JCS、SHA-256、规则解释和证书 checker 仍属于其自身可信计算基；这些边界必须随独立结果报告。

## 拒绝边界

以下情况必须拒绝整份 Evidence，而不是降级为 warning：

- 未知 `evidence_version`、profile、顶层 / 嵌套字段、tag、状态、support 或摘要算法；
- 非规范 JSON、JSON number、`null`、重复成员、非规范整数或错误数组顺序；
- 条目 ID、artifact digest、IR 文档域摘要或语义摘要不匹配；
- subject 变体与 conclusion / profile 不一致；
- 义务缺失、多余、重复、anchor 无法解析或 expectation 错误；
- 用 `checked` / `trusted` 完成 `prove`，用 `proved` 完成具体 check，或把核心义务伪装成 trust-boundary；
- `proved` 无 support，`failed` 无可重放 counterexample，`unknown` 无真实 attempt，`trusted` 无 trust 项；
- 反例不满足 `WF` / `Pre`、没有违反目标义务，或非干扰 worlds 不公开等价；
- 把启发式缩减标为 `proved-minimal`；
- conclusion 与确定性聚合结果不一致；
- production producer 同时冒充独立检查结果。

artifact 暂不可取得、证书格式不受支持或 checker 资源不足，应使独立结果为 `incomplete`，不是篡改 Evidence 或断言原结论为假。

## 版本与迁移

`0.1` 是预稳定公共 Evidence 版本，不承诺与未来 v0.x 或 v1 字节兼容。读取器必须声明精确支持集合；遇到 `0.2` 或 `1.0` 默认拒绝。

以下变化至少提升 v0 minor：

- 顶层或嵌套成员、tag、状态、support、profile 或义务类别变化；
- JCS profile、整数、排序、ID、域分离、摘要或 artifact 绑定变化；
- 义务完整性、结论聚合、反例、信任或独立结果含义变化；
- 从拒绝变忽略、从必需变可选或扩大未知格式的默认接受范围。

迁移必须严格读取源版本，使用具名迁移规则产生目标 Evidence，重新绑定目标版本的义务、状态、摘要和独立检查结果。不得原地把旧 Evidence 解释成新语义；不得跨 IR 文档摘要或语义摘要复用 `proved` / `checked` 结果。

进入 `1.0` 前至少要求四个基准的正例、错误候选、状态误用、反例重放、义务遗漏、artifact 篡改、独立检查不一致和一次显式迁移演练全部通过。v1 后兼容承诺另行冻结。

## 信任与明确不包含

Axiom Evidence v0.1 仍不证明或提供：

- 任务规范完整表达真实业务意图；
- 数据来源真实、完整、及时或敏感标签正确；
- 求解器、编译器、运行时、JCS 或 SHA-256 实现无缺陷；
- 未提供可检查证书时的后端证明已经被独立复核；
- 常数时间、无内存 / 日志侧信道、性能上界或生产规模终止；
- 数字签名、发布者身份、时间戳、公证、透明日志或长期归档真实性；
- 归档、压缩、分块、流式传输、网络媒体类型或远程 artifact resolver；
- 真实秘密、生产数据或法规合规的安全处理；首批 Evidence 和反例只使用合成数据；
- 任意外部效果、并发、事务、网络工作流或通用语言能力。

这些边界不能用 `satisfied`、`accepted`、`verified` 或产品宣传省略。

## 基准映射与验证门禁

四个基准的状态映射、必需负向矩阵和进入实现前的 Evidence 验收条件见 [Axiom Evidence v0.1 验证矩阵](axiom-evidence-v0-validation.md)。该文件是本文的规范性验证伴随文档；本文完成不授权创建编译器、验证器、求解器接线或独立检查器骨架。

## 变更要求

修改 v0.1 顶层模型、规范化、内容标识、profile、义务完整性、五种状态、反例、trust / uncovered、结论聚合、拒绝边界或独立检查语义，属于公共格式变化，必须同步本文、现行语义、Axiom IR、ADR 0002 基准语料库、独立检查器与兼容性测试。只改变人类摘要、CLI 展示或伴随实验记录而不改变规范字节和判断，不要求提升 Evidence 版本。
