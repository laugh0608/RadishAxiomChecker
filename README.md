# RadishAxiom Independent Checker (Go)

这是 RadishAxiom 独立 checker 的 Go 实现仓库。它与生产 Rust `raxc` 分仓、分依赖图、分发布流水线和分进程，只通过版本化协议和按摘要锁定的离线制品交换信息。

当前定位是有键有限表受限 profile 的独立语义与证据检查器：已实现独立解析、义务重建、有限执行、反例和具体输出重放、证明支撑材料审计、结论重算及 CLI。当前 kernel / certificate 独立证明能力为空；接受一份 Evidence 不等于证明候选程序正确，也不表示外层产品隔离已经验收。

## 阅读入口

- [当前状态与能力边界](docs/status/current.md)：现在能检查什么、尚不能保证什么、下一步与待复核问题。
- [开发目标与验收计划](docs/development-plan.md)：反例目标归因、同域泛化、正向证明链、结果可解释性和测试验收。
- [文档索引与结构导航](docs/README.md)：协议、语义检查、资源、构建与分发专题。
- [仓库治理](docs/repository-governance.md)、[贡献指南](CONTRIBUTING.md)和[安全报告](SECURITY.md)。

专题切片保存其实现范围和验收背景；跨切片进度以当前状态为入口。精确 payload 的构建、发布、登记与激活事实由仓库外 provenance / acceptance 和主仓 registry 给出，源码 HEAD 或本地测试不能替代这些身份。

## 本地验证

模块固定 `go 1.26.0` 与 `toolchain go1.26.7`，core 只使用标准库，无 `go.sum`、vendor 或 `cgo`。正式 Go 质量门禁使用精确 `go1.26.7`，禁止自动下载工具链或 module：

```sh
./scripts/check-repo.sh
GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go test -count=1 ./...
GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go vet ./...
gofmt -l .
./scripts/check-source-identity.sh
./scripts/check-module-closure.sh
```

`gofmt -l .` 应无输出。非固定版本的本地检查如有执行，必须单独标注实际版本，仅作补充，不能替代精确工具链门禁或 payload acceptance。

文档、测试和构建代码都属于 [checker.source 输入集合](docs/checker-source-v0.1.md)。有意修改后运行 `./scripts/update-source-identity.sh`，审阅唯一 sidecar diff，再运行源码身份检查。临时文件、cache 和构建产物放在仓库之外；未跟踪或 ignored 文件也不会被源码身份自动排除。

## CLI 与交付边界

唯一产品入口为 `check --bundle-root=<canonical-realpath>`，详见 [CLI 契约](docs/checker-cli-v0.1.md)。正式构建必须注入 source / 精确版本，并在 bundle I/O 前绑定实际 `go1.26.7` 与自身 executable 摘要；未注入身份的普通本机构建不充当正式 checker artifact。

退出码 `0` 表示完整 canonical result 已输出；调用方仍须读取四态结果和 Evidence 的程序结论。`rejected` 或 `incomplete` 也可能具有完整结果；无结果的进程失败由外层另行处理。

受控构建、payload / distribution acceptance 和归档命令只消费显式 canonical path，不负责下载、安装、上传、登记或发布。binary 和归档不提交到源码仓库。

## CI 与分支

`dev` 是常态开发与集成分支，`master` 是默认稳定主线。常规 CI 在面向两者的 PR 和手工触发时运行，普通分支 push 不自动触发；稳定聚合 context 为 `Candidate Quality`。payload 候选流程独立且仅手工触发，具体授权与远程规则见仓库治理。
