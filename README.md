# chix

[![Go Reference](https://pkg.go.dev/badge/github.com/kanata996/chix.svg)](https://pkg.go.dev/github.com/kanata996/chix)
[![CI](https://github.com/kanata996/chix/workflows/CI/badge.svg)](https://github.com/kanata996/chix/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/github/kanata996/chix/graph/badge.svg)](https://codecov.io/github/kanata996/chix)

`chix` 是一个 `chi-first`、同时保持 `net/http` 兼容的轻量 HTTP boundary 层，用来把 JSON API 的公开行为收敛成一套稳定约定。

它不替代 router，不引入新的 framework runtime，也不要求固定目录结构。默认推荐和 `chi` 一起使用；如果你继续使用原生 `net/http` 或其他 router，也可以把边界层统一到 `chix`。

## Why chix

- 🧱 稳定的边界错误分层：`Request Error` / `Domain Error` / `Internal Error`
- 🍃 `chi-first` 接入方式：文档与示例优先围绕 `chi.Router`
- 📦 统一 success / error envelope，避免 handler 各写各的
- 🔒 fail-closed 写回策略：响应一旦开始，就不再偷偷改写公开结果
- 🧰 内置请求解码与校验 facade：通过 `chix.Decode*` / `chix.Validate` 直接使用
- 🧯 可选 panic recovery middleware，与统一错误出口协同工作
- 🧭 核心形状保持 `net/http` 兼容，不绑死在特定 router 上

## Install

```bash
go get github.com/kanata996/chix@latest
```

## Quick Start

```go
package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kanata996/chix"
	chixmiddleware "github.com/kanata996/chix/middleware"
)

var errUserNotFound = errors.New("user not found")

func mapUserError(err error) *chix.Error {
	if errors.Is(err, errUserNotFound) {
		return chix.DomainError(http.StatusNotFound, "user_not_found", "user not found")
	}
	return nil
}

func main() {
	wrap := func(h chix.Handler) http.HandlerFunc {
		return chix.Wrap(h, chix.WithErrorMapper(mapUserError))
	}

	r := chi.NewRouter()
	r.Use(chixmiddleware.Recoverer())

	r.Get("/users", wrap(func(w http.ResponseWriter, r *http.Request) error {
		var query struct {
			ID string `query:"id"`
		}
		if err := chix.DecodeQuery(r, &query); err != nil {
			return err
		}
		if query.ID == "missing" {
			return errUserNotFound
		}

		return chix.Write(w, http.StatusOK, map[string]any{"id": query.ID})
	}))

	log.Fatal(http.ListenAndServe(":8080", r))
}
```

默认推荐直接看 `chi` 示例；如果你需要原生标准库接法，也有 `ServeMux` 版本：

- [`_examples/chi`](./_examples/chi)
- [`_examples/nethttp`](./_examples/nethttp)

## What chix standardizes

- handler 采用 `func(http.ResponseWriter, *http.Request) error`
- 成功路径只走 `Write` / `WriteMeta` / `WriteEmpty`
- 失败路径统一返回 `error`，由 `Wrap` 或 `WriteError` 收口
- router 级 `404/405`、middleware 级 auth / rate limit 拒绝，直接走 `WriteError`
- 请求解码与输入校验通过 `chix.Decode*` / `chix.Validate` 完成
- panic recovery 通过 `middleware.Recoverer(...)` 显式接入

## Response Contract

`chix` 对外只保留三种响应形态：

1. 带 body 的成功响应
2. 带 body 的错误响应
3. 不带 body 的空响应

成功响应：

```json
{
  "data": {},
  "meta": {}
}
```

错误响应：

```json
{
  "error": {
    "code": "invalid_request",
    "message": "request contains invalid fields",
    "details": []
  }
}
```

关键约束：

- `data` 必须存在，且不能编码成 `null`
- `meta` 没内容时可省略；如果存在，必须是 JSON object
- `error.code` 必须是稳定机器码
- `error.message` 必须是可安全暴露的文案
- `error.details` 始终存在；没有内容时输出 `[]`

## Core Public API

首页只展示 `chix` 直接暴露给调用方的核心公开面。请求解码与校验能力底层由 `reqx` 提供，但项目接入时通常直接使用 `chix` 根包提供的 facade。

### Handler wiring

```go
type Handler func(w http.ResponseWriter, r *http.Request) error
type Option func(*config)

func Wrap(h Handler, opts ...Option) http.HandlerFunc
func WriteError(w http.ResponseWriter, r *http.Request, err error, opts ...Option)
```

### Success responses

```go
func Write(w http.ResponseWriter, status int, data any) error
func WriteMeta(w http.ResponseWriter, status int, data any, meta any) error
func WriteEmpty(w http.ResponseWriter, status int) error
```

`Write` and `WriteMeta` reject statuses that do not permit a response body, such as `1xx`, `204`, `205`, and `304`. Use `WriteEmpty` for body-less responses.

### Error model

```go
type ErrorKind string

const (
	KindRequest  ErrorKind = "request"
	KindDomain   ErrorKind = "domain"
	KindInternal ErrorKind = "internal"
)

type Error struct {
	// unexported fields
}

func RequestError(status int, code, message string, details ...any) *Error
func DomainError(status int, code, message string, details ...any) *Error
func InternalError(status int, code, message string, details ...any) *Error

func (e *Error) Error() string
func (e *Error) Kind() ErrorKind
func (e *Error) Status() int
func (e *Error) Code() string
func (e *Error) Message() string
func (e *Error) Details() []any
```

### Error mapping

```go
type ErrorMapper func(err error) *Error

func WithErrorMapper(mapper ErrorMapper) Option
func WithErrorMappers(mappers ...ErrorMapper) Option
func ChainMappers(mappers ...ErrorMapper) ErrorMapper
```

### Request decode and validation facade

```go
type DecodeOption = reqx.DecodeOption
type QueryOption = reqx.QueryOption
type Violation = reqx.Violation
type ValidateFunc[T any] func(*T) []Violation

func WithMaxBodyBytes(limit int64) DecodeOption
func AllowUnknownFields() DecodeOption
func AllowEmptyBody() DecodeOption
func AllowUnknownQueryFields() QueryOption

func DecodeJSON[T any](r *http.Request, dst *T, opts ...DecodeOption) error
func DecodeAndValidateJSON[T any](r *http.Request, dst *T, fn ValidateFunc[T], opts ...DecodeOption) error
func DecodeQuery[T any](r *http.Request, dst *T, opts ...QueryOption) error
func DecodeAndValidateQuery[T any](r *http.Request, dst *T, fn ValidateFunc[T], opts ...QueryOption) error
func Validate[T any](dst *T, fn ValidateFunc[T]) error
```

这些 API 的公开语义是 `chix` 的一部分：调用方只依赖 `chix`，请求形状错误会被自动适配进统一错误响应。

### Optional middleware

```go
package middleware

type Option func(*config)

func Recoverer(opts ...Option) func(http.Handler) http.Handler
func WithPanicReporter(reporter PanicReporter) Option

type PanicReport struct {
	Request         *http.Request
	Panic           any
	Stack           []byte
	ResponseStarted bool
	Upgrade         bool
}

type PanicReporter func(PanicReport)
```

`Recoverer` 的行为边界：

- 捕获 panic 并记录请求信息与 stack
- 如果响应尚未开始写回，则输出统一 JSON `500`
- 如果响应已经开始，或请求已经 `Upgrade` / `Hijack`，则不再改写公开结果
- `http.ErrAbortHandler` 会继续透传

## Recommended Usage Model

- 在 router setup 里组合 `ErrorMapper`
- 定义本地 `wrap(...)` 和 `writeError(...)` 小封装
- handler 只关注 decode、service 调用、success write 和返回 error
- 默认按 `chi-first` 方式组织路由层，需要时再退回原生 `net/http`
- 让 router / middleware / panic recovery 都走同一个公开错误出口

## Non-goals

`chix` 不试图做这些事：

- 替代现有 router 或 web framework
- 引入全局 runtime object
- 强制固定目录结构或 DDD 分层
- 把所有 middleware 行为收编成一套自有框架协议

## Examples

- [`_examples/chi`](./_examples/chi)：`chi.Router` 集成
- [`_examples/nethttp`](./_examples/nethttp)：原生 `ServeMux` 集成

这两个目录都是独立 Go module，可以直接运行：

```bash
cd _examples/chi
go test ./...
go run .

cd ../nethttp
go test ./...
go run .
```
