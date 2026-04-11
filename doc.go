// Package chix 提供面向 chi 的错误响应与日志预设。
//
// 适合在大多数 handler 中直接使用：
//   - 面向 chi + httplog + traceid 的 ErrorResponder 预设
//   - 默认错误写回入口 WriteError
//   - 可选的请求日志属性中间件
//
// 请求绑定、输入校验和共享错误模型由 github.com/kanata996/hah 提供；
// 成功响应也建议直接使用 hah 的实现。
package chix
