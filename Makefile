# Genie CLI Makefile

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT_HASH ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS = -X main.version=$(VERSION)

# Go variables
BINARY_NAME = genie
MAIN_PATH = ./cmd
BUILD_DIR = build

.PHONY: build clean test lint run install dev generate help release snapshot

# Default target
.DEFAULT_GOAL := help

generate: ## Generate code using Wire
	@echo "Generating code with Wire..."
	go generate ./...

build: generate ## Build the binary
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

dev: generate ## Build for development (fast build)
	@echo "Building $(BINARY_NAME) for development..."
	go build -o $(BINARY_NAME) $(MAIN_PATH)

test: ## Run tests
	go test ./...

test-race: ## Run tests with race detection
	go test -race ./...

test-coverage: ## Run tests with coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

run: ## Run the application
	go run $(MAIN_PATH)

deps: ## Install dependencies
	go mod download
	go mod tidy
	go install github.com/goreleaser/goreleaser@latest

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html

install: ## Install binary to $GOPATH/bin
	go install -ldflags "$(LDFLAGS)" $(MAIN_PATH)

lint: ## Run linter (if golangci-lint is installed)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

release: ## Release the application using GoReleaser
	@echo "Releasing $(BINARY_NAME) with GoReleaser..."
	goreleaser release --rm-dist

snapshot: ## Build a snapshot release using GoReleaser
	@echo "Building $(BINARY_NAME) snapshot with GoReleaser..."
	goreleaser release --snapshot --rm-dist

version: ## Show version info
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT_HASH)"
	@echo "Build Date: $(BUILD_DATE)"

help: ## Show help
	@echo "Available commands:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)