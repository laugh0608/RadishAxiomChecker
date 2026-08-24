# Axiom Evidence v0.1 严格结构与身份切片

本文冻结独立 checker 首个 Axiom Evidence parser 的实际声明范围。规范真相源仍是 RadishAxiom 主仓库按摘要锁定的 `docs/evidence/axiom-evidence-v0.md`；本实现没有导入、复制或调用生产 Rust `raxc` 的 Evidence parser、聚合器、义务生成器、反例重放器或测试 helper。

## 输入与身份分层

处理顺序固定为：

1. bundle 层先核对 request / manifest、普通文件、声明长度、原始字节 SHA-256 与资源限制；失败时不得调用 Evidence parser；
2. `strictjson.ParseCanonical` 拒绝重复 member、非法 UTF-8、JSON number、`null`、BOM、空白、非规范转义和非 JCS member 顺序；
3. Evidence 层拒绝未知顶层 / 嵌套 member、版本、tag、摘要算法、profile 和声明范围外的结构；
4. 对 tool、execution、obligation、trust 与 uncovered 的每个 `definition` 重放 canonical bytes，并按对应 domain 加 `NUL` 后重算 SHA-256 ID；obligation 的 `result` 不进入 obligation ID；
5. 建立 artifact、tool、execution、obligation、trust 与 IR document 的闭合引用索引，拒绝悬空或歧义引用；
6. 对完整 canonical Evidence 原始字节按 `axiom-evidence-v0.1:document` domain 重算文档摘要，并与锁定外部身份显式比较；
7. 将 subject 的 IR 原始 content digest 与 Axiom IR document domain digest 分别绑定到独立 `axiomir.ParseStructure` 结果。

Evidence raw content SHA-256、Evidence document domain SHA-256、IR raw content SHA-256 与 IR document domain SHA-256 是四个不可互换的身份。Git commit / tree 也不能替代任一协议 SHA-256。

## 当前闭合 profile

顶层 object 必须精确包含规范的 13 个 member，并固定：

- `format = axiom-evidence`；
- `evidence_version = 0.1`；
- `digest_algorithm = sha-256`；
- subject 为 `axiom-ir` `0.1`，语义名称与摘要等于已锁定的 keyed-finite-table 规范；
- obligation profile 为 `keyed-finite-table-verification` 或 `keyed-finite-table-benchmark` `0.1`。

本切片只接受 28 个版本化 bundle 实际使用到的闭合 tag 集合：

- tool role：`evidence-producer`、`fixture-checker`、`host-executor`、`ir-normalizer`、`obligation-generator`、`output-comparator`、`prover`；
- execution：`check-fixture`、`compare-output`、`execute-host`、`prove`、`replay-counterexample`；execution result 为 `completed`、`timeout` 或 `unavailable`；
- obligation expectation：`check`、`prove`、`trust`；kind 为锁定语料实际使用的 14 个 v0.1 kind；
- obligation subject：`artifact`、`contract`、`contract-path`、`document`、`field`、`interface`、`node`、`node-path`、`program`、`trust`；
- obligation result：`checked`、`failed`、`proved`、`trusted`、`unknown`；proof support 为 `backend-attestation` 或 `kernel-replay`；
- counterexample：`group`、`missing-key`、`paired-input`、`row-pair`、`single-row`；当前 witness value 为 `enum`、`int`、`text`；
- trust category 为规范的八类，scope 为 `program` 或 `tool`；当前 mitigation 数组为空；
- uncovered category 为规范的七类，scope 为 `program`；
- conclusion 为 `implementation_inconsistent`、`inconclusive`、`input_rejected`、`satisfied` 或 `violated`。

声明范围外的 `rejected-ir` subject、certificate support、其余 execution result、counterexample、trace、value、scope 或 mitigation 组合失败关闭。它们只能通过单独的小切片、正负例和源码身份重放扩大，不能静默忽略或按相近 tag 解释。

## 规范顺序与定义身份

parser 显式核对以下确定性顺序：

- artifact 按 `content_digest` 严格递增；
- tool、execution、obligation、trust、uncovered 按 `id` 严格递增；
- tool role、摘要集合、结论 refs、assumptions、attempts、preconditions 与 required field / key 集合严格递增且唯一；
- execution input / output 按 `(role, artifact)` 严格递增；limit 按 `(name, unit)` 严格递增；
- counterexample table 按 input name、record field 按 field name 严格递增。

map 只用于已知摘要查找，不参与规范输出、摘要或错误顺序。所有 definition ID 都由 checker 自有 JCS 重放与 SHA-256 实现独立复算；生产侧提交的 ID 不能作为事实。

## 当前引用闭合与 subject 绑定

当前索引拒绝：

- subject、tool、execution、result、support 或 counterexample observation 引用不存在的 artifact；
- producer、execution 或 trust scope 引用不存在的 tool，或 producer 缺少 `evidence-producer` role；
- result / support / unknown attempt 引用不存在的 execution；
- counterexample trace / observed 或 conclusion 引用不存在、歧义或错误集合的 obligation / execution；
- assumption、support、trusted result 或 trust obligation subject 引用不存在的 trust；
- program / document scope 与 counterexample document trace 引用 subject 之外的 IR document digest；
- subject artifact 不解析到顶层 artifact 清单中的 `axiom-ir` `0.1` 描述；
- subject 的 IR raw content digest 或 IR document domain digest 与独立 Axiom IR parser 结果不一致。

锁定 bundle 的 Evidence artifact 清单包含由 pipeline receipt 间接绑定、但未在 Evidence 顶层字段再次直接引用的 options / policy 制品。当前结构 parser 不读取 receipt 内部，因此只保证所有直接引用闭合，不在本切片声称完成“顶层 artifact 是否最终被完整消费”的语义检查；该判断必须与后续 execution / support、artifact graph 和 obligation 完整性切片一起收口。

## 明确不形成的结论

`ParseStructure` 的成功结果只包含 Evidence raw content digest、document domain digest、subject 双摘要和顶层计数，不是 checker 四态结果。

当前实现不：

- 判断 obligation `expectation` 与五种 result 状态是否配对正确；
- 从 IR 重建或比较完整 obligation set，也不解析 obligation anchor 到具体 IR node / contract / expression；
- 判断 execution kind、tool role、execution result、support、assumption 或 trust category 的语义搭配；
- 检查未直接引用 artifact 的最终可达性；
- 重放 concrete check、counterexample、world、`WF` / `Pre` 或 witness 最小性；
- 检查 certificate、backend attestation 真值或 kernel rule；
- 重算 conclusion、应用 assurance policy、形成 remaining trust / missing artifact 或独立四态 result；
- 执行 solver、Node、生产工具或网络 resolver，也不生成 binary、`checker.artifact` 或 CLI。

任何调用方都不得把结构解析成功升级为 `checked`、`proved`、`accepted` 或六平台结论。

## 锁定语料与负例

28 个 bundle 的入口边界保持如下：

- 25 个身份有效场景进入 Evidence parser；它们覆盖 25 份唯一 Evidence 和 12 份唯一 IR；
- `chk-bundle-01`、`chk-digest-01`、`chk-resource-01` 分别在缺失 artifact、raw content SHA-256、资源限制层拒绝，Evidence parser 不得越过这些失败；
- 25 份 Evidence 的 raw content digest 与 manifest 一致，document domain digest 与版本化 bundle-set / expected result 一致，subject 双摘要与独立 Axiom IR parser 一致；
- parser 负例覆盖未知 member / version / support tag、非规范 artifact / ref 顺序、definition domain ID 漂移、悬空 producer、重复 conclusion ref、错误 subject artifact，以及 Evidence document / IR subject 外部绑定不匹配；
- 既有 request / manifest、bundle、Axiom IR、严格 JSON / JCS 与源码身份负例保持独立通过。

所有 Go 验证继续显式使用 `GOTOOLCHAIN=local`、`CGO_ENABLED=0` 与 `GOPROXY=off`。源码变化后必须更新并审阅 `checker.source` manifest；使用本机非精确补丁版本运行只能形成局部开发证据，不能冒充 `go1.26.7` 或六平台结果。
