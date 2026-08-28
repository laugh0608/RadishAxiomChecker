# Axiom Evidence v0.1 concrete input / Pre 切片

本文冻结独立 checker 对锁定 keyed-finite-table benchmark profile 的完整输入制品与前置条件检查。调用方必须依次通过 bundle、Axiom IR、Axiom Evidence 结构 / 身份、obligation completeness、state / support 和 counterexample world / `WF`；生产 `check-fixture` 的 `completed`、Evidence result 与预期独立结果都只是待核对声明。

后续 proof-failure IR 执行与目标违反已经在 [counterexample target replay 切片](axiom-evidence-counterexample-targets-v0.1.md) 实现，host / golden 制品关系已经在 [concrete output comparison 切片](axiom-evidence-concrete-outputs-v0.1.md) 实现；本文的停止线只界定 `VerifyConcreteInputs` 自身，不能用来跳过这些后续入口。

## Artifact 绑定与严格 JSON

`VerifyConcreteInputs` 从 artifact-level `input-conformance` obligation 独立重建目标集合，并要求它与所有 execution 中标记为 `host-input` 的 artifact 集合精确一致。每个目标必须在 Evidence artifact 清单中声明为 `axiom-benchmark-data` `0.1`，再通过 `bundle.Verified.ReadArtifact` 重新打开普通文件并复核声明长度和原始 SHA-256；既有 bundle 验证不会被当作可变字节缓存。

benchmark fixture 是带空白和末尾换行的 pretty JSON，而不是 JCS。新增 `strictjson.ParseDocument` 只把空白与 object member order 视为表示细节，仍失败关闭拒绝 duplicate member、BOM、非法 UTF-8、JSON number、`null` 和 byte / depth / item / step 超限；profile decoder 再拒绝未知或缺失结构。canonical request、manifest、IR 与 Evidence 继续使用原 `ParseCanonical`，没有放宽 JCS 身份边界。

顶层恰好包含 `benchmark_id`、`data_version`、`format`、`role` 与 `tables`：

- `format = axiom-benchmark-data`、`data_version = 0.1`；
- 当前锁定 interface profile 分别绑定 `AX-B01` 至 `AX-B04`，不能只接受任意非空 benchmark ID；
- 输入 role 只接受 `input` 与 `invalid-input`；
- table array 按 interface name 严格递增且唯一，row 保留显式数组顺序；
- Bool 读取 JSON boolean，Int / Text / Enum 读取 JSON string 后由 IR 期望类型解释；没有 number、`null`、默认值或按内容猜测类型。

## 完整输入、WF 与 Pre

artifact world 必须包含恰好完整的 IR input interface 集。每个 row 从对应 table 的 record type 独立重建，随后复用同一 concrete model 检查闭合字段、值 kind、规范十进制整数与范围、名义 enum、容量、主键唯一性和 canonical primary-key order。

IR parser 保留 assume formula definition，而不只保留 ID。当前 12 份唯一 IR 实际只需要：

- AX-B01：`forall_rows`、`bound`、`field` 与 Int `le`；
- AX-B02：`forall_rows`、`bound`、`field`、按完整主键 `lookup`、`match_option` 与 `literal_bool`；
- AX-B03 / AX-B04：没有 assume。

独立 evaluator 同时闭合支持这些公式可直接依赖的 scalar literal、`not`、`and`、`eq` 与 `if`，但不执行 transform node、aggregate 或 guarantee。`lookup` 扫描已物化输入并要求主键至多一个匹配；De Bruijn binder 与 IR 类型检查使用相同“新绑定插入索引 0”规则。求值是单个输入上的动态检查，不是 proof。

## checked / failed 分类

- `checked` input-conformance 要求 artifact role 为 `input`、完整输入 `WF` 且全部 assume / `Pre` 为真；
- `failed` input-conformance 要求 role 为 `invalid-input`，且完整输入至少一项 `WF` 或 `Pre` 失败；
- failed counterexample 的每个内嵌 world 必须是绑定 artifact 的真实 table / row 投影，投影 row 使用一一匹配，不能凭空添加或改写 witness；
- 关系不成立统一形成已登记的 `concrete-check-mismatch`，raw digest 漂移和资源失败仍保持各自更早的 `digest-mismatch` / `resource-limit`。

当前 24 个 obligation / state / world 完整场景全部通过，覆盖 12 个唯一 `host-input`：8 个 role `input`、4 个 role `invalid-input`。AX-B01 invalid 由金额 `Pre` 失败，AX-B02 invalid 由外键 lookup `Pre` 失败，AX-B03 与 AX-B04 invalid 由重复主键 `WF` 失败；这些分类由 checker 独立计算，不读取场景名称作为真相。

负例覆盖 artifact format、execution binding、checked / failed 状态漂移、world / artifact 不一致、benchmark ID / version / role、JSON number、enum / scalar 漂移、缺失 interface、重复 / 逆序主键、raw digest、逻辑内存和 semantic step 超限；同一 AX-B02 invalid 输入重复验证 100 次结果稳定。

## 资源与停止线

JSON parser 消费 request 的 artifact byte、depth、collection item 与 semantic step 边界；concrete model 另形成与 Go heap 实现无关的确定性逻辑字节计数，并受 `working-memory` 上限约束；assume evaluator 按 expression、row 与 lookup scan 累计 semantic step。局部入口仍保留这些方法级上限；完整调用通过 [invocation 累计账本](invocation-resource-budget-v0.1.md) 合并计数和内部 wall-clock，并将可绑定的资源不足物化为 `incomplete`，外层进程限制仍保持独立。

本切片不执行 filter / map / lookup_join / group 等 transform node，不检查 guarantee，不比较 host output / golden output，不判断非输入 failed obligation 的目标违反、observed、trace、公开等价或 minimality，不验证 kernel rule、backend attestation 或 certificate，不重算 conclusion，也不生成独立四态 result。

`VerifyConcreteInputs` 成功只能把锁定 input-conformance 的 `checked` / `failed` 提升为独立动态确认；不能升级任何 `proved`，不能确认完整 counterexample replay，也不能形成 `accepted` 或六平台结论。
