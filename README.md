# chix

[![Go Reference](https://pkg.go.dev/badge/github.com/kanata996/chix.svg)](https://pkg.go.dev/github.com/kanata996/chix)
[![CI](https://github.com/kanata996/chix/workflows/CI/badge.svg)](https://github.com/kanata996/chix/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/github/kanata996/chix/graph/badge.svg)](https://codecov.io/github/kanata996/chix)

`chix` 是一个基于 `chi` 和 `net/http` 的轻量级 JSON API HTTP 边界工具包。

它聚焦在服务的请求与响应边界：

- 将 path/query/header/JSON body 绑定到 DTO
- 使用 `validator/v10` 对 DTO 进行校验
- 写出 JSON 成功响应
- 写出一致的结构化错误响应

当前仓库对外主要暴露三个包：

- `github.com/kanata996/chix`：常用能力的根包 facade
- `github.com/kanata996/chix/reqx`：请求侧绑定与校验辅助
- `github.com/kanata996/chix/resp`：响应侧 JSON 与错误写回辅助

## 状态

`chix` 当前仍处于 `v1.0.0` 之前阶段。库已经可用，但在 `v1.0.0` 之前，小版本发布中仍可能出现破坏性变更。

公开 API 以及 README 中描述的 HTTP 行为变更，应在 [CHANGELOG.md](./CHANGELOG.md) 中明确记录。

## 安装

要求：

- Go `1.25` 或更高版本

```bash
go get github.com/kanata996/chix@latest
```

## 快速开始

如果你想直接看一份可运行的 `chi + chix` 推荐接入示例，见 [`_example`](./_example)。

下面的示例展示了 `chix` 最常见的 handler 形态：

- `chi` 负责路由
- `chix.BindAndValidate(...)` 负责请求输入边界
- `chix.Created(...)` 负责成功 JSON 响应
- `chix.WriteError(...)` 负责统一错误响应

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

启动服务后，可以直接验证成功和失败两条路径。

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

- `param:"..."`：匹配到的请求 path 命名 wildcard（通过 `http.Request.PathValue(...)` 读取；以 `http.Request.Pattern` 中的命名 wildcard 为准，例如 `/{id}`、`/{path...}`）
- `query:"..."`：查询字符串参数
- `json:"..."`：JSON 请求体
- `header:"..."`：请求头
- `validate:"..."`：`validator/v10` 校验规则

`Bind(...)` 遵循 Echo 风格的绑定顺序：

1. path 参数
2. `GET` 和 `DELETE` 请求上的 query 参数
3. JSON body

这个顺序意味着：

- 后绑定的数据会覆盖先绑定的数据，例如 body 会覆盖同名 path/query 字段
- `POST`、`PUT`、`PATCH` 等请求不会在 `Bind(...)` 中自动绑定 query；如果你需要它们，显式调用 `BindQueryParams(...)` 或 `BindAndValidateQuery(...)`
- 缺失的 path/query/header/body 不会把目标 DTO 清零，而是保留目标对象已有值
- 任一绑定阶段失败时，不会对目标 DTO 部分落值，调用方拿到的仍是原对象

path 绑定的兼容边界：

- `reqx` 只依赖 `net/http` 暴露的 `PathValue` / `Pattern` 语义
- `param:"..."` 只面向命名 wildcard；它不是 `chi.RouteContext` 的兼容层
- `chi` 专有的 `*` catch-all 不属于 `reqx` 的公开 path 契约

根包中常用的请求侧 API：

- `Bind`、`BindBody`、`BindQueryParams`、`BindPathValues`、`BindHeaders`
- `BindAndValidate`、`BindAndValidateBody`、`BindAndValidateQuery`、`BindAndValidatePath`、`BindAndValidateHeaders`
- `ParamString`、`ParamInt`、`ParamUUID`
- `WithMaxBodyBytes`

默认请求行为：

- 未知的 query 和 header 字段默认忽略
- 未知 JSON 字段默认忽略
- JSON body 使用 Go 标准库解码
- 非空 body 需要 `application/json` 或 `application/*+json` 类型的 `Content-Type`
- `Bind(...)` 在空 body 下会把 body 阶段视为 no-op，并忽略空 body 场景下的无效 `Content-Type`
- `BindBody(...)` 和 `BindAndValidateBody(...)` 在空 body 下也会保留已有值；但如果请求显式声明了非 JSON `Content-Type`，仍会返回 `415 Unsupported Media Type`
- 默认 body 读取上限为 `1 MiB`；可通过 `WithMaxBodyBytes(...)` 覆盖

如果你只想绑定单一来源，优先使用源专用 API：

- 只处理 JSON body：`BindBody(...)`、`BindAndValidateBody(...)`
- 只处理 query：`BindQueryParams(...)`、`BindAndValidateQuery(...)`
- 只处理 path：`BindPathValues(...)`、`BindAndValidatePath(...)`
- 只处理 header：`BindHeaders(...)`、`BindAndValidateHeaders(...)`

`BindAndValidate*` 会在 `validator/v10` 校验前自动执行 DTO 的 `Normalize()`。如果你的 DTO 实现了根包导出的 `Normalizer` 接口，可以在其中做裁剪、大小写归一化或默认值补齐。

```go
type listAccountsRequest struct {
	Cursor string `query:"cursor" validate:"omitempty,nospace"`
}

func (r *listAccountsRequest) Normalize() {
	r.Cursor = strings.TrimSpace(r.Cursor)
}
```

校验错误中的字段名会优先使用请求侧 tag 名，而不是 Go struct 字段名。例如 `json:"name"` 失败时，返回的错误项会是 `field: "name", in: "body"`。

## 响应

成功响应辅助：

- `OK`
- `Created`
- `NoContent`
- `JSON`、`JSONPretty`、`JSONBlob`

`JSON(...)`、`OK(...)` 和 `Created(...)` 会在请求 URL 带有 `?pretty` 时自动输出 pretty JSON。

`WriteError(...)` 会将任意错误归一化为稳定的公开错误响应结构：

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

当前公开错误模型采用 problem 风格字段命名，但不返回 `type` 和 `instance`。字段约定如下：

- 成功 JSON 响应使用 `application/json`
- 错误 JSON 响应使用 `application/problem+json`
- 顶层固定字段为 `title`、`status`、`detail`、`code`
- `title` 始终由 HTTP status text 生成，例如 `422 -> "Unprocessable Entity"`
- `detail` 承载对外公开的人类可读说明
- `code` 承载稳定的机器错误码，便于客户端分支处理
- `errors` 仅在存在结构化字段错误时出现
- `errors[]` 子项固定为 `field`、`in`、`code`、`detail`
- `in` 表示错误来源，当前可能为 `body`、`query`、`path`、`header`、`request`

常见归一化规则：

- `reqx` 产生的绑定/校验错误默认返回 `422 Unprocessable Entity`，错误码为 `invalid_request`
- 非法 JSON 返回 `400 Bad Request`，错误码为 `invalid_json`
- 非 JSON `Content-Type` 返回 `415 Unsupported Media Type`，错误码为 `unsupported_media_type`
- 超过 body 上限返回 `413 Request Entity Too Large`，错误码为 `request_too_large`
- `context.Canceled` 返回 `499 Client Closed Request`，错误码为 `client_closed_request`
- `context.DeadlineExceeded` 返回 `504 Gateway Timeout`，错误码为 `timeout`
- 未知错误默认返回 `500 Internal Server Error`，错误码为 `internal_error`
- `HEAD` 错误响应只写状态码和 `application/problem+json`，不写响应体
- `title` 始终由状态码生成，`detail` 承载公开说明，`code` 承载稳定机器码
- `errors` 仅在存在结构化字段错误时出现；单个错误项使用 `field`、`in`、`code`、`detail`

如果你需要可复用的公共错误值，可以直接使用 `resp.HTTPError`，以及 `resp.BadRequest(...)`、`resp.NotFound(...)`、`resp.UnprocessableEntity(...)` 等辅助构造函数。

例如：

```go
if err := repo.DeleteAccount(ctx, accountID); err != nil {
	if errors.Is(err, sql.ErrNoRows) {
		_ = chix.WriteError(w, r, resp.NotFound("account_not_found", "account not found"))
		return
	}

	_ = chix.WriteError(w, r, err)
	return
}
```

## 错误日志

`chix` 不再统一接管 access log。请求日志配置建议由每个服务直接使用
`chi + httplog` 自己决定，例如：

- 是否挂 `RequestID` / `traceid.Middleware`
- 是否记录 request / response headers
- 是否额外挂 `middleware.RequestLogAttrs()` 给所有 access log 追加 `traceId` / `request.id`
- 是否跳过 `/healthz` 等低价值路由
- 成功请求是否还需要额外业务日志

`resp.WriteError(...)` 负责的是错误语义，而不是整套日志策略。

当错误最终收敛为 `5xx` 时，`WriteError(...)` 会做两件事：

- 如果当前请求经过了 `httplog.RequestLogger(...)`，通过 `httplog.SetAttrs(...)` 给 request log 补少量错误字段和关联字段：`error.code`、`error.timeout`、`error.canceled`，以及上下文里已有的 `traceId` / `request.id`
- 通过 `slog.Default()` 输出一条独立的 error 日志，便于在 access log 之外排查问题

默认示例不挂 `middleware.RequestLogAttrs()`；它是一个可选辅助中间件，适合你希望所有
access log 都带上 `traceId` / `request.id` 的场景。

当错误响应自身写出失败时，还会再输出一条：

- `resp: failed to write error response`

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
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(traceid.Middleware)
	r.Use(httplog.RequestLogger(logger, &httplog.Options{
		Level:         slog.LevelInfo,
		Schema:        httplog.SchemaECS,
		RecoverPanics: true,
	}))

	_ = http.ListenAndServe(":8080", r)
}
```

完整的错误日志行为说明见 [docs/error-logging.md](./docs/error-logging.md)。

## 包结构

- `chix`：面向大多数 handler 的常用 facade
- `reqx`：完整的请求侧 API
- `resp`：完整的响应侧 API

如果你只需要常用能力，优先使用根包；如果你需要完整能力面，再直接导入 `reqx` 或 `resp`。
