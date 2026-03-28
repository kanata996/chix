# 开发指导

本文定义 `chix` 的运行时边界、设计约束和 review 标准。

`chix` 是一个 `chi`-first 的 JSON API micro runtime。
文档中的约束默认视为当前开发规范。

## 1. 产品定义

`chix` 不是 router，也不是通用 web framework。

`chix` 是运行在 `chi` API 边界上的那一层 runtime：

1. `chi` 负责匹配路由、组织 middleware、处理 ingress concerns
2. `chix` 负责绑定输入、校验输入、执行业务 handler、映射错误、写 JSON、
   发出边界观测
3. 业务代码负责领域逻辑，返回成功结果或错误

## 2. 核心原则

`chix` 的 runtime 设计应持续遵守以下原则：

- success / failure / observation 共享同一条 API 边界执行路径
- runtime 拥有最终 HTTP 输出权
- ingress concern 保持在 runtime 外层
- route-local 策略使用显式声明，而不是回退到胶水式 helper
- 普通 JSON API 的能力边界保持收敛

## 3. Runtime 应负责什么

runtime 应只负责以下步骤：

1. 从 `chi` 与 `*http.Request` 提取请求上下文
2. 绑定并校验 endpoint 输入
3. 调用业务 handler
4. 把成功结果编码成标准 success envelope
5. 把内部错误映射成稳定的公开 `HTTPError`
6. 把错误编码成标准 error envelope
7. 在失败路径上发出结构化观测

runtime 不应负责：

- 路由
- ingress middleware 链
- recover/auth/rate limit/CORS 等接入层问题
- tracing 或 access log
- 普通 JSON API 之外的 transport runtime
- 为了“接管一切”去包装通用 `ResponseWriter`

## 4. Chi-First 的含义

项目默认不追求 router-agnostic 设计。

`chix` 直接面向 `chi` 优化，因为：

- route grouping 是 API ergonomics 的核心能力
- `chi` middleware composition 已经是成熟的 ingress shell
- 路由参数和 request-scoped policy 在 `chi` 中更自然
- request id、recover、timeout、auth 等配套能力 `chi` 已经做得足够好

runtime 仍然可以暴露 `http.Handler`，但产品语言、文档、示例和集成能力都应
默认围绕 `chi`。

## 5. Handler 模型

runtime 应拥有最终 HTTP 输出权。handler 不再自己写成功响应或错误响应。

目标形状：

```go
type Operation[I any, O any] struct {
	Name          string
	Method        string
	Pattern       string
	SuccessStatus int
	Validate      Validator[I]
	ErrorMappers  []ErrorMapper
}

type Handler[I any, O any] func(context.Context, *I) (*O, error)
```

通过 runtime 挂载：

```go
func Handle[I any, O any](rt *Runtime, op Operation[I, O], h Handler[I, O]) http.Handler
```

这是最关键的设计决策。没有 runtime-owned handler 模型，`chix` 就无法形成
稳定的边界执行路径。

`Operation` 应只承载 runtime 执行真正需要的 route-local 策略。
这意味着：

- 可以放 success status、operation-level validation hook、operation-level error mappers
- 不应重新塞回 summary/tags/openapi/middleware 之类文档或 ingress 元数据

## 6. 输入模型

输入绑定应显式、面向 JSON API，并保持足够收敛。

runtime 直接支持：

- path binding
- query binding
- JSON body binding
- validation hook / adapter point

runtime 不承担：

- multipart form workflow
- 任意 content negotiation
- streaming request body
- 单一 endpoint 上混合多种 transport 语义

执行顺序也必须固定：

1. 先完成 path / query / body 绑定
2. 绑定成功后再执行 validation
3. validation 成功后才进入业务 handler

任何绑定失败或 validation 失败都应直接终止请求，不进入业务 handler。

其中：

- 请求形状错误由 runtime 直接标准化为稳定的公开 4xx 错误
- validation 错误由 runtime 直接标准化为 `422 invalid_request`
- `422 invalid_request` 的 `details` 应由一个或多个 violation 组成
- runtime 自己产出的公开边界错误不再重新进入 `ErrorMapper` 链

输入绑定规则也必须写死：

- path / query 绑定必须显式声明来源标签
- JSON body 字段使用标准 `json` tag；未声明绑定来源的导出字段不应被隐式视为 body 字段
- 同一个字段最多只能声明一个输入来源
- path / query 缺失值只保留 Go 零值；required 语义属于 validation，而不是 binding
- query 绑定到标量字段时，如果同名参数出现多次，应视为请求形状错误
- body 非空且 `Content-Type` 不是 `application/json` 或 `application/*+json` 时，应返回 `415 unsupported_media_type`
- malformed JSON、JSON 类型不匹配、unknown field 都应视为请求形状错误

校验契约应固定为最小集合：

```go
type Violation struct {
	Source  string `json:"source"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Validator[I any] func(context.Context, *I) []Violation
```

其中：

- `Source` 只允许 `path`、`query`、`body`
- 至少要支持 operation-level validation hook
- 如果后续补 adapter-style 校验集成，也必须先归一化成相同的 `[]Violation` 契约，再交给 runtime
- 如果提供现成 adapter，也应只是 `Validator[I]` 的薄适配层；例如可在独立子包中提供 `go-playground/validator` 到 `[]Violation` 的归一化 helper，但不应把 validator 直接塞进 binding 阶段

runtime 自己直接产出的公开错误也应保持收敛：

- `400 bad_request`：path / query / body 形状错误
- `415 unsupported_media_type`：body 存在但媒体类型不受支持
- `422 invalid_request`：validation 失败，`details` 为 violation 数组

## 7. 响应模型

响应模型应保持小而明确。

成功 envelope：

```json
{
  "data": {}
}
```

错误 envelope：

```json
{
  "error": {
    "code": "internal_error",
    "message": "internal server error",
    "details": []
  }
}
```

核心 wire contract 必须固定：

- 成功响应固定写成 `{"data": ...}`
- 错误响应固定写成 `{"error": {...}}`
- 错误 `details` 在 wire 上始终是数组；没有 details 时写空数组
- `204 No Content` 是唯一允许省略 success envelope 的成功响应
- 非 `204` 成功响应即使 handler 返回 `nil`，也应写成 `{"data": null}`
- 成功状态码优先级固定为：`Operation.SuccessStatus` > 最近一层 scope/runtime 默认值 > runtime 内建默认值
- runtime 内建默认成功状态码固定为：`POST -> 201`，其余方法 -> `200`

不应先暴露自定义成功 headers 或 metadata builder API。
这些可以作为后续扩展方向，但不应成为当前 runtime 设计前提。

不要一开始就做出一个庞大的 response builder DSL。

## 8. 错误模型

错误映射仍然是 `chix` 存在的核心理由之一。

业务代码返回 domain error，runtime 通过有序 mapper 和 fallback 规则把它
转换成稳定的公开错误。

必须保留的部件：

- `HTTPError`
- `ErrorMapper`
- 默认 internal error fallback
- route/group/runtime 级 mapper 组合

mapper 组合规则必须固定：

- operation-level mapper 优先于 scope-level mapper
- 更内层的 scope mapper 优先于更外层的 scope mapper
- scope-level mapper 优先于 runtime-level mapper
- 同一层内保持声明顺序
- 第一个返回非 nil 的 mapper 生效，剩余 mapper 不再执行
- 已经是公开边界错误的值不再进入 mapper 链
- 所有 mapper 都未命中时走 internal error fallback

公开边界错误的识别方式也必须固定：

- `*HTTPError` 是唯一的公开边界错误载体
- runtime 应通过 `errors.As(err, &httpErr)` 识别已经公开化的错误
- handler 或 mapper 返回的 `*HTTPError` 可以被包装，但一旦能被识别出来，就必须直接进入响应路径，而不是重新参与 mapper
- runtime 自己产出的 `400` / `415` / `422` 公开错误同样不再进入 mapper 链

runtime 需要持续区分两类东西：

- 用于观测的原始错误
- 用于对外响应的公开错误

`Scope` 的继承语义也应固定，避免实现各自理解：

- `Scope(opts...)` 产生的是 parent config 的逻辑子作用域，而不是共享可变状态
- error mappers 在 runtime / scope / operation 三层之间按规则叠加组合，而不是互相覆盖
- observer、extractor、success status default 这类单值配置采用“最近一层覆盖更外层”

## 9. 观测模型

`chix` 应继续保留结构化边界观测 hook，而不是直接绑定某个日志后端。

目标 event 结构应保持克制，只承载边界排障真正需要的信息。

event 结构应固定为：

```go
type Event struct {
	Error      error
	Public     *HTTPError
	RequestID  string
	Method     string
	Target     string
	RemoteAddr string
}
```

不要把 event 演变成 request dump。应用需要更多上下文时，应通过扩展点
显式加入，而不是默认把整个 request 对象塞进事件结构。

观测触发时机也必须明确：

- observer 只处理失败路径上的边界事件
- runtime 应在已经解析出公开错误、但尚未尝试写错误响应时发出失败观测
- 这样即使 response 已经开始，或后续错误写回失败，也仍然保留边界观测
- 如果错误响应写回本身失败，runtime 应再发出一个 internal-error 观测事件

## 10. Request ID 策略

runtime 不管理 request id。

runtime 只在 request id 存在时读取并记录它，典型来源是
`middleware.RequestID`。如果不存在，对应字段就为空。

这能让 request id 回到 ingress 层，而不是继续让 runtime 维护内部状态机。

## 11. 非目标

应明确避免以下方向：

- 重新长成通用 framework shell
- 试图替代 `chi`
- 发明一套 router DSL
- 在核心 runtime 稳定前就先做 OpenAPI/codegen
- 再次退回 helper-first 的公开叙事
- 为了少数 transport 场景过早扩张能力边界

## 12. Review 标准

以后审查 runtime 相关改动时，至少应回答这几个问题：

1. 这个改动是否让 `chix` 更像一个 `chi`-first API runtime？
2. 它是否加强了 success / failure 的单一路径？
3. 它是否把 ingress concern 继续留在 runtime 外层？
4. 它是否仍然保持普通 JSON API 的窄边界？
5. 它是否保持了文档已经写死的 mapper / validation / response / observer 语义？
6. 它引入的复杂度是否真的换来了更大的运行时杠杆？
