# Independent Checker CLI v0.1

本文记录 Go independent checker 的唯一产品命令入口及其进程内失败关闭边界。调用契约来自 RadishAxiom 主仓库锁定的 checker execution profile；本实现不修改 `axiom-check-request`、bundle manifest、Axiom IR、Axiom Evidence、canonical companion 或 invocation failure 的公共格式。

## 唯一调用形状

可执行文件名固定为 `radishaxiom-independent-checker-go`，只接受以下两个精确参数 token：

```text
check
--bundle-root=<caller-mounted-readonly-canonical-realpath>
```

CLI 不接受参数别名、分离的 option value、额外参数、相对路径、冗余路径、符号链接路径或不存在的路径。bundle root 必须由调用方先挂载为只读目录并解析成 absolute canonical realpath；checker 再次执行 `EvalSymlinks` 与目录类型核对，任何漂移都在读取 bundle 前失败。stdin 必须为空并结束；一个字节的输入也会拒绝。

入口只调用既有的 `InvokeBundle`，不建立第二套解析、聚合、资源或 companion 编码路径。它不读取环境变量、不调用 PATH 中的工具、不执行网络请求，也没有 solver、Node、`raxc`、adapter 或生产工具 fallback。

## 运行时身份前置门禁

一个调用只有在以下身份全部可用时才会检查 bundle：

- `checker.source` 由受控构建通过 linker value 注入，必须是合法的 `sha256:` 身份；普通 `go build` 的空值不可用；
- checker 实现版本同样由受控构建注入，必须是非空且非 `latest` 的精确版本；
- toolchain 直接取运行二进制报告的 `runtime.Version()`，并必须精确等于 `go1.26.7`，不能由参数、环境或构建声明替代；
- checker executable 必须是 absolute canonical realpath 下无 setuid / setgid / sticky 位的普通可执行文件；checker 流式复算其原始字节 SHA-256，并在读取前后核对同一文件、mode 与 byte length；
- 实际 executable digest 绑定 `checker.artifact`，当前单体实现的四类 runtime TCB 也绑定同一 artifact，并与 source-level TCB 的 category / version 精确对应；source digest 不能冒充 binary 或 runtime TCB artifact。

CLI 在 bundle I/O 之前调用 companion 层的同一 runtime identity validator。身份缺失、版本漂移、非精确 toolchain、路径替换或 source / artifact 折叠都不会形成四态结果。

## stdout、stderr 与退出

成功调用以退出码 `0` 在 stdout 写入恰好一个 canonical `axiom-independent-check-result` v0.1 文档，不增加尾随换行或其他文本，stderr 为空。CLI 参数或 stdin 边界错误使用退出码 `2`；身份、bundle、检查或输出错误使用退出码 `1`。退出码是当前实现的进程诊断，不替代 canonical stdout 的严格解析，也不新增公共结果状态。

所有错误只写入一条有界 UTF-8 stderr 诊断，最大 `65,536` bytes，不把诊断伪装成结果。stdout 写入中断时，已经写出的前缀无法由 checker 撤回；它必须以非零退出，调用方必须将不可严格解析的输出或截断观察记录为外层 `not-produced` failure，不能重解释为内部 `incomplete`。

## 验证与停止线

`internal/checkercli` 的测试以合成 executable bytes 和明确标注的 `0.1-test` 身份覆盖 normal、`CHK-DIGEST-01`、`CHK-RESOURCE-01` 三条真实 bundle 路径，严格参数 / stdin / realpath 拒绝、重复运行字节一致、身份失配、bundle 失败、stderr 上限与 stdout 截断。合成 identity 只验证接口和进程内逻辑，不是正式 `checker.artifact`。

本切片不构建、验收、登记、安装或发布正式 checker binary，不生成 `checker.artifact` sidecar，不形成 reproduction / acceptance receipt，也不实现 launcher 的空环境、只读文件系统、网络隔离、`6,000 ms` hard wall、`128 MiB` memory limit 或 stdout hard cap。当前本机非 `go1.26.7` 的开发构建只能验证编译和失败关闭，不能形成正式 companion；下一切片必须先实现独立、可审阅的受控构建与 artifact identity，再进入 OS launcher 和六平台安装协调。
