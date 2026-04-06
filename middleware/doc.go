// Package middleware provides small optional chi middlewares used by chix.
//
// This package does not own the full request logging stack. Access log
// configuration should stay with the service, typically via chi + httplog.
//
// RequestLogAttrs is an opt-in bridge that copies request-correlated attrs such
// as traceId and request.id into the current httplog request log.
package middleware
