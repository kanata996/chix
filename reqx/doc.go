// Package reqx provides request decoding and validation helpers that return
// structured request/input Problems.
//
// reqx focuses on request-shape concerns at the HTTP boundary:
//
//   - decoding JSON request bodies into DTOs
//   - decoding URL query parameters into DTOs
//   - rejecting unsupported Content-Types
//   - rejecting empty, malformed, oversized, or multi-value JSON bodies
//   - rejecting unknown fields and basic type mismatches
//   - translating validation failures into stable 422 request problems
//
// Its role is intentionally narrow. reqx is not a new binding framework, does
// not impose a validation library, and does not perform domain validation.
//
// The intended boundary is:
//
//   - handler: decode request DTOs, run DTO/input validation, call services
//   - service/domain: enforce business rules and return domain errors
//
// Callers can use reqx's built-in validation hook, adapt their preferred
// third-party validator, or write custom validation functions directly.
//
// When used with chix, chix.Wrap and chix.WriteError adapt reqx Problems into
// chix's standardized error envelope automatically.
//
// Current helpers focus on JSON request bodies and URL query parameters. Path
// and header binding are intentionally left to the caller so reqx stays router-
// agnostic and avoids turning into a general binding framework.
package reqx
