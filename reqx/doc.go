// Package reqx provides request-side helpers for chi-based JSON APIs.
//
// It focuses on HTTP input boundaries:
//   - bind JSON/query/path values into structs
//   - validate bound values with validator/v10
//   - normalize common request violations into consistent HTTP errors
//
// Typical usage:
//   - BindAndValidate for Echo-style path/query/body binding
//   - BindBody or BindAndValidateBody for JSON request bodies
//   - BindQueryParams + ValidateQuery for query DTOs
//   - BindPathValues / BindHeaders for direct-source binding
//   - ParamString/ParamInt/ParamUUID for simple path params
package reqx
