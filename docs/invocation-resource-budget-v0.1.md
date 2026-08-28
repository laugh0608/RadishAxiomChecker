# Independent Checker invocation 与累计资源边界

本文冻结 `checkresult.InvokeBundle` 的完整调用边界：一次调用只建立一个 `resourcebudget.Ledger`，从只读 bundle 身份检查、Evidence / IR 身份形成、十类独立检查一直延续到 canonical companion 编码前。规范真相源仍是 RadishAxiom 主仓库按摘要锁定的 Independent Check Contract v0.1、Execution Profile Contract v0.1 与 ADR 0008；本实现没有新增公共字段、结果状态、错误码或 fallback。

## 调用状态与结果形成

调用先验证显式 checker source / toolchain / version / TCB 边界，再读取 canonical request。request 不可读、非规范、无法形成 document identity，或 checker source 身份缺失时，不形成四态结果。request 与 manifest 已绑定后，bundle 层先按 manifest 声明预计算唯一 artifact 总字节，再逐个核对现存 blob 的普通文件、长度与原始 SHA-256；缺失 blob 保留为 `MissingArtifacts`，确定摘要矛盾则保留为真实 identity finding。

Evidence 与 IR 的 raw content 和 document-domain identity 足以绑定结果后，request 预算开始生效。已经观察到的摘要、subject 或结构矛盾物化为 `rejected`；累计内部预算耗尽物化为 `incomplete`，并以 `isolation-report` / `tcb-incomplete` 保留 checker 未完成边界。若两者并存，四态聚合继续让确定拒绝优先，但资源不足仍留在 checks 与调用资源快照中。结果编码前再做最后一次墙钟检查，不能让刚刚耗尽预算的调用保持 `accepted` 或 `accepted-with-trust`。

`RecordInvocationFailure` 是同一进程边界的另一侧：launcher kill、crash、输出截断或 checker 根本没有产出完整 canonical result 时，只能形成绑定有效 request 的 `axiom-checker-invocation-failure` `0.1`。`Invocation` 是 `result` / `failure` 闭合 union，两侧不能共存；截断的 companion 也不能被重新解释为内部 `incomplete`。

## 单一累计账本

request 自己尚未解析时，严格 parser 只受实现硬上限保护；解析期间发生的 item、depth 与 step 消费已经进入账本，但 request 级累计超限暂不对外暴露。在单 artifact 前置上限与实现硬上限允许形成 manifest、Evidence、IR 身份后，`Activate` 一次性检查此前累计值。这样 `CHK-RESOURCE-01` 可以产生规范 `incomplete`，又不会用合成身份、expected result 或无限预算补造结果。

账本按一次调用累计：

- `bundle-bytes` 只按 manifest 中唯一 content digest 的声明长度计一次，重复打开不重复增加；单 artifact 仍受 `artifact-bytes` 前置上限；
- request、manifest、初次 blob 校验与后续重新打开都按每 64 KiB 一个 digest block 增加 semantic step；重新散列是真实工作，不由首次成功缓存替代；
- 所有接入的 strict JSON parser 将 token、object member / array item 和最大打开容器深度汇入同一计数器；局部硬上限仍保留，不能被累计账本放宽；
- obligation comparison、state / execution 遍历、counterexample world / target、expression、row / lookup / node、concrete comparison、proof artifact 与 conclusion traversal 将既有确定性 semantic step 汇入同一账本；
- concrete data、有限执行保留表、proof artifact cache 与 conclusion ref 使用稳定 owner 记录逻辑内存，只在对应预算对象丢弃时释放，不以 Go heap 或 GC 时机作为规范计量；
- 墙钟使用 Go `time.Time` 的单调时间分量，在十类 check 前后、每跨过 1,024 semantic steps 以及编码前检查；首个 resource exhaustion 是 sticky finding，不因后续阶段或释放内存而消失。

`Invocation.Resources` 只暴露与宿主调度无关的确定性快照：bundle bytes、collection items、digest blocks、最大 JSON depth、semantic steps 与当前 logical working memory。它是测试和诊断证据，不进入公共 result 字节，避免把性能观测混入 canonical Evidence / result identity。

## 锁定场景与停止线

统一测试覆盖正常 `ax-b01-correct` 的十类完整检查与累计快照、`CHK-DIGEST-01` 的真实 `rejected`、`CHK-RESOURCE-01` 的真实 `incomplete`、`CHK-PROCESS-01` 的 `not-produced` failure、摘要矛盾与预算同时发生时的拒绝优先，以及 request 不可读、request 非规范、checker source 缺失、墙钟耗尽、编码前耗尽和结果截断。实际形成路径不读取 `expected-result.jcs` 决定结果或字节。

本切片不实现产品 CLI，不构建、验收或发布 checker binary / `checker.artifact`，不实现 launcher 的 OS hard limit，不执行 solver、Node 或生产 compiler，不检查 counterexample minimality，也不增加 kernel / certificate 能力。测试使用的 runtime binary / TCB digest 仍是合成契约身份，不是可发布 payload 或六平台证据。

后续 [Independent Checker CLI v0.1](checker-cli-v0.1.md) 已将本入口接入唯一产品命令，但没有改变这里的累计账本、内部 / 外层资源分类或本切片当时的合成身份边界；正式构建、artifact 登记与 launcher OS hard limit 仍须独立完成。
