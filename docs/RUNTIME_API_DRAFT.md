# Runtime API

本文定义 `chix` 当前的公开 runtime API 形状。
它与 [TECHNICAL_GUIDE.md](TECHNICAL_GUIDE.md) 保持一致，聚焦类型、挂载方式和公开契约。

## 1. 目标

这套公开 API 必须满足：

- 在 `chi` 项目中自然可用
- 保持 runtime 小而有明确意见
- 让 success、failure、observation 落到同一条执行路径
- 把 router 和 ingress concern 继续留给 `chi`

## 2. 核心类型

```go
package chix

type Runtime struct {
	// unexported
}

type Operation[I any, O any] struct {
	Method        string
	SuccessStatus int
	ErrorMappers  []ErrorMapper
}

type Handler[I any, O any] func(context.Context, *I) (*O, error)
```

`Operation` 应保持收敛，只包含 runtime 行为和排障真正需要的字段。
operation-level error mappers 应直接挂在 `Operation` 上，而不是重新回到
middleware 风格。

## 3. 构造与作用域

```go
func New(opts ...Option) *Runtime
func (rt *Runtime) Scope(opts ...Option) *Runtime
```

`Scope` 用来表达 route group 级策略，而不是把 runtime API 做成 middleware 风格。

语义上：

- `Scope(opts...)` 产生 parent config 的逻辑子作用域
- error mappers 在 runtime / scope / operation 三层之间叠加组合
- observer、extractor、success status default 这类单值配置采用最近一层覆盖

## 4. 挂到 Chi

```go
func Handle[I any, O any](rt *Runtime, op Operation[I, O], h Handler[I, O]) http.Handler
```

示例：

```go
r := chi.NewRouter()
r.Use(middleware.RequestID)
r.Use(middleware.RealIP)
r.Use(middleware.Recoverer)

rt := chix.New(
	chix.WithObserver(chix.DefaultLogger(slog.Default())),
)

users := rt.Scope(
	chix.WithErrorMapper(mapUserError),
)

r.Method("GET", "/users/{id}", chix.Handle(users, GetUser, HandleGetUser))
```

## 5. 输入绑定

runtime 应把请求绑定到 typed input struct。

```go
type GetUserInput struct {
	ID string `path:"id"`
}
```

公开 API 直接支持的输入来源：

- path
- query
- json body
- 基于 `validate` tag 的内置 validator/v10 校验

绑定规则应固定为：

- path / query 绑定必须显式声明来源标签
- body 字段使用标准 `json` tag
- 未声明来源的导出字段不应隐式进入 body
- 同一个字段最多只能声明一个输入来源
- 无效 input schema 属于配置错误，应在 `Handle(...)` 挂载时尽早暴露
- path / query 缺失值只保留 Go 零值，required 语义交给 validation
- query 绑定到标量字段时，如果同名参数出现多次，应视为 `400 bad_request`
- body 非空且 `Content-Type` 不是 `application/json` 或 `application/*+json` 时，应返回 `415 unsupported_media_type`
- malformed JSON、JSON 类型不匹配、unknown field 都应视为 `400 bad_request`

生命周期应固定为：

1. 先绑定输入
2. 绑定成功后再校验
3. 校验成功后才调用业务 handler

绑定失败应直接返回标准化请求错误；校验失败应直接返回标准化
`422 invalid_request`。这类 runtime 自己产出的公开错误不再重新进入
`ErrorMapper` 链。

最小校验用法：

```go
type CreateUserInput struct {
	ID   string `path:"id" validate:"min=4"`
	Name string `json:"name" validate:"required"`
}
```

runtime 在 bind 成功后自动执行 validator/v10。`422 invalid_request` 的
`details` 仍然输出 `source`、`field`、`code`、`message`，但这些 detail
只作为 wire contract，不作为根包公开类型暴露。

## 6. 成功输出

默认成功结果就是 handler 返回的 typed output。

```go
type User struct {
	ID string `json:"id"`
}
```

runtime 写出的 JSON：

```json
{
  "id": "u_1"
}
```

当前公开 API 只承诺标准 success body 和 success status。metadata 与
自定义 success headers 可以作为后续扩展，但不应成为当前公开 API 的前提。

success 语义也应固定为：

- `204 No Content` 是唯一允许省略 success body 的成功响应
- 非 `204` 成功响应即使 handler 返回 `nil`，也应写成 `null`
- success status 优先级固定为：`Operation.SuccessStatus` > 最近一层 scope/runtime 默认值 > runtime 内建默认值
- runtime 内建默认成功状态码固定为：`POST -> 201`，其余方法 -> `200`

## 7. 错误映射

```go
type HTTPError struct {
	Status  int
	Code    string
	Message string
	Details []any
}

func (e *HTTPError) Error() string

type ErrorMapper func(error) *HTTPError
```

`*HTTPError` 是唯一的公开边界错误载体。runtime 应通过
`errors.As(err, &httpErr)` 识别它，因此 handler 或 mapper 返回的
`*HTTPError` 即使被包装，只要能被识别出来，也不应再重新进入 mapper 链。

组合规则固定为：

- operation-level mapper 优先于 scope-level mapper
- 更内层 scope mapper 优先于更外层 scope mapper
- scope-level mapper 优先于 runtime-level mapper
- 同一层内按声明顺序执行
- 第一个返回非 nil 的 mapper 生效
- 已经是公开边界错误的值不再进入 mapper 链
- 没有命中时走 internal error fallback

runtime 自己直接产出的公开错误也应固定为最小集合：

- `400 bad_request`：path / query / body 形状错误
- `415 unsupported_media_type`：body 存在但媒体类型不受支持
- `422 invalid_request`：validation 失败，`details` 为 violation 数组

## 8. 观测

```go
type Event struct {
	Error      error
	Public     *HTTPError
	RequestID  string
	Method     string
	Target     string
	RemoteAddr string
}

type Observer func(Event)
```

observer 接收的是边界失败事件。触发时机应由 runtime 统一拥有，并固定为：

- 在已经解析出公开错误、但尚未尝试写错误响应时发出失败事件
- 如果错误响应写回本身失败，再补发一个 internal-error 事件

## 9. 请求上下文提取

```go
type Extractor func(*http.Request) RequestContext

type RequestContext struct {
	RequestID  string
	Method     string
	Target     string
	RemoteAddr string
}
```

对于 `chi`-first runtime，默认 extractor 应直接集成 `chi` request context。
同时保留可配置能力，让应用可以覆盖 runtime 的观测上下文提取逻辑。

## 10. Option 集

```go
type Option interface {
	apply(*config)
}

func WithErrorMapper(m ErrorMapper) Option
func WithObserver(o Observer) Option
func WithExtractor(e Extractor) Option
func WithSuccessStatus(status int) Option
```

option 集应保持克制。其中：

- `WithErrorMapper` 向当前 runtime/scope 层追加 mapper
- `WithObserver`、`WithExtractor`、`WithSuccessStatus` 在当前层覆盖更外层默认值

## 11. 明确不做的事

当前公开 API 不试图解决：

- OpenAPI generation
- SDK generation
- multipart/form workflow
- streaming 或 websocket endpoint
- 替代 framework-level middleware system
- 庞大的 response builder API
