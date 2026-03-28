# chix

[![Go Reference](https://pkg.go.dev/badge/github.com/kanata996/chix.svg)](https://pkg.go.dev/github.com/kanata996/chix)
[![CI](https://github.com/kanata996/chix/workflows/CI/badge.svg)](https://github.com/kanata996/chix/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/github/kanata996/chix/graph/badge.svg)](https://codecov.io/github/kanata996/chix)

`chix` 是一个 `chi`-first 的 JSON API micro runtime。

它不是 router，也不是通用 web framework。产品边界很明确：

- `chi` 负责路由匹配、route grouping、middleware 和 ingress concerns
- `chix` 负责绑定输入、校验输入、调用业务 handler、映射错误、写 JSON、发出失败观测
- 业务代码负责领域逻辑，只返回成功结果或错误

当前公开叙事围绕 runtime core，而不是旧的 `App/Register/OpenAPI` 模型。

## 当前状态

当前已经稳定到 v1 runtime core 方向的部件包括：

- `Runtime`
- `Scope`
- `Handle(rt, op, h)`
- `HTTPError`
- `ErrorMapper`
- `Observer`
- `path` / `query` / JSON body 输入绑定
- `Validator` 与 `validatorx.Adapter(...)`

当前明确不做：

- router DSL
- framework-level middleware system
- OpenAPI / Swagger UI
- `App` / `Register` 风格入口
- multipart、streaming、通用 transport 扩张

## 安装

```bash
go get github.com/kanata996/chix
```

## 快速开始

```go
package main

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/kanata996/chix"
)

var errUserExists = errors.New("user already exists")

type CreateUserInput struct {
	ID      string `path:"id"`
	Verbose bool   `query:"verbose"`
	Name    string `json:"name"`
}

type CreateUserOutput struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Verbose bool   `json:"verbose"`
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	rt := chix.New(
		chix.WithObserver(chix.DefaultLogger(nil)),
		chix.WithErrorMapper(func(err error) *chix.HTTPError {
			if errors.Is(err, errUserExists) {
				return &chix.HTTPError{
					Status:  http.StatusConflict,
					Code:    "user_exists",
					Message: "user already exists",
				}
			}
			return nil
		}),
	)

	users := rt.Scope()

	r.Method(http.MethodPost, "/users/{id}", chix.Handle(users, chix.Operation[CreateUserInput, CreateUserOutput]{
		Method: http.MethodPost,
		Validate: func(_ context.Context, in *CreateUserInput) []chix.Violation {
			if in.Name == "" {
				return []chix.Violation{{
					Source:  "body",
					Field:   "name",
					Code:    "required",
					Message: "name is required",
				}}
			}
			return nil
		},
	}, func(_ context.Context, in *CreateUserInput) (*CreateUserOutput, error) {
		if in.Name == "taken" {
			return nil, errUserExists
		}

		return &CreateUserOutput{
			ID:      in.ID,
			Name:    in.Name,
			Verbose: in.Verbose,
		}, nil
	}))

	log.Fatal(http.ListenAndServe(":8080", r))
}
```

成功响应：

```json
{
  "data": {
    "id": "u_1",
    "name": "Ada",
    "verbose": true
  }
}
```

错误响应：

```json
{
  "error": {
    "code": "invalid_request",
    "message": "invalid request",
    "details": [
      {
        "source": "body",
        "field": "name",
        "code": "required",
        "message": "name is required"
      }
    ]
  }
}
```

## Runtime 边界

`chix` 只负责 API 边界上的这一小段执行路径：

1. 绑定输入
2. 执行 validation
3. 调用业务 handler
4. 写 success envelope
5. 把内部错误映射成公开 `HTTPError`
6. 写 error envelope
7. 在失败路径发出 `Observer` 事件

这意味着下面这些事仍然属于 `chi` 或外层 ingress：

- 路由
- middleware 链
- request id 生成
- recover / auth / rate limit / CORS
- tracing / access log

## 输入模型

runtime 当前只支持三种输入来源：

- `path`
- `query`
- JSON body

绑定规则保持收敛：

- `path` / `query` 字段必须显式声明来源 tag
- body 字段必须显式写 `json` tag
- 未声明来源的导出字段不会被隐式视为 body 字段
- query 标量字段重复出现会返回 `400 bad_request`
- 非空 body 且 `Content-Type` 不是 `application/json` 或 `application/*+json` 时返回 `415 unsupported_media_type`
- malformed JSON、类型不匹配、unknown field 都会返回 `400 bad_request`
- required 语义属于 validation，不属于 binding

## 校验与错误

runtime 自己直接产出的公开 4xx 错误固定为：

- `400 bad_request`
- `415 unsupported_media_type`
- `422 invalid_request`

这些 runtime 自产公开错误不会再重新进入 `ErrorMapper` 链。

业务错误则通过 `ErrorMapper` 归一化为稳定的 `HTTPError`。如果没有 mapper 命中，会回退到：

```json
{
  "error": {
    "code": "internal_error",
    "message": "internal server error",
    "details": []
  }
}
```

如果你使用 `go-playground/validator`，推荐通过 `validatorx.Adapter(...)` 接到 `Operation.Validate`：

```go
op := chix.Operation[CreateUserInput, CreateUserOutput]{
	Method:   http.MethodPost,
	Validate: validatorx.Adapter[CreateUserInput](validator.New(validator.WithRequiredStructEnabled())),
}
```

## Scope

`Scope(opts...)` 用来表达 route group 级策略，而不是把 runtime 做成 middleware 风格：

- error mappers 在 runtime / scope / operation 之间按优先级叠加
- observer、extractor、success status default 采用最近一层覆盖

这让你可以把一组 route 的错误映射和失败观测收在一起，同时保持 `chi` 的 route grouping 体验。

## 技术文档

当前 source of truth 与设计说明在这里：

- [docs/TECHNICAL_GUIDE.md](docs/TECHNICAL_GUIDE.md)
- [docs/RUNTIME_API_DRAFT.md](docs/RUNTIME_API_DRAFT.md)
- [docs/BINDING_STATUS.md](docs/BINDING_STATUS.md)
