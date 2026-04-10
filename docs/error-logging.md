# 错误响应与日志

本文只描述 `chix` 当前对外承诺的行为，不展开 `hah` 内部调用顺序。

相关入口：

- `chix.WriteError(...)`
- `chix.NewErrorResponder()`
- `middleware.RequestLogAttrs()`

相关源码：

- `error_responder.go`
- `middleware/httplog_attrs.go`
- `error_responder_test.go`
- `middleware/httplog_attrs_test.go`

## 分层定位

- `hah.WriteError(...)` / `hah.ErrorResponder`：纯 `net/http` 错误收敛与错误响应写回
- `chix.WriteError(...)` / `chix.NewErrorResponder()`：在 `hah` 之上补 `chi + httplog + traceid` 预设

`chix` 不改变客户端看到的公共错误 JSON 契约；它补的是 request log 注解和独立 server error log 的上下文字段。

## 默认预设做了什么

`chix.NewErrorResponder()` 基于 `hah.NewErrorResponder()` 创建 responder，并额外配置：

- 从请求 context 中提取 `traceId`
- 在存在 `httplog` request logger 时，把错误诊断字段注入当前 request log

`chix.WriteError(...)` 只是调用这个默认 responder。

## Request Log 行为

`hah.WriteError(...)` 默认不会主动集成 request log。

`chix.WriteError(...)` 在 `5xx` 场景下会尝试给当前 `httplog` request log 补充低噪音诊断字段：

- `error.code`
- `error.timeout`
- `error.canceled`

如果当前请求链路没有 `httplog` request logger，这一步会退化为 no-op，不影响错误响应本身。

如果你希望 access log 同时带上关联字段，需要显式挂载：

- `traceid.Middleware`
- `middleware.RequestLogAttrs()`

如果你还希望 access log 里有 `request.id`，通常也需要挂载：

- `chi/middleware.RequestID`

## 独立 Error Log 行为

当错误最终收敛为 `5xx` 时，responder 会输出一条独立 `error` 日志。

当前日志消息为：

- `resp: request failed with server error`

稳定字段：

- `http.response.status_code`
- `error.code`

按条件追加：

- `error.message`
- `error.type`
- `error.root_message`
- `error.root_type`
- `error.timeout`
- `error.canceled`
- `http.request.method`
- `url.path`

说明：

- `error.message` / `error.type` / `error.root_message` / `error.root_type` 只有在能从诊断错误链中提取到值时才会出现
- `error.timeout` 仅在 `errors.Is(err, context.DeadlineExceeded)` 时写入
- `error.canceled` 仅在 `errors.Is(err, context.Canceled)` 时写入
- `http.request.method` / `url.path` 只有请求对象可用且字段非空时才会出现

在 `chix` 预设下，如果请求 context 中有 `traceId`，独立 error log 还会补：

- `traceId`

`request.id` 不会由 `chix.WriteError(...)` 自动补到独立 error log；它主要用于 access log 关联。

## 错误响应写出失败

如果错误响应自身写出失败，responder 还会额外输出一条独立 `error` 日志。

当前日志消息为：

- `resp: failed to write error response`

稳定字段：

- `http.response.status_code`
- `error.code`

当底层错误可解为 `ErrorWriteDegraded` 时，日志里还会补充：

- `resp.error_degraded`
- `resp.public_response_preserved`

和普通独立 error log 一样，`traceId`、`http.request.method`、`url.path` 仍然取决于请求 context / 请求对象是否可用；`error.message` / `error.type` / `error.root_message` / `error.root_type` 也属于条件字段。

## 推荐接入方式

典型 `chi` 链路：

1. `chi/middleware.RequestID`
2. `traceid.Middleware`
3. `httplog.RequestLogger(...)`
4. `middleware.RequestLogAttrs()`
5. handler 中使用 `chix.WriteError(...)`

这样可以得到：

- 客户端侧统一错误 JSON
- access log 中的 `traceId` / `request.id`
- `5xx` 时附带 `traceId` 的独立 error log
