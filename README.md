# chix

[![Go Reference](https://pkg.go.dev/badge/github.com/kanata996/chix.svg)](https://pkg.go.dev/github.com/kanata996/chix)
[![CI](https://github.com/kanata996/chix/workflows/CI/badge.svg)](https://github.com/kanata996/chix/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/github/kanata996/chix/graph/badge.svg)](https://codecov.io/github/kanata996/chix)

`chix` 是一个面向 `chi` 的轻量级 JSON API HTTP 边界工具包。

它主要解决四件事：

- 将 path/query/header/JSON body 绑定到 DTO
- 使用 `validator/v10` 校验 DTO
- 写出 JSON 成功响应
- 写出统一的结构化错误响应

当前仓库对外主要暴露五个包：

- `github.com/kanata996/chix`：大多数 handler 直接使用的根包入口
- `github.com/kanata996/chix/reqx`：请求侧绑定、校验与 path 参数辅助
- `github.com/kanata996/chix/resp`：响应写回与错误响应输出
- `github.com/kanata996/chix/errx`：共享公共 HTTP 错误模型
- `github.com/kanata996/chix/middleware`：可选的 request log 辅助中间件

## 状态

`chix` 仍处于 `v1.0.0` 之前阶段。在 `v1.0.0` 之前，小版本发布中仍可能出现破坏性变更。

公开 API 和对外 HTTP 行为变更会记录在 [CHANGELOG.md](./CHANGELOG.md)。

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
)

type createAccountRequest struct {
	OrgID string `param:"org_id"`
	Name  string `json:"name" validate:"required"`
}

func main() {
	r := chi.NewRouter()

	r.Post("/orgs/{org_id}/accounts", func(w http.ResponseWriter, r *http.Request) {
		var req createAccountRequest
		if err := chix.BindAndValidate(r, &req); err != nil {
			_ = chix.WriteError(w, r, err)
			return
		}

		_ = chix.Created(w, r, map[string]any{
			"id":     "acct_123",
			"org_id": req.OrgID,
			"name":   req.Name,
		})
	})

	_ = http.ListenAndServe(":8080", r)
}
```

成功请求：

```bash
curl -i \
  -X POST http://localhost:8080/orgs/org_123/accounts \
  -H 'Content-Type: application/json' \
  -d '{"name":"Acme"}'
```

预期响应：

```http
HTTP/1.1 201 Created
Content-Type: application/json

{"id":"acct_123","org_id":"org_123","name":"Acme"}
```

校验失败请求：

```bash
curl -i \
  -X POST http://localhost:8080/orgs/org_123/accounts \
  -H 'Content-Type: application/json' \
  -d '{}'
```

预期响应：

```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/problem+json

{"title":"Unprocessable Entity","status":422,"detail":"request contains invalid fields","code":"invalid_request","errors":[{"field":"name","in":"body","code":"required","detail":"is required"}]}
```

## 请求绑定与校验

支持的 tag：

- `param:"..."`：path 参数
- `query:"..."`：query 参数
- `json:"..."`：JSON body
- `header:"..."`：header
- `validate:"..."`：`validator/v10` 校验规则

请求侧 JSON body 接受 `application/json` 和 `application/*+json`。

根包常用请求 API：

- `Bind`
- `BindBody`
- `BindQueryParams`
- `BindPathValues`
- `BindHeaders`
- `BindAndValidate`
- `BindAndValidateBody`
- `BindAndValidateQuery`
- `BindAndValidatePath`
- `BindAndValidateHeaders`
- `ParamString`
- `ParamInt`
- `ParamUUID`
- `WithMaxBodyBytes`

`Bind(...)` 的绑定顺序是：

1. path
2. query（仅 `GET` / `DELETE`）
3. JSON body

如果你只处理单一输入来源，优先使用源专用 API：

- 只处理 JSON body：`BindBody(...)`、`BindAndValidateBody(...)`
- 只处理 query：`BindQueryParams(...)`、`BindAndValidateQuery(...)`
- 只处理 path：`BindPathValues(...)`、`BindAndValidatePath(...)`
- 只处理 header：`BindHeaders(...)`、`BindAndValidateHeaders(...)`

如果 DTO 需要在校验前做裁剪、大小写归一化或默认值补齐，实现 `Normalize()` 即可：

```go
type listAccountsRequest struct {
	Cursor string `query:"cursor" validate:"omitempty,nospace"`
}

func (r *listAccountsRequest) Normalize() {
	r.Cursor = strings.TrimSpace(r.Cursor)
}
```

默认 body 读取上限是 `chix.DefaultMaxBodyBytes`（当前为 `1 MiB`）。
如果需要限制 body 大小，可以传入 `WithMaxBodyBytes(...)`，例如把单个请求体收紧到 `256 KiB`：

```go
var req createAccountRequest
if err := chix.BindAndValidate(r, &req, chix.WithMaxBodyBytes(256<<10)); err != nil {
	_ = chix.WriteError(w, r, err)
	return
}
```

## 响应

根包常用响应 API：

- `OK`
- `Created`
- `NoContent`
- `JSON`
- `JSONPretty`
- `JSONBlob`
- `WriteError`

`WriteError(...)` 会把错误写成统一的 problem 风格 JSON：

```json
{
  "title": "Unprocessable Entity",
  "status": 422,
  "detail": "request contains invalid fields",
  "code": "invalid_request",
  "errors": [
    {
      "field": "name",
      "in": "body",
      "code": "required",
      "detail": "is required"
    }
  ]
}
```

成功响应使用 `application/json`，错误响应使用 `application/problem+json`。

## 共享错误模型

如果你需要在 handler、service、repository 之间共享公共 HTTP 错误，直接使用 `errx`：

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

例如：

```go
if err := repo.DeleteAccount(ctx, accountID); err != nil {
	if errors.Is(err, sql.ErrNoRows) {
		_ = chix.WriteError(w, r, errx.NotFound("account_not_found", "account not found"))
		return
	}

	_ = chix.WriteError(w, r, err)
	return
}
```

## 可选中间件

`middleware.RequestLogAttrs()` 用于把请求上下文中的关联字段补到当前 request log。
`WriteError(...)` 只会在 5xx access log 上补 `error.*` 诊断字段，不再隐式注入
`traceId` / `request.id`。

建议挂载顺序：

1. `chimw.RequestID`
2. `traceid.Middleware`
3. `httplog.RequestLogger(...)`
4. `middleware.RequestLogAttrs()`

示例：

```go
package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/go-chi/traceid"
	"github.com/kanata996/chix/middleware"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(traceid.Middleware)
	r.Use(httplog.RequestLogger(logger, &httplog.Options{
		Level:         slog.LevelInfo,
		Schema:        httplog.SchemaECS,
		RecoverPanics: true,
	}))
	r.Use(middleware.RequestLogAttrs())

	_ = http.ListenAndServe(":8080", r)
}
```

## 什么时候用哪个包

- `chix`：大多数 handler 直接用这个
- `reqx`：只想用请求绑定和校验
- `resp`：只想用响应写回
- `errx`：想显式构造和传递公共 HTTP 错误
- `middleware`：想给 request log 增加关联字段
