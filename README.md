# chix

[![Go Reference](https://pkg.go.dev/badge/github.com/kanata996/chix.svg)](https://pkg.go.dev/github.com/kanata996/chix)
[![CI](https://github.com/kanata996/chix/workflows/CI/badge.svg)](https://github.com/kanata996/chix/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/github/kanata996/chix/graph/badge.svg)](https://codecov.io/github/kanata996/chix)

`chix` 是一个面向 `chi` 的 JSON API 配套包。

它只做两件事：

- 根包 `github.com/kanata996/chix` 提供面向 `chi + httplog + traceid` 的错误响应预设
- `github.com/kanata996/chix/middleware` 提供与 `chi + httplog + traceid` 配套的 request log 关联字段中间件

`chix` 不负责请求绑定、输入校验、成功响应和共享错误模型；这些能力由 [`hah`](https://github.com/kanata996/hah) 提供。

## 安装

要求：

- Go `1.25` 或更高版本

```bash
go get github.com/kanata996/chix@latest
```

## 快速开始

如果你想直接看一份可运行的 `chi + chix` 示例，见 [`_example`](./_example)。

```go
package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kanata996/chix"
	"github.com/kanata996/hah"
)

type createAccountRequest struct {
	OrgID string `param:"org_id"`
	Name  string `json:"name" validate:"required"`
}

func main() {
	r := chi.NewRouter()

	r.Post("/orgs/{org_id}/accounts", func(w http.ResponseWriter, r *http.Request) {
		var req createAccountRequest
		if err := hah.BindAndValidate(r, &req); err != nil {
			_ = chix.WriteError(w, r, err)
			return
		}

		_ = hah.Created(w, map[string]any{
			"id":     "acct_123",
			"org_id": req.OrgID,
			"name":   req.Name,
		})
	})

	_ = http.ListenAndServe(":8080", r)
}
```

上面是最小使用方式：

- `hah.BindAndValidate(...)` 负责请求绑定和输入校验
- `chix.WriteError(...)` 负责统一错误响应
- `hah.Created(...)` / `hah.OK(...)` 等负责成功响应

如果你不想定义 DTO，只想在 `chi` handler 里读取少量 path / query 参数，优先直接使用 `hah.Path(...)` / `hah.Query(...)` 做单参数读取和常见校验：

```go
r.Get("/orgs/{org_id}/accounts", func(w http.ResponseWriter, r *http.Request) {
	orgID, err := hah.Path(r, "org_id").String().Required().Get()
	if err != nil {
		_ = chix.WriteError(w, r, err)
		return
	}

	limit, err := hah.Query(r, "limit").Int().Default(20).Min(1).Max(100).Get()
	if err != nil {
		_ = chix.WriteError(w, r, err)
		return
	}

	_ = hah.OK(w, map[string]any{
		"org_id": orgID,
		"limit":  limit,
	})
})
```

`hah.Path(...)` 面向 path segment 里的资源标识，建议只用于 `String()`、`UUID()`、`Int()`、`Int64()`、`Uint()`、`Uint64()` 这类窄类型。更宽的参数语义放到 `hah.Query(...)`：除了常见标量，还支持 `Bool()`、`Float64()`、`Duration()`、`Time()`、`UnixTime()`、`UnixMilliTime()`。

如果 query 参数允许重复 key，`hah.Query(...).String()` / `Int()` / `UUID()` 等标量 helper 默认只消费第一个值；要直接读取全部原始值，请使用 `hah.Query(...).Values()` / `Strings()`。如果你需要结构化多值解码，仍然优先使用 `hah.BindQueryParams(...)` 等 `Bind*` 入口。

如果你只想绑定单一来源，也可以直接使用根包 facade：`hah.BindQueryParams(...)` / `hah.BindPathValues(...)` / `hah.BindHeaders(...)` / `hah.BindBody(...)`；常见场景不需要再单独导入 `hah/bind`。

如果你还需要 access log 里的 `traceId` / `request.id` 关联字段，推荐把 `chi` 链路配置成：

1. `chi/middleware.RequestID`
2. `traceid.Middleware`
3. `httplog.RequestLogger(...)`
4. `middleware.RequestLogAttrs()`
5. handler 中使用 `chix.WriteError(...)`

## 公开 API

根包 `github.com/kanata996/chix` 当前公开两类能力：

- `WriteError`
- `ErrorResponder`
- `NewErrorResponder`

子包 `github.com/kanata996/chix/middleware` 当前公开：

- `RequestLogAttrs`

请求绑定、输入校验和共享错误模型请直接使用 `hah`：

- `hah.Path`
- `hah.Query`
- `hah.Bind`
- `hah.BindBody`
- `hah.BindQueryParams`
- `hah.BindPathValues`
- `hah.BindHeaders`
- `hah.BindAndValidate`
- `hah.RequireBody`
- `hah/errx`

重复 query key 的原始值读取可直接使用 `hah.Query(...).Values()` / `hah.Query(...).Strings()`。

## 错误与日志

`chix.WriteError(...)` 使用面向 `chi + httplog + traceid` 的预配置 `ErrorResponder`：

- 在存在当前 `httplog` request logger 时，5xx 会给 request log 补 `error.*` 诊断字段
- 5xx 时输出独立 error log
- 请求 context 里存在 `traceId` 时，独立 error log 会补 `traceId`

`middleware.RequestLogAttrs()` 只负责把关联字段补到当前 `httplog` request log；它不会替你创建 logger，也不负责独立 error log。

如果你需要 access log 同时带上 `traceId`、`request.id`，应显式挂载 `middleware.RequestLogAttrs()`，并确保前面已经挂了 `RequestID`、`traceid.Middleware`、`httplog.RequestLogger(...)`。

更完整说明见 [docs/error-logging.md](./docs/error-logging.md)。

## 共享错误模型

如果你需要在 handler、service、repository 之间共享公共 HTTP 错误，直接使用 `hah/errx`：

```go
import "github.com/kanata996/hah/errx"
```

常用构造：

- `errx.NewHTTPError(...)`
- `errx.NewHTTPErrorWithCause(...)`
- `errx.BadRequest(...)`
- `errx.Unauthorized(...)`
- `errx.Forbidden(...)`
- `errx.NotFound(...)`
- `errx.MethodNotAllowed(...)`
- `errx.Conflict(...)`
- `errx.UnprocessableEntity(...)`
- `errx.TooManyRequests(...)`

## 什么时候用哪个包

- `chix`：想要 `chi` 场景下的错误响应预设
- `middleware`：想把 `traceId`、`request.id` 这类关联字段补进当前 `httplog` request log
- `hah`：处理单参数读取（`Path` / `Query`；重复 query key 可用 `Values()` / `Strings()`）、DTO 绑定（`Bind` / `BindBody` / `BindQueryParams` / `BindPathValues` / `BindHeaders` / `BindAndValidate`）、成功响应和纯 `net/http` 错误模型
- `hah/errx`：想显式构造并跨层传递公共 HTTP 错误
