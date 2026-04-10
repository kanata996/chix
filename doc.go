// Package chix 提供面向 chi 的请求/响应边界预设。
//
// 适合在大多数 handler 中直接使用：
//   - 常用的请求绑定、校验和响应写回入口
//   - 面向 chi + httplog + traceid 的 ErrorResponder 预设
//   - 可选的 request log attrs middleware
//
// 更底层的 net/http 边界能力由 github.com/kanata996/hah 提供。
package chix
