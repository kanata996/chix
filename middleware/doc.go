// Package middleware provides reusable chi middlewares for request logging and
// related HTTP boundary concerns.
//
// The request logging middleware composes:
//   - chi request id injection
//   - trace id propagation
//   - httplog request logging with panic recovery
//   - stable base request log attrs such as request.id and http.route
package middleware
