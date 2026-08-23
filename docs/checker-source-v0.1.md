# `checker.source` 源码快照身份 v0.1

状态：Frozen

本规范冻结 `axiom-independent-check-result` 中 `checker.source` 所需的不可变源码快照表示。它只定义源码 sidecar manifest 与其 SHA-256 身份，不定义 checker binary、`checker.artifact`、Axiom IR、obligation、Axiom Evidence、独立 result 或产品 CLI。

## 身份

版本 `0.1` 的源码快照身份是：

```text
sha256:<SHA-256(checker-source-v0.1.jcs 原始规范字节) 的 64 个小写十六进制字符>
```

manifest 使用 RFC 8785 JCS、UTF-8、无 BOM、空白或末尾换行。摘要直接覆盖完整 manifest 原始字节，不增加域前缀，也不把摘要写回 manifest。Git commit、tree、tag 或 remote 只可作为来源追溯；它们不进入上述公式，也不能直接填入 `checker.source`。

## 输入集合与排除规则

生成器从显式给定且不是 symlink 的仓库根开始递归枚举。目录只用于发现叶子 entry，空目录不形成源码身份；除以下两项外，根下每个非目录 entry 都属于闭合输入集合。生成器不读取 Git index、ignore 规则、remote、网络、PATH 或 cache：

1. 根目录的 `.git` entry 及其全部后代，无论 `.git` 是目录还是 worktree metadata 文件；
2. 精确路径 `source-identity/checker-source-v0.1.jcs` 的 sidecar 内容，以避免自引用。

第二项只排除 sidecar 字节，不放宽其类型和 mode：若该路径存在，它必须是非 symlink 的 `0644` 普通文件。除此以外，没有 build output、ignored file、临时文件、未跟踪文件或测试 fixture 的隐式排除。新增、删除或修改任何其他非目录 entry 都会改变 manifest，或在不支持的类型 / mode 上直接失败。

## 路径、类型、mode 与内容

- manifest path 使用 `/` 分隔的仓库根相对路径；每个组件只能包含 ASCII `A-Z`、`a-z`、`0-9`、`.`、`_`、`-`，不得为空、为 `.` / `..`、包含 `\\` 或形成绝对路径。
- `files` 按 path 的 UTF-8 原始字节严格升序排列。v0.1 的 ASCII 子集使该顺序在受支持平台上唯一；不得使用 locale、大小写折叠或 Git tree 顺序。
- 只有非 symlink 普通文件可成为输入；symlink、socket、FIFO、device 和其他特殊类型失败关闭。目录只用于递归，不进入 `files`。
- 普通文件的规范 permission mode 只能是 `0644` 或 `0755`。每个 descriptor 同时记录 `type: "regular"` 与四位 mode；setuid、setgid、sticky 或其他 mode 失败关闭。
- `byte_length` 是完整文件原始字节数的无前导零十进制字符串；`content_digest` 是 `sha256:<原始字节 SHA-256>`。内容可为任意字节，不执行文本解码、换行、Unicode、BOM、压缩或权限归一化。

## Manifest 闭合结构

顶层恰好固定以下语义字段，并由实现按 JCS member 顺序编码：

- `content_encoding: "raw-bytes"`；
- `digest_algorithm: "sha256"`；
- `exclusions`：上述两个有序排除规则；
- `files`：按 path 排序的闭合 descriptor；
- `identity: "checker.source"`；
- `manifest_encoding: "RFC8785-JCS-UTF-8"`；
- `module`：`module_path: "radishaxiom.dev/independent-checker-go"`、`go_language: "1.26.0"`、`go_toolchain: "go1.26.7"`；
- `path_encoding: "UTF-8"`、`path_order: "UTF-8-byte-lexicographic"`；
- `source_version: "0.1"`。

每个 file descriptor 恰好包含按 JCS 顺序编码的 `byte_length`、`content_digest`、`mode`、`path`、`type`。v0.1 同时要求根 `go.mod` 原始字节精确为：

```text
module radishaxiom.dev/independent-checker-go

go 1.26.0

toolchain go1.26.7
```

这使 module、语言基线和精确工具链声明的漂移在允许更新 manifest 之前先失败关闭。`CGO_ENABLED=0` 与构建参数属于后续 artifact 构建身份，不混入本源码摘要。

## 生成、复算与门禁

零第三方依赖实现位于 `internal/sourceidentity`。日常门禁：

```sh
./scripts/check-source-identity.sh
```

有意修改源码输入后，使用以下维护入口重写唯一 sidecar，再审阅完整 diff：

```sh
./scripts/update-source-identity.sh
```

两个入口都显式使用 `GOTOOLCHAIN=local` 与 `CGO_ENABLED=0`，不允许 Go 自动下载工具链。更新入口只重写上述 manifest，不生成或保留 checker binary。门禁重新枚举整个输入集合，逐文件复算长度和 SHA-256，再要求 sidecar 与生成字节完全相同；额外文件、缺失文件、路径 / 类型 / mode 异常、module / toolchain 漂移和生成字节漂移均失败关闭。

门禁通过只说明当前实现路径重放得到同一 SHA-256 身份，不构成可复现 binary、形式证明、独立检查结果或六平台运行证据。
