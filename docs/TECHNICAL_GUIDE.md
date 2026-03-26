# chix 技术指导

本文档面向 `chix` 项目的维护者与贡献者，说明当前 MVP 的技术结构、核心流程、边界约束，以及下一阶段演进建议。

## 1. 项目定位

`chix` 的核心目标不是做一个“通用 HTTP 抽象层”，而是构建一个直接绑定 `chi` 的 API 微框架。

这意味着：

- `chi` 是一等依赖，不做路由器无关设计
- 优先复用 `chi` 及其周边中间件生态
- `chix` 负责补上 API 场景中重复出现的高层能力
- 面向“快速交付服务接口”而不是“提供最底层 primitives”

从产品判断上看，`chix` 应该更像：

- `chi` + 声明式操作注册
- `chi` + 类型化输入输出
- `chi` + OpenAPI 自动生成

而不应该变成：

- 一个隐藏 `chi` 的二次路由器
- 一个试图兼容所有 HTTP 框架的适配层
- 一个与实际运行时脱节、只管生成文档的工具

## 2. 当前代码结构

当前核心文件如下：

- [app.go](../app.go)：应用入口、默认中间件、内建文档路由
- [register.go](../register.go)：声明式操作注册与处理函数桥接
- [reqx/](../reqx)：请求解码、参数绑定与 `validator/v10` 校验
- [error.go](../error.go)：统一错误模型与 Problem Details 输出
- [openapi.go](../openapi.go)：根包的 OpenAPI 兼容入口与公开类型别名
- [internal/openapi](../internal/openapi)：OpenAPI 文档模型、schema 推导与文档装配
- [internal/reqmeta](../internal/reqmeta)：请求字段标签与反射规则的共享元数据层
- [app_test.go](../app_test.go)：当前 MVP 的行为测试

## 3. 核心运行流程

一次请求从注册到执行的主流程如下：

1. 调用 `Register(app, operation, handler)` 注册一个操作。
2. `Register` 根据泛型输入输出类型构造运行时处理器。
3. 注册时同步生成对应的 OpenAPI 操作文档并写入内存中的 `Document`。
4. 请求进入时，运行时处理器调用 `reqx.Decode(...)` 完成路径、查询、请求头和 JSON Body 绑定。
5. `reqx` 在绑定完成后统一执行 `validator/v10` 校验。
6. 用户处理函数返回输出对象或错误。
7. 成功时统一写出 JSON 响应；失败时统一写出 `application/problem+json`。

这个流程的关键点是：

- 运行时行为和 OpenAPI 文档都从同一份输入输出类型推导
- 路由注册与文档注册在同一个入口完成
- `chi` 仍然是最终的 HTTP 路由与中间件承载层

## 4. App 设计原则

[app.go](../app.go) 负责维护 `chi.Router` 和 OpenAPI 文档状态。

当前职责包括：

- 创建 `chi` 路由器
- 安装默认中间件
- 注册 `/openapi.json`
- 注册 `/docs`
- 保存请求解码器，如 `reqx.Decoder`
- 保存已注册操作对应的 OpenAPI `Document`

这里有两个原则不要轻易破坏：

- `App` 应该是薄的运行时容器，不要堆积业务语义
- `App` 负责“装配”，而不是承担复杂绑定或校验逻辑

后续如果功能继续增长，优先考虑把能力拆到独立文件或子包，而不是把 `App` 做成一个巨型类型。

## 5. 注册模型设计

[register.go](../register.go) 是当前 API 设计的核心。

`Operation` 当前承载：

- HTTP method
- 路径
- `operationId`
- 摘要与描述
- tags
- 路由级中间件
- 兼容用的单成功响应状态码及描述
- 可选的显式响应列表 `Responses`，用于描述多个成功响应、显式错误响应和响应头

泛型处理函数签名：

```go
type Handler[In any, Out any] func(ctx context.Context, input *In) (*Out, error)
```

这个签名的价值在于：

- 输入输出类型可以直接用于运行时绑定与文档推导
- 处理函数层不需要再手动解析 `http.Request`
- 保持足够轻量，没有引入复杂上下文对象

当前这一层新增了两个约束明确的扩展点：

- `Operation.Responses` 是响应文档的主扩展面。它不会破坏现有 `Register(...)` 调用方式，也不会强行引入新的 handler 结果类型。
- `OperationResponse.OpenAPIModel` 只用于显式成功响应的文档 schema 覆盖，不改变运行时实际返回值。
- 运行时只补最小对齐能力：成功返回值可选实现 `ResponseStatus() int`、`ResponseHeaders() http.Header` 与 `ResponseContentType() string`；错误路径可通过 `(*HTTPError).WithHeader(...)` / `WithHeaders(...)` 和 `WithContentType(...)` 附带最小响应元数据。

后续扩展建议：

- 不要急着引入过大的“上下文对象”
- 如果要增加能力，优先通过 `Operation` 元数据或可选 hook 扩展
- 保证最基础的 handler 签名长期稳定

## 6. 请求解码与校验

[`reqx`](../reqx) 负责输入解码与校验。

当前支持：

- `path`
- `query`
- `header`
- JSON Body
- `validate` 标签驱动的 `validator/v10` 校验

当前规则：

- 导出字段若带有 `path/query/header` 标签，则视为参数字段
- 其他导出字段默认视为 JSON Body 字段
- JSON Body 使用 `encoding/json` 解码，并启用 `DisallowUnknownFields`
- 若请求体 schema 中存在非 `omitempty` 且非指针字段，则无请求体时返回 `400`
- 解码、缺参、非法参数等请求格式问题返回 `400`
- `validate` 规则失败返回 `422`

当前的职责分层应当保持清晰：

- `reqx` 负责“提取、转换、校验”
- `chix` 根包负责把 `reqx` 的 typed error 映射为 HTTP 错误响应
- `internal/reqmeta` 负责让运行时和 OpenAPI 共用同一套字段判定规则

这个实现仍然是实用型 MVP。后续需要重点演进的方向：

- 参数校验规则，如最小值、最大长度、枚举、正则
- 更细粒度的 body 可选/必填控制
- 对 `multipart/form-data`、`application/x-www-form-urlencoded` 的支持
- 更稳定的匿名字段、嵌套结构处理规则
- 更清晰的 tag 设计，避免未来 tag 语义冲突

## 7. 错误模型

[error.go](../error.go) 当前使用 `HTTPError` 作为内部错误承载，并统一输出 `Problem`。

设计目标：

- 让框架层错误总能稳定映射为 HTTP 状态码
- 输出格式固定，便于 API 使用者消费
- 默认把 `chi` 的 Request ID 回写到错误响应中
- 对校验失败输出稳定的字段级 `violations`

后续建议：

- 增加错误类型码，而不只依赖 `status/title/detail`
- 为校验错误定义更稳定的错误结构
- 为业务错误预留统一扩展点
- 继续谨慎扩展错误响应头能力，避免把 `HTTPError` 膨胀成通用响应容器

## 8. OpenAPI 生成策略

当前 OpenAPI 能力由根包的 [openapi.go](../openapi.go) 作为兼容入口，对外继续暴露 `Document`、`Schema` 等类型；实际的模型和 builder 已经下沉到 [internal/openapi](../internal/openapi)。

目前已经覆盖：

- 基本标量类型
- 数组
- map
- 结构体
- `time.Time`
- `components/schemas` 与 `$ref` 去重
- 参数与 body 的拆分
- 请求模型上常见 `validate` 规则到 schema 约束的映射
- 显式的 `400` / `422` problem 响应文档生成
- 多响应描述，包括 `200` / `201` / `204` 成功响应
- 显式错误响应文档，如 `404`、`409`
- 响应头文档描述
- 按响应维度控制 `content-type`
- 显式成功响应的 OpenAPI schema 覆盖
- 字段级文档元数据，如 `doc`、`example`、`deprecated`

当前限制同样很明显：

- 还没有每个响应独立模型的完整运行时表达
- 还没有 security schemes
- 对递归类型与复杂泛型结构的表达还比较保守
- 当前只映射稳定子集的 `validate` 规则，复杂组合规则和自定义 validator 仍不会自动进入 OpenAPI

当前已经提供一个显式的组件命名定制点：`chix.Config.OpenAPISchemaNamer`。它适合解决跨包同名类型、请求/响应希望使用不同组件名等问题；如果未提供，则回退到默认自动命名与冲突避让规则。

当前响应文档装配规则是：

- 若 `Operation.Responses` 没有显式成功响应，则继续从 `SuccessStatus` / `SuccessDescription` 与默认 method 规则推导单成功响应。
- 若 `Operation.Responses` 显式声明了成功响应，则成功响应文档改由该列表完全表达。
- 显式响应可单独声明 `ContentType`；未声明时，成功响应默认使用 `application/json`，错误响应默认使用 `application/problem+json`。
- 显式成功响应可通过 `OpenAPIModel` 覆盖文档 schema；未覆盖时默认使用输出类型的 schema。
- 显式错误响应当前仍固定使用 `Problem` schema。
- `400` / `422` 的自动请求错误响应和 `default` problem 响应仍会保留；若显式声明了相同状态码，则以显式定义覆盖。

下一阶段建议优先做这几件事：

1. 给参数、请求体、响应体补更细粒度的内容类型和元数据扩展点。
2. 为常见标签继续扩展文档语义，如更完整的 `example` / `examples`、`deprecated` 与 `doc` 规则。
3. 为认证、分页、错误模型建立标准化描述。
4. 在不引入过大运行时抽象的前提下，继续补运行时与文档的一致性缺口。

这里最重要的原则是：OpenAPI 不是附属品，必须和运行时行为保持一致。不能出现文档能表达但运行时不支持，或运行时行为存在但文档缺失的长期分裂。

## 9. docs 页面策略

当前 `/docs` 通过 CDN 方式加载 Swagger UI。

优点：

- 实现简单
- 维护成本低
- 对 MVP 足够实用

缺点：

- 依赖外部网络
- UI 可控性有限
- 生产环境若有严格静态资源策略，需要改成本地托管

后续可以考虑两个方向：

- 保持默认 CDN 方式，同时提供可切换的静态资源托管方案
- 在项目稳定后提供更贴近框架定位的默认文档前端

## 10. 推荐的后续模块拆分

随着功能增长，建议在当前边界上继续细化，而不是回到“大根包”：

- `reqx`：请求解码、绑定、校验
- `resp`：成功响应输出
- `errx`：错误模型与错误码
- `internal/schema`：schema 推导
- `internal/openapi`：文档装配

但现阶段不建议为了“看起来整洁”过早拆得太碎。当前代码量仍然适合在根包快速迭代。

## 11. 下一阶段开发优先级

建议按下面顺序推进：

1. 校验能力
原因：这会直接影响 API 可用性，也会反过来驱动 OpenAPI 表达能力。

2. OpenAPI schema 强化
原因：当前已经有自动文档，接下来需要把文档质量做实。

3. 响应内容类型与更细粒度运行时元数据
原因：多状态码和响应头已经有了基本支撑，下一步需要把内容协商和更复杂的运行时对齐补实。

4. 路由分组与模块化注册
原因：服务一旦长大，单点注册会开始失控。

5. 安全方案与中间件预设
原因：这是框架真正进入“开箱即用”阶段的重要一环。

## 12. 贡献时的约束

后续开发建议遵守以下约束：

- 不为了抽象而抽象，优先服务实际 API 场景
- 不削弱 `chi` 的存在感，不把 `chi` 包装成不可见实现细节
- 保证运行时行为和 OpenAPI 文档同步演进
- 新增能力时优先补测试，尤其是文档与运行时一致性测试
- 公共 API 一旦放出，尽量保持稳定，不频繁改 handler 基础签名

## 13. 当前文档与代码关系

如果后续实现发生变化，优先同步更新以下两处文档：

- [README.md](../README.md)：面向用户的定位、示例与能力说明
- [docs/TECHNICAL_GUIDE.md](TECHNICAL_GUIDE.md)：面向维护者的技术设计与演进说明

这份技术指导文档的目标不是替代代码，而是帮助后续重构与扩展时保持方向一致。
