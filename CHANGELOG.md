# Changelog

本文档用于记录 `chix` 在当前开源仓库中的公开变更。

格式参考 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，在 `v1.0.0` 之后按 SemVer 理解版本语义。

## 版本说明

- 当前版本的公开边界以 [README.md](./README.md) 中描述的 API 和 HTTP 行为为准。
- `chix` 与 `middleware` 中已文档化的导出能力属于公开接口。
- 在 `v1.0.0` 之前，小版本发布中仍可能出现破坏性变更，但应在本文件中明确说明。

## [Unreleased]

## [v0.4.3] - 2026-04-11

### Changed

- `chix` 根包移除 `JSON`、`JSONBlob`、`OK`、`Created`、`NoContent` 成功响应直通入口；成功响应请直接使用 `hah`。

## [v0.4.2] - 2026-04-10

### Changed

- 依赖 `github.com/kanata996/hah` 从 `v0.3.1` 升级到 `v0.3.2`，本次发布仅同步底层补丁版本，不新增 `chix` 公开 API。

## [v0.4.1] - 2026-04-10

### Added

- `chix` 根包新增 `JSON`、`JSONBlob`、`OK`、`Created`、`NoContent` 直通入口，便于 handler 统一使用 `chix.*` 写响应；其底层行为仍委托 `hah`。

### Changed

- README、示例和专题文档已同步到当前公开边界：`middleware` 子包只提供 request log 关联字段中间件，独立 error log 行为由 `chix` 根包的错误响应预设负责。

## [v0.4.0] - 2026-04-10

### Changed

- `chix` 根包当前只保留 `chi` 侧错误响应预设：`WriteError`、`ErrorResponder`、`NewErrorResponder`。
- 请求绑定、输入校验、成功响应和共享错误模型统一从 `hah` 暴露。
- `middleware` 提供与 `chi + httplog + traceid` 配套的 request log attrs 能力。
- 文档和示例已调整为 `hah` 处理 binding / response，`chix` 处理 chi 侧扩展。

### Docs

- README、示例和内部文档已同步到当前结构，不再引用仓库内已移除的子包实现文件。
