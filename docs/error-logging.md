# 错误日志当前行为

本文说明两层行为：

- `hah.WriteError(...)` / `hah.ErrorResponder` 零值：纯 `net/http` 核心行为
- `chix.WriteError(...)`：基于 `chi + httplog + traceid` 预设好的行为

相关源码：

- `chix/error_responder.go`
- `hah` 仓库中的 `resp/write_error.go`
- `hah` 仓库中的 `resp/error_responder.go`
- `hah` 仓库中的 `resp/error_log.go`

## `ErrorResponder.Respond(...)` 的主流程

未命中提前返回时，执行顺序为：

1. `AsHTTPError(err)` 或默认错误归一化逻辑
2. `AnnotateRequestLog(r, RequestLogAttrs(err, httpErr))`
3. `logServerError(r, httpErr, err)`
4. `writeHTTPError(w, r, httpErr)`
5. `logErrorResponseWriteFailure(r, httpErr, writeErr)`
6. 返回 `writeErr`

其中：

- `hah` 的零值 responder 不会主动集成 request log
- 独立 server error log 只在 `httpErr.Status() >= 500` 时输出
- 错误响应写出失败时会额外再打一条独立 error log

## Request Log 行为

`hah.WriteError(...)` 默认不会集成任何 request log。

`chix.WriteError(...)` 的默认预设会在 `5xx` 场景下把以下字段写入当前 `httplog` request log：

- `error.code`
- `error.timeout`
- `error.canceled`

如果你希望 access log 带上 `traceId`、`request.id`，需要在服务自己的
`chi + httplog` 链路里显式挂 `middleware.RequestLogAttrs()`。

## 独立错误日志行为

当 `WriteError(...)` 最终收敛为 `5xx` 时，会通过 responder 的 `Logger` 输出一条独立 `error` 日志：

- 消息：`resp: request failed with server error`

字段至少包括：

- `http.response.status_code`
- `error.code`
- `error.message`
- `error.type`
- `error.root_message`
- `error.root_type`
- `http.request.method`
- `url.path`

`hah.WriteError(...)` 默认不会补 `traceId`；`chix.WriteError(...)` 的默认预设会额外补：

- `traceId`

如果错误响应自身写出失败，还会输出一条：

- 消息：`resp: failed to write error response`

并在可解出 `ErrorWriteDegraded` 时追加：

- `resp.error_degraded`
- `resp.public_response_preserved`
