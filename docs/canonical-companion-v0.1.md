# Independent Check canonical companion 与 invocation failure

本文冻结 checker 将已经聚合的内存 `Result` 物化为 `axiom-independent-check-result` `0.1` canonical companion 的实现边界，并区分外层进程没有产生四态结果时的 `axiom-checker-invocation-failure` `0.1`。规范真相源仍是 RadishAxiom 主仓库按摘要锁定的 Independent Check Contract v0.1、ADR 0008 与 keyed-finite-table checker bundle contract；本实现没有修改公共字段、tag、排序或域摘要。

## Companion 形成入口

`checkresult.EncodeCompanion` 只接受已经由 `Aggregate` 形成的内存结果，以及调用方显式提供的 `RuntimeIdentity`。正式 companion 必须同时具备：

- `checker.source`、精确实现版本与 `go1.26.7`；
- 当前平台 checker binary 的非空 `checker.artifact`；
- 与内存 source-level TCB 的 category / version 一一对应的实际 runtime TCB artifact；
- Evidence 与 request 的 raw content identity，并在非拒绝结果中具有 document-domain identity；
- 已经形成的 checks、missing artifact、remaining trust 与四态结果。

encoder 不从 Git commit、tree、源码摘要、expected result 或相邻文件推导 binary。`checker.artifact == checker.source`、TCB source 冒充 runtime artifact、缺失或错版本 TCB、非 `go1.26.7` toolchain 均停止形成 companion。当前仓库没有实际构建 checker binary，因此真实开发调用仍只保留内存结果；测试中的 binary / TCB digest 是合成契约身份，不是已验收 payload。

规范编码按顶层 JCS member 顺序输出；checks 按 ID、code 按字典序、ref 按 `(kind, ref)`、missing / trust / result refs 按 digest、TCB 按 `(category, artifact)` 排序且唯一。完整结果使用

`SHA-256("axiom-independent-check-v0.1:result" || NUL || canonical_result_bytes)`

形成 document-domain identity，摘要不写回自身。encoder 在返回前使用同一公开 parser 重新解析自己的字节，但 parser 自己重算 check ID、四态优先级、引用存在性、必需 check 集合与 document-domain identity，不信任 encoder 的中间布尔值。

## 严格 parser 与身份复核

`ParseCompanion` 使用 checker 自有 strict JSON / JCS 入口，拒绝未知 member / tag / version、非法 UTF-8、重复 member、number / `null`、非规范字节和数组顺序。每项 check definition 都重新进入闭合 registry 并重算 `axiom-independent-check-v0.1:check` ID；顶层结果按 rejected、incomplete / missing、remaining trust、无 trust 的顺位复算。

`result.refs` 表示实际检查路径选择的因果 check，不等于机械收集某一种 outcome。parser 要求 ref 非空、排序唯一并指向文档内真实 check，同时不发明公共契约没有冻结的第二套因果选择算法。调用方随后通过 `VerifyIdentity` 把解析结果与本次 invocation 的 source、binary、toolchain、版本、Evidence / request 和 TCB 实际身份逐项比较，通过 `VerifyDomainDigest` 核对外部保存的 result identity。

锁定严格 Evidence 拒绝 fixture 精确重放为 1,685 bytes、raw SHA-256 `sha256:f61b746a3e85f8edf6e8e5d5bbbba2550081565b6a2d22bc73003dbe136b3089` 与 result document digest `sha256:24e81c66e17150c70c1b2d2eac50b47f16fc20c6a094111be3410546c7b6e608`。27 份指定态 expected companion 全部由 parser 接受；实际 evaluator 的 25 个结果层场景全部完成 encode → strict parse round-trip，expected result 仍只在事后提供 outcome / trust / missing oracle，不进入实际形成路径。

## Invocation failure 分层

`NewInvocationFailure` / `ParseInvocationFailure` 实现锁定的 `axiom-checker-invocation-failure` `0.1` envelope。它只在 canonical request raw bytes 与 request document identity 已形成时绑定：

- `code: checker-process-failure`；
- request raw content digest 与 request document-domain digest；
- `result: not-produced`。

该记录没有 checker 四态、checks、remaining trust 或 TCB，不能被 `ParseCompanion` 接受，也不能因为外层 kill、崩溃或输出截断而伪造 `incomplete`。parser 只能确认 envelope 自洽；launcher 仍须保存实际进程观察，调用方用 `VerifyRequest` 对真实 request bytes 重算双身份。`CHK-PROCESS-01` 的生成字节与锁定 `expected-process-failure.jcs` 精确一致。

## 验证与停止线

测试覆盖严格拒绝黄金字节 / 双摘要、27 份指定 companion、25 个实际结果 round-trip、checker binary 缺失、source / binary / TCB 混用、check ID、result kind / ref、check / trust / ref / TCB 顺序、document availability、runtime TCB 漂移、request failure 绑定和 100 次确定性重复。

完整调用入口现由 [invocation 与累计资源边界](invocation-resource-budget-v0.1.md) 承载：它在真实身份足以绑定规范结果时，将 `chk-digest-01` 的确定摘要矛盾物化为 `rejected`，将 `chk-resource-01` 的累计内部预算不足物化为 `incomplete`，并在十类 check 与编码前执行 wall-clock 门禁。局部 encoder 仍只编码调用方交付的既有 `Result`，不会自行读取 bundle 或建立第二套聚合 / 预算路径。

本切片不实现产品 CLI，不构建、验收或发布 checker binary / `checker.artifact`，不执行 solver、Node、生产 compiler 或 adapter，不增加 kernel / certificate 能力，不检查 minimality，也不产生六平台运行证据。
