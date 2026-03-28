// Package chix 提供一个 chi-first 的 JSON API micro runtime。
//
// 产品边界保持收敛：
//   - chi 负责路由匹配、route grouping、middleware 和其他 ingress concerns
//   - chix 负责绑定输入、执行 validation、调用业务 handler、映射错误、写 JSON、发出失败观测
//   - 业务代码只返回成功结果或错误，不直接拥有最终 HTTP 输出权
//
// 当前根包公开的核心入口包括：
//   - Runtime / Scope
//   - Handle(rt, op, h)
//   - HTTPError / ErrorMapper
//   - Observer / Extractor
//   - 内置输入校验（validator/v10）
//
// 当前 runtime 只面向普通 JSON API，直接支持 path、query 和 JSON body 输入绑定。
// 它不是 router，也不是通用 web framework；OpenAPI、router DSL、framework-level middleware
// system、multipart 和 streaming transport 都不在当前产品边界内。
//
// 当前 source of truth 见 docs/TECHNICAL_GUIDE.md 与 docs/RUNTIME_API_DRAFT.md。
package chix
