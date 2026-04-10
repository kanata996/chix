# Changelog

本文档用于记录 `chix` 在当前开源仓库中的公开变更。

格式参考 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，在 `v1.0.0` 之后按 SemVer 理解版本语义。

## 版本说明

- 当前版本的公开边界以 [README.md](./README.md) 中描述的 API 和 HTTP 行为为准。
- `chix` 与 `middleware` 中已文档化的导出能力属于公开接口。
- 在 `v1.0.0` 之前，小版本发布中仍可能出现破坏性变更，但应在本文件中明确说明。

## [Unreleased]

### Changed

- `chix` 当前版本聚焦根包请求/响应入口和 `middleware` 中的 chi 日志封装。
- `chix.WriteError(...)`、`chix.Bind*`、`chix.BindAndValidate*`、`chix.JSON*` 等根包 API 建立在 `hah` 提供的 `net/http` 核心能力之上。
- `middleware` 提供与 `chi + httplog + traceid` 配套的 error responder preset 和 request log attrs 能力。
- 文档和示例中的共享错误模型入口统一为 `github.com/kanata996/hah/errx`。

### Docs

- README、示例和内部文档已同步到当前结构，不再引用仓库内已移除的子包实现文件。
