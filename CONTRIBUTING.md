# Contributing to chix

Thanks for your interest in contributing.

## Development setup

1. Install Go (version from `go.mod`).
2. Clone the repo.
3. Run checks locally:

```bash
make ci
```

## Common commands

```bash
make test
make test-pkg PKG=./reqx
make test-name PKG=./reqx RUN=TestDecodeJSON
make test-cover
make test-race
```

## Pull requests

1. Keep PRs focused and small.
2. Add or update tests for behavior changes.
3. Ensure `make ci` passes before opening a PR.
4. Update docs when API behavior changes.

## Code style

- Follow standard Go formatting (`gofmt`).
- Prefer clear naming and small, testable functions.
- Keep package boundaries strict (`chix`, `reqx`, `errx`, `resp`).
