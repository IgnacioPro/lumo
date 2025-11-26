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
LDFLAGS=-ldflags "-X github.com/ignacio/lumo/internal/version.Version=$(VERSION)"

# Colors for output
COLOR_RESET=\033[0m
COLOR_BOLD=\033[1m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m
COLOR_BLUE=\033[34m

.PHONY: help build run clean test test-verbose test-ci test-ssh fmt fmt-check vet lint install coverage coverage-report coverage-html deps check ci ci-lint ci-test ci-build all diagnose-local diagnose-local-json version proto proto-gen proto-clean proto-fmt proto-lint docker-build-cli docker-build-agent docker-build docker-push-cli docker-push-agent docker-push setup

# Default target
.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo "$(COLOR_BOLD)Lumo - Available Make Targets:$(COLOR_RESET)"
	@echo ""
	@grep -E '^##' $(MAKEFILE_LIST) | sed -e 's/^## /  /' -e 's/:/\t/'
	@echo ""

## build: Build both CLI and Agent binaries
build:
	@echo "$(COLOR_BLUE)Building $(BINARY_NAME)...$(COLOR_RESET)"
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "$(COLOR_GREEN)✓ Build complete: $(BINARY_NAME)$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)Building $(BINARY_NAME)-agent...$(COLOR_RESET)"
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME)-agent ./cmd/lumo-agent
	@echo "$(COLOR_GREEN)✓ Build complete: $(BINARY_NAME)-agent$(COLOR_RESET)"

## run: Run the application (use ARGS to pass arguments, e.g., make run ARGS="diagnose localhost")
run: build
	@echo "$(COLOR_BLUE)Running $(BINARY_NAME)...$(COLOR_RESET)"
	./$(BINARY_NAME) $(ARGS)

## clean: Remove build artifacts and temporary files
clean:
	@echo "$(COLOR_YELLOW)Cleaning build artifacts...$(COLOR_RESET)"
	@rm -f $(BINARY_NAME) $(BINARY_NAME)-agent
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

## setup: Set up development environment for new contributors
setup:
	@echo "$(COLOR_BOLD)Setting up Lumo development environment...$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BLUE)1. Downloading Go dependencies...$(COLOR_RESET)"
	@$(GO) mod download
	@echo "$(COLOR_GREEN)✓ Dependencies downloaded$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BLUE)2. Installing development tools...$(COLOR_RESET)"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest 2>/dev/null || echo "$(COLOR_YELLOW)⚠ golangci-lint install failed (optional)$(COLOR_RESET)"
	@go install golang.org/x/vuln/cmd/govulncheck@latest 2>/dev/null || echo "$(COLOR_YELLOW)⚠ govulncheck install failed (optional)$(COLOR_RESET)"
	@echo "$(COLOR_GREEN)✓ Tools installed$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_BLUE)3. Starting local services (PostgreSQL + Redis)...$(COLOR_RESET)"
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose up -d 2>/dev/null && echo "$(COLOR_GREEN)✓ Services started$(COLOR_RESET)"; \
	elif command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then \
		docker compose up -d 2>/dev/null && echo "$(COLOR_GREEN)✓ Services started$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_YELLOW)⚠ Docker not available - skipping services$(COLOR_RESET)"; \
	fi
	@echo ""
	@echo "$(COLOR_BLUE)4. Building binaries...$(COLOR_RESET)"
	@$(GO) build $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PATH)
	@$(GO) build $(LDFLAGS) -o $(BINARY_NAME)-agent ./cmd/lumo-agent
	@echo "$(COLOR_GREEN)✓ Binaries built$(COLOR_RESET)"
	@echo ""
	@echo "$(COLOR_GREEN)$(COLOR_BOLD)✓ Setup complete!$(COLOR_RESET)"
	@echo ""
	@echo "Next steps:"
	@echo "  - Run 'make ci' to verify everything works"
	@echo "  - Run './lumo doctor' to check configuration"
	@echo "  - See CONTRIBUTING.md for development workflow"
	@echo ""

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
	$(GO) build -v $(LDFLAGS) ./cmd/lumo
	@echo "$(COLOR_BLUE)Building Agent binary...$(COLOR_RESET)"
	$(GO) build -v $(LDFLAGS) ./cmd/lumo-agent
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

# Docker build targets

# Docker image name and tag configuration
DOCKER_REGISTRY?=
DOCKER_IMAGE_CLI?=lumo
DOCKER_IMAGE_AGENT?=lumo-agent
DOCKER_TAG?=$(VERSION)

## docker-build-cli: Build Docker image for lumo CLI
docker-build-cli:
	@echo "$(COLOR_BLUE)Building Docker image for lumo CLI...$(COLOR_RESET)"
	@docker build -t $(DOCKER_REGISTRY)$(DOCKER_IMAGE_CLI):$(DOCKER_TAG) -f Dockerfile .
	@docker tag $(DOCKER_REGISTRY)$(DOCKER_IMAGE_CLI):$(DOCKER_TAG) $(DOCKER_REGISTRY)$(DOCKER_IMAGE_CLI):latest
	@echo "$(COLOR_GREEN)✓ Built $(DOCKER_REGISTRY)$(DOCKER_IMAGE_CLI):$(DOCKER_TAG)$(COLOR_RESET)"

## docker-build-agent: Build Docker image for lumo-agent
docker-build-agent:
	@echo "$(COLOR_BLUE)Building Docker image for lumo-agent...$(COLOR_RESET)"
	@docker build -t $(DOCKER_REGISTRY)$(DOCKER_IMAGE_AGENT):$(DOCKER_TAG) -f Dockerfile.agent .
	@docker tag $(DOCKER_REGISTRY)$(DOCKER_IMAGE_AGENT):$(DOCKER_TAG) $(DOCKER_REGISTRY)$(DOCKER_IMAGE_AGENT):latest
	@echo "$(COLOR_GREEN)✓ Built $(DOCKER_REGISTRY)$(DOCKER_IMAGE_AGENT):$(DOCKER_TAG)$(COLOR_RESET)"

## docker-build: Build both CLI and agent Docker images
docker-build: docker-build-cli docker-build-agent
	@echo "$(COLOR_GREEN)✓ All Docker images built$(COLOR_RESET)"

## docker-push-cli: Push lumo CLI Docker image to registry
docker-push-cli:
	@echo "$(COLOR_BLUE)Pushing Docker image for lumo CLI...$(COLOR_RESET)"
	@docker push $(DOCKER_REGISTRY)$(DOCKER_IMAGE_CLI):$(DOCKER_TAG)
	@docker push $(DOCKER_REGISTRY)$(DOCKER_IMAGE_CLI):latest
	@echo "$(COLOR_GREEN)✓ Pushed $(DOCKER_REGISTRY)$(DOCKER_IMAGE_CLI):$(DOCKER_TAG)$(COLOR_RESET)"

## docker-push-agent: Push lumo-agent Docker image to registry
docker-push-agent:
	@echo "$(COLOR_BLUE)Pushing Docker image for lumo-agent...$(COLOR_RESET)"
	@docker push $(DOCKER_REGISTRY)$(DOCKER_IMAGE_AGENT):$(DOCKER_TAG)
	@docker push $(DOCKER_REGISTRY)$(DOCKER_IMAGE_AGENT):latest
	@echo "$(COLOR_GREEN)✓ Pushed $(DOCKER_REGISTRY)$(DOCKER_IMAGE_AGENT):$(DOCKER_TAG)$(COLOR_RESET)"

## docker-push: Push both CLI and agent Docker images to registry
docker-push: docker-push-cli docker-push-agent
	@echo "$(COLOR_GREEN)✓ All Docker images pushed$(COLOR_RESET)"
