# chix

`chix` 是一个基于 `chi` 的、强约束的 JSON API 边界内核。

它不是新的 Web 框架，也不打算替代 `chi`。它做的是把 `chi` 项目里最容易重复、最容易失控的边界动作收紧成一组稳定约定：

- path/query/header 参数读取
- JSON body 解码与 DTO 校验
- 请求错误建模
- 业务错误映射
- 成功/错误响应写回

仓库与模块路径：

- `github.com/kanata996/chix`

安装：

```bash
go get github.com/kanata996/chix
```

## 设计边界

做什么：

- handler 只做解请求、调 service、写响应
- path/query/header 和 body 分流，不做大而全 binding
- 请求错误与业务错误分流，不混用
- 边界自故障 fail-closed

不做什么：

- 不引入自己的 `Context` / `Engine` / `App`
- 不做 path/query/header/body 混合自动绑定
- 不做 ORM、配置、鉴权、OpenAPI 生成
- 不把 `chi` 包成另一个大而全框架

## 包结构

- `chix`：推荐入口，暴露 path/query/header facade
- `reqx`：JSON body 解码、DTO 校验、请求错误建模
- `errx`：业务/系统错误语义、feature mapper、Mapping 校验
- `resp`：统一成功/请求错误/业务错误响应写回

内部实现：

- `internal/paramx`：`chi` 下的 path/query/header 参数读取与基础解析，由根包 `chix` 对外转发

## 最小调用链

请求参数/请求体错误：

```text
handler -> chix/reqx -> resp.Problem
```

业务/系统错误：

```text
service/repo -> errx -> resp.Error
```

最小示例见 [`examples/basic/main.go`](./examples/basic/main.go)。

## 快速示例

```go
package main

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kanata996/chix"
	"github.com/kanata996/chix/errx"
	"github.com/kanata996/chix/reqx"
	"github.com/kanata996/chix/resp"
)

type createItemRequest struct {
	Name string `json:"name" validate:"required"`
}

var ErrItemNotFound = errors.New("item not found")

var itemMapper = errx.NewMapper(500101,
	errx.Map(ErrItemNotFound, errx.AsNotFound(404101, "item not found")),
)

func main() {
	r := chi.NewRouter()
	r.Get("/items/{uuid}", getItem)
	r.Post("/items", createItem)

	_ = http.ListenAndServe(":8080", r)
}

func getItem(w http.ResponseWriter, r *http.Request) {
	itemUUID, err := chix.Path(r).UUID("uuid")
	if err != nil {
		resp.Problem(w, r, err)
		return
	}

	if itemUUID == "00000000-0000-0000-0000-000000000000" {
		resp.Error(w, r, ErrItemNotFound, itemMapper)
		return
	}

	resp.Success(w, map[string]any{"uuid": itemUUID})
}

func createItem(w http.ResponseWriter, r *http.Request) {
	var body createItemRequest
	if err := reqx.DecodeValidateJSON(w, r, &body); err != nil {
		resp.Problem(w, r, err)
		return
	}

	resp.Created(w, map[string]any{"name": body.Name})
}
```

## Public API Contract

对人和 AI Agent，都按下面的规则理解这个库的公开面：

- 只依赖本 README 中列出的 `chix`、`reqx`、`errx`、`resp` 公开 API
- 不要导入或依赖 `internal/*`
- `internal/*` 的实现和结构可以随时调整，不承诺兼容
- 如果某个导出符号没有出现在本 README 的 API 总览中，则不视为稳定公开契约
- 预发布阶段允许小幅调整，但会优先保持本 README 中已列出的 API 和语义稳定

## API 总览

下面按公开包列出所有对外 API。

### `chix`

职责：

- 读取 path 参数
- 读取 query 参数
- 读取 header 参数

入口：

```go
type PathReader = paramx.PathReader
type QueryReader = paramx.QueryReader
type HeaderReader = paramx.HeaderReader

func Path(r *http.Request) PathReader
func Query(r *http.Request) QueryReader
func Header(r *http.Request) HeaderReader
```

`PathReader` 方法：

```go
func (PathReader) String(name string) (string, error)
func (PathReader) UUID(name string) (string, error)
func (PathReader) Int(name string) (int, error)
```

`QueryReader` 方法：

```go
func (QueryReader) String(name string) (string, bool, error)
func (QueryReader) Strings(name string) ([]string, bool, error)
func (QueryReader) RequiredString(name string) (string, error)
func (QueryReader) RequiredStrings(name string) ([]string, error)
func (QueryReader) Int(name string) (int, bool, error)
func (QueryReader) RequiredInt(name string) (int, error)
func (QueryReader) Int16(name string) (int16, bool, error)
func (QueryReader) RequiredInt16(name string) (int16, error)
func (QueryReader) UUID(name string) (string, bool, error)
func (QueryReader) UUIDs(name string) ([]string, bool, error)
func (QueryReader) RequiredUUID(name string) (string, error)
func (QueryReader) RequiredUUIDs(name string) ([]string, error)
func (QueryReader) Bool(name string) (bool, bool, error)
func (QueryReader) RequiredBool(name string) (bool, error)
```

`HeaderReader` 方法：

```go
func (HeaderReader) String(name string) (string, bool, error)
func (HeaderReader) Strings(name string) ([]string, bool, error)
func (HeaderReader) RequiredString(name string) (string, error)
func (HeaderReader) RequiredStrings(name string) ([]string, error)
func (HeaderReader) Int(name string) (int, bool, error)
func (HeaderReader) RequiredInt(name string) (int, error)
func (HeaderReader) UUID(name string) (string, bool, error)
func (HeaderReader) RequiredUUID(name string) (string, error)
func (HeaderReader) Bool(name string) (bool, bool, error)
func (HeaderReader) RequiredBool(name string) (bool, error)
```

稳定语义：

- `Path(r).Xxx(...)` 默认是 required 语义，缺失或空值直接返回请求错误
- `Query(r).Xxx(...)` 默认是 optional 语义，缺失时返回 `ok=false`
- `Header(r).Xxx(...)` 默认是 optional 语义，缺失时返回 `ok=false`
- optional query/header 参数如果“出现但无效”，仍然返回请求错误，不会偷偷按缺失处理
- query/header 的标量参数重复出现时，统一返回 `multiple_values`
- query 列表使用重复 key 形式读取，例如 `?tag=a&tag=b`
- header 列表使用重复字段形式读取，不做 CSV 自动拆分
- `bool` 只接受小写 `true` / `false`
- `UUID` 解析成功后统一规范化为 canonical 字符串

### `reqx`

职责：

- 解 JSON body
- 校验 body/query/path DTO
- 统一建模请求侧错误

JSON 解码：

```go
const DefaultJSONMaxBytes int64 = 1 << 20

type DecodeOptions struct {
	MaxBytes             int64
	AllowUnknownFields   bool
	SkipContentTypeCheck bool
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, target any) error
func DecodeJSONWith(w http.ResponseWriter, r *http.Request, target any, options DecodeOptions) error
func DecodeValidateJSON(w http.ResponseWriter, r *http.Request, target any) error
```

校验：

```go
type Normalizer interface {
	Normalize()
}

func ValidateBody(target any) error
func ValidateQuery(target any) error
func ValidatePath(target any) error
```

请求错误模型：

```go
const (
	InBody   = "body"
	InHeader = "header"
	InPath   = "path"
	InQuery  = "query"
)

const (
	DetailCodeRequired             = "required"
	DetailCodeMalformedJSON        = "malformed_json"
	DetailCodeInvalidType          = "invalid_type"
	DetailCodeUnknownField         = "unknown_field"
	DetailCodeTrailingData         = "trailing_data"
	DetailCodeMultipleValues       = "multiple_values"
	DetailCodeUnsupportedMediaType = "unsupported_media_type"
	DetailCodePayloadTooLarge      = "payload_too_large"
	DetailCodeInvalidUUID          = "invalid_uuid"
	DetailCodeInvalidInteger       = "invalid_integer"
	DetailCodeOutOfRange           = "out_of_range"
	DetailCodeInvalidValue         = "invalid_value"
)

type Detail struct {
	In    string
	Field string
	Code  string
}

type Problem struct {
	StatusCode int
	Details    []Detail
}

func (*Problem) Error() string
func AsProblem(err error) (*Problem, bool)
func BadRequest(details ...Detail) *Problem
func ValidationFailed(details ...Detail) *Problem
func UnsupportedMediaType(details ...Detail) *Problem
func PayloadTooLarge(details ...Detail) *Problem
```

`Detail` helper：

```go
func Required(in string, field string) Detail
func InvalidType(in string, field string) Detail
func UnknownField(field string) Detail
func MalformedJSON() Detail
func TrailingData() Detail
func MultipleValues(in string, field string) Detail
func InvalidUUID(in string, field string) Detail
func InvalidInteger(in string, field string) Detail
func OutOfRange(in string, field string) Detail
func InvalidValue(in string, field string) Detail
```

编程错误：

```go
var (
	ErrNilResponseWriter   error
	ErrNilRequest          error
	ErrNilTarget           error
	ErrInvalidDecodeTarget error
	ErrInvalidValidateTarget error
)
```

稳定语义：

- `DecodeJSON` 默认检查 `Content-Type`
- 默认 body 大小上限是 `1 MiB`
- 默认拒绝 unknown field
- 拒绝 trailing data
- 空 body 和 top-level `null` body 统一按缺失 payload 处理
- `DecodeValidateJSON` 只是 `DecodeJSON + ValidateBody` 的组合 helper
- `ValidateBody` 只接受非 nil `*struct`，校验失败返回 `422`
- `ValidateQuery` / `ValidatePath` 只接受非 nil `*struct`，校验失败返回 `400`
- DTO 若实现 `Normalizer`，会先执行 `Normalize()` 再校验

### `errx`

职责：

- 定义通用业务/系统错误语义
- 提供 feature 级错误到 HTTP 响应语义的映射
- 对 `Mapping` 和 mapper 配置做构造期校验

标准 code：

```go
const (
	CodeInvalidRequest       int64 = 400000
	CodeUnauthorized         int64 = 400001
	CodeForbidden            int64 = 400003
	CodeNotFound             int64 = 400004
	CodeConflict             int64 = 400009
	CodePayloadTooLarge      int64 = 400013
	CodeUnsupportedMediaType int64 = 400015
	CodeUnprocessableEntity  int64 = 400022
	CodeTooManyRequests      int64 = 400029
	CodeClientClosed         int64 = 499000
	CodeInternal             int64 = 500000
	CodeServiceUnavailable   int64 = 500003
	CodeTimeout              int64 = 500004
)
```

标准语义错误：

```go
var (
	ErrInvalidRequest      error
	ErrUnauthorized        error
	ErrForbidden           error
	ErrNotFound            error
	ErrConflict            error
	ErrUnprocessableEntity error
	ErrTooManyRequests     error
	ErrServiceUnavailable  error
	ErrTimeout             error
)
```

映射模型：

```go
type Mapping struct {
	StatusCode int
	Code       int64
	Message    string
}

func (Mapping) Validate() error

func Lookup(err error) (Mapping, bool)
func Internal(code int64) Mapping
func FormatChain(err error) string
```

feature rule：

```go
type Rule struct { /* opaque */ }
type Mapper func(error) Mapping

func (Mapper) Map(err error) Mapping
func Map(match error, mapping Mapping) Rule
func NewMapper(fallbackCode int64, rules ...Rule) Mapper
```

状态预设 constructor：

```go
func AsUnauthorized(code int64, message string) Mapping
func AsForbidden(code int64, message string) Mapping
func AsNotFound(code int64, message string) Mapping
func AsConflict(code int64, message string) Mapping
func AsUnprocessable(code int64, message string) Mapping
func AsTooManyRequests(code int64, message string) Mapping
func AsServiceUnavailable(code int64, message string) Mapping
func AsTimeout(code int64, message string) Mapping
```

稳定语义：

- `Lookup(err)` 只处理内建标准语义和 transport 生命周期错误
- `context.Canceled` 会映射到 `499 / CodeClientClosed`
- `context.DeadlineExceeded` 会映射到 `504 / CodeTimeout`
- `Map(match, mapping)` 会在构造期校验 `mapping`
- `NewMapper(fallbackCode, rules...)` 的顺序是 `rules -> Lookup -> fallback`
- `fallbackCode` 会通过 `Internal(code)` 构造，非法 code 会直接 panic
- 预设 `AsXxx(...)` 固定 HTTP status，业务方提供自定义 `code` 和 `message`

推荐模式：

```go
var (
	ErrItemNotFound = errors.New("item not found")
	ErrTagExists    = errors.New("tag exists")
)

var mapper = errx.NewMapper(500101,
	errx.Map(ErrItemNotFound, errx.AsNotFound(404101, "item not found")),
	errx.Map(ErrTagExists, errx.AsConflict(409201, "tag already exists")),
)
```

### `resp`

职责：

- 写统一成功响应
- 写请求错误响应
- 写业务/系统错误响应

公开 API：

```go
func Success(w http.ResponseWriter, data any)
func Created(w http.ResponseWriter, data any)
func NoContent(w http.ResponseWriter)

func Problem(w http.ResponseWriter, r *http.Request, err error)
func Error(w http.ResponseWriter, r *http.Request, err error, mapper errx.Mapper)
```

稳定语义：

- `Success` 写 `200`
- `Created` 写 `201`
- `NoContent` 写 `204`
- `Success` / `Created` 要求 `data` 顶层编码后不能是 `null`
- success 边界自身编码失败时，fail-closed 为裸 `500`
- `Problem` 只接受 `reqx.Problem`
- `Problem` 收到非 `reqx.Problem` 或非法请求状态码时，会回退到 internal 包络
- `Error` 有 `mapper` 时优先走 feature 规则；未命中时再回落到 `errx` 内建语义或 fallback
- `Error` 收到非法 `Mapping` 时，会回退到 internal 包络并记录原因

## 选型建议

什么时候直接用标准 `errx`：

- 只需要通用的 `401/403/404/409/422/429/503/504`
- 不需要 feature 级业务 code

什么时候用 `errx.NewMapper(...)`：

- 同一个 HTTP status 下需要区分多个业务 code
- 想给终端用户返回更明确的业务 message
- feature 需要自己的 fallback internal code

什么时候只用 `chix + reqx + resp.Problem`：

- 业务层还没有稳定错误模型
- 当前接口只涉及请求边界，不涉及业务错误分层

## 本地质量检查

```bash
make ci
```

## 社区与治理

- 贡献指南：[`CONTRIBUTING.md`](./CONTRIBUTING.md)
- 行为准则：[`CODE_OF_CONDUCT.md`](./CODE_OF_CONDUCT.md)
- 安全策略：[`SECURITY.md`](./SECURITY.md)
- 许可证：[`LICENSE`](./LICENSE)
