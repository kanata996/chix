// Package middleware 提供 chix 使用的一些小型、可选 chi 中间件。
//
// 该包不负责完整的请求日志链路。访问日志相关配置应由具体服务维护，
// 通常配合 chi + httplog 使用。
//
// RequestLogAttrs 是一个按需启用的桥接中间件，用于将与请求关联的属性
// （如 traceId 和 request.id）复制到当前 httplog 请求日志中。
package middleware
