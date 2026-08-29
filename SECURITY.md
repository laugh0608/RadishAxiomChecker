# 安全策略

RadishAxiom Independent Checker 尚未发布稳定版本，也不承诺固定响应 SLA；但错误接受无效 Evidence、隐藏信任、身份混淆和资源隔离失效从项目早期开始就按安全问题处理。

## 私下报告

请不要为未修复漏洞创建公开 Issue，也不要附带真实凭据、个人数据或生产输入。

远程仓库建立并启用对应能力后，优先使用 GitHub Security 页面中的 Private vulnerability reporting。若该入口不可用，可发送邮件至 `laugh0608@foxmail.com`，主题包含 `[RadishAxiom Checker Security]`。

报告应尽量包含受影响提交、最小复现、预期保证、实际行为、可信基或隔离影响、已知利用条件和建议缓解。公开时间应由报告者与维护者协调。

## 安全问题范围

- 把未证明或未检查的性质错误升级为 `proved` 或 `checked`；
- 遗漏、隐藏或错误传播 `trusted`、missing artifact 或外部能力；
- 接受无效、被替换、截断、错误归属或身份不闭合的 request、bundle、Axiom IR、Axiom Evidence 或 companion；
- 与生产实现共享本应隔离的 parser、义务重建、解释或聚合路径；
- 路径穿越、symlink / alias 混淆、越权 I/O、运行时网络或可变 cache 导致的替换；
- 恶意输入造成任意代码执行、资源失控或拒绝服务；
- toolchain、GitHub Actions、构建、payload、registry 或发布流水线的供应链风险；
- 凭据、私有 fixture、真实用户数据或验证材料泄露。

普通拼写、无安全影响的文档错误和不改变验证结论的功能建议可以使用公开 Issue。
