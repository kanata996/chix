// Package resp 为基于 chi 的 JSON API 提供响应侧辅助能力。
//
// 它聚焦在 HTTP 输出边界：
//   - JSON 响应写回
//   - 编码成功响应 payload
//   - 编码结构化错误响应 payload
//   - 统一对外 HTTP 错误语义
//
// 典型用法：
//   - 使用 JSON / JSONPretty / JSONBlob 进行底层 JSON 输出
//   - 使用 OK / Created / NoContent 写成功响应
//   - 使用 WriteError 写结构化错误响应
//   - 在 5xx 场景通过 WriteError 补 request log 诊断字段，并输出独立错误日志
//   - 使用 HTTPError 及相关辅助构造可复用的公共错误值
package resp
