# chi example

这个目录用于放置 `chix` 与 `chi` 的集成示例。

目标：

- 展示 `Wrap` 在 `chi.Router` 中的推荐接法
- 展示 `Write` / `WriteMeta` / `WriteEmpty` 三种成功响应
- 展示 `DecodeJSON` / `DecodeAndValidateJSON` / `DecodeQuery` / `DecodeAndValidateQuery` / `Validate`
- 展示 `WithErrorMapper` / `WithErrorMappers` / `ChainMappers` 如何组合业务错误映射
- 展示 `RequestError` / `DomainError` / `InternalError` 三类公开错误
- 展示 `NotFound` / `MethodNotAllowed` / auth middleware / `Recoverer` 如何统一走 `WriteError`

约束：

- 这里只放示例，不放核心实现
- 这里是独立 Go module，有自己的 `go.mod`
- 示例应尽量短小，可直接运行或作为 smoke example 使用
- 示例风格优先贴近真实 `chi` 项目，而不是演示型伪代码

主要路由覆盖：

- `GET /users`：`DecodeAndValidateQuery` + `WriteMeta`
- `GET /users/{userID}`：`Write` + `WithErrorMapper` + `ChainMappers`
- `POST /users`：`DecodeAndValidateJSON` + `WithMaxBodyBytes` + `WithErrorMappers`
- `PATCH /users/{userID}/profile`：`DecodeJSON` + `AllowUnknownFields` + `Validate`
- `POST /sessions/refresh`：`DecodeJSON` + `AllowEmptyBody`
- `GET /reports/export`：`DecodeQuery` + `AllowUnknownQueryFields` + direct `RequestError`
- `POST /invites/{code}/accept`：mapped `DomainError` + `WriteEmpty`
- `POST /users/{userID}/suspend`：direct `DomainError`
- `GET /internal/upstream`：direct `InternalError`
- `/partial` / `/panic`：已开始响应保护 + `chix/middleware.Recoverer`

示例代码入口：

- `main.go`
- `smoke_test.go`

常用命令：

- `go test ./...`
- `go run .`
