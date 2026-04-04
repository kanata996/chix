// Package middleware provides reusable chi middlewares for request logging and
// related HTTP boundary concerns.
//
// RequestLogger is the supported request-logging entry point for this package.
// It is an opinionated chi + httplog composition that bundles:
//   - chi request id injection
//   - trace id propagation
//   - httplog request logging with panic recovery
//   - stable base request log attrs such as request.id and http.route
//
// This package does not define a separate supported mode where callers
// self-assemble the equivalent chi/httplog middleware chain. When using
// RequestLogger, do not also mount chi's Logger/Recoverer, traceid.Middleware,
// chi's RequestID, or httplog.RequestLogger on the same route chain. Those
// concerns are already handled by this package's wrapper.
package middleware
