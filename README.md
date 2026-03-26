# chix

[![Go Reference](https://pkg.go.dev/badge/github.com/kanata996/chix.svg)](https://pkg.go.dev/github.com/kanata996/chix)
[![CI](https://github.com/kanata996/chix/workflows/CI/badge.svg)](https://github.com/kanata996/chix/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/github/kanata996/chix/graph/badge.svg)](https://codecov.io/github/kanata996/chix)

`chix` 是一个直接绑定 `chi` 的 API 微框架，目标是让你在保留 `chi` 路由与中间件生态的前提下，更快地搭建开箱即用的 JSON API 服务。

它不追求“适配任何路由器”，而是明确以 `chi` 为核心，补齐 API 服务开发里常见但重复的那一层能力：

- 声明式操作注册
- 类型化请求绑定，支持 `path`、`query`、`header` 和 JSON Body
- 基于 `validator/v10` 的声明式请求校验
- 自动生成 OpenAPI 3.1 文档
- 内建 Swagger UI 文档页
- 统一输出 `application/problem+json` 错误响应
- 提供适合 API 服务的默认 `chi` 中间件组合

整体定位更接近 Huma 这类“面向 API 的框架”，但实现上不抽象掉 `chi`，而是直接站在 `chi` 上扩展。

## 当前状态

仓库已经按新的方向完成第一版 MVP 重建，目前已经提供：

- 基于 `chi` 的 `App`
- 通过泛型注册类型化处理函数
- `reqx` 子包承接请求解码与校验
- JSON 请求与响应处理
- `/openapi.json` OpenAPI 文档输出
- `/docs` Swagger UI 页面
- 多响应、响应头与按响应内容类型的 OpenAPI 描述

当前版本仍然是起步阶段，后续还会继续补强：

- 更完整的 schema 生成能力
- 更完整的校验规则到 OpenAPI 的映射
- 更丰富的响应内容类型与安全方案
- 更丰富的 `chi` 中间件预设组合

## 安装

```bash
go get github.com/kanata996/chix
```

## 快速开始

```go
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/kanata996/chix"
)

type CreateUserInput struct {
	ID      string `path:"id" doc:"用户 ID"`
	Verbose bool   `query:"verbose"`
	Name    string `json:"name"`
}

type CreateUserOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func main() {
	app := chix.New(chix.Config{
		Title:       "Chix 示例 API",
		Version:     "0.1.0",
		Description: "一个基于 chix 与 chi 的 API 服务",
	})

	chix.Register(app, chix.Operation{
		Method:      http.MethodPost,
		Path:        "/users/{id}",
		OperationID: "createUser",
		Summary:     "创建用户",
		Tags:        []string{"users"},
	}, func(ctx context.Context, input *CreateUserInput) (*CreateUserOutput, error) {
		return &CreateUserOutput{
			ID:   input.ID,
			Name: input.Name,
		}, nil
	})

	log.Fatal(http.ListenAndServe(":8080", app))
}
```

启动后：

- `POST /users/{id}` 提供业务接口
- `GET /openapi.json` 返回自动生成的 OpenAPI 文档
- `GET /docs` 提供 Swagger UI 文档页

## 输入模型

`chix` 通过结构体标签把请求数据绑定到输入对象上：

- ``path:"id"`` 绑定 `chi` 路径参数
- ``query:"limit"`` 绑定查询参数
- ``header:"X-Trace-ID"`` 绑定请求头
- ``json:"name"`` 绑定 JSON Body 字段

例如：

```go
type ListUsersInput struct {
	AccountID string `path:"accountID"`
	Limit     int    `query:"limit"`
	TraceID   string `header:"X-Trace-ID"`
}
```

任意导出字段只要没有被标记为 `path`、`query` 或 `header`，就会被视为 JSON Body 的一部分。

字段上的 OpenAPI 文档元数据目前支持：

- ``doc:"..."`` 描述字段
- ``example:"..."`` 声明示例值
- ``deprecated:"true"`` 标记字段或参数已废弃

你也可以直接使用 `validator/v10` 的 `validate` 标签定义请求校验规则：

```go
type CreateUserInput struct {
	ID    string `path:"id" validate:"required,uuid4"`
	Role  string `query:"role" validate:"oneof=admin member"`
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=3,max=50"`
}
```

当前 `Register(...)` 的默认请求链路会先解码，再统一执行校验。

## 请求校验

请求语义分成两层：

- 解码或绑定失败返回 `400 Bad Request`
- `validate` 规则失败返回 `422 Unprocessable Entity`

校验失败时，框架会继续输出 `application/problem+json`，并在响应体中附带 `violations` 字段，包含字段来源、字段名和失败规则。

如果你需要自定义 `validator` 实例，可以在 `chix.Config.RequestDecoder` 中注入 `reqx.New(reqx.WithValidator(v))`。

## 默认行为

默认情况下，`chix` 会安装以下 `chi` 中间件：

- `middleware.RequestID`
- `middleware.RealIP`
- `middleware.Recoverer`
- `middleware.StripSlashes`

你可以通过 `chix.Config.Middlewares` 增加全局中间件，也可以通过 `app.Use(...)` 继续挂载。

## OpenAPI

每个注册的操作都会自动写入 OpenAPI 3.1 文档。

当前会生成的内容包括：

- 操作元数据，如 `operationId`、`summary`、`description`、`tags`
- `components/schemas` 与 `$ref` 复用
- `path`、`query`、`header` 参数描述
- JSON 请求体 schema
- 常见 `validate` 规则映射出的请求约束，如 `required`、`min/max`、`len`、`oneof`、`email`、`uuid`
- 多个响应码的描述，包括常见成功响应 `200` / `201` / `204`
- 显式错误响应文档，如 `404`、`409`
- 响应头文档描述
- 按响应维度控制 `content-type`，例如 `application/json`、`application/problem+json`、`text/plain`
- 字段级文档元数据，如 `doc`、`example`、`deprecated`
- 显式的请求错误响应文档，包括解码/绑定失败的 `400` 和校验失败的 `422`
- JSON 成功响应 schema
- 默认的 `application/problem+json` 错误响应

目前 `/docs` 页面通过 CDN 加载 Swagger UI 资源。

如果你需要控制 `components/schemas` 的命名，可以通过 `chix.Config.OpenAPISchemaNamer` 自定义。命名函数会收到 `reflect.Type` 和 `Request` 标记；返回空字符串时会回退到默认命名策略。

如果一个操作需要多个响应码或响应头文档，可以使用 `Operation.Responses`：

```go
type UpsertUserInput struct {
	ID string `path:"id"`
}

type UpsertUserOutput struct {
	ID      string `json:"id"`
	Created bool   `json:"created"`
}

func (o *UpsertUserOutput) ResponseStatus() int {
	if o.Created {
		return http.StatusCreated
	}
	return http.StatusOK
}

func (o *UpsertUserOutput) ResponseHeaders() http.Header {
	if !o.Created {
		return nil
	}
	headers := http.Header{}
	headers.Set("Location", "/users/"+o.ID)
	return headers
}

func (o *UpsertUserOutput) ResponseContentType() string {
	return "application/json"
}

chix.Register(app, chix.Operation{
	Method: http.MethodPut,
	Path:   "/users/{id}",
	Responses: []chix.OperationResponse{
		{Status: http.StatusOK, Description: "用户已更新", ContentType: "application/json"},
		{
			Status:      http.StatusCreated,
			Description: "用户已创建",
			ContentType: "application/json",
			OpenAPIModel: (*struct {
				ID string `json:"id"`
			})(nil),
			Headers: map[string]chix.HeaderDoc{
				"Location": {
					Description: "新资源地址",
					Schema:      &chix.Schema{Type: "string", Format: "uri"},
				},
			},
		},
		{Status: http.StatusConflict, Description: "用户名冲突"},
	},
}, func(ctx context.Context, input *UpsertUserInput) (*UpsertUserOutput, error) {
	// ...
})
```

说明：

- 未声明成功响应时，仍沿用 `SuccessStatus` / `SuccessDescription` 与默认 `200` / `201` 行为。
- 一旦在 `Responses` 里显式声明成功响应，OpenAPI 中的成功响应文档就由该列表控制。
- 如果一个操作有多个成功响应，运行时状态码仍建议通过 `SuccessStatus` 或 `ResponseStatus() int` 明确给出。
- `OperationResponse.ContentType` 用来声明该响应的文档 media type；`NoBody` 仍然表示这个响应不输出 body。
- `OperationResponse.OpenAPIModel` 可以为显式成功响应指定单独的 OpenAPI schema；它只影响文档，不改变运行时实际写出的 body。
- `ResponseStatus() int`、`ResponseHeaders() http.Header` 和 `ResponseContentType() string` 是可选运行时对齐点，用来让实际成功响应更接近文档描述。
- 错误响应如果需要附带响应头，可以在 `*HTTPError` 上调用 `WithHeader(...)` 或 `WithHeaders(...)`。
- 错误响应如果需要调整运行时 `Content-Type`，可以在 `*HTTPError` 上调用 `WithContentType(...)`；当前仍建议保持 JSON/problem JSON 语义一致。

## 技术文档

更详细的内部设计说明见 [docs/TECHNICAL_GUIDE.md](docs/TECHNICAL_GUIDE.md)。
