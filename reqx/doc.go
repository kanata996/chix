// Package reqx 为基于 net/http 的 JSON API 提供请求侧辅助能力。
//
// 它聚焦在 HTTP 输入边界：
//   - 将 JSON / query / path 值绑定到结构体
//   - 使用 validator/v10 校验绑定后的输入
//   - 将常见请求违规统一收敛为稳定的 HTTP 错误
//
// 典型用法：
//   - 使用 BindAndValidate 处理 path/query/body 综合绑定
//   - 使用 BindBody 或 BindAndValidateBody 处理 JSON 请求体
//   - 使用 BindQueryParams + ValidateQuery 处理 query DTO
//   - 使用 BindPathValues / BindHeaders 处理单一来源绑定
//   - 使用 ParamString / ParamInt / ParamUUID 读取简单 path 参数
//
// path 输入只依赖 net/http 暴露的 PathValue / Pattern 命名 wildcard 语义，
// 不依赖 chi.RouteContext，也不承诺 chi 专有 `*` catch-all 的兼容行为。
package reqx
