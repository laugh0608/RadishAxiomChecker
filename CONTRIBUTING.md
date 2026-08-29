# 参与 RadishAxiom Independent Checker

本仓库只实现与生产 `raxc` 分离的独立 checker。贡献必须保留仓库、依赖图、发布流水线和进程隔离，不得为了复用方便引入生产 parser、normalizer、义务生成器、解释器、Evidence 聚合器或测试 helper。

开始前请阅读 [README](README.md)、[协作约定](AGENTS.md)和[仓库治理](docs/repository-governance.md)。安全问题不要创建公开 Issue，请遵循 [SECURITY.md](SECURITY.md)。

## 分支与 PR

- `master` 是默认稳定主线，只接收阶段性稳定化或 hotfix，并在远程基线建立后只通过 PR 更新。
- `dev` 是常态集成分支；普通贡献从 `feature/*`、`fix/*`、`docs/*`、`proposal/*`、`experiment/*` 或 `chore/*` 向 `dev` 发起 PR。
- `dev -> master` 使用可审计的阶段 PR；合并后必须把 `master` 回流到 `dev`，再开始下一批开发。
- 禁止 force push 或破坏性重写共享分支；不使用 squash merge 压平可审计提交。

PR 应说明目标、范围、实际验证、未验证内容、信任边界、风险和回滚。协议、Evidence、验证状态、资源限制、构建身份或发布边界变化还必须说明兼容性、失败模式和独立复核影响。

## 提交信息

提交使用 Conventional Commits，例如：

```text
feat(checker): 重放新的反例目标
fix(parser): 拒绝重复成员
docs(checker): 澄清 trusted 边界
ci(repo): 建立独立质量门禁
```

常用类型为 `feat`、`fix`、`docs`、`refactor`、`test`、`chore`、`ci`、`build`、`perf` 和 `revert`。提交使用贡献者自己的 Git 身份，不添加 AI 协作者署名。

## 本地验证

当前完整本地门禁为：

```sh
./scripts/check-repo.sh
GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go test -count=1 ./...
GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go vet ./...
./scripts/check-source-identity.sh
./scripts/check-module-closure.sh
test -z "$(gofmt -l .)"
```

有意改变闭合源码输入时，先运行 `./scripts/update-source-identity.sh`，审阅 sidecar 差异后再执行上述门禁。测试或 CI 成功只表示实现路径通过检查，不构成形式证明、可发布 payload 或跨平台结论。

## 许可证与第三方材料

除非另有明确书面约定，贡献按 [Apache License 2.0](LICENSE) 提交和分发。新增第三方代码、action、数据或 fixture 必须记录精确来源、版本、许可证与可信基影响；Go core 继续只允许标准库，新增依赖或构建时下载需要单独审查和授权。
