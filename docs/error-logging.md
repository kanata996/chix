# 错误日志当前行为

本文只说明 `resp.WriteError(...)` 相关日志记录的当前行为。

相关源码：

- `resp/write_error.go`
- `resp/error_log.go`
- `middleware/log.go`

## `WriteError(...)` 的日志分支

### `err == nil`

- 直接返回 `nil`
- 不写日志

### 响应已经开始写出

如果传入错误能解出 `responseWriteError`，且 `responseStarted=true`：

- 直接返回原错误
- 不补请求日志字段
- 不写独立错误日志

### 常规错误响应路径

未命中提前返回时，执行顺序为：

1. `asHTTPError(err)`
2. `annotateRequestErrorLog(r, err, httpErr)`
3. `writeHTTPError(w, r, httpErr)`
4. `logErrorResponseWriteFailure(r, httpErr, writeErr)`
5. 返回 `writeErr`

其中第 4 步只有在 `writeErr != nil` 时才会实际输出独立错误日志。

## 请求日志行为

`annotateRequestErrorLog(...)` 只通过 `httplog.SetAttrs(...)` 给当前请求日志补字段，本身不直接输出日志。

### 4xx

当 `httpErr.Status() < 500` 时：

- 不补任何 `error.*` 字段
- 只保留外层请求日志原有字段

### 5xx

当 `httpErr.Status() >= 500` 时，会补以下字段：

- `error.code`
- `error.message`
- `error.type`
- `error.root_message`
- `error.root_type`
- `error.timeout`
- `error.canceled`

其中：

- `error.code` 始终来自 `HTTPError.Code()`
- `error.message`、`error.type` 来自诊断起点错误
- `error.root_message`、`error.root_type` 来自错误链遍历尾部摘要
- `error.timeout` 仅在 `errors.Is(err, context.DeadlineExceeded)` 时写入
- `error.canceled` 仅在 `errors.Is(err, context.Canceled)` 时写入

### 不再写入请求日志的字段

当前不会写入 request log 的字段有：

- `error.details`
- `error.details_count`
- `error.details_dropped`
- `error.chain`
- `error.chain_types`
- `error.wrapped`
- `error.public_message`
- `error.category`
- `error.expected`

## 基础请求字段

错误注解路径在需要时会补：

- `request.id`
- `http.route`

补充条件为：

- 当前上下文未被标记为已注入基础请求字段

如果请求已经经过 `middleware.RequestLogger(...)`，错误注解路径不会重复补充。

## 诊断起点与错误链

5xx 的诊断起点规则为：

- 如果 `HTTPError` 持有 `cause`，优先从 `httpErr.cause` 开始
- 否则从原始 `err` 开始

错误链展开同时兼容：

- `Unwrap() error`
- `Unwrap() []error`

限制为：

- 最大深度 `8`
- 使用 `seen` 集合避免循环引用

`error.root_message` 和 `error.root_type` 在普通单链包装场景里通常对应最底层错误；在 `errors.Join(...)` 场景里，它们表示本次遍历尾部摘要，不保证是唯一根因。

## 独立错误日志行为

`logErrorResponseWriteFailure(...)` 只在错误响应写出路径返回非空错误时输出独立 `error` 日志。

日志消息为：

- `resp: failed to write error response`

固定字段为：

- `http.response.status_code`
- `error.code`
- `error`

如果错误可解出 `ErrorWriteDegraded`，还会追加：

- `resp.error_degraded`
- `resp.public_response_preserved`

## 独立错误日志的 logger 来源

独立错误日志按以下顺序选择 logger：

1. 请求上下文里的 request logger
2. `slog.Default()`

## `details` 的当前行为

`details` 当前只属于公共错误响应，不写入 request log。
