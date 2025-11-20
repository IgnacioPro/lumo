# Makefile for Lumo - Intelligent SRE/DevOps Automation Agent

# Variables
BINARY_NAME=lumo
MAIN_PATH=./cmd/lumo
GO=go
GOFLAGS=-v
INSTALL_PATH=$(shell go env GOPATH)/bin

# Build variables
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Colors for output
COLOR_RESET=\033[0m
COLOR_BOLD=\033[1m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m
COLOR_BLUE=\033[34m

.PHONY: help build run clean test test-verbose test-ci test-ssh fmt fmt-check vet lint install coverage coverage-report coverage-html deps check ci ci-lint ci-test ci-build all diagnose-local diagnose-local-json version proto proto-gen proto-clean proto-fmt proto-lint

# Default target
.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo "$(COLOR_BOLD)Lumo - Available Make Targets:$(COLOR_RESET)"
	@echo ""
	@grep -E '^##' $(MAKEFILE_LIST) | sed -e 's/^## /  /' -e 's/:/\t/'
	@echo ""

## build: Build the lumo binary
build:
	@echo "$(COLOR_BLUE)Building $(BINARY_NAME)...$(COLOR_RESET)"
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "$(COLOR_GREEN)✓ Build complete: $(BINARY_NAME)$(COLOR_RESET)"

## run: Run the application (use ARGS to pass arguments, e.g., make run ARGS="diagnose localhost")
run: build
	@echo "$(COLOR_BLUE)Running $(BINARY_NAME)...$(COLOR_RESET)"
	./$(BINARY_NAME) $(ARGS)

## clean: Remove build artifacts and temporary files
clean:
	@echo "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@$(GO) clean -cache -testcache
	@echo "$(COLOR_GREEN)✓ Clean complete$(COLOR_RESET)"

## test: Run all tests
test:
	@echo "$(COLOR_BLUE)Running tests...$(COLOR_RESET)"
	$(GO) test $(GOFLAGS) ./...
	@echo "$(COLOR_GREEN)✓ Tests complete$(COLOR_RESET)"

## test-verbose: Run tests with verbose output
test-verbose:
	@echo "$(COLOR_BLUE)Running tests (verbose)...$(COLOR_RESET)"
	$(GO) test -v ./...

## test-ci: Run tests as they run in CI (fast packages only)
test-ci:
	@echo "$(COLOR_BLUE)Running CI tests (fast packages only)...$(COLOR_RESET)"
	$(GO) test -timeout=2m -v ./internal/config ./internal/diagnostics/formatters
	@echo "$(COLOR_GREEN)✓ CI tests complete$(COLOR_RESET)"

## test-ssh: Run only SSH package tests
test-ssh:
	@echo "$(COLOR_BLUE)Running SSH package tests...$(COLOR_RESET)"
	$(GO) test -v ./internal/ssh/...
	@echo "$(COLOR_GREEN)✓ SSH tests complete$(COLOR_RESET)"

## coverage: Run tests with coverage report
coverage:
	@echo "$(COLOR_BLUE)Running tests with coverage...$(COLOR_RESET)"
	$(GO) test -cover ./...
	@echo "$(COLOR_GREEN)✓ Coverage report complete$(COLOR_RESET)"

## coverage-report: Generate detailed coverage report
coverage-report:
	@echo "$(COLOR_BLUE)Generating coverage report...$(COLOR_RESET)"
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out
	@echo "$(COLOR_GREEN)✓ Coverage report saved to coverage.out$(COLOR_RESET)"

## coverage-html: Generate HTML coverage report and open in browser
coverage-html:
	@echo "$(COLOR_BLUE)Generating HTML coverage report...$(COLOR_RESET)"
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(COLOR_GREEN)✓ HTML coverage report saved to coverage.html$(COLOR_RESET)"

## fmt: Format and simplify all Go source files
fmt:
	@echo "$(COLOR_BLUE)Formatting and simplifying code...$(COLOR_RESET)"
	gofmt -s -w .
	@echo "$(COLOR_GREEN)✓ Format complete$(COLOR_RESET)"

## fmt-check: Check if code is properly formatted and simplified (matches CI)
fmt-check:
	@echo "$(COLOR_BLUE)Checking code formatting and simplifications...$(COLOR_RESET)"
	@if [ -n "$$(gofmt -s -l .)" ]; then \
		echo "$(COLOR_RED)❌ Code is not formatted or simplified:$(COLOR_RESET)"; \
		gofmt -s -d .; \
		exit 1; \
	else \
		echo "$(COLOR_GREEN)✓ Code is properly formatted and simplified$(COLOR_RESET)"; \
	fi

## vet: Run go vet on all packages
vet:
	@echo "$(COLOR_BLUE)Running go vet...$(COLOR_RESET)"
	$(GO) vet ./...
	@echo "$(COLOR_GREEN)✓ Vet complete$(COLOR_RESET)"

## lint: Run golangci-lint (requires golangci-lint to be installed)
lint:
	@echo "$(COLOR_BLUE)Running linter...$(COLOR_RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
		echo "$(COLOR_GREEN)✓ Lint complete$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠ golangci-lint not installed. Skipping...$(COLOR_RESET)"; \
		echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## install: Install lumo binary to GOPATH/bin
install:
	@echo "$(COLOR_BLUE)Installing $(BINARY_NAME)...$(COLOR_RESET)"
	$(GO) install $(LDFLAGS) $(MAIN_PATH)
	@echo "$(COLOR_GREEN)✓ Installed to $(INSTALL_PATH)/$(BINARY_NAME)$(COLOR_RESET)"

## deps: Download and tidy dependencies
deps:
	@echo "$(COLOR_BLUE)Downloading dependencies...$(COLOR_RESET)"
	$(GO) mod download
	$(GO) mod tidy
	@echo "$(COLOR_GREEN)✓ Dependencies updated$(COLOR_RESET)"

## check: Run fmt-check, vet, and test
check: fmt-check vet test
	@echo "$(COLOR_GREEN)✓ All checks passed$(COLOR_RESET)"

## ci-lint: Run linters and security checks (used by GitHub CI)
ci-lint:
	@echo "$(COLOR_BLUE)Running golangci-lint...$(COLOR_RESET)"
	golangci-lint run --timeout=5m
	@echo "$(COLOR_BLUE)Running vulnerability check...$(COLOR_RESET)"
	govulncheck ./...
	@echo "$(COLOR_GREEN)✓ Lint and security checks passed$(COLOR_RESET)"

## ci-test: Run tests with race detection (used by GitHub CI)
ci-test:
	@echo "$(COLOR_BLUE)Running tests with race detector...$(COLOR_RESET)"
	$(GO) test -race -timeout=5m -short ./...
	@echo "$(COLOR_GREEN)✓ Tests passed$(COLOR_RESET)"

## ci-build: Build both CLI and Agent binaries (used by GitHub CI)
ci-build:
	@echo "$(COLOR_BLUE)Building CLI binary...$(COLOR_RESET)"
	$(GO) build -v ./cmd/lumo
	@echo "$(COLOR_BLUE)Building Agent binary...$(COLOR_RESET)"
	$(GO) build -v ./cmd/lumo-agent
	@echo "$(COLOR_GREEN)✓ Build complete$(COLOR_RESET)"

## ci: Run all CI checks (matches GitHub CI workflow)
ci: ci-lint ci-test ci-build
	@echo "$(COLOR_GREEN)✓ All CI checks passed$(COLOR_RESET)"

## all: Run check and build
all: check build
	@echo "$(COLOR_GREEN)✓ Build pipeline complete$(COLOR_RESET)"

## diagnose-local: Quick test - diagnose localhost
diagnose-local: build
	@echo "$(COLOR_BLUE)Running diagnostic on localhost...$(COLOR_RESET)"
	./$(BINARY_NAME) diagnose localhost

## diagnose-local-json: Quick test - diagnose localhost with JSON output
diagnose-local-json: build
	@echo "$(COLOR_BLUE)Running diagnostic on localhost (JSON)...$(COLOR_RESET)"
	./$(BINARY_NAME) diagnose localhost --format json

## version: Show version information
version:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"

# Protocol Buffer generation targets

## proto: Generate protobuf code (clean, generate, format)
proto: proto-clean proto-gen proto-fmt
	@echo "$(COLOR_GREEN)✓ Protobuf generation complete$(COLOR_RESET)"

## proto-gen: Generate Go code from .proto files
proto-gen:
	@echo "$(COLOR_BLUE)Generating protobuf code...$(COLOR_RESET)"
	@export PATH=$$PATH:/root/go/bin && protoc \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=. \
		--go-grpc_opt=paths=source_relative \
		api/proto/v1/*.proto
	@echo "$(COLOR_GREEN)✓ Protobuf code generated$(COLOR_RESET)"

## proto-clean: Remove generated protobuf files
proto-clean:
	@echo "$(COLOR_YELLOW)Cleaning generated protobuf files...$(COLOR_RESET)"
	@find api/proto -name "*.pb.go" -delete
	@echo "$(COLOR_GREEN)✓ Cleaned$(COLOR_RESET)"

## proto-fmt: Format proto files
proto-fmt:
	@echo "$(COLOR_BLUE)Formatting proto files...$(COLOR_RESET)"
	@find api/proto -name "*.proto" -exec clang-format -i {} \; 2>/dev/null || true
	@echo "$(COLOR_GREEN)✓ Formatted (or clang-format not installed)$(COLOR_RESET)"

## proto-lint: Lint proto files (requires buf)
proto-lint:
	@echo "$(COLOR_BLUE)Linting proto files...$(COLOR_RESET)"
	@if command -v buf >/dev/null 2>&1; then \
		buf lint api/proto; \
		echo "$(COLOR_GREEN)✓ Proto lint complete$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠ buf not installed. Skipping proto lint...$(COLOR_RESET)"; \
		echo "Install with: brew install buf or go install github.com/bufbuild/buf/cmd/buf@latest"; \
	fi
