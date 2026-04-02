# chix

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

## 请求绑定

支持的 tag：

- `param:"..."`：`chi` 路由参数
- `query:"..."`：查询字符串参数
- `json:"..."`：JSON 请求体
- `header:"..."`：请求头
- `validate:"..."`：`validator/v10` 校验规则

`Bind(...)` 遵循 Echo 风格的绑定顺序：

1. path 参数
2. `GET` 和 `DELETE` 请求上的 query 参数
3. JSON body

根包中常用的请求侧 API：

- `Bind`、`BindBody`、`BindQueryParams`、`BindPathValues`、`BindHeaders`
- `BindAndValidate`、`BindAndValidateBody`、`BindAndValidateQuery`、`BindAndValidatePath`、`BindAndValidateHeaders`
- `ParamString`、`ParamInt`、`ParamUUID`
- `WithMaxBodyBytes`

默认请求行为：

- 未知的 query 和 header 字段默认忽略
- 空 JSON body 默认视为 no-op
- JSON body 使用 Go 标准库解码
- 未知 JSON 字段默认忽略

## 响应

成功响应辅助：

- `OK`
- `Created`
- `NoContent`
- `JSON`、`JSONPretty`、`JSONBlob`

`WriteError(...)` 会将任意错误归一化为稳定的公开错误响应结构：

```json
{
  "error": {
    "code": "invalid_request",
    "message": "request contains invalid fields",
    "details": [
      {
        "field": "name",
        "code": "required",
        "message": "is required"
      }
    ]
  }
}
```

如果你需要可复用的公共错误值，可以直接使用 `resp.HTTPError`，以及 `resp.BadRequest(...)`、`resp.NotFound(...)`、`resp.UnprocessableEntity(...)` 等辅助构造函数。

## 日志中间件

`middleware` 当前提供一套组合好的请求日志链路：

- `RequestLogger(...)`：统一装配 `RequestID`、`traceid`、`httplog.RequestLogger` 与基础请求日志字段
- `NewLogger(...)`：构造适配上述中间件的 `slog.Logger`

## 包结构

- `chix`：面向大多数 handler 的常用 facade
- `reqx`：完整的请求侧 API
- `resp`：完整的响应侧 API

如果你只需要常用能力，优先使用根包；如果你需要完整能力面，再直接导入 `reqx` 或 `resp`。

## 开发

```bash
make test
make ci
```

更多信息：

- [CHANGELOG.md](./CHANGELOG.md)
- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [错误日志处理方案](./docs/error-logging.md)
- [SECURITY.md](./SECURITY.md)
- [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md)
- [LICENSE](./LICENSE)
