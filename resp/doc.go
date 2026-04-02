// Package resp provides response-side helpers for chi-based JSON APIs.
//
// It focuses on HTTP output boundaries:
//   - Echo-style JSON response writers
//   - encode success payloads
//   - encode structured error payloads
//   - normalize public HTTP error semantics
//
// Typical usage:
//   - JSON / JSONPretty / JSONBlob for low-level JSON output
//   - OK / Created / NoContent for successful responses
//   - WriteError for structured error output
//   - HTTPError and helpers for reusable public error values
package resp
