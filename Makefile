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

.PHONY: help build run clean test test-verbose test-ci test-ssh fmt fmt-check vet lint install coverage coverage-report coverage-html deps check all diagnose-local diagnose-local-json version

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

## fmt: Format all Go source files
fmt:
	@echo "$(COLOR_BLUE)Formatting code...$(COLOR_RESET)"
	$(GO) fmt ./...
	@echo "$(COLOR_GREEN)✓ Format complete$(COLOR_RESET)"

## fmt-check: Check if code is properly formatted (matches CI)
fmt-check:
	@echo "$(COLOR_BLUE)Checking code formatting...$(COLOR_RESET)"
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "$(COLOR_RED)❌ Code is not formatted:$(COLOR_RESET)"; \
		gofmt -d .; \
		exit 1; \
	else \
		echo "$(COLOR_GREEN)✓ Code is properly formatted$(COLOR_RESET)"; \
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

## check: Run fmt, vet, and test
check: fmt vet test
	@echo "$(COLOR_GREEN)✓ All checks passed$(COLOR_RESET)"

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
