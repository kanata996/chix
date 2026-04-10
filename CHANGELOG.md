# Changelog

本文档用于记录 `chix` 在当前开源仓库中的公开变更。

格式参考 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，在 `v1.0.0` 之后按 SemVer 理解版本语义。

## 版本说明

- 当前版本的公开边界以 [README.md](./README.md) 中描述的 API 和 HTTP 行为为准。
- `chix` 与 `middleware` 中已文档化的导出能力属于公开接口。
- 在 `v1.0.0` 之前，小版本发布中仍可能出现破坏性变更，但应在本文件中明确说明。

## [Unreleased]

### Changed

- `chix` 根包当前只保留 `chi` 侧错误响应预设：`WriteError`、`ErrorResponder`、`NewErrorResponder`。
- 请求绑定、输入校验、成功响应和共享错误模型统一从 `hah` 暴露。
- `middleware` 提供与 `chi + httplog + traceid` 配套的 error responder preset 和 request log attrs 能力。
- 文档和示例已调整为 `hah` 处理 binding / response，`chix` 处理 chi 侧扩展。

### Docs

- README、示例和内部文档已同步到当前结构，不再引用仓库内已移除的子包实现文件。
