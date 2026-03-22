# 错误处理

## 1. 核心原则

`chix` 的错误处理只围绕三类错误展开：

- `Request Error`
- `Domain Error`
- `Internal Error`

目标不是建立复杂理论体系，而是把错误稳定映射成统一 HTTP 响应。

## 2. 三类错误

### 2.1 Request Error

定义：

- 请求在进入业务语义前就失败

典型场景：

- 非法 JSON
- 缺失必填字段
- 字段类型或格式错误
- 不支持的 `Content-Type`
- 路由未命中
- method 不允许
- 认证或准入在边界层被拒绝
- rate limit 拒绝

常见状态码：

- `400`
- `401`
- `403`
- `404`
- `405`
- `406`
- `413`
- `415`
- `422`
- `429`

### 2.2 Domain Error

定义：

- 请求已被业务层接受，但业务规则或领域不变量不成立

典型场景：

- 资源不存在
- 业务动作不允许
- 唯一性冲突或版本冲突
- 规则校验失败

常见状态码：

- `403`
- `404`
- `409`
- `422`

### 2.3 Internal Error

定义：

- 边界层或依赖协作失稳，无法继续安全维持原结果

典型场景：

- 编码或写回前内部失败
- 未预期 panic
- 上游依赖超时或返回非法结果

常见状态码：

- `500`
- `502`
- `503`
- `504`

## 3. 分层约束

- 请求错误和业务错误不能混用
- 业务层不直接构造 HTTP 响应
- 统一由边界层把 `error` 映射为 HTTP 结果
- 如果分类信息不足，优先保守回退为内部错误

## 4. 统一写回

项目应有单一错误写回出口，例如：

```go
type Handler func(w http.ResponseWriter, r *http.Request) error

func Wrap(h Handler) http.HandlerFunc
func WriteError(w http.ResponseWriter, r *http.Request, err error)
```

要求：

- handler 可以返回 `error`
- 路由级 `404/405` 也应复用同一错误写回机制
- auth、rate limit 等 middleware 最终也应走同一错误出口
- panic recovery 不应由 `Wrap` 隐式接管；如需统一 panic 输出，应显式使用 `middleware.Recoverer(...)`

## 5. Fail-Closed

在响应尚未开始写回前，如果边界层失稳：

- 应优先写出安全的内部错误响应
- 不应泄露 panic、堆栈、SQL 或依赖原始报文

如果响应已经开始写回：

- 不再改写公开结果
- 只做 best-effort 日志和观测

对于 `Wrap` 注入或其他可报告写回状态的 writer，这个判断可以精确成立。
如果边界层拿到的只是裸 `http.ResponseWriter`，则只能 best-effort 地遵守这条约束。

## 6. Service 边界

推荐保持以下边界：

- handler 负责读取请求、调用 service、写回响应
- service 负责业务语义，不直接写 HTTP 响应
- `Domain Error` 可由单点 mapper 转成统一的 HTTP 错误

`Request Helper`、`Command Builder`、`Domain Error Mapper` 可以作为局部实现技巧存在，但不再是全局协议术语或强制阶段。
