# Changelog

All notable changes to this project will be documented in this file.

This project follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Versioning is intended to follow SemVer once `v1.0.0` is released.

## Compatibility Policy

- The current public compatibility boundary is the API and HTTP behavior documented in [README.md](./README.md) and [docs/RUNTIME_API_DRAFT.md](./docs/RUNTIME_API_DRAFT.md).
- [docs/TECHNICAL_GUIDE.md](./docs/TECHNICAL_GUIDE.md) is the maintainer source of truth for runtime semantics and product boundary.
- `internal/*` packages, benchmark layouts, test helpers, and implementation details are not public API.
- Before `v1.0.0`, breaking changes may still happen in minor releases, but they should be called out explicitly in this changelog.
- After `v1.0.0`, breaking public API or HTTP contract changes should only ship in a new major version.
- Historical entries below may describe pre-v1 API experiments. If an older entry conflicts with the current runtime docs, follow the current docs.

## [Unreleased]

### Changed

- Rewrote the README around the current `chi`-first runtime model: `Runtime`, `Scope`, `Handle(...)`, typed input binding, `Validator`, `HTTPError`, `ErrorMapper`, and `Observer`.
- Added package-level docs so pkg.go.dev reflects the current runtime model instead of the deleted framework/App narrative.
- Removed outdated README references to the deleted `App/Register/OpenAPI/Swagger UI` product shape.
- Changed the success HTTP contract to write the handler DTO directly instead of wrapping every non-`204` success response in `{"data": ...}`.
- Simplified `Operation` to per-route overrides only: it is now non-generic and no longer carries a redundant `Method` field.
- Changed `Handle(...)` to accept an optional trailing `Operation`, so the default path is `Handle(rt, h)` and route-local overrides stay inline at the call site.
- Added `HandleNoContent(...)` for common `204 No Content` handlers.
- Changed `Handle(...)` to return `http.HandlerFunc`, so `chi` method helpers like `Get/Post/Delete` can use it directly.

### Breaking Changes

- Non-`204` success responses now serialize the handler return value as the top-level JSON body. Existing clients that read success payloads from `response.data` must switch to reading the DTO directly.
- When a non-`204` handler returns `nil`, the success body is now JSON `null` instead of `{"data": null}`.
- `Operation[I, O]` is now `Operation`; call sites must drop generic type arguments.
- `Operation.Method` has been removed. Default success status now always derives from the incoming request method when `SuccessStatus` is unset.
- `Handle(rt, op, h)` is now `Handle(rt, h, op)`, and the common case no longer passes an explicit zero-value `Operation`.

### Migration Notes

- Update success-response consumers from `response.data.<field>` to `response.<field>`.
- Keep treating `204 No Content` as the only success status with no response body.
- Error responses are unchanged and still use `{"error": {...}}`.
- Replace `Handle(rt, Operation[I, O]{}, h)` with `Handle(rt, h)` when you do not need operation-level overrides.
- Replace delete-style `Handle(..., Operation{SuccessStatus: http.StatusNoContent}, ...)` boilerplate with `HandleNoContent(...)` where appropriate.

## [v0.3.0] - 2026-03-26

Historical note: this release described a pre-reset API experiment built around `Contract(...)` / `WriteError(...)`.
It is kept for project history only and should not be used as the current runtime API reference.

### Highlights

- Introduced an explicit business-boundary error model built around `Contract(...)`, immediate `WriteError(w, r, err)`, and explicit `Respond*` success writes.
- Added route-scoped boundary configuration with `ContractOption`, `WithContractErrorMappers(...)`, and `WithContractErrorReporter(...)`.
- Added the `errcode` subpackage as the unified public entry point for shared error-code constants, including the cross-domain codes `resource_not_found`, `already_exists`, `operation_not_allowed`, and `state_conflict`.

### Breaking Changes

- Removed the old runtime-style APIs `HandleErrors(...)`, `Error(...)`, `UseErrorMappers(...)`, `NotFoundHandler()`, and `MethodNotAllowedHandler()`.
- `WriteError(...)` now composes route-scoped `Contract(...)` configuration with call-site `ErrorOption`s. `WithErrorMappers(...)` and `WithErrorReporter(...)` now produce `ErrorOption`s instead of the old runtime `Option`.
- The documented HTTP contract now only covers responses written by `Respond(...)`, `RespondWithMeta(...)`, `RespondEmpty(...)`, and `WriteError(...)`. Router-level `404/405`, outer middleware interceptions, and panic/recover responses are outside the `chix` contract.
- Default error observation now focuses on the business-boundary stages `decode`, `validate`, `processing`, and `write_response`. Router-level `routing` is no longer part of the default `chix` stage model.
- Removed root-package error-code constants from `chix`; shared public codes now live in `errcode`, while `reqx` keeps owning its own request and validation codes.
- Removed the overlapping `errcode` violation constants `too_short`, `too_long`, and `out_of_range`; prefer `min_length`, `max_length`, and `range`.

### Migration Notes

- Replace outer `HandleErrors(...)` installation with `Contract(...)` on the business-boundary route group or handler subtree when you want route-scoped configuration and started-response tracking. `WriteError(...)` itself can still be used without `Contract(...)`.
- Replace `if chix.Error(r, err) { return }` with `if chix.WriteError(w, r, err) { return }`.
- Replace `UseErrorMappers(...)` with `Contract(chix.WithContractErrorMappers(...))` for route-level defaults, or `WriteError(..., chix.WithErrorMappers(...))` for one-shot overrides.
- Import shared error-code constants from `github.com/kanata996/chix/errcode`.

### Docs and Examples

- Refreshed the README to make the business-boundary contract, `Contract(...)` usage, and explicit `WriteError(...)` paths clearer.
- Expanded the technical guide with request-flow, boundary, fail-closed, and error-handling-mode guidance for maintainers.
- Simplified the `chi` and `net/http` examples around the core `Contract(...)` + `WriteError(...)` flow.

### Performance

- Improved `reqx.DecodeQuery(...)` steady-state performance by caching query decode plans and unknown-field lookup state per destination struct type.

## [v0.2.1] - 2026-03-25

Historical note: this release also predates the current `Runtime` / `Scope` / `Handle(...)` model.

### Added

- Added helper constructors for common client-facing HTTP errors: `BadRequest(...)`, `Unauthorized(...)`, `Forbidden(...)`, `NotFound(...)`, `MethodNotAllowed(...)`, `Conflict(...)`, `Gone(...)`, `UnprocessableEntity(...)`, and `TooManyRequests(...)`.

### Changed

- Breaking: simplified the centralized error API from `Error(w, r, err)` to `Error(r, err)`. The response writer is no longer part of the public `Error(...)` signature; use `WriteError(...)` for one-shot immediate handling without middleware.
