# middleware

这个子包用于放置基于 `chix` 核心能力构建的可选中间件。

当前提供：

- `Recoverer(opts ...middleware.Option) func(http.Handler) http.Handler`
- `WithPanicReporter(reporter PanicReporter) middleware.Option`

当前 `Recoverer` 的关键行为：

- panic 时默认通过 Go 标准库 logger 记录 panic 值、`X-Request-Id` 与 stack 到 `stderr`
- 可安全写回时输出统一 JSON `500`
- `http.ErrAbortHandler` 继续透传
- 响应已开始或连接已 hijack 后不再改写公开结果
- 只有明确的协议升级握手请求才不主动写 JSON `500`（例如同时带 `Connection: Upgrade` 和 `Upgrade: websocket`）

设计约束：

- 保持标准 `net/http` middleware 形态
- 可以依赖根包 `chix`
- 不引入对特定 router 的绑定
- 不反向要求根包依赖本子包
