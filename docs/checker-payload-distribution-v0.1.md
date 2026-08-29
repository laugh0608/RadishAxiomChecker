# Independent Checker runtime distribution package v0.1

本文实现 RadishAxiom ADR 0010 冻结的首个 macOS arm64 runtime distribution 前置。它在既有 payload acceptance 与内层候选归档之后，形成独立 distribution acceptance、闭合 staging root 和确定性外层 USTAR。它不启用 immutable releases，不创建 tag / Release，不上传、登记、安装或激活 runtime。

## 独立 distribution acceptance

`radishaxiom-checker-distribution-accept accept` 只消费以下显式 canonical realpath：

- 当前 Checker source root；
- 已通过严格内层 verifier 的 `checker-payload-candidate.tar`；
- 已接受的精确 `go1.26.7.darwin-arm64.tar.gz`；
- 一个尚不存在的输出目录；
- 精确 Checker implementation version。

acceptor 在物化前后复算 `checker.source`，并在物化前后重新读取候选与 Go archive，任何 source 或输入字节漂移都失败。内层候选的 source / version 必须与当前源码和请求一致。生产策略只接受下列精确工具链与法律材料身份：

| 输入或材料 | 长度 | 原始 SHA-256 |
| --- | ---: | --- |
| `go1.26.7.darwin-arm64.tar.gz` | `64772572` | `sha256:020a1e8224811be75163e920bc77e0926a1390a6aeea19bdcf23f74b9d749f6d` |
| `go/LICENSE` | `1453` | `sha256:911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad` |
| `go/PATENTS` | `1303` | `sha256:96f408bfae65bf137fc2525d3ecb030271c50c1e90799f87abf8846d8dd505cc` |
| Checker `LICENSE` | `11357` | `sha256:c71d239df91726fc519c6eb72d318ec65820627232b2f796219e87dcf35d0ab4` |

Go 法律材料必须直接从上述已验收 archive 的精确 `go/LICENSE` 与 `go/PATENTS` 普通成员取得；不从网络、系统 Go、其他目录或 fallback 补齐。Checker `LICENSE` 必须来自当前 source root。成功时 acceptor 以 exclusive create 形成以下闭合目录，文件均为 `0644`，目录均为 `0755`；失败时删除本次创建的输出目录，不保留半成品：

```text
checker-payload-candidate.tar
checker-payload-distribution-acceptance-v0.1.jcs
licenses/go/LICENSE
licenses/go/PATENTS
licenses/radishaxiom-checker/LICENSE
```

canonical acceptance 绑定 source、version、darwin / arm64 / v8.0 / Mach-O target、`go1.26.7` payload、内层候选及其 retention manifest，并逐项绑定三份法律材料。decision 为 `accepted-for-controlled-durable-publication-candidate`，只接受 `distribution-byte-inventory`、`license-material-inclusion`、`payload-identity-binding` 与 `target-scoped-distribution`；明确排除跨平台等价、安装、所有司法辖区法律结论、launcher hard isolation、provider publication、release signing 和 runtime activation。

## 确定性外层归档

`radishaxiom-checker-payload-distribution pack` 只接受上述闭合 acceptance root、当前 source root、新 output file 和精确版本。packer 不信任目录名或 acceptor 的进程成功状态，而是重新严格解析内层候选和 distribution acceptance，复核固定工具链 / 法律材料身份、source / version / target 与闭合目录，再生成 `checker-payload-distribution-manifest-v0.1.jcs`。

外层 `radishaxiom-checker-runtime-distribution` `0.1` 按以下唯一顺序包含 6 个普通文件：

```text
checker-payload-candidate.tar
checker-payload-distribution-acceptance-v0.1.jcs
checker-payload-distribution-manifest-v0.1.jcs
licenses/go/LICENSE
licenses/go/PATENTS
licenses/radishaxiom-checker/LICENSE
```

manifest 为每个非 manifest 内容记录 role、path、mode、长度和原始 SHA-256，并绑定 implementation、source、version、toolchain 与 target。USTAR header 固定为 `0644` 普通文件、uid / gid `0`、空 user / group、Unix epoch mtime、无 link / PAX / xattr；绝对路径、生成时间、PID、runner 和 staging 目录不进入 archive。输出文件名必须精确为：

```text
radishaxiom-checker-go<version>-darwin-arm64-v8.0-sha256-<checker-source-hex>.distribution.tar
```

`verify` 严格核对成员集合、顺序、header、canonical manifest / acceptance、内层候选、全部摘要和固定身份，再从解析结果重建完整 USTAR 并逐字比较。额外 / 缺失成员、尾随字节、错误文件名、未知 JSON 字段、非规范 JSON、法律材料或内层候选篡改全部拒绝。

## 命令

```sh
GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go run ./cmd/radishaxiom-checker-distribution-accept \
  accept \
  --source-root=/canonical/RadishAxiomChecker \
  --candidate-archive=/canonical/staging/checker-payload-candidate.tar \
  --toolchain-archive=/canonical/go1.26.7.darwin-arm64.tar.gz \
  --output-root=/canonical/staging/accepted-distribution \
  --version=0.1-dev

GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go run ./cmd/radishaxiom-checker-payload-distribution \
  pack \
  --source-root=/canonical/RadishAxiomChecker \
  --distribution-root=/canonical/staging/accepted-distribution \
  --output-file=/canonical/staging/radishaxiom-checker-go0.1-dev-darwin-arm64-v8.0-sha256-<checker-source-hex>.distribution.tar \
  --version=0.1-dev

GOTOOLCHAIN=local CGO_ENABLED=0 GOPROXY=off go run ./cmd/radishaxiom-checker-payload-distribution \
  verify \
  --archive=/canonical/staging/radishaxiom-checker-go0.1-dev-darwin-arm64-v8.0-sha256-<checker-source-hex>.distribution.tar
```

这些入口不下载 Go archive、不发布或上传任何字节，也不生成 tag、Release、registration 或 launcher 配置。workflow 可在另行授权后编排精确下载、构建、两级 acceptance / pack 与按 artifact ID 回读，但 Actions artifact 仍只是最长 90 天的 `distribution-candidate-retained-temporarily`，不是 durable Release 或 active runtime。

## 信任与停止线

- distribution acceptance 只对上述精确 source、版本、target、工具链和材料 inventory 作受限决定，不是普遍法律意见，也不证明 checker 正确或跨平台等价。
- 外层 archive digest 不替代内层 artifact、provenance、payload acceptance、retention manifest、distribution acceptance 或 `checker.source` 各自身份。
- pack / verify 或 workflow 成功不表示 immutable releases 已启用，不形成 provider attestation、durable publication、主仓 registration、安装、签名或激活。
- Checker 增加第三方 module、`NOTICE`、生成器、`cgo`、动态库或其他需随分发保留的材料时，必须扩展精确 inventory、acceptance、manifest、负例与 source identity，不能沿用本版本默认接受。
