# Axiom IR v0.1：规范化形式与版本策略

更新日期：2026-08-20

状态：Normative（现行规范）

用途：定义首个目标领域的 Axiom IR v0.1 数据模型、规范化规则、内容标识、无损人类投影、语义差异和版本演进策略，使编译器、验证器、Evidence 生成器与独立检查器共享一个机器语义真相源。

读者：前端、IR、验证与 Evidence 设计者，实现者，基准维护者，以及准备实现独立规范检查器的审阅者。

不包含：`.rax` 表面语法、验证义务的求解算法、Axiom Evidence 容器与证书、宿主运行时、代码生成、优化器、源码位置格式或实现语言。

## 规范地位与输入语义

本文投影[有键有限表转换的现行语义](../semantics/keyed-finite-table-semantics.md)，不得改变其中的值域、表、算术、连接、聚合、非干扰、效果或失败含义。Axiom IR v0.1 绑定该语义文档的 UTF-8 文件内容摘要：

`sha256:6b18d65eefa439956db8eebe1f4ce90e08b4def4abf7c718c2605e7528598d0d`

IR 中必须记录这一精确摘要。语义文件发生任何字节变化后，新生成的 IR 必须记录新摘要；工具不能把 `latest`、分支名或可变 URL 当作语义版本。将来冻结稳定语义版本后，可以增加从稳定版本到精确摘要的受审计映射，但不能抹去原始摘要。

本文中的 JSON 片段若含 `<...>`，仅用于展示结构，不是有效 Axiom IR 制品。规范制品中的标识、摘要和全部字段必须满足本文规则。

## 设计结论

Axiom IR v0.1 采用以下边界：

1. IR 是带类型契约的不可变有向无环图（DAG），不是指令流、AST 转储或宿主语言对象图。
2. 抽象数据模型使用严格、闭合的 tagged object；规范机器编码使用 canonical JSON。
3. canonical JSON 字节采用 [RFC 8785 JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785.html)（JCS）。项目把该行为作为格式规则，即使 RFC 8785 本身属于 Informational RFC。
4. 所有语义整数、范围、scale、容量和绑定索引都编码为十进制 JSON 字符串；IR v0.1 不使用 JSON number，避免 IEEE 754 精度进入语义。
5. enum、记录类型、表类型、图节点和契约条目都以 SHA-256 内容寻址；标识可以被独立重算。
6. IR 制品不内嵌自身摘要；Axiom Evidence 和外部清单计算并记录规范文档摘要，避免自引用。
7. 未知版本、字段、tag、效果或算法一律拒绝，不做“尽力理解”或静默忽略。
8. 源码 span、注释、显示名、优化提示和后端私有数据不进入规范 IR；它们只能存在于按 IR 摘要绑定的伴随制品中。

### “规范化唯一”的准确含义

规范化保证：一个已经确定的 Axiom IR 抽象值只有一份规范字节表示；字段顺序、空白、无意义数组顺序、alpha 名称和重复子图等表示差异不会制造不同规范结果。

规范化**不**声称解决任意程序等价。除本文明确列出的布尔交换、结合、幂等等规则外，数学上等价但结构不同的表达式仍可产生不同 IR，例如 `x + 0` 与 `x`。验证器可以证明它们等价，但规范器不得借助未记录的求解器结论改写 IR。

## 顶层文档

规范文档是一个恰好包含以下成员的 JSON object：

```json
{
  "contracts": [],
  "digest_algorithm": "sha-256",
  "effects": [],
  "enum_types": [],
  "format": "axiom-ir",
  "ir_version": "0.1",
  "nodes": [],
  "outputs": [],
  "record_types": [],
  "semantics": {
    "name": "keyed-finite-table-semantics",
    "sha256": "6b18d65eefa439956db8eebe1f4ce90e08b4def4abf7c718c2605e7528598d0d"
  },
  "table_types": []
}
```

该片段只有在声明、节点、契约和输出的基数规则也满足时才可能成为有效制品；首版完整程序至少有一个 `input` 节点和一个命名输出。

顶层成员含义：

| 成员 | 含义 | v0.1 规则 |
| --- | --- | --- |
| `format` | 格式族 | 必须精确为 `axiom-ir` |
| `ir_version` | IR 数据模型与规范化版本 | 必须精确为 `0.1` |
| `semantics` | 采用的语义真相源 | 名称和 SHA-256 必须明确且受支持 |
| `digest_algorithm` | 内容寻址算法 | 必须精确为 `sha-256` |
| `enum_types` | 名义枚举声明 | 按 `id` 排序，不得重复 |
| `record_types` | 闭合记录类型声明 | 按 `id` 排序，不得重复 |
| `table_types` | 有键有限表类型声明 | 按 `id` 排序，不得重复 |
| `nodes` | 转换 DAG 节点 | 按 `id` 排序，不得重复或成环 |
| `outputs` | 对外可观察的命名输出 | 按 `name` 排序，名称唯一，至少一个 |
| `contracts` | 输入假设、输出保证和非干扰策略 | 按 `id` 排序，全部为必需契约 |
| `effects` | 核心效果声明 | v0.1 必须是空数组 `[]` |

对象是闭合的：缺少任何成员、增加未知成员、使用别名或用 `null` 代替空数组都必须拒绝。

## JSON 与标量规范化

### 输入约束

可规范化输入必须满足 I-JSON / JCS 的基础限制：JSON object 不得含重复成员名，字符串必须是合法 Unicode，解析后字符串内容不得被规范化或改写。Axiom IR 进一步规定：

- 禁止全部 JSON number token；
- 禁止 `null`；缺失值只能使用 Axiom `Option` tag；
- 只有布尔字面量使用 JSON `true` / `false`；
- 所有 object 都必须符合本文列出的精确成员集合；
- 字符串不得包含未配对 surrogate；名称不得包含 C0 / C1 控制字符；
- 名称非空，按 Unicode scalar value 序列精确比较，不做 NFC、大小写折叠或 locale 处理。

JCS 负责 object member 排序、字符串转义、无额外空白和 UTF-8 输出。Axiom 规则负责数组语义顺序、类型 tag、十进制字符串和内容标识。

### 十进制字符串

有符号整数必须匹配：

`0|-?[1-9][0-9]*`

非负整数必须匹配：

`0|[1-9][0-9]*`

因此禁止 `+1`、`01`、`-0`、指数、小数点和空字符串。解析器必须按数学整数处理，不得先转为宿主浮点数。范围上下界、定点系数、scale、表容量和绑定索引分别使用适用的有符号或非负形式。

### 规范机器字节与人类投影

规范机器字节是规范抽象值经 JCS 产生的 UTF-8 字节，不含 BOM、额外空白或末尾换行。摘要只对这些字节计算。

无损人类投影是同一抽象值的 pretty JSON：object member 仍按 JCS 顺序、数组仍按本文顺序，使用两个空格缩进、LF 和一个末尾换行。它不得包含注释或额外元数据。人类投影不是第二真相源；重新解析并规范化后必须逐字节恢复机器表示。

## 内容寻址与摘要

### 标识格式

所有内容标识写作：

`sha256:<64 个小写十六进制字符>`

不接受大写十六进制、Base64、缩写或无算法前缀。SHA-256 碰撞抗性属于明确密码学信任假设；内容标识只证明字节一致，不证明语义正确、来源可信或程序安全。

### 域分离

对类型、节点和契约的 `definition` 先完成 Axiom 数组规范化，再经 JCS 得到字节 `B`。标识按以下公式计算，其中 `NUL` 是单个 `0x00` 字节：

`id = "sha256:" + hex_lower(SHA-256(UTF8(domain) || NUL || B))`

固定域为：

| 对象 | `domain` |
| --- | --- |
| 枚举类型 | `axiom-ir-v0.1:enum-type` |
| 记录类型 | `axiom-ir-v0.1:record-type` |
| 表类型 | `axiom-ir-v0.1:table-type` |
| 节点 | `axiom-ir-v0.1:node` |
| 契约 | `axiom-ir-v0.1:contract` |
| 完整文档 | `axiom-ir-v0.1:document` |

声明条目、节点和契约条目统一表示为：

```json
{
  "definition": { "...": "..." },
  "id": "sha256:<重算所得值>"
}
```

`id` 不进入自身 `definition` 的摘要。若同一数组中两个条目重算出相同 ID，只保留一个；规范输入包含重复 ID 必须拒绝，而不是由读取器静默去重。

### 文档摘要

完整文档摘要使用同一公式和 `axiom-ir-v0.1:document` 域，对规范机器字节计算。摘要不写回 IR 文档；Axiom Evidence 必须同时记录 `ir_version`、语义摘要和该文档摘要。

任何声明、节点、契约、接口名称、顺序有语义的数组或语义摘要变化都会改变文档摘要。pretty 格式、源码 span 和伴随诊断变化不应改变摘要。

## 类型声明

### 内联值类型

值类型是带 `kind` 的闭合 object：

| `kind` | 必需成员 | 语义 |
| --- | --- | --- |
| `bool` | `kind` | `Bool` |
| `int` | `kind`, `lower`, `upper` | `Int[lower, upper]` |
| `fixed` | `kind`, `scale`, `lower`, `upper` | `Fixed[scale, lower, upper]` |
| `text` | `kind` | `Text` |
| `enum` | `kind`, `enum_type` | 引用名义枚举 ID |
| `option` | `kind`, `inner` | `Option<inner>` |
| `record` | `kind`, `record_type` | 引用闭合记录类型 ID |

`int` / `fixed` 的 `lower`、`upper` 是有符号十进制字符串且 `lower <= upper`；`fixed.scale` 是非负十进制字符串。`option.inner` 必须是完整值类型。递归记录类型不属于 v0.1；记录引用图必须无环。

### 枚举类型

枚举 `definition` 恰好包含：

```json
{
  "members": ["first", "second"],
  "name": "Example"
}
```

`name` 参与名义相等，`members` 非空、唯一且顺序有语义，用于枚举规范顺序。成员不能排序或去重后改变原声明顺序。两个具有相同成员但名称不同的枚举是不同类型。

### 记录类型

记录 `definition` 恰好包含 `fields`：

```json
{
  "fields": [
    {
      "label": "public",
      "name": "field_name",
      "type": { "kind": "text" }
    }
  ]
}
```

`fields` 非空，按 `name` 的 Unicode scalar 顺序排列且名称唯一。`label` 只能是 `public` 或 `sensitive`。记录是结构类型：规范化后字段、类型和标签相同的记录共享同一 ID；源代码中的类型别名不进入 IR。

### 表类型

表 `definition` 恰好包含：

```json
{
  "capacity": "100",
  "primary_key": ["order_id"],
  "record_type": "sha256:<record-type-id>"
}
```

`capacity` 是非负十进制字符串。`primary_key` 非空、字段唯一且顺序有语义；字段必须存在于记录中、标记 `public`，并满足现行语义的可键类型约束。相同记录、容量和主键序列产生相同表类型 ID。

外键不是表类型的一部分，因为它关联具体输入接口。外键使用输入契约中的 `lookup` / `is_some` 公式表达，从而避免类型声明相互内容寻址形成循环。

## 表达式核心

### 绑定环境

表达式用无名称绑定索引消除 alpha 重命名差异：

```json
{ "index": "0", "op": "bound" }
```

`index = 0` 指当前环境第一个值。新 binder 总是插入索引 0，原有索引依次加一。节点的初始环境固定为：

- `filter` / `map`：`[source_row]`；
- `lookup_join` 投影：`[left_row, right_row]`；
- 顶层契约公式：`[]`。

`forall_rows`、`exists_rows`、`count_where`、`sum_where` 和 `match_option` 的绑定分支都会在现有环境前插入一个值。越界索引必须拒绝。

字段读取使用：

```json
{
  "field": "order_id",
  "op": "field",
  "record": { "index": "0", "op": "bound" }
}
```

### 字面值与构造

v0.1 表达式 tag 包括：

- `literal_bool`：成员 `value` 为 JSON boolean；
- `literal_int`：成员 `type`、`value`，值必须落入 `int` 类型；
- `literal_fixed`：成员 `type`、`coefficient`，系数必须落入 `fixed` 类型；
- `literal_text`：成员 `value` 为 JSON string；
- `literal_enum`：成员 `enum_type`、`member`；
- `none`：成员 `type` 必须是完整 `option` 类型；
- `some`：成员 `value`；结果类型为 `Option<T>`，其中 `T` 从 `value` 推导；
- `record`：成员 `record_type`、`fields`；字段数组按名称排序并恰好覆盖记录类型；
- `bound` 与 `field`：按上一节定义。

所有 tag 都使用闭合 object，不能额外缓存推导类型或敏感性。类型、字段标签和控制依赖由独立检查器从声明与表达式重算；冗余缓存会形成第二语义源，因此不进入规范 IR。

### 标量操作

| `op` | 结构与规范化 |
| --- | --- |
| `not` | 一个 `value: Bool` |
| `and`, `or` | `values` 至少两个；递归展平同类节点，按子表达式 JCS 字节排序，去除完全相同项；若只剩一项则以该项替代 |
| `eq` | `left` / `right` 类型完全相同；按两侧 JCS 字节排序 |
| `lt`, `le`, `gt`, `ge` | 保留有语义的 `left` / `right` 顺序，只接受现行语义允许的数值比较 |
| `int_add` | `values` 恰好两个，按 JCS 字节排序；带 `result_type: int` |
| `int_sub` | `left` / `right` 顺序有语义；带 `result_type: int` |
| `fixed_add` | 与 `int_add` 相同，但操作数同 scale 且结果为 `fixed` |
| `fixed_sub` | 与 `int_sub` 相同，但操作数同 scale且结果为 `fixed` |
| `if` | `condition: Bool`、`then`、`else` 和 `result_type`；两分支类型等于结果类型 |
| `match_option` | `subject`、`none`、`some`、`result_type`；`some` 分支环境索引 0 为内部值 |

交换与结合规范化只改变结构表示，不引入短路语义。所有表达式仍纯且总；超出 `result_type` 范围产生验证义务，不能靠 IR 规范器扩大范围。

### 契约专用表操作

以下表达式只允许出现在契约公式，不能出现在转换节点的逐行表达式中：

| `op` | 必需成员 | 结果 |
| --- | --- | --- |
| `forall_rows` | `table`, `body` | 为表中每行绑定索引 0，结果 `Bool` |
| `exists_rows` | `table`, `body` | 为表中每行绑定索引 0，结果 `Bool` |
| `lookup` | `table`, `keys` | 按主键顺序查找，结果 `Option<Record>` |
| `count_where` | `table`, `predicate`, `result_type` | 绑定行，返回满足谓词的数学计数 |
| `sum_where` | `table`, `predicate`, `value`, `result_type` | 绑定行，返回数学和 |

`table` 是接口引用，恰好为以下两种之一：

```json
{ "kind": "input", "name": "orders" }
```

```json
{ "kind": "output", "name": "result" }
```

`keys` 按目标表主键字段顺序排列。`count_where` / `sum_where` 在求值 `predicate` 和 `value` 时把当前行插入索引 0；`sum_where.value` 必须是非可选的 `Int` 或 `Fixed`。空集合的数学和为相应类型的零，但仍须证明零落入结果范围。

## 转换 DAG

每个节点条目包含 `id` 和闭合 `definition`。节点 ID 对 definition 内容寻址；definition 中的前驱使用节点 ID。所有引用必须解析，节点图必须有限且无环。相同 definition 只能存在一个节点，多个下游可以共享它。

### `input`

```json
{
  "kind": "input",
  "port": "orders",
  "table_type": "sha256:<table-type-id>"
}
```

`port` 在全部 input 节点中唯一，并定义契约的 input 接口名称。输入节点没有前驱，不执行文件、数据库或网络读取。

### `filter`

```json
{
  "kind": "filter",
  "predicate": { "...": "Bool expression" },
  "source": "sha256:<node-id>",
  "table_type": "sha256:<output-table-type-id>"
}
```

`predicate` 的初始环境为 `[source_row]`。输出记录与主键必须等于源表，容量不得大于源容量；标签与控制依赖按现行语义重算。

### `map`

```json
{
  "fields": [
    { "expression": { "...": "expression" }, "name": "order_id" }
  ],
  "kind": "map",
  "source": "sha256:<node-id>",
  "table_type": "sha256:<output-table-type-id>"
}
```

`fields` 按输出字段名排序并恰好覆盖输出记录。表达式环境为 `[source_row]`。输出主键字段必须是源主键的逐值保留或一一重命名，容量等于源容量上界。

### `lookup_join`

```json
{
  "fields": [],
  "kind": "lookup_join",
  "left": "sha256:<left-node-id>",
  "pairs": [
    { "left": "customer_id", "right": "customer_id" }
  ],
  "right": "sha256:<right-node-id>",
  "table_type": "sha256:<output-table-type-id>"
}
```

`pairs` 非空，字段类型逐对相同；由于合取次序无语义，按 `(left, right)` 的 Unicode scalar 顺序排序并拒绝重复。`fields` 与 `map.fields` 相同但表达式环境为 `[left_row, right_row]`。输出主键保留左键，容量等于左容量。每个左行恰好一个右匹配是必需验证义务，不由规范器假设成功。

### `group`

```json
{
  "aggregates": [
    { "kind": "count", "name": "event_count" },
    { "field": "units", "kind": "sum", "name": "total_units" }
  ],
  "keys": [
    { "name": "account_id", "source_field": "account_id" }
  ],
  "kind": "group",
  "source": "sha256:<node-id>",
  "table_type": "sha256:<output-table-type-id>"
}
```

`keys` 非空，顺序等于输出主键顺序，因此不得重排；`name` 唯一。`aggregates` 按输出 `name` 排序且与 key 名不冲突。`count` 与 `sum` 的结果类型从输出表字段读取并按现行语义产生范围与控制依赖义务。

### 对外输出

`outputs` 条目恰好包含 `name` 与 `node`。名称唯一、按 Unicode scalar 顺序排序；节点必须解析且可以由 input 节点到达。输出名称属于程序接口，重命名是语义差异。

没有独立 `output` 节点，也没有未使用节点：每个非 input 节点必须能到达至少一个命名输出。含死节点的输入必须拒绝；生产者应在提交规范化前删除死节点，防止不可观察内容改变摘要。

## 契约

契约与实现 DAG 分离但共享输入 / 输出接口。这样修改候选实现不会无意义地改变任务契约 ID，Evidence 也能分别引用“要求”和“实现”。每个契约 definition 是以下三种之一。

### 输入假设

```json
{
  "expression": { "...": "Bool contract expression" },
  "kind": "formula",
  "role": "assume"
}
```

假设只能引用 input 接口，不能引用 output 或内部节点。它补充模式、容量、主键等结构事实，例如逐行金额关系和通过 `lookup` 表达的外键。现实来源真实性不能写成可证明假设冒充语言结论。

### 输出保证

```json
{
  "expression": { "...": "Bool contract expression" },
  "kind": "formula",
  "role": "guarantee"
}
```

保证只能引用 input 与命名 output 接口，不能引用内部节点。它描述行覆盖、字段等式、连接来源、聚合定义或其他任务后置条件。所有保证都是必需义务；v0.1 没有 warning、advisory 或默认成功。

### 非干扰

```json
{
  "inputs": ["tickets"],
  "kind": "noninterference",
  "outputs": ["export"]
}
```

`inputs` 和 `outputs` 分别按名称排序、非空且唯一；名称必须解析到 input port 与命名 output。敏感字段集合来自输入记录标签，公开等价与输出相等按现行语义定义。非干扰不是单次运行 Bool 公式，不能降级为普通字段白名单检查。

### 契约范围与内建义务

节点良构、类型、效果、键、容量、恰好一次连接、聚合范围和控制依赖等内建义务由 IR 与现行语义自动产生，不需要复制成 contract 条目。契约只表达任务额外的 `Pre` / `Post` 和非干扰要求。

重复且内容相同的契约共享同一 ID；相同 role 下重复 ID 必须拒绝。彼此逻辑等价但结构不同的公式不会由规范器调用求解器合并。

## 规范化流程

工具必须按以下顺序处理输入，不能先忽略未知内容再计算摘要：

1. 解析 I-JSON，拒绝重复 object member、非法 Unicode、JSON number 和 `null`。
2. 检查 `format`、精确 `ir_version`、语义摘要、摘要算法和每个闭合 object 的成员集合。
3. 校验十进制字符串、名称、枚举成员、类型构造和所有引用格式。
4. 规范表达式：校验无名称绑定索引，展平 / 排序 / 去重 `and`、`or`，排序 `eq` 与加法操作数，规范字段数组和无语义顺序数组。
5. 规范 enum、记录和表类型 definition，重算 ID，解析无环类型依赖。
6. 规范节点 definition，重算节点 ID，解析引用，检查 DAG、类型、效果和可达性。
7. 规范契约 definition，重算契约 ID，检查接口可见性、绑定范围和类型。
8. 按 ID 或名称排列顶层数组，确认无重复、无未知效果且至少一个输入与输出。
9. 生成 JCS 规范机器字节；严格规范检查器还必须确认原始输入字节与结果完全一致。
10. 使用文档域计算摘要，供 Evidence 或外部清单记录。

普通 normalizer 可以接受多余空白、不同 object member 顺序和可安全重排的数组，然后输出规范字节。它不能修复未知字段、错误 ID、悬空引用、循环、类型错误、非空效果或不受支持版本；这些必须拒绝。

## 人类审阅与语义差异

### 伴随来源映射

源码文件、span、注释、Agent 生成轮次、显示标签和诊断提示属于伴随来源映射。该制品必须记录其所对应的 IR 文档摘要，内容可以独立版本化，但不能被验证器当作语义输入。若来源映射丢失，IR 仍必须可检查；若摘要不匹配，映射必须拒绝。

### 结构化差异

IR diff 先规范化两份输入，再按下列层级比较，不能只显示级联 hash 变化：

| 类别 | 示例 | 影响 |
| --- | --- | --- |
| 格式 / 语义版本 | `ir_version`、语义摘要、算法改变 | 必须先迁移或拒绝，不能直接比较结论 |
| 接口 | input port、output 名称或表类型改变 | 调用与数据兼容性变化 |
| 契约 | assume、guarantee、非干扰变化 | 正确性定义或信任边界变化，最高风险 |
| 程序 | 节点、表达式、依赖、操作改变 | 实现语义变化，需重新验证 |
| 类型 | enum、记录、表范围、标签、键、容量改变 | 可能同时影响接口、义务与反例 |
| 效果 | v0.1 中任何非空或未知值 | 不兼容并拒绝 |

节点内容变化会级联改变下游 ID。diff 工具应递归比较 definition 和前驱关系，展示最早变化，不把所有下游 hash 更新误报为独立编辑。

pretty 空白、object member 输入顺序和伴随来源映射不构成语义差异。名称、枚举成员顺序、主键顺序、减法左右顺序和 group key 顺序构成语义差异。

## 四个基准的 IR 映射

本节固定映射结构，不内嵌会漂移的制品摘要。精确规范字节、pretty 投影和摘要清单见[有键有限表基准语料库 v0.1](../benchmarks/keyed-finite-table-corpus-v0.md)。

### AX-B01：结算订单净额

- 声明：订单状态 enum；订单输入记录 / 表；订单净额输出记录 / 表。
- DAG：`input(orders) -> filter(state == settled) -> map(order_id, subtotal_cents - discount_cents)`。
- assume：`forall_rows(orders, discount_cents <= subtotal_cents)`。
- guarantee：对每个输入订单，按键 `lookup` 输出；当且仅当状态为 `settled` 时为 `Some`，且 `net_cents` 等于受界减法。
- 内建义务：筛选谓词总定义、输出键保持、算术范围、字段闭合和效果 `[]`。

### AX-B02：客户等级连接

- 声明：订单、客户和输出记录 / 表；客户等级 enum（若采用枚举）。
- DAG：`input(orders)`、`input(customers)` -> `lookup_join(customer_id == customer_id)` -> 命名输出。
- assume：对每个订单，以 `customer_id` 对客户主键执行 `lookup` 必须为 `Some`。
- guarantee：每个订单键对应一个输出，输出 `tier` 等于其所引用客户的 `tier`。
- 内建义务：连接字段类型一致、每个左行恰好一个右匹配、无扇出 / 丢行、输出键保持。

### AX-B03：账户用量汇总

- 声明：用量事件输入记录 / 表；账户汇总输出记录 / 表，容量沿输入上界推导。
- DAG：`input(usage_events) -> group(account_id, count, sum(units))`。
- assume：事件键、范围和容量由类型与输入符合检查承担，不重复为公式。
- guarantee：对每个输出组，`event_count` 等于相同账户的 `count_where`，`total_units` 等于 `sum_where`；对每个输入事件，输出中存在其账户组。
- 内建义务：分区覆盖 / 不相交、结果范围、计数与数学和、控制依赖和效果 `[]`。

### AX-B04：工单最小化导出

- 声明：含 `sensitive` 邮箱 / 内部备注的输入记录；只含公开字段的输出记录。
- DAG：`input(tickets) -> map(ticket_id, category, priority)`。
- guarantee：每个输入键恰好一个输出，三个公开字段逐值相等；输出没有额外字段。
- noninterference：`inputs = [tickets]`，`outputs = [export]`。
- 内建义务：主键保持、字段标签 / 控制依赖重算、行覆盖和效果 `[]`。

## 拒绝边界

必须区分“不是合法 / 规范 IR”和“合法 IR 但程序不满足契约”。

### 结构上必须拒绝

- 重复 JSON member、JSON number、`null`、非法 Unicode 或控制字符名称；
- 不支持的 `ir_version`、语义摘要、摘要算法、tag、成员或非空 `effects`；
- 非规范十进制字符串、未知 enum 成员、递归记录类型或不良构范围；
- ID 与 definition 重算结果不符、重复 ID、悬空引用、节点环或死节点；
- 字段缺失 / 重复、表达式绑定越界、类型不匹配或非法隐式转换；
- map 派生新主键、join 字段类型不同、group key 可选 / 敏感；
- assume 引用输出、guarantee 引用内部节点、非干扰名称无法解析；
- strict canonical 模式下，原始字节不等于规范机器字节。

### 应进入验证而不是结构拒绝

- B01 把减法写成加法；
- B02 按非唯一 `region` 连接；
- B03 丢弃或重复计算有效事件；
- B04 按敏感邮箱筛选但最终不输出邮箱；
- 任意良构表达式可能超出声明范围、违反保证或无法被当前后端证明。

这些候选保留了可解释的语义，验证器必须生成 `failed` / `unknown` 与对应义务或反例。把它们提前当作“解析错误”会破坏验证反馈闭环。

## 版本与兼容策略

### v0 阶段

`0.1` 是预稳定公共 IR 版本，不承诺与未来 v0.x 或 v1 字节兼容。读取器必须声明精确支持的版本集合；只支持 `0.1` 的读取器遇到 `0.2` 必须拒绝。

以下变化要求至少提升 v0 minor：

- 顶层、类型、表达式、节点或契约的成员 / tag 集合变化；
- 数组规范化、JCS profile、十进制规则、ID、域分离或摘要算法变化；
- 任一现有结构的语义、绑定、可见性或拒绝行为变化；
- 从必需变可选、从拒绝变忽略等兼容策略变化。

仅修正文案、链接或不改变可接受文档集合、规范字节、摘要和语义的说明错误，可以保留 `0.1`。语义文档字节变化即使不改变 IR schema，也必须更新 `semantics.sha256`，因此文档摘要会变化。

### 显式迁移

跨版本转换必须作为显式迁移：

1. 按源版本严格验证并计算源摘要；
2. 应用具名、版本化迁移规则；
3. 按目标版本重新规范化、检查并计算目标摘要；
4. 记录迁移工具、规则版本、源 / 目标摘要和任何无法保持的语义；
5. 若不能无损映射，拒绝或要求人工决策，不能填默认值伪造成功。

读取器不得把旧文档原地解释为新语义。Evidence 也不能跨 IR 摘要或语义摘要复用，除非迁移后重新生成并检查。

### 进入 v1 的条件

只有四个基准的正确 / 错误制品、规范化一致性、独立读取、Evidence 引用和至少一次显式迁移演练都通过后，才评估 `1.0`。`1.0` 之后的兼容承诺必须另行冻结，不能从 v0 策略自动推断。

## 必需验证矩阵

实现 Axiom IR 工具链前，版本化语料库至少包含以下测试；正例、负例和兼容性结果必须由独立入口复核：

| ID | 输入 / 变化 | 预期结果 |
| --- | --- | --- |
| `IR-CAN-01` | 只改变 JSON 空白和 object member 输入顺序 | normalizer 输出相同规范字节与摘要；strict 模式拒绝非规范原字节 |
| `IR-CAN-02` | 打乱按 ID / 名称排序的无语义数组 | normalizer 恢复相同结果 |
| `IR-CAN-03` | `and` 嵌套、换序或重复相同子式 | 展平、排序、去重后相同 |
| `IR-CAN-04` | Unicode 视觉相同但 scalar 序列不同的名称 | 保持不同，产生不同 ID；不得自动 NFC |
| `IR-HASH-01` | 修改 definition 但保留旧 ID | 拒绝 ID 不一致 |
| `IR-HASH-02` | 相同子图重复声明 | 拒绝重复 ID；生成器应共享节点 |
| `IR-REF-01` | 悬空类型 / 节点引用或节点环 | 拒绝并给出引用路径 |
| `IR-TYPE-01` | `null`、JSON number、越界字面量或类型不匹配 | 结构拒绝，不生成证明成功 |
| `IR-EFFECT-01` | `effects` 非空或节点含外部能力 tag | 拒绝 |
| `IR-CONTRACT-01` | assume 引用 output 或 guarantee 引用内部节点 | 拒绝可见性越界 |
| `IR-SEM-01` | 四个错误基准候选 | IR 良构；验证产生 `failed` 或 `unknown`，不能当解析失败 |
| `IR-ROUNDTRIP-01` | 规范字节 -> pretty -> 规范字节 | 逐字节相同，摘要相同 |
| `IR-VERSION-01` | 未知 `0.2` 或 `1.0` | 精确拒绝，不尝试按 `0.1` 读取 |
| `IR-MIGRATE-01` | 受支持的未来显式迁移 | 记录源 / 目标摘要并在目标版本重新验证 |

命令成功退出不足以证明实现符合本矩阵；还必须核对规范字节、摘要、错误类别、引用路径和负向拒绝结果。

## 信任与未覆盖边界

- JCS 实现、UTF-8 处理和 SHA-256 实现属于规范化可信计算基，除非用独立实现交叉检查；
- 内容摘要不验证任务规范、字段标签、程序或证明结论；
- IR 检查器验证结构、类型和规范化，不能替代后端对契约义务的证明；
- source-to-IR 前端正确性仍需 Evidence 或交叉检查，不能由 IR 自身证明；
- pretty 投影可供人工审阅，但人工阅读不能替代规范字节重算；
- Unicode 混淆字符、超长字符串、深层表达式和大数组仍需要资源上限；具体限制尚未冻结，资源耗尽必须报告操作性失败或 `unknown`；
- v0.1 没有数字签名、压缩、分块、流式解码、网络媒体类型或长期归档承诺。

## 仍未冻结

- `.rax` 表面语法与 source-to-IR 映射；
- Axiom Evidence 的具体证明证书格式、制品打包容器和独立检查器实现；
- 验证后端、证明策略和 `unknown` 的后端细分；
- 编译器实现语言、宿主运行时、解释或代码生成路径；
- 伴随来源映射的具体容器、诊断协议和编辑器集成；
- 资源上限、媒体类型、签名、发布和长期兼容承诺。

## 变更要求

修改 v0.1 抽象结构、规范化、JCS profile、数值字符串、内容寻址、绑定索引、节点或契约可见性、拒绝边界和版本策略，属于公共格式变化，必须同步本文、现行语义、Axiom Evidence、基准语料库、独立检查器与兼容性测试。

若变化同时改变首域可观察语义或 ADR 0002 的纯效果 / 基准边界，必须先更新语义规范或以新 ADR 替代 ADR 0002；不能通过 IR 新 tag 偷渡尚未接受的语言能力。
