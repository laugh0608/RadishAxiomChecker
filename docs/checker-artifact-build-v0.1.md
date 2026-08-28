# Independent Checker macOS arm64 受控构建与 payload acceptance v0.1

本文记录 `radishaxiom-independent-checker-go` 首个 macOS arm64 payload 的受控构建与独立验收路径。它消费已经由 RadishAxiom 主仓库 Toolchain Payload Acceptance v0.1 接受为受控构建输入的官方 `go1.26.7.darwin-arm64.tar.gz` 原始字节，不安装 Go、不修改 `PATH` / `GOROOT`，也不从网络、系统 Go 或可变 cache 补齐构建输入。

## 受控构建输入

`radishaxiom-checker-build` 只接受四项显式输入：

- 当前 checker source root 的 absolute canonical realpath；
- 官方 Go archive 的 absolute canonical realpath；
- 一个已经存在且为空的 output root absolute canonical realpath；
- 非空、非 `latest`、长度不超过 64 bytes 的精确 ASCII 实现版本。

归档必须精确为 `64,772,572` bytes，SHA-256 必须为 `020a1e8224811be75163e920bc77e0926a1390a6aeea19bdcf23f74b9d749f6d`。构建器使用 Go 标准库重新检查原始摘要并安全解压，只接受 `go/` 下规范路径、`0755` 目录、`0644` / `0755` 普通文件和已登记的 `16,701` member / `228,748,173` regular bytes；symlink、hardlink、特殊文件、路径别名、重复 path 或 inventory 漂移全部失败。

source root 必须通过当前 `checker.source` sidecar 重放。工具链 `VERSION` 与实际 `<extracted>/go/bin/go version` 必须同时精确为 `go1.26.7` / `darwin-arm64`；构建器不搜索 PATH，也不允许 `GOTOOLCHAIN` 自动下载。

## 两次隔离构建

构建器在两个不同的临时 root 中分别建立独立 `GOCACHE`、`GOMODCACHE`、`GOPATH`、`HOME` 与 `TMPDIR`，固定：

- `GOTOOLCHAIN=local`、`CGO_ENABLED=0`、`GOPROXY=off`、`GOSUMDB=off`、`GOTELEMETRY=off`、`GOWORK=off`、`GOENV=off`；
- `GOOS=darwin`、`GOARCH=arm64`、`GOARM64=v8.0`；
- 空 `PATH`，固定 `LANG=C`、`LC_ALL=C`、`TZ=UTC`；
- `go build -mod=readonly -trimpath -buildvcs=false -p=1`；
- linker build ID 为空，并注入已经重放的 `checker.source` 与精确实现版本。

只有两次 binary 原始 bytes 完全相同，构建器才向空 output root 写入一个 `0755` checker artifact 和一个 `0644` canonical build provenance。provenance 不含绝对路径、时间戳、PID 或 cache 名，绑定 payload 摘要、source、版本、target、命令 / 环境 profile、artifact byte length / SHA-256 与两次字节一致事实。临时 toolchain、cache 和中间 binary 随构建器退出清理。

构建器在初始重放之外，还会在每次隔离构建后以及最终物化前重新核对 source sidecar；任一时点的源码身份漂移都会失败，空 output root 不会收到载荷。

开发入口示例；运行 orchestrator 的本机 Go 只负责启动受控流程，正式 checker binary 只能由归档内精确 `go1.26.7` 产生：

```sh
GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go run ./cmd/radishaxiom-checker-build \
  build \
  --source-root=/canonical/RadishAxiomChecker \
  --toolchain-archive=/canonical/go1.26.7.darwin-arm64.tar.gz \
  --output-root=/canonical/empty-output \
  --version=0.1-dev
```

## 独立 payload acceptance

`radishaxiom-checker-accept` 不调用编译器，也不读取 builder 的成功状态。它从 output root 重新完成：

1. 闭合目录检查，只允许 checker artifact 与 build provenance；
2. artifact 原始 byte length / SHA-256、`0755` 普通文件与 canonical realpath 检查；
3. 使用 Go 标准库的独立 Mach-O / build info reader，要求 64-bit arm64 executable、`go1.26.7`、module / command identity、零 module dependency、`CGO_ENABLED=0`、darwin / arm64 / v8.0、`-trimpath`，并拒绝 VCS 或额外 build setting；
4. 根据 source、版本与实际 artifact 独立重建唯一 provenance bytes，再与 builder 文件逐字比较；
5. 在空环境、隔离 cwd 和 6 秒 hard deadline 下直接按唯一产品 CLI 运行 normal、`CHK-DIGEST-01`、`CHK-RESOURCE-01` 三个锁定 bundle；用不同于生产 checker strict parser 的标准库 JSON / canonical round-trip 小路径核对 artifact / source / toolchain / version / TCB 和 `accepted-with-trust` / `rejected` / `incomplete` 三种结果；
6. 成功后新增 canonical payload acceptance record，绑定 provenance、artifact、三次 result 原始摘要和明确 accepted / excluded scope。

```sh
GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go run ./cmd/radishaxiom-checker-accept \
  accept \
  --source-root=/canonical/RadishAxiomChecker \
  --build-root=/canonical/build-output \
  --version=0.1-dev
```

## 信任边界与停止线

两次字节一致只说明当前 source、精确 Go payload、target、flags 与固定环境在本机受控流程中产生相同 bytes；它不是源码可证明正确、Go host 可由 source 复现、跨平台等价或形式证明。acceptance 的 `accepted-for-controlled-runtime-registration` 只表示该 payload 已具备登记候选所需的本地构建与身份 / 场景证据，不会自动修改主仓 registry、execution profile、安装位置、Release 或 Evidence。

本切片不提交 checker binary，不安装或发布工具链，不修改 shell / 系统配置，不形成签名、公证、安装包、launcher hard memory / filesystem / network 隔离或其他五个平台结论。使用 `0.1-dev` 只形成精确开发实现身份；正式产品版本仍须按项目版本治理另行冻结，不能把 dev payload 冒充 release。
