# Axiom IR v0.1 严格结构切片

本文冻结独立 checker 首个 Axiom IR parser 的实际声明范围。规范真相源仍是 RadishAxiom 主仓库按摘要锁定的 `docs/ir/axiom-ir-v0.md`；本实现没有导入、复制或调用生产 Rust `raxc` 的 parser、normalizer、类型检查器或测试 helper。

## 输入与身份分层

处理顺序固定为：

1. bundle 层先核对普通文件、声明长度和原始字节 SHA-256；失败时不得调用 IR parser；
2. `strictjson.ParseCanonical` 拒绝重复 member、非法 UTF-8、JSON number、`null`、BOM、空白、非规范转义和非 JCS member 顺序；
3. IR 层拒绝未知 member、版本、tag、摘要算法、语义身份和非空效果；
4. 对 enum、record、table、node 与 contract 的每个 `definition` 重放 canonical bytes，并按对应 domain 加 `NUL` 后重算 SHA-256 ID；
5. 核对声明引用、节点前驱、输出节点、接口引用、节点 DAG 与非 input 死节点；
6. 对完整 canonical 原始字节按 `axiom-ir-v0.1:document` domain 重算文档摘要，并与调用方提供的外部身份显式比较。

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

Axiom IR v0.1 规范中尚未被锁定语料使用的 `fixed`、`option`、`record` value type，以及相应 literal / constructor、`or`、其他比较、fixed 算术、`exists_rows` 等 tag，在本切片中失败关闭为 `unknown-tag`。后续只能通过单独的小切片、正负例和源码身份重放扩大该集合，不能静默忽略或按相近 tag 解释。

## 明确不形成的结论

`ParseStructure` 的成功结果只包含 raw content digest、document domain digest 和顶层计数，不是 checker 四态结果。当前不检查完整表达式类型推导、记录字段覆盖、主键公开性 / 可键性、节点输入输出类型关系、容量、算术或聚合义务、连接恰好一次、控制依赖或非干扰语义；也不重建 obligation、不解释 Evidence、不消费 certificate、不执行 solver / Node、不累计全阶段 wall-clock / working-memory，不生成 binary、`checker.artifact`、result 或 CLI。

任何调用方都不得把结构解析成功升级为 `checked` 或 `proved`。声明范围外的 tag 和结构失败关闭；声明范围外的语义没有默认成功路径，因为当前仓库尚无形成接受结果的入口。

## 锁定语料与负例

28 个 bundle 的入口边界保持如下：

- 25 个身份有效场景进入 IR parser；它们覆盖 12 份唯一 IR 原始字节；
- `chk-bundle-01`、`chk-digest-01`、`chk-resource-01` 分别在缺失 artifact、raw content SHA-256、资源限制层拒绝，parser 不得越过这些失败；
- 12 份唯一 IR 的 document domain digest 与锁定 bundle 中既有外部记录逐一比较，但不解释 Evidence 语义；
- parser 负例覆盖未知 member / version / tag、语义数组非规范顺序、definition domain ID 漂移、悬空输出节点引用和 document domain digest 不匹配；严格字节层继续覆盖 object member 顺序及 JSON/JCS 负例。

所有 Go 验证继续显式使用 `GOTOOLCHAIN=local` 与 `CGO_ENABLED=0`，源码变更后必须更新并审阅 `checker.source` manifest。
