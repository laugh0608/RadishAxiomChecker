# Axiom IR v0.1 严格结构与类型良构切片

本文冻结独立 checker 首个 Axiom IR parser 的实际声明范围。规范真相源仍是 RadishAxiom 主仓库按摘要锁定的 `docs/ir/axiom-ir-v0.md`；本实现没有导入、复制或调用生产 Rust `raxc` 的 parser、normalizer、类型检查器或测试 helper。

## 输入与身份分层

处理顺序固定为：

1. bundle 层先核对普通文件、声明长度和原始字节 SHA-256；失败时不得调用 IR parser；
2. `strictjson.ParseCanonical` 拒绝重复 member、非法 UTF-8、JSON number、`null`、BOM、空白、非规范转义和非 JCS member 顺序；
3. IR 层拒绝未知 member、版本、tag、摘要算法、语义身份和非空效果；
4. 对 enum、record、table、node 与 contract 的每个 `definition` 重放 canonical bytes，并按对应 domain 加 `NUL` 后重算 SHA-256 ID；
5. 建立 enum / record / table 声明索引，核对声明引用、record field、table primary key、节点前驱、输出节点、接口引用、节点 DAG 与非 input 死节点；
6. 在完整声明和节点索引上独立推导 18 个锁定 expression op 的返回类型，核对无名称绑定环境、字段、操作数、分支、表 binder、lookup key，并要求 filter predicate 与 formula contract 顶层为 `Bool`；
7. 核对 filter / map / lookup_join / group 的输出字段覆盖、projection 类型、主键保存 / 重命名、容量关系、join pair 与 group key / aggregate 声明关系；
8. 对完整 canonical 原始字节按 `axiom-ir-v0.1:document` domain 重算文档摘要，并与调用方提供的外部身份显式比较。

raw content SHA-256 与 Axiom IR document domain SHA-256 是两个不可互换的身份。Git commit / tree 只作来源追溯，也不能替代任一协议 SHA-256。

## 当前闭合 profile

顶层 object 必须精确包含规范的 11 个 member，并固定：

- `format = axiom-ir`；
- `ir_version = 0.1`；
- `digest_algorithm = sha-256`；
- semantics 名称与原始 SHA-256 等于已锁定的 keyed-finite-table 规范；
- `effects = []`。

本切片只接受 28 个版本化 bundle 实际使用到的闭合 tag 集合：

- value type：`bool`、`int`、`text`、`enum`；
- node：`input`、`filter`、`map`、`lookup_join`、`group`；
- expression：`and`、`bound`、`count_where`、`eq`、`field`、`forall_rows`、`if`、`int_add`、`int_sub`、`le`、`literal_bool`、`literal_enum`、`literal_int`、`literal_text`、`lookup`、`match_option`、`not`、`sum_where`；
- contract：`formula`（`assume` / `guarantee`）与 `noninterference`；
- group aggregate：`count` 与 `sum`；contract table reference：`input` 与 `output`。

每种 tag 的 object member 集合闭合。数组规则覆盖顶层 ID / output 名称排序、record / projection field 排序、join pair 排序、aggregate 排序、`and` 去重排序、交换式 operand 排序，以及 enum member、primary key、group key 和 lookup key 的有语义顺序。名称、十进制字符串、整数边界、enum member、无名称绑定索引和声明 / 接口引用也在构造领域对象前检查。

### 声明类型索引与主键良构

parser 在 definition domain ID 核对之外独立保存当前 profile 的闭合声明元数据：

- enum 索引保存成员集合，拒绝空成员、重复成员和悬空 enum type 引用；
- record 索引保存按名称唯一的 field、`public` / `sensitive` 标签和完整 value type；
- table 索引保存 capacity、record type 与有语义顺序的 primary key；
- 每个 primary key field 必须存在于对应 record、唯一、标记为 `public` 且属于当前可键类型集合。

现行语义允许非可选 `Bool`、`Int`、`Fixed`、`Text`、`Enum` 作键；当前锁定 profile 已支持其中 `bool`、`int`、`text`、`enum`，合成 canonical 正例分别覆盖四种类型。`fixed` 尚未进入 profile；`option`、`record` 等非当前 tag 在形成 record / table 领域值前失败关闭，不能绕过主键规则。

Axiom IR v0.1 规范中尚未被锁定语料使用的 `fixed`、`option`、`record` value type，以及相应 literal / constructor、`or`、其他比较、fixed 算术、`exists_rows` 等 tag，在本切片中失败关闭为 `unknown-tag`。后续只能通过单独的小切片、正负例和源码身份重放扩大该集合，不能静默忽略或按相近 tag 解释。

### 独立表达式类型推导

类型检查器不调用生产 parser、normalizer、类型检查器或测试 helper；它在严格结构检查完成后重新遍历闭合 expression object，并以显式 switch 处理上述 18 个 op。`valueType` 保存完整 `Int` 上下界和名义 enum ID，因此相等类型不是只比较宽泛 kind。`Record` 与 `Option<Record>` 只作为 table binder、`lookup` 和 `match_option` 的内部推导类型，不扩大可声明 value type 或表面 tag 集合。

节点初始环境固定为：

- `filter` / `map`：前驱节点声明表的 `[source_row]`；
- `lookup_join` projection：两个前驱节点声明表的 `[left_row, right_row]`；
- contract 顶层：`[]`。

`bound` 按无名称索引取得完整类型；`field` 只接受 record 并要求字段存在。Bool literal / `not` / `and`、同型基础值 `eq`、同型 `Int` 的 `le` / `int_add` / `int_sub`、`if` 的 Bool condition 和同型分支都被核对。当前 profile 没有可由表达式构造的 `Option<Text>` 等类型；`lookup` 产生的 `Option<Record>` 也不能借 `eq` 获得规范未定义的 record 通用相等。

契约专用表操作继续在 node scope 失败关闭。`forall_rows`、`count_where`、`sum_where` 把目标表 record 插入环境索引 0，并分别要求 Bool body / predicate 和非可选 `Int` sum value；`lookup.keys` 必须与目标表 primary key 的有序字段逐项等型且 arity 完全相同；`match_option` 只接受已推导的 `Option<Record>`，并只在 `some` 分支插入内部 record。`filter.predicate` 和 `formula.expression` 顶层必须是 `Bool`；`map` / `lookup_join` 的每个 projection expression 必须能独立推导出类型。

### 节点 table type 与字段覆盖

节点关系检查在全部 node、record 与 table 声明解析完成后按 canonical node ID 顺序运行；map 只用于已知 ID / field 查找，不参与规范输出或失败顺序。

- `input` 的 `table_type` 必须解析；它不产生文件、网络或其他宿主读取。
- `filter` 的输出 record type 与有序 primary key 必须等于前驱，输出 capacity 不得大于前驱；predicate 的总定义性、筛选后实际基数和控制依赖仍是义务。
- `map.fields` 必须恰好覆盖输出 record；每个 expression 的完整类型必须等于同名输出 field。输出 capacity 等于源表，输出 primary key 按位置由直接读取 source row 的源 primary key field 逐值保留或一一重命名，不能用同型常量、派生式或非键字段替代。
- `lookup_join.pairs` 的左右 field 必须分别存在且完整类型相同；非键 pair 仍可静态良构，不在此冒充“每个左行恰好一个右匹配”。projection 与 map 同样闭合且等型，环境为 `[left_row, right_row]`；capacity 等于左表，输出 primary key 只能直接保留或重命名左表 primary key。
- `group.keys` 按顺序恰好等于输出 primary key；源 field 必须存在、`public`、可键，输出同名 field 与源 field 完整等型。keys 与 aggregates 合计恰好覆盖输出 record，capacity 不得大于输入。`count` 输出类型固定为 `Int[0, source_capacity]`；当前 profile 的 `sum` 源 field 与输出 field 都必须为非可选 `Int`，但具体结果范围与守恒仍由后续义务判定。

本切片不传播 projection / aggregate / predicate 的 field label，也不从敏感控制流推导输出 label。它不检查 filter 的更小 capacity 是否对所有输入足够，不证明 map / join key 的运行唯一性，不证明 join 存在性 / 唯一性，也不证明 group 范围、覆盖、不相交或守恒；这些都不能因静态关系通过而默认成功。

## 明确不形成的结论

`ParseStructure` 的成功结果只包含 raw content digest、document domain digest 和顶层计数，不是 checker 四态结果。类型索引只在 parser 内用于良构核对；当前不检查算术或聚合范围义务、连接恰好一次、行覆盖、标签 / 控制依赖、守恒或非干扰语义；也不重建 obligation、不解释 Evidence、不消费 certificate、不执行 solver / Node。完整 invocation 会累计 parser 事件和后续阶段的 wall-clock / working-memory，但该局部 API 本身不生成 binary、`checker.artifact`、result 或 CLI。

任何调用方都不得把结构解析成功升级为 `checked` 或 `proved`。声明范围外的 tag 和结构失败关闭；声明范围外的语义没有默认成功路径，只有后续全部检查与 assurance policy 进入结果聚合后才能形成独立接受结果。

## 锁定语料与负例

28 个 bundle 的入口边界保持如下：

- 26 个身份有效场景进入 IR parser；它们覆盖 12 份唯一 IR 原始字节；
- `chk-bundle-01` 保留缺失的非主体 proof artifact 并进入结果 `incomplete`；局部 parser 不越过 `chk-digest-01` / `chk-resource-01` 前置 finding，完整 invocation 在身份可绑定时分别形成 `rejected` / `incomplete`；
- 12 份唯一 IR 的 document domain digest 与锁定 bundle 中既有外部记录逐一比较，但不解释 Evidence 语义；
- parser 负例覆盖未知 member / version / tag、语义数组非规范顺序、definition domain ID 漂移、悬空输出节点引用、document domain digest 不匹配，以及空 / 重复 enum、悬空 enum / record type、字段缺失 / 重复、未知标签、错误整数范围、空 / 重复 / 缺失 / sensitive primary key；
- expression 负例均从合成 canonical definition 重算 domain ID，覆盖缺失字段、非 record field operand、错误 bound operand、Bool / `eq` / `le` / 算术 / 分支类型不一致、非 Bool filter / formula / table predicate、错误 `match_option` subject / branch、lookup key arity / type、非 Int sum value 及 node scope 表操作；
- node relationship 负例同样重算全部受影响 definition domain ID，覆盖 filter record / primary key / capacity、projection 缺失 / 额外 / 类型、map / join key 保存与容量、join pair 字段 / 类型，以及 group capacity、key 字段 / 标签 / 类型 / 顺序、count bounds、sum source / output 类型和输出覆盖；
- 严格字节层继续覆盖 object member 顺序及 JSON/JCS 负例。

所有 Go 验证继续显式使用 `GOTOOLCHAIN=local` 与 `CGO_ENABLED=0`，源码变更后必须更新并审阅 `checker.source` manifest。
