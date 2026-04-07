# Changelog

本文档用于记录 `chix` 在当前开源仓库中的公开变更。

迁移前的内部历史记录不再保留，后续版本记录从这里重新开始。

格式参考 [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)，在 `v1.0.0` 之后按 SemVer 理解版本兼容性。

## 兼容性说明

- 公开兼容边界以 [README.md](./README.md) 中描述的 API 和 HTTP 行为为准。
- `chix`、`reqx`、`resp` 中已文档化的导出能力属于公开接口。
- 测试辅助、基准布局、未文档化实现细节不属于公开 API。
- 在 `v1.0.0` 之前，小版本发布中仍可能出现破坏性变更，但应在本文件中明确说明。
- 在 `v1.0.0` 之后，破坏公开 API 或 HTTP 契约的变更应只出现在新的主版本中。

## [Unreleased]

### Removed

- 根包 `chix` 不再重导出 `reqx.Violation`、`resp.HTTPError`、`resp.ErrorWriteDegraded`；需要这些底层类型时请直接导入 `reqx` 或 `resp`。
- `reqx` 收回了手写校验/错误构造辅助的导出面；当前公开入口聚焦在 `Bind*`、`BindAndValidate*`、`Param*`、`Normalizer`、`Violation`、绑定 option，以及已文档化的错误码 / violation 常量。
- `reqx.Violation` 不再保留 `Message` 兼容字段，字段级公开错误统一使用 `detail` / `Detail`。
- `resp.HTTPError` 不再保留 `Message()`、`Details()` 兼容别名；统一使用 `Detail()`、`Errors()`。

### Changed

- 公开错误响应从包裹式 `{"error": {...}}` 调整为顶层 problem 风格对象；当前顶层字段固定为 `title`、`status`、`detail`、`code`，并在存在公开结构化错误详情时附带 `errors`。
- 对于 `reqx` 生成的字段级错误，`errors[]` 子项统一为 `field`、`in`、`code`、`detail`；`in` 用于标识错误输入来源，取值覆盖 `body`、`query`、`path`、`header`、`request`。
- `title` 现在统一由 HTTP 状态码生成，`detail` 承载公开错误说明，`code` 承载稳定机器码；公开错误响应不包含 `type` 和 `instance`。
- `resp.HTTPError.Error()` 现在仅在底层 `cause` 文本可用且非空白时返回该文本；当 `cause` 不可用、为空白或其 `Error()` 实现不安全时，会稳定回退到公开 `Detail()`。
- `reqx` 产生的绑定与校验错误会尽可能映射到请求侧 tag 名和来源位置，例如 `json:"name"` 会返回 `field: "name", in: "body"`。
- 请求侧 JSON body 明确接受 `application/json` 和 `application/*+json`；错误响应的 `Content-Type` 明确为 `application/problem+json`，以对齐 Huma 的 problem 响应约定。
- `reqx` 的 path 参数读取不再依赖 `chi.RouteContext`；`param:"..."` 现在只基于 `http.Request.PathValue(...)` / `http.Request.Pattern` 的命名 wildcard 语义工作，例如 `/{id}`、`/{path...}`；`chi` 专有的 `*` catch-all 不属于公开 path 契约。

### Docs

- 更新 [README.md](./README.md) 和包注释，补充当前错误响应契约、字段语义和示例，并收窄根包与 `reqx` / `resp` 的职责描述。
