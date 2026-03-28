# chix

[![Go Reference](https://pkg.go.dev/badge/github.com/kanata996/chix.svg)](https://pkg.go.dev/github.com/kanata996/chix)
[![CI](https://github.com/kanata996/chix/workflows/CI/badge.svg)](https://github.com/kanata996/chix/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/github/kanata996/chix/graph/badge.svg)](https://codecov.io/github/kanata996/chix)

`chix` 是一个 `chi`-first 的 JSON API micro runtime。

它不是 router，也不是通用 web framework。产品边界很明确：

- `chi` 负责路由匹配、route grouping、middleware 和 ingress concerns
- `chix` 负责绑定输入、校验输入、调用业务 handler、映射错误、写 JSON、发出失败观测
- 业务代码负责领域逻辑，只返回成功结果或错误

当前公开叙事围绕 runtime，而不是旧的 `App/Register/OpenAPI` 模型。

## 当前状态

当前已经稳定到 v1 runtime 方向的部件包括：

- `Runtime`
- `Scope`
- `Handle(rt, op, h)`
- `HTTPError`
- `ErrorMapper`
- `Observer`
- `path` / `query` / JSON body 输入绑定
- 内置 `validator/v10` 输入校验

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
	Name    string `json:"name" validate:"required"`
}

type CreateUserOutput struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Verbose bool   `json:"verbose"`
}

func mapUserError(err error) *chix.HTTPError {
	if errors.Is(err, errUserExists) {
		return &chix.HTTPError{
			Status:  http.StatusConflict,
			Code:    "user_exists",
			Message: "user already exists",
		}
	}
	return nil
}

func createUser(_ context.Context, in *CreateUserInput) (*CreateUserOutput, error) {
	if in.Name == "taken" {
		return nil, errUserExists
	}

	return &CreateUserOutput{
		ID:      in.ID,
		Name:    in.Name,
		Verbose: in.Verbose,
	}, nil
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	rt := chix.New(
		chix.WithObserver(chix.DefaultLogger(nil)),
		chix.WithErrorMapper(mapUserError),
	)

	createUserOp := chix.Operation[CreateUserInput, CreateUserOutput]{
		Method: http.MethodPost,
	}

	r.Method(http.MethodPost, "/users/{id}", chix.Handle(rt, createUserOp, createUser))

	log.Fatal(http.ListenAndServe(":8080", r))
}
```

运行：

```bash
go run .
```

请求：

```bash
curl -i \
  -X POST 'http://localhost:8080/users/u_1?verbose=true' \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada"}'
```

成功响应。`POST` 在未显式指定状态码时默认返回 `201 Created`：

```json
{
  "id": "u_1",
  "name": "Ada",
  "verbose": true
}
```

错误请求：

```bash
curl -i \
  -X POST 'http://localhost:8080/users/u_1' \
  -H 'Content-Type: application/json' \
  -d '{"name":"taken"}'
```

错误响应：

```json
{
  "error": {
    "code": "user_exists",
    "message": "user already exists",
    "details": []
  }
}
```

## 怎么用 chix

`chix` 的典型接入顺序很固定：

1. 用 `chix.New(...)` 建一个 runtime，放全局默认策略
2. 需要 route group 策略时，用 `rt.Scope(...)` 派生子作用域
3. 定义输入/输出 struct，并显式标注 `path` / `query` / `json` 来源
4. 用 `chix.Handle(scopeOrRuntime, op, handler)` 挂到 `chi`，业务 handler 只返回输出或错误

如果一个接口的处理流程可以表述成“绑定输入 -> 校验 -> 执行业务 -> 写标准 JSON 响应”，那它就是 `chix` 的目标场景。

## 1. 定义输入模型

输入 struct 只支持三种来源：

- `path`
- `query`
- JSON body

推荐把不同来源拆成小块，再组合成最终 input：

```go
type RouteParams struct {
	ID string `path:"id" validate:"min=3"`
}

type QueryParams struct {
	Verbose bool     `query:"verbose"`
	Tags    []string `query:"tag"`
}

type Body struct {
	Name string `json:"name" validate:"required"`
}

type CreateUserInput struct {
	RouteParams
	QueryParams
	Body
}
```

这里的规则比较硬：

- `path` / `query` 字段必须显式写来源 tag
- body 字段必须显式写 `json` tag
- 未声明来源的导出字段不会被隐式当成 body 字段
- required 语义属于 `validate`，不属于 binding
- query 绑定到标量字段时，如果同名参数出现多次，会直接返回 `400 bad_request`

## 2. 挂载 handler

`chix` 不让业务代码直接写 `http.ResponseWriter`。你把类型化 handler 交给 runtime，runtime 负责成功响应、错误响应和失败观测。

```go
r.Method(http.MethodGet, "/users/{id}", chix.Handle(rt, chix.Operation[GetUserInput, UserOutput]{
	Method: http.MethodGet,
}, func(ctx context.Context, in *GetUserInput) (*UserOutput, error) {
	return svc.GetUser(ctx, in.ID)
}))
```

`Handle` 的行为有几个关键点：

- 挂载时就会解析输入 schema；schema 非法会直接 panic，而不是拖到请求期
- 成功状态码默认是 `POST -> 201`，其他方法 `-> 200`
- `Operation.SuccessStatus` 可以覆盖默认值
- `204 No Content` 是唯一会跳过 success body 的成功响应
- 非 `204` 成功响应即使 handler 返回 `nil`，也会写成 `null`

删除接口通常这样写：

```go
r.Method(http.MethodDelete, "/users/{id}", chix.Handle(rt, chix.Operation[DeleteUserInput, struct{}]{
	Method:        http.MethodDelete,
	SuccessStatus: http.StatusNoContent,
}, func(ctx context.Context, in *DeleteUserInput) (*struct{}, error) {
	return nil, svc.DeleteUser(ctx, in.ID)
}))
```

## 3. 错误映射

业务代码返回 domain error，`chix` 负责把它映射成稳定的公开 HTTP 错误：

```go
func mapUserError(err error) *chix.HTTPError {
	if errors.Is(err, errUserExists) {
		return &chix.HTTPError{
			Status:  http.StatusConflict,
			Code:    "user_exists",
			Message: "user already exists",
		}
	}
	return nil
}
```

错误解析顺序固定为：

- `handler` 或 `mapper` 已经返回 `*chix.HTTPError` 时，直接写回，不再继续走 mapper 链
- `Operation.ErrorMappers`
- 当前 scope 到外层 scope 的 mapper
- runtime-level mapper
- 都没命中时回退到 `500 internal_error`

runtime 自己产出的公开 4xx 错误也不会再进入 mapper 链：

- `400 bad_request`
- `415 unsupported_media_type`
- `422 invalid_request`

## 4. 用 Scope 表达 route group 策略

`Scope(opts...)` 不是 middleware 替身，它表达的是一组路由共享的 runtime 策略。

```go
rt := chix.New(
	chix.WithObserver(chix.DefaultLogger(nil)),
)

users := rt.Scope(
	chix.WithErrorMapper(mapUserError),
)

admin := rt.Scope(
	chix.WithErrorMapper(mapAdminError),
	chix.WithSuccessStatus(http.StatusAccepted),
)
```

继承规则也很简单：

- `ErrorMapper` 在 `runtime / scope / operation` 之间按内层优先追加
- `Observer`、`Extractor`、`SuccessStatus` 采用最近一层覆盖
- 适合放 route group 级错误映射、失败观测和默认成功状态码

配合 `chi` 的 route grouping，常见写法就是：

```go
r.Route("/users", func(r chi.Router) {
	r.Method(http.MethodPost, "/{id}", chix.Handle(users, createUserOp, createUser))
	r.Method(http.MethodDelete, "/{id}", chix.Handle(users, deleteUserOp, deleteUser))
})

r.Route("/admin", func(r chi.Router) {
	r.Method(http.MethodPost, "/rebuild-index", chix.Handle(admin, rebuildIndexOp, rebuildIndex))
})
```

## 输入、响应和默认行为

输入绑定规则：

- 非空 body 且 `Content-Type` 不是 `application/json` 或 `application/*+json` 时返回 `415 unsupported_media_type`
- malformed JSON、JSON 类型不匹配、unknown field 都返回 `400 bad_request`
- bind 成功后才会执行 `validator/v10`
- validation 失败固定返回 `422 invalid_request`

响应 wire contract：

- 成功响应直接写业务 body
- 错误响应固定为 `{"error": {...}}`
- 错误 `details` 在 wire 上始终是数组
- 没有 mapper 命中的业务错误会回退到固定的内部错误：

```json
{
  "error": {
    "code": "internal_error",
    "message": "internal server error",
    "details": []
  }
}
```

## Runtime 边界

`chix` 只负责 API 边界上的这一小段执行路径：

1. 绑定输入
2. 执行 validation
3. 调用业务 handler
4. 写 success body
5. 把内部错误映射成公开 `HTTPError`
6. 写 error envelope
7. 在失败路径发出 `Observer` 事件

这意味着下面这些事仍然继续留在 `chi` 或更外层 ingress：

- 路由
- middleware 链
- request id 生成
- recover / auth / rate limit / CORS
- tracing / access log

runtime 内置 `go-playground/validator/v10`，会在 bind 成功后自动根据 `validate` tag 执行校验：

```go
type CreateUserInput struct {
	ID   string `path:"id" validate:"min=4"`
	Name string `json:"name" validate:"required"`
}
```

## 技术文档

当前 source of truth 与设计说明在这里：

- [docs/TECHNICAL_GUIDE.md](docs/TECHNICAL_GUIDE.md)
- [docs/RUNTIME_API_DRAFT.md](docs/RUNTIME_API_DRAFT.md)
- [docs/BINDING_STATUS.md](docs/BINDING_STATUS.md)
