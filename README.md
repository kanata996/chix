# chix

[![Go Reference](https://pkg.go.dev/badge/github.com/kanata996/chix.svg)](https://pkg.go.dev/github.com/kanata996/chix)
[![CI](https://github.com/kanata996/chix/workflows/CI/badge.svg)](https://github.com/kanata996/chix/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/github/kanata996/chix/graph/badge.svg)](https://codecov.io/github/kanata996/chix)

`chix` 是一个面向 `chi` 的 JSON API HTTP 边界工具包。

当前版本聚焦两类能力：

- 根包 `github.com/kanata996/chix` 提供常用的请求绑定、校验和响应写回入口
- `github.com/kanata996/chix/middleware` 提供与 `chi + httplog + traceid` 配套的 request log / error log 封装

它建立在 [`hah`](https://github.com/kanata996/hah) 提供的 `net/http` 核心能力之上；如果你需要更细粒度的底层能力，可以直接使用 `hah` 及其子包。

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

## 根包 API

根包提供最常用的请求/响应入口：

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
- `RequireBody`
- `WriteError`
- `JSON`
- `JSONBlob`
- `OK`
- `Created`
- `NoContent`
- `Binder`
- `DefaultBinder`
- `BindUnmarshaler`
- `RequestValidator`
- `Normalizer`
- `ErrorResponder`
- `NewErrorResponder`

这些 API 组成了 `chix` 当前对外的主路径。

`Bind(...)` 的顺序仍然是：

1. path
2. query（仅 `GET` / `DELETE` / `HEAD`）
3. body

body 绑定当前只支持 `application/json`，实际读取到零字节 body 时默认视为 no-op。更完整说明见 [docs/request-binding.md](./docs/request-binding.md)。

## 错误与日志

`chix.WriteError(...)` 使用面向 `chi + httplog + traceid` 的预配置 `ErrorResponder`：

- 5xx 时给当前 request log 补 `error.*` 诊断字段
- 5xx 时输出独立 error log
- 独立 error log 会补 `traceId`

如果你需要 access log 同时带上 `traceId`、`request.id`，应显式挂载 `middleware.RequestLogAttrs()`。

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

- `chix`：大多数 `chi` handler 直接用这个
- `hah`：想要纯 `net/http` 的根包边界能力
- `hah/errx`：想显式构造和传递公共 HTTP 错误
- `middleware`：想给 request log 增加关联字段
