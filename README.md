# chix

`chix` 是一个面向 `chi` 的轻量级 JSON API HTTP 边界工具包。

当前主要提供两个子包：

- `reqx`：请求绑定、path/query helper 与校验
- `resp`：Echo 风格 JSON 响应写回与结构化错误响应

这个库目前先在 `app-mall` 内孵化，后续再拆分出来做更广泛的复用。

根包 `chix` 提供了一层精简的 facade，用于暴露常用的请求绑定和响应写回入口。
如果你需要完整能力面，直接导入 `reqx` / `resp` 即可。

`reqx` 当前暴露的绑定入口尽量对齐 Echo 风格：

- `Bind`: path -> query (`GET`/`DELETE`) -> body
- `BindBody`, `BindQueryParams`, `BindPathValues`, `BindHeaders`

在当前支持的数据源范围内，默认行为尽量贴近 Echo binding 语义：

- unknown query 和 header key 默认忽略
- 空 JSON body 视为 no-op
- JSON request body 使用 Go 标准库解码，默认不拒绝 unknown field

`resp` 当前同时提供两层响应能力：

- Echo 风格 JSON 核心入口：`JSON`、`JSONPretty`、`JSONBlob`
- 更高层成功响应 helper：`OK`、`Created`、`NoContent`
- `WriteError(...)` 在存在 `httplog` request logger 时，会给同一条请求日志补充 `error.code`、`error.message`、`error.type`、`error.root_message`、`error.details` 等排障字段
