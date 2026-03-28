# Binding 阶段性说明

本文是 `chix` v1 当前 `binding` 实现的阶段性文档。

它的用途是记录当前实现边界、内部结构和后续开发约束，避免在继续推进其他核心能力时重新把输入绑定做回“大而全”的 framework 层。

本文不是公开 API 承诺，也不替代 [TECHNICAL_GUIDE.md](TECHNICAL_GUIDE.md)。
`TECHNICAL_GUIDE.md` 仍然是 source of truth；如果本文与技术手册冲突，应以技术手册为准，再回头修正文档或实现。

## 0. 本轮 Review 结论

本轮 review 的结论是：

- 对 v1 目标来说，`binding` 当前已经形成一个相对清晰的内部闭环
- `binding` / `inputschema` 之间的元数据分工已经稳定
- 当前没有发现需要继续扩抽象层才能解决的结构性问题

本轮 review 后，仍然建议保持克制：

- 不再继续拆新的内部包
- 不把 `binding` 做成通用 decoder framework
- 后续如果改动 `binding`，优先只做 bugfix、约束收口和与技术手册对齐

## 1. 当前目标

`binding` 当前只服务于 runtime 的 typed input 绑定阶段。

它负责：

- 把请求输入绑定到 endpoint input struct
- 支持 `path` / `query` / JSON body 三种来源
- 在 bind 成功后执行内置 validator/v10 校验
- 把请求形状错误归类为 runtime 可消费的内部错误
- 产出 runtime 可归一化的 `422 invalid_request` detail

它不负责：

- required 语义
- validation 执行
- OpenAPI
- router DSL
- multipart/form-data
- content negotiation
- transport 抽象

## 2. 当前结构

当前输入绑定实现分成两层：

### `internal/inputschema`

`internal/inputschema` 负责一次性解析 input struct 的元数据：

- 字段来源：`path` / `query` / `body`
- 公开字段名
- violation / body path
- `reflect` index
- 匿名嵌入字段的透明展开
- 顶层 body 字段表
- `validator.StructNamespace()` 到公开 location 的映射
- input schema 合法性检查
- `reflect.Type -> schema` 内部缓存

这个包是 `binding` 内部绑定与校验阶段共享的最小元数据层。

### `internal/binding`

`internal/binding` 只消费 schema 做实际绑定：

- `bind.go`
  - 入口校验
  - 取 schema
  - 串起 parameter/body 绑定
- `params.go`
  - 从 `chi` path param 和 URL query 取值
  - 把参数字符串赋值到目标字段
- `body.go`
  - 校验 body 是否存在
  - 校验 `Content-Type`
  - 校验顶层 unknown body field
  - 解码单个 body 字段
- `error.go`
  - binding 内部错误分类
- `reflect.go`
  - 反射写入辅助

## 3. 当前已固定的行为

### schema 规则

- `path` / `query` 必须显式声明来源 tag
- body 字段必须显式使用 `json` tag
- 未声明来源的导出字段不会被隐式视为 body 字段
- 同一字段不能同时声明多个输入来源
- 嵌套 body 字段也必须遵守显式 `json` tag 规则
- `json:"-"` 视为显式忽略
- body 内部不能再混入 `path` / `query` source
- 不合法 input schema 属于配置错误
- 不合法 input schema 会在 `Handle(...)` 构造阶段尽早暴露

### parameter 绑定规则

- `path` / `query` 缺失值保留 Go 零值
- query 标量字段出现重复值时返回请求形状错误
- slice query 允许多值
- 当前支持的 parameter 字段类型保持收敛：
  - string / bool
  - int / uint / float 各标量
  - `time.Time`
  - 实现 `encoding.TextUnmarshaler` 的类型
  - 上述标量的 slice
  - 指向上述标量的指针
- 不支持指向 slice 的指针参数字段，例如 `*[]string`

### body 绑定规则

- body 仅支持 JSON
- body 非空但媒体类型不是 `application/json` 或 `application/*+json` 时，返回 `415 unsupported_media_type`
- 顶层 unknown body field 视为请求形状错误
- 嵌套对象中的 unknown field 也视为请求形状错误
- 单个 body field 解码时要求只有一个 JSON value

## 4. 当前测试覆盖

当前测试已经覆盖以下重点：

- path / query / body 基本绑定
- 匿名嵌入字段绑定
- duplicate query scalar
- unsupported media type
- 嵌套 body unknown field
- 嵌套 body 未标注 `json` tag 的配置错误
- `json:"-"` 的嵌套字段忽略
- nested body violation path
- embedded query/body violation path
- invalid input schema 的 fail-fast 行为
- parameter 支持类型和实际赋值路径的一致性

## 5. 设计取舍

### 为什么保留 `inputschema`

这里不是为了抽象而抽象。

绑定和内置校验都依赖同一类事实：

- 字段来自哪里
- 公开字段名是什么
- 路径怎么生成
- 匿名嵌入如何展开
- `reflect` index 是什么

这类元数据如果分别维护，会很快分叉。
因此共享一层最小 schema 元数据是合理的。

### 为什么不继续扩 `binding`

v1 阶段，`binding` 已经足够支撑当前 runtime 实现。

继续往这里叠更多能力，风险是：

- 把 runtime 拉回通用 web framework 方向
- 把 validation 和 binding 重新耦合
- 提前引入并不急需的 transport 复杂度

因此，`binding` 后续应优先保持收敛，而不是继续扩成输入 DSL。

### 当前 review 后仍需注意的点

当前没有阻塞 v1 的已知结构性问题，但后续修改时要注意两件事：

- parameter 支持类型的 schema 校验与实际赋值逻辑必须同步修改
- body 绑定当前仍然是面向 JSON object 的收敛实现，不应顺手扩成 transport/plugin system

## 6. 后续开发约束

下一步准备开发其他核心能力时，默认遵守以下约束：

- 不恢复旧的 `App` / `Register` / `OpenAPI` 入口
- 不给 `binding` 增加公开 API
- 不在 `binding` 内引入 middleware 风格扩展点
- 不为了“未来可能支持更多 transport”而提前泛化当前实现

如果后续还要改 `binding`，优先级应是：

1. 修 bug
2. 让实现和技术手册继续对齐
3. 为 runtime 当前明确需要的能力留出位置

不应优先做：

- 更复杂的抽象层
- 用户可配置的 schema/cache 策略
- 面向 OpenAPI 的输入元数据扩张
- 通用 request decoder/plugin system

## 7. 当前可以视为稳定的内部边界

可以把当前 `binding` 视为下面这个内部分工：

- runtime 负责执行顺序和公开错误输出
- `binding` 负责把请求绑定到 typed input
- `binding` 负责在 bind 后执行内置 validator/v10 校验
- `inputschema` 负责共享输入元数据

这个边界已经足够支撑下一阶段继续开发 runtime 的其他核心能力。
