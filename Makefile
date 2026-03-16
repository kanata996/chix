GO ?= go
PKG ?= ./...
RUN ?= .
COVER_FILE ?= coverage.out

.PHONY: help fmt fmt-check vet lint test test-pkg test-name test-cover test-race ci

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
