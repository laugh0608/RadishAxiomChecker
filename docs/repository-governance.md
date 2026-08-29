# RadishAxiom Independent Checker 仓库治理

日期：2026-08-29

状态：远程建仓参数已于 2026-08-29 确认；实际远程状态以写后读取结果为准

## 目标与边界

本仓库负责独立 checker 的源码、测试和自身发布边界。它与生产 Rust `raxc` 保持独立 Git 仓库、依赖图、CI / 发布流水线和进程，不复用生产实现自证正确。

仓库治理只保证贡献和自动化遵守已声明边界；测试、CI、Git commit、tag 或远程状态不能替代 `checker.source`、checker binary、payload acceptance、Independent Check result 或形式证明。

## 分支与合并

- `master` 是 GitHub 默认稳定主线，只通过 PR 接收阶段性稳定化和 hotfix。
- `dev` 是常态集成分支。单人维护阶段允许维护者在执行完整本地门禁后直接推进 `dev`；外部贡献和并行工作使用 topic branch PR。
- 普通拓扑为 `topic -> dev -> master -> dev`。`dev -> master` 优先使用 merge commit，进入 `master` 后必须回流到 `dev`。
- 允许 merge commit 和 rebase merge，禁用 squash merge；共享分支禁止 force push、删除和破坏性历史重写。
- 提交遵循 Conventional Commits；语义、Evidence、信任、隔离、构建和 payload 边界变化必须在 PR 中显式说明。

远程 Ruleset 只保护 `master`。它必须等待 CI 的稳定 context `Candidate Quality` 在远程实际成功产生后另行设计和授权；本地模板不能冒充远程强制状态。

## CI 契约

`.github/workflows/pr-check.yml` 在 `master` / `dev` push、面向两者的 PR 和手工触发时运行，只授予 `contents: read`。两个组件 job 为：

1. `Repository Governance`：必需文件、Agent 入口同步、文本与差异卫生、action 精确固定、稳定聚合名和 PR commit 的 Conventional Commits。
2. `Checker Go Quality`：精确 `go1.26.7`、`go test`、`go vet`、`gofmt`、`checker.source` 和零第三方 module 闭包。

`.github/workflows/checker-payload-candidate.yml` 是隔离的手工候选流程，不进入常规 push / PR CI。它只接受显式 source、version 与上传确认，在 `macos-15` arm64 runner 上复跑门禁并走受控 build → payload acceptance → inner pack → distribution acceptance → outer pack，然后以 direct-file Actions artifact 保留唯一 distribution candidate 最多 90 天；独立 read-back job 只按 artifact ID 下载、运行严格 distribution verifier 并复核 provider API 元数据。该文件进入默认分支、触发 workflow、下载 Go archive、构建与上传都不是仓库内配置自行获得的授权，详见 [payload 候选归档与留存边界 v0.1](checker-payload-retention-v0.1.md)和 [runtime distribution package v0.1](checker-payload-distribution-v0.1.md)。

唯一供后续 Ruleset 绑定的稳定聚合 context 是 `Candidate Quality`。组件可在不改变该 context 的前提下扩展，但不得加入生产 Rust、solver、Node 或其他与 checker 无关的门禁。

CI 使用以下精确固定的官方 GitHub Actions：

- `actions/checkout` v6.0.2，commit `de0fac2e4500dabe0009e67214ff5f5447ce83dd`；
- `actions/setup-go` v7.0.0，commit `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e`；
- 手工候选上传使用 `actions/upload-artifact` v7.0.1，commit `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a`；
- 手工候选回读使用 `actions/download-artifact` v8.0.1，commit `3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c`。

`setup-go` 在隔离 CI runner 中取得官方 Go distribution，这是显式 CI 供应链依赖；`cache: false`，后续命令同时使用 `GOTOOLCHAIN=local` 与 `GOPROXY=off`，禁止 Go 自动切换工具链或取得 module。该 CI 工具链只提供测试和静态检查证据，不充当受控 payload builder 或已验收 `checker.artifact`。

## 远程建仓决策单

本治理切片开始前确认的本地事实是：仓库有 25 笔既有提交、仅有 `dev`、没有 `master`、remote、远程 CI 或 Ruleset。本切片形成的治理基线提交将成为第 26 笔。下表参数已在 2026-08-29 任务内获得所有者明确授权；治理基线 Git SHA 在提交形成后由远程 refs 写后读取和 RadishAxiom 主仓状态记录，不循环写入作为 `checker.source` 输入的本文件。

| 参数 | 建议值 | 状态 |
| --- | --- | --- |
| GitHub owner | `laugh0608` | 已确认 |
| 仓库名 | `RadishAxiomChecker` | 已确认 |
| 可见性 | `public` | 已确认；Apache-2.0 本身不决定可见性 |
| 描述 | `Independent Go checker for RadishAxiom evidence bundles.` | 已确认 |
| 许可证呈现 | 保留仓库现有 Apache-2.0 `LICENSE`，远程不生成新许可证 | 已确认 |
| remote 名 | `origin` | 已确认 |
| 默认分支 | `master` | 已确认 |
| 常态集成分支 | `dev` | 已确认 |
| 初始 `master` | 本地治理基线提交的精确 SHA | 已确认；提交后读取具体值 |
| 初始 `dev` | 与初始 `master` 相同，随后继续承接开发 | 已确认 |
| 首次推送 refs | `refs/heads/master`、`refs/heads/dev` | 已确认 |
| merge options | merge commit 与 rebase merge 开启，squash merge 关闭 | 已确认 |
| Issues / Wiki / Projects / Discussions | Issues 开启；其余关闭 | 已确认 |
| Ruleset | 本轮不创建 | 已由停止线确定 |
| Release / tag / payload | 本轮不创建或上传 | 已由停止线确定 |

建议首次落地顺序：创建空仓库且不自动生成 README / `.gitignore` / license；设置 `origin`；从同一治理基线提交建立本地 `master` 与 `dev`；精确推送两个 refs；设置默认 `master` 和 merge options；读取远程仓库、refs、设置及 CI run 复核。任一步失败都停止，不用 force push、删库或历史重写掩盖部分状态。

回退分层处理：尚未推送时只移除精确的本地 remote 配置；已经创建但未推送时可在获得单独删除授权后删除空远程仓库；已经推送后不自动删除仓库或 refs，而是保留可审计状态并报告差异，由所有者决定后续动作。

## 停止线

- 建仓、设置 remote、首次 push、修改远程设置、启用私密漏洞报告和创建 Ruleset 都是分别授权的外部动作。
- required context 没有在真实 CI 中成功产生前不创建 Ruleset。
- 不自动发布、创建 tag / Release、上传 binary、登记 payload、安装工具链或运行产品 checker。
- 不让 GitHub commit、workflow 成功或 repository visibility 冒充 `checker.source`、binary identity、acceptance 或 runtime companion。
