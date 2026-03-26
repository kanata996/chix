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

当前版本仍然是起步阶段，后续还会继续补强：

- 更完整的 schema 生成能力
- 更完整的校验规则到 OpenAPI 的映射
- 响应头与多响应描述
- 安全方案与 OpenAPI 自定义能力
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
- `path`、`query`、`header` 参数描述
- JSON 请求体 schema
- JSON 成功响应 schema
- 默认的 `application/problem+json` 错误响应

目前 `/docs` 页面通过 CDN 加载 Swagger UI 资源。

## 技术文档

更详细的内部设计说明见 [docs/TECHNICAL_GUIDE.md](docs/TECHNICAL_GUIDE.md)。
