# Independent Checker payload 候选归档与留存边界 v0.1

本文定义 macOS arm64 checker payload 在完成受控构建与独立 acceptance 后、进入 RadishAxiom 主仓 active registration 前的确定性归档和候选留存边界。它不修改构建或 acceptance 语义，不选择最终产品发布载体；仓库内 workflow 只是受审实现，提交、推送、进入默认分支、手工运行以及其中的下载、构建和上传仍分别受外部动作授权约束。本切片不授权 Release、tag、安装或产品执行。

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

## distribution candidate 与持久 provider

候选阶段选择 GitHub Actions workflow artifact 作为有限期暂存候选，而不是 active runtime store。`.github/workflows/checker-payload-candidate.yml` 只有 `workflow_dispatch` 入口，并要求操作者显式填写当前 `checker.source`、精确实现版本和候选上传确认；它只接受 `dev` / `master` branch ref，在 arm64 `macos-15` runner 上复跑仓库门禁，再取得、复核并仅把已经接受的 `go1.26.7.darwin-arm64.tar.gz` 交给受控构建器。`actions/setup-go` 只运行仓库内 orchestrator，不是 payload 编译器；两个正式候选 build 仍只由构建器解开的精确已接受 archive 产生。

workflow 在既有 build → payload acceptance → inner pack 之后继续执行独立 distribution acceptance 与 outer pack。它上传的是 [runtime distribution package v0.1](checker-payload-distribution-v0.1.md) 唯一 `.distribution.tar`；原有候选 USTAR 保留为包内 `checker-payload-candidate.tar`，不再单独上传。distribution candidate 同时包含独立 acceptance、distribution manifest、Checker Apache-2.0 `LICENSE` 与精确 Go `LICENSE` / `PATENTS`，但有限期 provider 不因此升级为 durable publication。

上传与回读边界为：

- 使用精确固定的 `actions/upload-artifact@v7.0.1` direct-file 模式上传上述单一 `.distribution.tar`，`archive: false` 禁止 GitHub 再形成外层 ZIP；provider name 只是文件名，不是 payload identity；
- 必须绑定 repository、workflow run ID / attempt、ref、head SHA、artifact ID、创建 / 到期时间、provider digest / size，以及外层 archive、distribution manifest、distribution acceptance 和内层 candidate 的原始长度 / SHA-256；direct-file 模式下 provider digest / size 应与外层原始身份相同，任何差异都失败；
- 上传后必须由另一个 Ubuntu read-back job 使用精确固定的 `actions/download-artifact@v8.0.1` 和唯一 artifact ID 从 GitHub 原样下载，复算外层身份并运行严格 distribution `verify`，由其继续复核内层候选与法律材料，再以 artifact REST API 检查 ID、name、size、digest、run、head SHA、created / expires 和未过期状态；上传 job 本地文件不能替代 provider 回读；
- read-back job 在 workflow summary 写出 `radishaxiom-checker-candidate-provider-readback` v0.1 JSON 交接记录，但它不是 JCS、不是主仓 registration，也不能脱离对应 workflow run / provider API 事实单独证明可取得性；fetch 不允许 name、latest 或“最近成功 run”；
- public repository 的 workflow artifact 最长只保留 90 天，删除 workflow run 也会删除其 artifacts。因此该层只允许 `distribution-candidate-retained-temporarily`，永远不能把主仓 active runtime 从 `0` 提升为 `1`。

GitHub 只会对已经存在于默认分支的 `workflow_dispatch` 文件接收手工触发。因此 checker `dev` 上的实现完成并不等于可运行；按仓库治理进入 `master`、随后选择精确 `dev` 或 `master` ref 触发，都是后续单独的远程动作。workflow 不监听 push、pull request、schedule、workflow run 或 repository dispatch，不自动生成候选。

RadishAxiom ADR 0010 已选择 Checker 公开仓库的 GitHub immutable Release asset 作为首个 durable provider，但当前状态仍是 `selected-setting-not-verified-release-not-materialized`。仓库 setting 尚未验证，tag、Release 与 asset 尚未创建；启用 release immutability、创建 draft、上传、发布和主仓登记都是后续分别授权并写后回读的远程动作，不能由候选 workflow 自动执行。

GitHub 平台事实参考：[workflow artifact 留存与删除](https://docs.github.com/en/actions/how-tos/manage-workflow-runs/remove-workflow-artifacts)、[artifact API 的 ID / digest / expires_at](https://docs.github.com/en/rest/actions/artifacts)、[immutable releases](https://docs.github.com/en/code-security/concepts/supply-chain-security/immutable-releases)。

## 信任与停止线

- archive digest 不替代 artifact、provenance、acceptance 或 `checker.source` 各自身份；GitHub direct-file provider digest 即使与 tar digest 相同，也不替代内层成员摘要、provider artifact ID 或有效期。
- inner / outer `pack`、`verify` 与 distribution acceptance 成功不是 runtime registration、provider publication、installation、launcher isolation、签名或形式证明。
- 未完成 provider 回读或候选已经 expired / deleted 时，主仓只能记录 `unavailable` / `expired`，不能继续保持可取得声明。
- 没有 immutable setting 写后确认、精确 Release asset、provider attestation、独立重新下载复核、主仓登记和单独激活授权前，不形成 active registration 或正式 runtime companion。
