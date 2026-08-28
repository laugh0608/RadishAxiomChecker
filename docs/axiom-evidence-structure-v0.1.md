# Axiom Evidence v0.1 严格结构与身份切片

本文冻结独立 checker 首个 Axiom Evidence parser 的实际声明范围。规范真相源仍是 RadishAxiom 主仓库按摘要锁定的 `docs/evidence/axiom-evidence-v0.md`；本实现没有导入、复制或调用生产 Rust `raxc` 的 Evidence parser、聚合器、义务生成器、反例重放器或测试 helper。后续已经增加的义务集合比较入口见 [Axiom Evidence v0.1 obligation completeness 切片](axiom-evidence-obligation-completeness-v0.1.md)，状态关系入口见 [Axiom Evidence v0.1 state / support 切片](axiom-evidence-state-support-v0.1.md)，有限 world 良构入口见 [Axiom Evidence v0.1 counterexample world / WF 切片](axiom-evidence-counterexample-worlds-v0.1.md)，完整输入与 `Pre` 入口见 [Axiom Evidence v0.1 concrete input / Pre 切片](axiom-evidence-concrete-inputs-v0.1.md)，proof-failure 目标与具体输出入口分别见 [counterexample target replay 切片](axiom-evidence-counterexample-targets-v0.1.md) 和 [concrete output comparison 切片](axiom-evidence-concrete-outputs-v0.1.md)，`proved` 支撑材料入口见 [proof support 真值与能力边界切片](axiom-evidence-proof-support-v0.1.md)；这些入口都不会改变本文的结构成功边界。

## 输入与身份分层

处理顺序固定为：

1. bundle 层先核对 request / manifest、所有现存 blob 的普通文件、声明长度、原始字节 SHA-256 与资源限制，并单独形成 manifest 已列出但缺失的 blob 清单；确定矛盾或 Evidence / IR 本体不可得时不得调用 Evidence parser，非主体 artifact 缺失可以进入后续 `incomplete` 结果层；
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

当前 parser 接受规范已登记并由后续闭合校验所需的 tag 集合；结构接受不表示对应能力已经执行：

- tool role：规范 v0.1 的九项闭合集合，包括 `certificate-checker` 与 `counterexample-replayer`；
- execution：规范 v0.1 的八项闭合集合；execution result 为 `completed`、`error`、`resource-exhausted`、`timeout`、`unavailable` 或 `unsupported`；
- obligation expectation：`check`、`prove`、`trust`；kind 为锁定语料实际使用的 14 个 v0.1 kind；
- obligation subject：`artifact`、`contract`、`contract-path`、`document`、`field`、`interface`、`node`、`node-path`、`program`、`trust`；
- obligation result：`checked`、`failed`、`proved`、`trusted`、`unknown`；proof support 为 `backend-attestation` 或 `kernel-replay`；
- counterexample：`group`、`missing-key`、`paired-input`、`row-pair`、`single-row`；witness value 闭合为 `bool`、`enum`、`int`、`text`，其中锁定 bundle 实际出现后三类，`bool` 由独立合成正负例覆盖；
- trust category 为规范的八类，scope 为 `program` 或 `tool`；当前 mitigation 数组为空；
- uncovered category 为规范的七类，scope 为 `program`；
- conclusion 为 `implementation_inconsistent`、`inconclusive`、`input_rejected`、`satisfied` 或 `violated`。

声明范围外的 `rejected-ir` subject、certificate support、其余 counterexample、trace、value、scope 或 mitigation 组合失败关闭。结构 parser 可以保留规范 execution / result tag，不代表 certificate、normalize 或 obligation generation 已由 checker 执行；能力只能通过对应后续切片、正负例和源码身份重放扩大，不能静默忽略或按相近 tag 解释。

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

锁定 bundle 的 Evidence artifact 清单包含由 pipeline receipt 间接绑定、但未在 Evidence 顶层字段再次直接引用的 options / policy 制品。当前结构 parser 不读取 receipt 内部，因此只保证所有直接引用闭合，不在本切片声称完成“顶层 artifact 是否最终被完整消费”的语义检查；该判断仍须由后续 execution / support 与 artifact graph 切片收口。义务完整性切片只消费明确的 benchmark I/O role，不把 receipt 或生产 obligation-set 当作真相源。

## 明确不形成的结论

`ParseStructure` 的成功结果只包含 Evidence raw content digest、document domain digest、subject 双摘要和顶层计数，不是 checker 四态结果。

`ParseStructure` 本身不：

- `ParseStructure` 本身不判定完整性；调用方必须显式传入独立解析的 IR，再调用 `VerifyObligationCompleteness`。该比较入口仍不判断 obligation result 状态；
- 判断 execution kind、tool role、execution result、support、assumption 或 trust category 的语义搭配；调用方必须在完整性通过后显式调用 `VerifyStateSupport`；
- 检查未直接引用 artifact 的最终可达性；
- `ParseStructure` 本身不重放 concrete check、counterexample、world、`WF` / `Pre` 或 witness 最小性；调用方可在前置边界通过后显式调用 `VerifyCounterexampleWorlds` 检查当前有限 world / WF 子集；
- `ParseStructure` 本身不读取 concrete artifact；调用方必须使用已验证 bundle 的 artifact reader 显式调用 `VerifyConcreteInputs`，才能检查当前锁定的完整输入 / `Pre` 子集；
- `ParseStructure` 本身不执行有限 IR 或比较输出；调用方必须继续调用 `VerifyCounterexampleTargets` 与 `VerifyConcreteOutputs`；
- `ParseStructure` 本身不审计 `proved` 支撑材料；调用方必须继续调用 `InspectProofSupports` 并消费其分类、剩余 trust 与缺失材料；
- 检查 certificate、backend attestation 真值或 kernel rule；
- `ParseStructure` 本身不重算 conclusion；调用方必须在前置语义检查与 proof support 审计结束后继续调用 `VerifyConclusion`；
- `ParseStructure` 本身不应用完整 assurance policy、形成累计 remaining trust / missing artifact 或独立四态 result；调用方必须进入 [Independent Check 内存结果聚合](independent-result-aggregation-v0.1.md)；
- 执行 solver、Node、生产工具或网络 resolver，也不生成 binary、`checker.artifact` 或 CLI。

任何调用方都不得把结构解析成功升级为 `checked`、`proved`、`accepted` 或六平台结论。

## 锁定语料与负例

28 个 bundle 的入口边界保持如下：

- 26 个身份有效场景进入 Evidence parser；它们覆盖 25 份唯一 Evidence 和 12 份唯一 IR，其中 `chk-bundle-01` 明确保留一个缺失的非主体 proof artifact 并进入后续 `incomplete` 结果层；其余场景中 24 个通过后续 obligation completeness，`chk-obligation-01` 在该后续边界拒绝；
- 24 个 obligation-complete 场景继续通过后续 state / support 闭合；bundle generator 已为 `replay-counterexample` fixture tool 补齐规范 `counterexample-replayer` role，并增加 kind / role 生成门禁；
- 同一 24 个场景继续通过 counterexample world / WF 边界；核心及 host/output 失败见证必须由 IR 声明确认 WF，四个 input-conformance 负例允许保持已锚定的范围、`Pre` 或键失败；
- 同一 24 个场景继续通过 concrete input / `Pre` 边界，覆盖 12 个唯一 `host-input`；四个 invalid-input 分别由真实 `Pre` 或重复键失败支撑；
- 同一 24 个场景继续通过 proof-failure target replay、concrete output comparison、proof support 审计与 production conclusion 重算；后两层分别保留独立证明数 0、160 项缺失 proof material，并得到 7 / 4 / 8 / 4 / 1 的 conclusion 分布，不能由结构成功升级；
- `chk-bundle-01` 的缺失 artifact 形成 identity `incomplete`；局部 Evidence parser 不越过 `chk-digest-01` / `chk-resource-01` 的前置 finding，完整 invocation 在身份可绑定时分别物化为 `rejected` / `incomplete`；
- 26 个进入 parser 的场景中，25 份唯一 Evidence 的 raw content digest 与 manifest 一致，document domain digest 与版本化 bundle-set / expected result 一致，subject 双摘要与独立 Axiom IR parser 一致；
- parser 负例覆盖未知 member / version / support tag、非规范 artifact / ref 顺序、definition domain ID 漂移、悬空 producer、重复 conclusion ref、错误 subject artifact，以及 Evidence document / IR subject 外部绑定不匹配；
- 既有 request / manifest、bundle、Axiom IR、严格 JSON / JCS 与源码身份负例保持独立通过。

所有 Go 验证继续显式使用 `GOTOOLCHAIN=local`、`CGO_ENABLED=0` 与 `GOPROXY=off`。源码变化后必须更新并审阅 `checker.source` manifest；使用本机非精确补丁版本运行只能形成局部开发证据，不能冒充 `go1.26.7` 或六平台结果。
