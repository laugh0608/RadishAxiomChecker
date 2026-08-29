## 目标与范围

请说明本次变更解决的问题、方案以及明确不包含的内容。

## 目标分支与变更类型

- 目标分支：`dev` / `master`
- [ ] checker 实现或缺陷修复
- [ ] 协议 / Evidence / 信任边界
- [ ] 测试 / fixture
- [ ] 构建 / payload / 供应链
- [ ] 文档 / CI / 仓库治理

如目标为 `master`，说明阶段性收口或 hotfix 理由，以及合并后的 `master -> dev` 回流方式。

## 隔离、语义与信任影响

- 是否改变 checker 与生产 `raxc` 的仓库、依赖、发布或进程隔离：
- 是否改变 `proved` / `checked` / `unknown` / `failed` / `trusted` 关系：
- 是否改变 request、bundle、Axiom IR、Axiom Evidence、companion 或 identity：
- 新增或改变的 trusted、外部能力、资源限制、dependency / action：
- 兼容性、失败模式与独立复核影响：

## 验证记录

只记录实际运行过的命令及结果：

```text
./scripts/check-repo.sh
GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go test -count=1 ./...
GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go vet ./...
./scripts/check-source-identity.sh
./scripts/check-module-closure.sh
test -z "$(gofmt -l .)"
```

## 检查清单

- [ ] 没有复用生产 parser、normalizer、义务生成器、解释器、Evidence 聚合器或测试 helper
- [ ] 没有把测试、CI 或 Git 身份表述为形式证明、payload acceptance 或 `checker.source`
- [ ] 新增外部材料已记录精确来源、版本、许可证与可信基影响
- [ ] 有意改变源码输入时已重建并审阅 `checker.source` sidecar
- [ ] 提交符合 Conventional Commits，未添加 AI 协作者署名
- [ ] 未提交凭据、真实敏感输入、cache、binary 或未登记生成物

## 未验证、风险与回滚

- 未验证内容：
- 已知风险：
- 回滚或兼容处理：
