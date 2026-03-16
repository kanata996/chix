GO ?= go
PKG ?= ./...
RUN ?= .
COVER_FILE ?= coverage.out
VERSION ?=
RELEASE_REF ?= origin/main

.PHONY: help fmt fmt-check vet lint test test-pkg test-name test-cover test-race ci release-tag release-gh release

help:
	@echo "Targets:"
	@echo "  make test                      Run tests for all packages"
	@echo "  make test-pkg PKG=./reqx       Run tests for a specific package"
	@echo "  make test-name PKG=./reqx RUN=TestDecodeJSON"
	@echo "                                 Run a specific test by name"
	@echo "  make test-cover                Run tests with coverage"
	@echo "  make test-race                 Run tests with race detector"
	@echo "  make vet                       Run go vet"
	@echo "  make lint                      Run golangci-lint"
	@echo "  make fmt-check                 Check gofmt status"
	@echo "  make ci                        Run fmt-check, vet, test, lint"
	@echo "  make release-tag VERSION=vX.Y.Z"
	@echo "                                 Create and push an annotated tag from origin/main"
	@echo "  make release-gh VERSION=vX.Y.Z Create GitHub release notes for a tag"
	@echo "  make release VERSION=vX.Y.Z    Run release-tag and release-gh"

fmt:
	@$(GO) fmt ./...

fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "The following files need gofmt:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

vet:
	@$(GO) vet ./...

lint:
	@golangci-lint run ./...

test:
	@$(GO) test ./...

test-pkg:
	@$(GO) test $(PKG)

test-name:
	@$(GO) test $(PKG) -run $(RUN)

test-cover:
	@$(GO) test ./... -cover -coverprofile=$(COVER_FILE)
	@$(GO) tool cover -func=$(COVER_FILE)

test-race:
	@$(GO) test ./... -race

ci: fmt-check vet test lint

release-tag:
	@test -n "$(VERSION)" || (echo "Usage: make release-tag VERSION=vX.Y.Z"; exit 1)
	@case "$(VERSION)" in v*) ;; *) echo "VERSION must start with v (for example: v0.2.0)"; exit 1;; esac
	@if ! git diff --quiet || ! git diff --cached --quiet; then \
		echo "Working tree is not clean. Commit or stash changes before release."; \
		exit 1; \
	fi
	@git fetch origin main
	@if git rev-parse -q --verify "refs/tags/$(VERSION)" >/dev/null; then \
		echo "Tag $(VERSION) already exists locally."; \
		exit 1; \
	fi
	@if git ls-remote --tags --exit-code origin "refs/tags/$(VERSION)" >/dev/null 2>&1; then \
		echo "Tag $(VERSION) already exists on origin."; \
		exit 1; \
	fi
	@git tag -a "$(VERSION)" "$(RELEASE_REF)" -m "release $(VERSION)"
	@git push origin "$(VERSION)"
	@echo "Created and pushed tag $(VERSION) from $(RELEASE_REF)"

release-gh:
	@test -n "$(VERSION)" || (echo "Usage: make release-gh VERSION=vX.Y.Z"; exit 1)
	@gh release create "$(VERSION)" --generate-notes --target main
	@echo "Created GitHub release $(VERSION)"

release: release-tag release-gh
