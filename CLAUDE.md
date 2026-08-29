# RadishAxiom Independent Checker 协作约定

- 默认使用中文讨论和文档；代码、协议字段、路径与外部名称保留原文。
- 开始任务先检查 `git status`，保留用户和并行协作者的现有更改。
- checker 必须与生产 `raxc` 保持仓库、依赖图、发布流水线和进程隔离；不得导入、复制或生成生产 parser、normalizer、义务生成器、解释器、Evidence 聚合器或测试 helper。
- 规范与 fixture 只能按精确来源提交和摘要导入；禁止 submodule、相对路径依赖、Go `replace`、运行时网络、PATH 搜索、当前目录 fallback 或可变 cache key。
- 当前 Go module 固定 `go 1.26.0`、`toolchain go1.26.7`、`GOTOOLCHAIN=local`、`CGO_ENABLED=0`，core 只允许标准库。依赖、代码生成、`cgo`、动态库和构建时下载必须单独授权并记录可信基影响。
- 严格入口默认拒绝未知字段、tag、版本、重复成员、非法 UTF-8、JSON number、`null`、非规范 JCS、路径别名、symlink、长度或摘要不一致。
- `proved`、`checked`、`unknown`、`failed` 与 `trusted` 不可互换；测试成功不得升级为证明。
- `dev` 是常态开发与集成分支；串行普通任务直接在 `dev` 推进，外部贡献、并行写入、风险隔离、明确评审需求或 hotfix 才使用主题分支。Agent 不自动创建 `codex/*` 分支或额外 worktree；普通 push 不自动触发 CI，直接开发执行完整本地门禁。
- 修改后复核 diff、状态、未跟踪文件和实际验证；Git 提交遵循 Conventional Commits，不添加 AI 署名，不擅自 push、发布、部署或改写历史。
