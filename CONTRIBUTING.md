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
make test-cover
make test-race
go test ./...
```

## Pull requests

1. Keep PRs focused and small.
2. Add or update tests for behavior changes.
3. Review test quality against the repository's public behavior:
   - root package `chix`: chi-oriented error responder preset and response entrypoints
   - `middleware`: request log correlation fields
   - `_example`: recommended integration path
4. Ensure `make ci` passes before opening a PR.
5. Update docs when API behavior changes.

## Release flow

1. Merge the release PR into `main`.
2. Update your local release branch:

```bash
git checkout main
git pull --ff-only origin main
```

3. Create and publish the tag plus GitHub release:

```bash
make release VERSION=vX.Y.Z
```

If you release from a branch other than `main`, override `MAIN_BRANCH`:

```bash
make release VERSION=vX.Y.Z MAIN_BRANCH=release-branch
```

If the tag already exists on `origin` and you only need the GitHub release entry:

```bash
make release-gh VERSION=vX.Y.Z
```

## Code style

- Follow standard Go formatting (`gofmt`).
- Prefer clear naming and small, testable functions.
- Keep package boundaries strict (`chix`, `middleware`, `_example`). Lower-level net/http boundary capabilities live in `hah`.
