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

### Added

- `reqx/chix` 新增请求级规则扩展点：DTO 可实现 `ValidateRequest(*http.Request) error`（`RequestValidator`）在 `BindAndValidate*` 的组合流程中声明请求契约；同时提供 `RequireBody(...)` helper 复用默认 binder 的 body-required 判定。

### Removed

- 根包 `chix` 不再重导出 `reqx.Violation`、`errx.HTTPError`、`resp.ErrorWriteDegraded`；需要这些底层类型时请直接导入 `reqx`、`errx` 或 `resp`。
- `reqx` 收回了手写校验/错误构造辅助的导出面；当前公开入口聚焦在 `Bind*`、`BindAndValidate*`、`Param*`、`Normalizer`、`Violation`、绑定 option，以及已文档化的错误码 / violation 常量。
- `reqx.Violation` 不再保留 `Message` 兼容字段，字段级公开错误统一使用 `detail` / `Detail`。
- `resp` 不再公开错误模型与快捷错误构造；统一改为从 `errx` 使用 `HTTPError`、`NewHTTPError(...)` 及各状态快捷构造。
- `resp.JSONPretty(...)` 和根包 `chix.JSONPretty(...)` 已移除；`resp/chix` 也不再支持 `?pretty` 触发的 JSON 格式化输出。
- `errx.NewError(...)` 和 `resp.NewError(...)` 已移除；统一使用 `NewHTTPError(...)`。
- `reqx/chix` 的旧 binding option 与 path helper 已移除：`BindOption*`、`WithMaxBodyBytes(...)`、`DefaultMaxBodyBytes`、`ParamString(...)`、`ParamInt(...)`、`ParamUUID(...)` 不再属于公开 API。

### Changed

- 公开错误响应从包裹式 `{"error": {...}}` 调整为顶层 problem 风格对象；当前顶层字段固定为 `title`、`status`、`detail`、`code`，并在存在公开结构化错误详情时附带 `errors`。
- 对于 `reqx` 生成的字段级错误，`errors[]` 子项统一为 `field`、`in`、`code`、`detail`；`in` 用于标识错误输入来源，取值覆盖 `body`、`query`、`path`、`header`、`request`。
- `title` 现在统一由 HTTP 状态码生成，`detail` 承载公开错误说明，`code` 承载稳定机器码；公开错误响应不包含 `type` 和 `instance`。
- `errx.HTTPError.Error()` 现在仅在底层 `cause` 文本可用且非空白时返回该文本；当 `cause` 不可用、为空白或其 `Error()` 实现不安全时，会稳定回退到公开 `Detail()`。
- `reqx` 产生的绑定与校验错误会尽可能映射到请求侧 tag 名和来源位置，例如 `json:"name"` 会返回 `field: "name", in: "body"`。
- `reqx` 的默认 binder 当前聚焦 JSON API：`Bind(...)` / `BindAndValidate(...)` / `BindBody(...)` / `BindAndValidateBody(...)` 在 `Content-Length == 0` 时默认跳过 body 阶段，非空 body 只支持 `application/json`。
- `BindAndValidate*` 当前固定按 `Bind -> Normalize() -> ValidateRequest() -> validator/v10` 的顺序执行；其中请求级规则不再回流到 binding 层定义。
- 顶层 `null`、数组、数字、字符串、布尔值现在统一按 body 形状错误处理，返回 `400 invalid_json`。
- `reqx` 的 path 参数读取不再依赖 `chi.RouteContext`；`param:"..."` 现在只基于 `http.Request.PathValue(...)` / `http.Request.Pattern` 的命名 wildcard 语义工作，例如 `/{id}`、`/{path...}`；`chi` 专有的 `*` catch-all 不属于公开 path 契约。
- `reqx/chix` 的公开 binding 入口现在统一采用核心签名：`Bind(...)`、`BindBody(...)`、`BindQueryParams(...)`、`BindPathValues(...)`、`BindHeaders(...)` 全部改为 `target any`，根包同时公开 `Binder`、`DefaultBinder`、`BindUnmarshaler`。

### Docs

- 更新 [README.md](./README.md) 和包注释，补充当前错误响应契约、字段语义和示例，并收窄根包与 `reqx` / `resp` 的职责描述。
- README 中的公开包清单与包结构说明已同步到当前导出面：补充 `middleware` 包，并明确 `resp` 只负责响应写回与错误响应输出；公共错误模型由 `errx` 提供。
- README 已收缩为面向使用者的公开 API 指南，优先展示包职责、常用入口和示例用法，不再展开实现边界与内部技术细节。
- 新增 [docs/request-binding.md](./docs/request-binding.md)，明确综合绑定顺序、默认空 body 语义，以及 binding / request-rules / validate 的分层边界。
