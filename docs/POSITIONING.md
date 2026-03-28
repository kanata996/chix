# 定位与价值

本文回答一个更基础的问题：在成功响应已经和 `echo` / `gin` / `huma` 一样直接返回
DTO 之后，`chix` 还剩下什么价值。

结论先写在前面：

- `chix` 的价值不在响应 envelope，也不在“长得像另一个 web framework”
- `chix` 的价值在于：它是否在 API 边界上替你消除了足够多的重复决策
- 如果这件事做不到，直接使用 `chi` 往往是更好的选择

## 1. `chix` 不应该解决什么

`chix` 不应该靠下面这些点证明自己存在：

- 自定义成功响应 envelope，比如 `{"data": ...}`
- 再发明一套 router DSL
- 接管 middleware 系统
- 把 OpenAPI、Swagger UI、router、runtime 全塞进一个大产品
- 为了和 `chi` 做出差异，堆新的 builder 或 DSL

这些方向要么已经被 `chi` 做得很好，要么很容易把 runtime 做胖，最后只得到一个
抽象层更厚、价值却不更清晰的框架。

## 2. `chix` 真正应该解决什么

如果 `chix` 有价值，这个价值应该集中在 API 边界上的统一执行路径。

当前版本最有可能成立的价值点只有这些：

- typed handler：业务只写 `func(ctx, *Input) (*Output, error)`
- 统一输入绑定：固定支持 `path` / `query` / `json`
- fail-closed 请求语义：媒体类型、unknown field、类型错误、重复 query 标量都由 runtime 统一收口
- 内建 validation：bind 成功后自动执行校验
- 统一错误模型：`HTTPError` / `ErrorMapper`
- route scope 级策略继承：把 group 级错误映射、默认状态码、observer 放在边界层表达
- failure observer：统一发出失败观测事件

如果这些能力组合起来，能显著减少每个 endpoint 都重复写一遍的边界胶水代码，
那 `chix` 就是成立的。

## 3. `chi` 和 `chix` 分别负责什么

两者不在同一层。

`chi` 负责：

- 路由匹配
- route grouping
- middleware 组合
- `http.Handler` 生态兼容
- ingress concerns，比如 request id、recover、auth、rate limit、CORS

`chix` 负责：

- 把请求绑定到 typed input
- 在固定生命周期里完成 bind -> validate -> handle
- 把业务错误收口成稳定的公开错误
- 在失败路径上发出统一观测
- 写成功 JSON body 和错误 JSON envelope

所以 `chix` 不是 `chi` 的替代品，而是一层运行在 `chi` API 边界上的小 runtime。

## 4. 什么情况下 `chix` 值得存在

当下面这些问题在真实项目里反复出现时，`chix` 才值得保留成独立库：

- 每个 handler 都在重复写 path/query/body decode
- 每个项目都在重新决定 unknown field、重复 query 参数、媒体类型错误该怎么处理
- validation 生命周期不一致，有的接口先执行业务再报参数错误
- 错误模型分散，客户端面对的错误 JSON 不稳定
- route group 级策略只能靠 middleware 和胶水约定维持
- 失败日志、request context、error reporting 缺乏统一边界

如果 `chix` 能把这些问题一起解决，并且 API 仍然保持小而硬，它就有明确价值。

## 5. 什么情况下应该直接用 `chi`

如果项目并不痛于这些边界重复，或者团队根本不想引入一层 opinionated runtime，
那直接用 `chi` 更合理。

尤其是下面几种情况，通常没必要上 `chix`：

- 项目很小，手写 binding / error handling 成本很低
- 团队已经有稳定的 `internal/httpx` 约定
- 需要大量非 JSON API、streaming、multipart、定制化 transport 能力
- 团队对 runtime 施加统一约束没有强需求
- 最终只是想少写几行样板，而不是统一边界语义

这时把 `chix` 做成公共库，收益通常不如直接保留在应用内。

## 6. 一个诚实的判断标准

不要问：`chix` 和 `echo` / `gin` / `huma` 的成功响应是不是一样。

应该问：

- 它是否让业务 handler 只关心领域输入和输出
- 它是否让请求错误和验证错误的语义稳定且可预测
- 它是否把错误映射和失败观测收敛成统一边界
- 它是否真的减少了每个 endpoint 的重复决策

如果答案是“没有明显减少”，那 `chix` 的价值就很弱。

如果答案是“有，而且这种统一很难靠散落的 helper 保持”，那 `chix` 的定位就是成立的。

## 7. 推荐的产品取向

`chix` 更适合被定义成：

`chi`-first 的 typed JSON API boundary runtime

而不是：

- 一个新的 web framework
- 一个靠 response envelope 做差异化的 API 库
- 一个试图覆盖 router / docs / middleware / transport 的大一统产品

这个取向意味着：

- 公开 API 保持很小
- 关注 bind / validate / error / observe 这条边界执行路径
- 尽量不把 `chi` 已经做好的东西重复做一遍
- 不为了证明存在感而增加额外协议层和抽象层

## 8. 对维护者的实际建议

如果后续要继续推进 `chix`，优先级应该是：

1. 证明它能稳定减少边界重复代码
2. 证明它能让错误语义和失败观测更一致
3. 保持 runtime 的能力边界收敛

而不是：

1. 继续发明新的响应 envelope
2. 扩张成更大的 framework
3. 为了微基准优势牺牲 API 清晰度

如果未来无法证明前一组目标成立，就应该接受一个更朴素的结论：

把 `chix` 收缩成应用内的边界层实现，甚至直接回到 `chi`，都是合理选择。
