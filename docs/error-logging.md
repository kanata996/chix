# 错误日志处理方案

本文描述 `resp.WriteError(...)` 相关的错误日志处理设计，重点覆盖：

- 普通业务错误为什么不额外打印一条独立错误日志
- 请求日志里会补充哪些错误诊断字段
- 错误链、`details` 和基础设施异常是如何处理的

相关源码：

- `resp/write_error.go`
- `resp/error_log.go`
- `middleware/log.go`

## 目标

这套方案的目标不是“遇到错误就打一条 `error` 日志”，而是把错误语义和诊断信息挂到当前请求日志上，让外层请求日志中间件统一输出。

这样做有几个直接好处：

- 避免同一个请求在 4xx / 5xx 时产生重复日志
- 日志和请求上下文天然绑定，包含 `request.id`、`http.route` 等字段
- 对外错误响应和对内排障信息可以解耦
- 日志内容可以围绕排障优化，而不是简单镜像响应 JSON

## 非目标

这套方案有意不做下面这些事：

- 不为普通 4xx / 5xx 再额外打印一条业务错误日志
- 不把原始错误对象直接透传给客户端
- 不让日志字段完全等同于对外错误响应结构
- 不在中间件层重复实现一套请求日志输出器

## 调用链路

`WriteError(...)` 的主要流程如下：

1. 将任意 `error` 收敛为 `HTTPError`
2. 调用 `annotateRequestErrorLog(...)` 为当前 request log 补充错误字段
3. 调用 `writeHTTPError(...)` 写回统一错误响应
4. 如果“错误响应自身写出失败”，调用 `logErrorResponseWriteFailure(...)` 单独记一条基础设施错误日志

可以把职责理解为两层：

- `write_error.go` 负责“对客户端返回什么”
- `error_log.go` 负责“日志里保留哪些诊断信息”

## 为什么普通错误不单独打一条日志

这里故意不把普通业务错误作为独立 `error` 日志输出，因为请求日志本身已经是更合适的承载点。

如果再额外打一条日志，通常会带来这些问题：

- 同一个失败请求出现两条高度相似的日志
- 请求级字段分散，检索和聚合变差
- 4xx 这类预期内失败也会被放大成噪音
- 上层告警规则更难区分“业务失败”和“基础设施异常”

因此，普通错误的策略是：

- 将错误诊断字段挂到当前请求日志
- 由外层 `httplog` 在请求结束时统一输出

## 请求日志字段

`requestErrorLogAttrs(...)` 会生成一组以排障为中心的字段。

### HTTP 语义

- `error.code`
  最终对外错误码，来自 `HTTPError.Code()`
- `error.category`
  按状态码粗分为 `client` 或 `server`
- `error.public_message`
  返回给客户端的公共错误文案
- `error.expected`
  状态码小于 `500` 时为 `true`

### 内部诊断

- `error.message`
  用于排障的错误消息，优先来自原始 `cause`
- `error.type`
  当前诊断错误的 Go 运行时类型名
- `error.root_message`
  错误链最底层的消息
- `error.root_type`
  错误链最底层的类型
- `error.wrapped`
  是否存在包装链
- `error.chain`
  展开后的错误消息链
- `error.chain_types`
  展开后的错误类型链

### 特殊标记

- `error.canceled`
  原始错误链命中 `context.Canceled`
- `error.timeout`
  原始错误链命中 `context.DeadlineExceeded`

### details 相关字段

- `error.details_count`
  `HTTPError.Details()` 的条目数
- `error.details`
  仅在可安全落日志时才写入
- `error.details_dropped`
  `details` 存在，但因不可编码或超限而被丢弃

### 基础请求字段

如果当前请求上下文里还没有基础字段，代码会补充：

- `request.id`
- `http.route`

这能保证不是所有调用路径都必须严格经过组合好的日志中间件，日志仍有基本可检索性。

## 对外响应与日志字段的分离

这里刻意区分了两套信息：

- 对外响应：`status`、`code`、`message`、`details`
- 对内日志：错误链、错误类型、根因、特殊标记等

尤其要注意：

- `error.public_message` 对应客户端能看到的文案
- `error.message` 对应更适合排障的内部消息

例如内部真实错误可能是 `db timeout`，但对外仍然返回 `Internal Server Error`。这种分离可以避免泄露内部实现细节，同时不牺牲诊断能力。

## 错误链处理

错误链处理的核心规则是“诊断优先从原始 `cause` 开始”。

如果 `HTTPError` 已经包住底层错误，`errorForDiagnostics(...)` 会优先使用 `httpErr.cause`，而不是把 `*HTTPError` 自身当成链头。这样做能避免 HTTP 包装层淹没真正的错误类型和消息。

`flattenErrorChain(...)` 负责展开错误链，兼容两种形式：

- `Unwrap() error`
- `Unwrap() []error`

因此它既支持标准单链包装，也支持 `errors.Join(...)` 形成的多分支错误。

为了避免日志失控，还有两个保护措施：

- 最大展开深度为 `8`
- 用 `seen` 集合防止循环引用

在展开后，`buildErrorChainInfo(...)` 会提炼出：

- 当前错误的 message / type
- 根错误的 message / type
- 整条错误消息链和类型链

## details 的日志安全策略

`details` 面向响应时允许比较灵活，但日志侧更保守。

`safeErrorLogDetails(...)` 的判断规则是：

1. 先对 `details` 做 JSON 编码
2. 编码失败则不记录
3. 编码结果超过 `4096` 字节则不记录
4. 再解码成普通 JSON 结构后，才作为 `error.details` 写入日志

这样做的目的有两个：

- 防止把不可控对象、复杂类型或大对象直接塞进日志
- 保证日志聚合系统看到的是稳定、可序列化的 JSON 结构

如果不满足条件，但 `details` 确实存在，则只会记录 `error.details_dropped=true`。

## 错误字符串限制

为了避免单条日志过大，错误文本在写入链路摘要前会做裁剪：

- 单条错误文本最大 `1024` 字节
- 超出后追加 `...(truncated)`

这只影响日志表现，不影响对外错误响应。

## 基础设施异常何时单独打日志

只有一种情况会额外输出独立的 `error` 日志：错误响应自身写出失败。

这不是普通业务失败，而是错误处理路径发生了异常。例如：

- 响应写到一半连接已关闭
- fallback 错误响应也写不出去
- `details` 无法编码，且后续写回过程继续失败

此时 `logErrorResponseWriteFailure(...)` 会输出一条日志，至少包含：

- `http.response.status_code`
- `error.code`
- `error`

如果错误属于 `ErrorWriteDegraded`，还会附带：

- `resp.error_degraded`
- `resp.public_response_preserved`

这两个字段用于区分：

- 是否发生了降级
- 虽然降级了，但客户端是否仍收到了合法的公共错误响应

## 典型场景

### 场景一：普通业务错误

例如 handler 返回一个包装过的数据库超时错误：

- `WriteError(...)` 收敛为 `HTTPError`
- 请求日志被补充错误链、根因、类型、路由、请求 ID 等字段
- 对外返回统一错误响应
- 不额外打印独立 `error` 日志

这是最常见、也是最推荐的路径。

### 场景二：客户端主动断开

如果错误链命中 `context.Canceled`：

- 对外状态会收敛为 `499`
- 请求日志会带上 `error.canceled=true`
- 仍然不会因为这类情况单独打印业务错误日志

### 场景三：超时

如果错误链命中 `context.DeadlineExceeded`：

- 对外状态会收敛为 `504`
- 请求日志会带上 `error.timeout=true`

### 场景四：`details` 不可序列化

如果 `details` 中包含函数等不可编码值：

- 响应写回会降级为只保留 `code` / `message`
- 返回 `ErrorWriteDegraded`
- 请求日志侧不会盲目记录原始 `details`
- 若响应仍成功写给客户端，`resp.public_response_preserved=true`

## 维护约束

后续如果继续扩展这条链路，建议保持这些约束不变：

- 普通 4xx / 5xx 继续只走请求日志注解，不新增重复业务错误日志
- 对外响应与对内诊断信息继续分离
- 诊断优先围绕原始 `cause`，而不是 `HTTPError` 包装层
- 日志字段必须保持结构化、可检索、可控大小
- `details` 只有在可安全编码时才写日志
- 只有基础设施级异常才升级为独立 `error` 日志

## 对应测试

当前实现至少有这些测试覆盖了核心行为：

- `TestWriteErrorEnrichesRequestLog`
- `TestWriteErrorDropsUnencodableDetails`
- `TestWriteErrorPayloadReturnsJoinedErrorWhenFallbackWriteFails`

这些测试说明：

- 请求日志字段会被实际注入
- 不可编码 `details` 会触发降级
- 响应写失败时会保留基础设施异常语义
