# Echo v5 对齐的 Binding 重构规格

本文定义 `bind/chix/reqx` 这一版分层后的 binding API 目标形态，用作重构实施和最终验收的唯一标准。

这不是当前实现说明，也不是兼容迁移指南。本文和现有 [request-binding.md](./request-binding.md) 的关系是：

- `request-binding.md` 描述当前公开契约
- 本文描述下一版 binding 的目标契约

## 1. 目标与范围

本次重构只定义 `binding` 层，不定义 `validate` 层。

必须同时满足下面三点：

- 默认 binding 语义直接对齐 Echo v5
- `binding` 和 `validate` 明确分层
- `BindAndValidate(...)` 只是组合入口，不再反向定义 binding 语义

本文明确不保留兼容性。只要和目标标准冲突，现有 API、错误模型、option、默认行为都可以移除或重命名。

## 2. 权威基线

快照时间：`2026-04-08`

Echo 基线版本：

- `github.com/labstack/echo/v5` `v5.1.0`
- 发布时间：`2026-03-31`

权威来源：

- Echo binding 文档：<https://echo.labstack.com/docs/binding>
- Echo `bind.go`：<https://github.com/labstack/echo/blob/v5.1.0/bind.go>
- Echo `binder.go`：<https://github.com/labstack/echo/blob/v5.1.0/binder.go>
- Echo `binder_generic.go`：<https://github.com/labstack/echo/blob/v5.1.0/binder_generic.go>
- Echo `bind_test.go`：<https://github.com/labstack/echo/blob/v5.1.0/bind_test.go>
- Echo `binder_test.go`：<https://github.com/labstack/echo/blob/v5.1.0/binder_test.go>

权威优先级：

1. Echo v5.1.0 源码
2. Echo v5.1.0 上游测试
3. Echo 官网文档页

原因很直接：官网文档页存在滞后项，当前至少有两处需要以源码和测试为准：

- 默认 `Bind` 的 query 阶段，官网文档页写的是 `GET/DELETE`，但 `v5.1.0` 源码实际是 `GET/DELETE/HEAD`
- 官网文档页列出 `JSON/XML/form` body 绑定，源码实际还支持 `multipart/form-data`

因此，后续所有 parity 测试必须以 Echo `v5.1.0` 源码和测试行为为准，而不是以官网文档页的简化表述为准。

## 3. 分层原则

下一版请求侧必须拆成两层：

- `binding`：只负责把 HTTP 输入映射到目标值
- `validate`：只负责 `Normalize()` 和结构校验

标准组合关系如下：

```go
func BindAndValidate(r *http.Request, dst any) error {
	if err := bind.Bind(r, dst); err != nil {
		return err
	}
	if n, ok := dst.(interface{ Normalize() }); ok {
		n.Normalize()
	}
	return validate(dst)
}
```

由此得到三个强约束：

- binding 层不得再输出 `422`
- body 是否必填，不属于默认 binding 语义
- “顶层必须是 JSON object”“空 body 必须报错”“unknown field 必须报错”这类严格策略，不得再塞进默认 binder

如果未来仍需要这些严格策略，只能作为默认 binder 之外的“严格 profile”或独立中间层存在，不能污染 Echo 对齐基线。

## 4. 宿主适配原则

Echo 基于 `*echo.Context`；`reqx/chix` 基于 `*http.Request`。本次对齐做的是“语义等价映射”，不是照搬 `Context` 类型。

等价映射如下：

- `c.Request()` 对应 `*http.Request`
- `c.QueryParams()` 对应 `r.URL.Query()`
- `c.FormValues()` / `c.MultipartForm()` 对应标准库 form 解析
- `c.Param(name)` / `c.PathValues()` 对应 `r.PathValue(name)` 和路由层提供的 path value 命名语义

只要外部可观察行为与 Echo `v5.1.0` 一致，就视为对齐；不要求内部实现结构和 Echo 完全同构。

## 5. 目标 API

### 5.1 `bind` 的规范导出面

`bind` 必须成为 binding 规范面的唯一权威包。`chix` 根包是否 re-export，只是便利层；`reqx` 只负责组合校验。

核心 binder 面：

```go
type Binder interface {
	Bind(r *http.Request, target any) error
}

type DefaultBinder struct{}

type BindUnmarshaler interface {
	UnmarshalParam(param string) error
}

func Bind(r *http.Request, target any) error
func BindPathValues(r *http.Request, target any) error
func BindQueryParams(r *http.Request, target any) error
func BindBody(r *http.Request, target any) error
func BindHeaders(r *http.Request, target any) error
```

值级 fluent binder 面：

```go
type BindingError struct {
	Field  string
	Values []string
	// 其余错误承载字段按最终错误类型设计，但必须能表达 Echo 同等信息
}

func NewBindingError(sourceParam string, values []string, message string, err error) error

type ValueBinder struct {
	ValueFunc  func(sourceParam string) string
	ValuesFunc func(sourceParam string) []string
	ErrorFunc  func(sourceParam string, values []string, message string, internalError error) error
}

func QueryParamsBinder(r *http.Request) *ValueBinder
func PathValuesBinder(r *http.Request) *ValueBinder
func FormFieldBinder(r *http.Request) *ValueBinder
```

`ValueBinder` 的方法族必须与 Echo v5 对齐，包括但不限于：

- `FailFast`
- `BindError`
- `BindErrors`
- `CustomFunc` / `MustCustomFunc`
- `BindUnmarshaler` / `MustBindUnmarshaler`
- `JSONUnmarshaler` / `MustJSONUnmarshaler`
- `TextUnmarshaler` / `MustTextUnmarshaler`
- `BindWithDelimiter` / `MustBindWithDelimiter`
- 标量与切片方法族：`String`、`Int*`、`Uint*`、`Bool`、`Float*`、`Time`、`Duration`
- Unix 时间方法族：`UnixTime`、`UnixTimeMilli`、`UnixTimeNano`
- 所有对应的 `Must<Type>` 和 `Must<Type>s`

泛型 parse/extractor 面：

```go
var ErrNonExistentKey error

type TimeLayout string
type TimeOpts struct {
	Layout          TimeLayout
	ParseInLocation *time.Location
	ToInLocation    *time.Location
}

const (
	TimeLayoutUnixTime      TimeLayout = "UnixTime"
	TimeLayoutUnixTimeMilli TimeLayout = "UnixTimeMilli"
	TimeLayoutUnixTimeNano  TimeLayout = "UnixTimeNano"
)

func PathParam[T any](r *http.Request, paramName string, opts ...any) (T, error)
func PathParamOr[T any](r *http.Request, paramName string, defaultValue T, opts ...any) (T, error)

func QueryParam[T any](r *http.Request, key string, opts ...any) (T, error)
func QueryParamOr[T any](r *http.Request, key string, defaultValue T, opts ...any) (T, error)
func QueryParams[T any](r *http.Request, key string, opts ...any) ([]T, error)
func QueryParamsOr[T any](r *http.Request, key string, defaultValue []T, opts ...any) ([]T, error)

func FormValue[T any](r *http.Request, key string, opts ...any) (T, error)
func FormValueOr[T any](r *http.Request, key string, defaultValue T, opts ...any) (T, error)
func FormValues[T any](r *http.Request, key string, opts ...any) ([]T, error)
func FormValuesOr[T any](r *http.Request, key string, defaultValue []T, opts ...any) ([]T, error)

func ParseValue[T any](value string, opts ...any) (T, error)
func ParseValueOr[T any](value string, defaultValue T, opts ...any) (T, error)
func ParseValues[T any](values []string, opts ...any) ([]T, error)
func ParseValuesOr[T any](values []string, defaultValue []T, opts ...any) ([]T, error)
```

### 5.2 `chix` / `reqx` 的定位

`chix` 根包只做薄封装即可。建议只 re-export 最常用的核心 binder 面：

- `Binder`
- `DefaultBinder`
- `Bind`
- `BindBody`
- `BindQueryParams`
- `BindPathValues`
- `BindHeaders`

`ValueBinder` 和泛型 extractor 更适合保留在 `bind`，避免根包继续膨胀。

`reqx` 只保留：

- `BindAndValidate*`
- `RequestValidator`
- `Normalizer`
- `RequireBody`
- `InvalidRequest`
- `Violation` / `CodeInvalidRequest`

### 5.3 当前 API 的处理原则

下列旧符号或旧约束不再是下一版 binding 的标准面：

- `BindOption`
- `BindBodyOption`
- `BindQueryParamsOption`
- `BindHeadersOption`
- 旧的 required-body 选项
- `WithMaxBodyBytes(...)`
- `ParamString`
- `ParamInt`
- `ParamUUID`

这些能力如果仍然需要，必须移到默认 binding 之外的独立层，不得继续定义默认 binder 的语义。

## 6. 规范行为

### 6.1 默认 `Bind` 顺序

`DefaultBinder.Bind` 必须与 Echo v5.1.0 一致：

1. `BindPathValues`
2. `BindQueryParams`，但仅在 `GET/DELETE/HEAD`
3. `BindBody`

明确约束：

- header 不参与默认 `Bind`
- 后一阶段可以覆盖前一阶段已写入的字段
- `POST/PUT/PATCH` 默认 `Bind` 不绑定 query

### 6.2 非原子绑定

下一版默认 binder 必须是 Echo 风格的“原地写入”，而不是当前 `reqx` 的 clone-then-commit。

这意味着：

- 同一阶段内，前面已经成功写入的字段，不因后续字段失败而回滚
- 前一阶段已经成功写入的字段，不因后一阶段失败而回滚

验收时必须覆盖这一点。例如：

- path 已成功写入，query 失败，path 修改必须保留
- query 已成功写入，body 失败，query 修改必须保留

### 6.3 body 语义

`BindBody` 在 MVP 阶段只要求服务 JSON API，不再追求 Echo `bind.go` 的完整 content-type 分派能力。

MVP 必须支持的 media type：

- `application/json`

必须不支持的默认 JSON 扩展 media type：

- `application/*+json`

也就是说，当前 `reqx` 的 `application/*+json` 宽松规则必须移除；是否支持 vendor JSON、XML、form、multipart，只能放到默认 binder 之外或留到后续版本。

### 6.4 空 body 语义

默认 binder 的空 body 判断必须直接对齐 Echo：

- 只要 `r.ContentLength == 0`，`BindBody` 直接返回 `nil`
- 这个分支发生在 content-type 检查之前
- 因此 `Content-Length: 0` 且 `Content-Type` 非法时，默认仍然返回成功

这里必须显式接受一个事实：Echo 的“空 body”判断不是“读完后发现没有字节”，而是“请求头上的 `ContentLength == 0`”。

因此：

- 空字符串 body 且 `ContentLength == 0`：成功 no-op
- 纯空白 body 但 `ContentLength > 0`：不是空 body，交给具体 decoder 处理
- chunked body `ContentLength == -1`：不是空 body，交给具体 decoder 处理

这也是为什么当前 `reqx` 的“trim 空白后视为空 body”和旧的 required-body 策略，都不能留在默认 binder 里。

### 6.5 JSON 语义

下一版默认 JSON 绑定必须遵循 Echo 默认 JSON serializer 的行为，不再追加 `reqx` 自己的 JSON 结构策略。

明确要求：

- binder 不得强制“顶层必须是 object”
- binder 不得额外拒绝顶层 `null`
- binder 是否接受数组，取决于目标类型和 JSON decoder
- 对切片目标，顶层 JSON 数组必须可以成功绑定

因此，当前 `reqx` 里的这些默认规则都必须移除：

- “顶层必须是 JSON object”
- “顶层 `null` 一律报错”
- “空白 body 视为空 body”

### 6.6 form 和 multipart 语义

MVP 默认 binder 不要求支持 `application/x-www-form-urlencoded` 和 `multipart/form-data`。如果未来扩展 body media type，再单独为 form / multipart 定义与 Echo 的对齐语义。

当前阶段不把这些能力列入默认 binder 的验收范围。

### 6.7 path/query/header 字段映射规则

对 `BindPathValues`、`BindQueryParams`、`BindHeaders` 这几类字符串源，必须与 Echo 当前 `bindData(...)` 语义一致。

必须满足：

- 只绑定声明了显式 tag 的字段
- 未声明 tag 的普通嵌套 struct，会继续向下递归查找有 tag 的字段
- 如果字段实现了 `BindUnmarshaler`，只有显式 tag 时才参与绑定
- 对 query/param/header，匿名 struct 字段如果自身带 tag，必须报错
- 先做大小写精确匹配，再做 case-insensitive fallback

### 6.8 map 目标支持

默认 binder 不得再把“只支持 struct”当作规则。

对 path/query/header 这类字符串源，必须对齐 Echo 当前 map 支持面：

- `map[string]string`
- `map[string][]string`
- `map[string]any`

绑定规则也要一致：

- `map[string]string` 取首值
- `map[string]any` 取首值
- `map[string][]string` 保留全部值

### 6.9 header 语义

必须保留 Echo 当前分层：

- `BindHeaders(...)` 存在
- header 不参与默认 `DefaultBinder.Bind(...)`

不得新增默认 header 阶段，也不得为了“更完整”而把 header 塞进默认 `Bind` 顺序。

### 6.10 错误语义

binding 层的错误语义必须回到 Echo 分层：

- binding 失败：`400` 或 `415`
- validate 失败：由 validate 层决定，通常是 `422`

因此，binding 层必须停止输出当前 `reqx` 这套 source-specific violation 聚合语义：

- `Violation`
- `invalid_request`
- `ViolationCodeType`
- `ViolationCodeMultiple`

这些模型如果仍然保留，只能属于 validate 或上层 error mapping，不得再作为默认 binder 的原生行为。

## 7. fluent binder 和泛型 extractor 的行为要求

### 7.1 `ValueBinder`

`ValueBinder` 必须与 Echo 当前行为一致：

- 默认 `FailFast(true)`
- `BindError()` 返回首个错误，并清空内部错误状态
- `BindErrors()` 返回全部错误，并清空内部错误状态
- `Must<Type>` 和 `Must<Type>s` 把“缺失值”和“空字符串值”统一视为 `required field value is empty`

不得新增 Echo 不存在的 fluent source，例如：

- `HeaderBinder(...)`

### 7.2 泛型 extractor

`PathParam`、`QueryParam`、`FormValue` 及其 `Or` / 复数版本必须与 Echo `binder_generic.go` 语义一致：

- key 不存在：返回 `ErrNonExistentKey`
- key 存在但值为空：返回类型零值或默认值，且不报错
- 值存在但解析失败：返回 `BindingError`

这意味着当前 `ParamString/ParamInt/ParamUUID` 这类“必填 path helper”不再是 binding 主标准，应该被泛型 extractor 取代。

## 8. 默认 binding 之外的能力边界

下列能力不再属于默认 binding 契约：

- body 必填
- body 大小限制
- `application/*+json`
- JSON 顶层 object 强约束
- unknown field 拒绝
- clone-then-commit 原子绑定
- binding 直接产出 `422`

如果产品仍然需要这些能力，必须放到下面两类扩展里：

- 严格 binding profile
- validate / middleware / request policy 层

默认 binder 必须先做到 Echo 对齐，再谈扩展。

## 9. 验收标准

### 9.1 API 面验收

必须存在并可用的导出面：

| 类别 | 必须存在 |
| --- | --- |
| 核心 | `Binder`、`DefaultBinder`、`BindPathValues`、`BindQueryParams`、`BindBody`、`BindHeaders` |
| 值级 binder | `BindingError`、`NewBindingError`、`ValueBinder`、`QueryParamsBinder`、`PathValuesBinder`、`FormFieldBinder` |
| 泛型 extractor | `ErrNonExistentKey`、`PathParam*`、`QueryParam*`、`FormValue*`、`ParseValue*` |
| 时间解析 | `TimeLayout`、`TimeOpts`、`TimeLayoutUnixTime`、`TimeLayoutUnixTimeMilli`、`TimeLayoutUnixTimeNano` |

### 9.2 默认 binder 行为验收

必须至少覆盖下面这些 parity 场景：

| 场景 | 目标结果 |
| --- | --- |
| `POST` path + query + body | query 不参与默认 `Bind` |
| `GET` path + query + body | query 参与，body 最后覆盖 |
| `DELETE` path + query + body | query 参与，body 最后覆盖 |
| `HEAD` path + query + body | query 参与，body 最后覆盖 |
| header 存在但调用默认 `Bind` | header 不参与 |
| query 成功后 body 解码失败 | query 侧已写入值保留 |
| path 成功后 query 失败 | path 侧已写入值保留 |
| `Content-Length: 0` 且 body 为空 | 成功 no-op |
| `Content-Length: 0` 且非法 `Content-Type` | 仍然成功 no-op |
| 空白 body 且 `ContentLength > 0` | 不视为空 body，交给 decoder |
| 非空 body 且无支持的 `Content-Type` | `415` |
| `application/json` + struct 目标 | 按默认 JSON decoder 绑定 |
| `application/json` + slice 目标 + 顶层数组 | 成功 |
| `application/json` + struct 目标 + 顶层数组 | `400` |
| `application/json` + struct 目标 + 顶层 `null` | 跟随默认 JSON decoder 语义，不做 binder 额外拦截 |
### 9.3 值级 binder 验收

必须至少覆盖下面这些 parity 场景：

| 场景 | 目标结果 |
| --- | --- |
| `FailFast()` 默认值 | `true` |
| `FailFast(false)` | 可以累计多个错误 |
| `BindError()` | 返回首错并重置错误状态 |
| `BindErrors()` | 返回全错并重置错误状态 |
| `MustString` / `MustInt` 等 | 缺失值和空值都报 `required field value is empty` |
| `BindWithDelimiter` | 与 Echo 同等拆分和转换语义 |
| `Time` / `Times` | 支持 layout、自定义 `TimeOpts`、Unix 时间 layout |
| `BindUnmarshaler` / `TextUnmarshaler` / `JSONUnmarshaler` | 行为与 Echo 当前测试一致 |

### 9.4 泛型 extractor 验收

必须至少覆盖下面这些 parity 场景：

| 场景 | 目标结果 |
| --- | --- |
| key 不存在 | `ErrNonExistentKey` |
| key 存在但值为空 | 返回零值或默认值，不报错 |
| key 存在但值非法 | `BindingError` |
| `ParseValue` / `ParseValues` 时间解析 | 与 `TimeLayout` / `TimeOpts` 一致 |

### 9.5 验收方式

建议直接采用“上游对照测试”：

1. 为 `bind` 写一组 adapter case，把 `*http.Request` 构造成 Echo 等价输入
2. 复用 Echo `bind_test.go` 和 `binder_test.go` 中最关键的行为样例
3. 对比下面四类结果：
   - 绑定后的目标值
   - HTTP 状态码
   - 错误字段名和原始值
   - 失败后的目标对象保留状态

如果最终保留自己的错误包裹类型，验收时可以不要求错误字符串逐字节一致，但下面这些结构化事实必须一致：

- `400/415` 分类一致
- `BindingError.Field` 一致
- `BindingError.Values` 一致
- 目标值和部分写入副作用一致

## 10. 建议实施顺序

建议按下面顺序落地：

1. 先重写默认 binder，去掉当前 `reqx` 的 JSON 严格策略和原子写入
2. 补齐 `HEAD` query 语义，并明确 JSON-only body 契约
3. 补齐 `ValueBinder` 与泛型 extractor
4. 最后把 `BindAndValidate*` 改成纯组合层

顺序不能反过来。只要默认 binder 还没和 Echo 对齐，validate 层就不应该继续定义 body 语义。
