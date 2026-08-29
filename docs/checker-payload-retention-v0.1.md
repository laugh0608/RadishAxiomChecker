# Independent Checker payload 候选归档与留存边界 v0.1

本文定义 macOS arm64 checker payload 在完成受控构建与独立 acceptance 后、进入 RadishAxiom 主仓 active registration 前的确定性归档和候选留存边界。它不修改构建或 acceptance 语义，不选择最终产品发布载体，也不授权 GitHub workflow、Release、tag、上传、安装或执行。

## 确定性内层归档

`radishaxiom-checker-payload-archive pack` 只接受当前 source root、已经含有 artifact / canonical provenance / canonical acceptance 的闭合 build root、新 output file 和精确版本。它重新核对 `checker.source`，要求 build root 只有以下三个普通文件及模式：

| 文件 | 模式 | 角色 |
| --- | --- | --- |
| `checker-build-provenance-v0.1.jcs` | `0644` | build provenance |
| `checker-payload-acceptance-v0.1.jcs` | `0644` | payload acceptance |
| `radishaxiom-independent-checker-go` | `0755` | checker artifact |

命令为三项原始字节分别记录长度和 SHA-256，绑定当前 source、`go1.26.7`、精确实现版本与 darwin / arm64 / v8.0 / Mach-O 身份，生成 `checker-payload-retention-manifest-v0.1.jcs`。四项成员按 UTF-8 字节序写入未压缩 USTAR；header 固定为普通文件、uid / gid `0`、空 user / group、Unix epoch mtime、无 link / PAX / xattr，路径、mtime、源目录和本机环境不进入 archive。

`verify` 严格解析四项成员、顺序、header、manifest canonical JSON 和三项内部摘要，再从解析结果重建完整 USTAR 并逐字比较。archive 尾随字节、成员篡改、额外文件、错误模式、未知 manifest 字段、非规范 JSON 或 header 漂移全部拒绝。`pack` 只在写后调用同一严格复核成功时保留 output；不覆盖已有文件。

```sh
GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go run ./cmd/radishaxiom-checker-payload-archive \
  pack \
  --source-root=/canonical/RadishAxiomChecker \
  --build-root=/canonical/accepted-build \
  --output-file=/canonical/staging/checker-payload.tar \
  --version=0.1-dev

GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go run ./cmd/radishaxiom-checker-payload-archive \
  verify \
  --archive=/canonical/staging/checker-payload.tar
```

归档器不重新执行 binary，也不解析或重做 acceptance；因此它能确认“这些精确字节被确定性封装”，不能自行声称 build root 真实通过 acceptance。调用顺序、acceptance 原始摘要和后续独立回读仍须由受控流程与主仓 registration 共同绑定。

## 两级留存策略

候选阶段选择 GitHub Actions workflow artifact 作为有限期暂存候选，而不是 active runtime store：

- 上传对象是上述单一内层 `.tar`，不把 GitHub 自动形成的外层 ZIP、文件权限或 artifact name 当成 payload identity；
- 必须绑定 repository、workflow run ID、head SHA、artifact ID、创建 / 到期时间、外层 provider digest / size，以及内层 tar 原始长度 / SHA-256；fetch 只允许精确 artifact ID，不允许 name、latest 或“最近成功 run”；
- 上传后必须由另一个 read-back job 从 GitHub 重新下载，复算内层 tar 身份并运行 `verify`；上传 job 本地文件不能替代 provider 回读；
- public repository 的 workflow artifact 最长只保留 90 天，删除 workflow run 也会删除其 artifacts。因此该层只允许 `candidate-retained-temporarily`，永远不能把主仓 active runtime 从 `0` 提升为 `1`。

长期 active storage 仍保持未选择。它至少必须提供不可变原始资产、稳定精确 fetch、原始 byte length / SHA-256、独立回读、撤销 / replacement 规则和不依赖 latest alias 的身份。GitHub immutable release assets 是可评估候选，但启用 release immutability、创建 tag / Release、上传或公开 payload 均属于另行设计和单独授权的发布动作；本切片不提前决定。

GitHub 平台事实参考：[workflow artifact 留存与删除](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/remove-workflow-artifacts)、[artifact API 的 ID / digest / expires_at](https://docs.github.com/en/rest/actions/artifacts)、[immutable releases](https://docs.github.com/en/code-security/concepts/supply-chain-security/immutable-releases)。

## 信任与停止线

- archive digest 不替代 artifact、provenance、acceptance 或 `checker.source` 各自身份；GitHub 外层 artifact digest 也不替代内层 tar 或成员摘要。
- `pack` / `verify` 成功不是 payload acceptance、runtime registration、publication、installation、launcher isolation、签名或形式证明。
- 未完成 provider 回读或候选已经 expired / deleted 时，主仓只能记录 `unavailable` / `expired`，不能继续保持可取得声明。
- 没有 durable active storage、独立重新下载复核和单独发布授权前，不形成 active registration 或正式 runtime companion。
