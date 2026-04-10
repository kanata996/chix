# chix 项目验收标准与测试规范

本文档定义 `chix` 仓库当前结构下的测试质量标准。

适用范围：

- 根包 `chix`
- `middleware`
- `_example`

## 总原则

- 测试应证明公开行为，而不是只抬高覆盖率
- 失败信息应能定位具体行为
- 不依赖网络、真实时钟、随机顺序或外部环境偶然性

## 根包 `chix`

根包是 facade，评审重点不是复杂逻辑，而是“导出的入口与 `hah` 契约一致，同时保留 chix 自己的 chi 预设”。

至少覆盖：

- facade 是否正确暴露 `hah` 的核心请求/响应入口
- 根包 helper 的行为是否与 `hah` 一致
- `WriteError(...)` 是否仍保留 `traceId` / request log 注解相关预设
- README 中承诺的示例路径是否能被测试支撑

## `middleware`

`middleware` 负责把请求关联字段补到当前 request log。

至少覆盖：

- `traceId` 与 `request.id` 的注入行为
- 在 panic recover 等异常路径下仍能保留关联字段
- 缺失上下文值时的 no-op 行为

## `_example`

示例不是玩具文件，评审时要保证它仍然代表推荐接入方式。

至少覆盖：

- 示例能成功编译
- 示例中的 `chix.WriteError(...)`、`chix.BindAndValidate(...)`、`middleware.RequestLogAttrs()` 组合仍成立
- 示例里涉及的共享错误模型导入路径与 README 一致
