# Variables
BINARY_NAME=itemize
GO_FILES=$(shell git ls-files '*.go')
STAGED_GO_FILES=$(shell git diff --cached --name-only --diff-filter=ACMR -- '*.go')
COVERAGE_FILE=coverage.out
COVERAGE_HTML=coverage.html

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Tool versions — the single source of truth for local runs AND CI (ci.yml calls these targets).
# `go run` builds the linter with this repo's Go toolchain, so it can never be older than go.mod.
GOLANGCI_LINT_VERSION=v2.14.0
GOLANGCI_LINT=$(GOCMD) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
NC=\033[0m # No Color

.PHONY: all build clean test coverage fmt check-fmt check-fmt-staged lint vet deps help install-tools install-hooks pre-commit pre-commit-local release version run-costco

## help: Display this help message
help:
	@echo "Available targets:"
	@grep -E '^##' Makefile | sed 's/## //'

## all: Run pre-commit checks and build
all: pre-commit build
	@echo "$(GREEN)Build complete!$(NC)"

## build: Build the CLI binary
build:
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	$(GOBUILD) -v -o $(BINARY_NAME) ./cmd/itemize
	@echo "$(GREEN)Build complete!$(NC)"

## clean: Remove build artifacts and temporary files
clean:
	@echo "$(YELLOW)Cleaning...$(NC)"
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(COVERAGE_FILE) $(COVERAGE_HTML)
	rm -rf dist/
	@echo "$(GREEN)Clean complete!$(NC)"

## test: Run all tests
test:
	@echo "$(GREEN)Running tests...$(NC)"
	$(GOTEST) -v -race -timeout 30s ./...
	@echo "$(GREEN)Tests complete!$(NC)"

## coverage: Run tests with coverage report
coverage:
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	$(GOTEST) -v -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	@echo "$(GREEN)Generating coverage report...$(NC)"
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "$(GREEN)Coverage report generated: $(COVERAGE_HTML)$(NC)"
	@echo "Coverage summary:"
	@$(GOCMD) tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print "Total coverage: " $$3}'

## fmt: Format code
fmt:
	@echo "$(GREEN)Formatting code...$(NC)"
	$(GOFMT) -s -w $(GO_FILES)
	@if command -v goimports &> /dev/null; then \
		goimports -w $(GO_FILES); \
	elif [ -f "$$(go env GOPATH)/bin/goimports" ]; then \
		$$(go env GOPATH)/bin/goimports -w $(GO_FILES); \
	else \
		echo "$(YELLOW)goimports not installed, skipping...$(NC)"; \
	fi
	@echo "$(GREEN)Formatting complete!$(NC)"

## check-fmt: Verify Go files are gofmt-formatted
check-fmt:
	@echo "$(GREEN)Checking Go formatting...$(NC)"
	@unformatted="$$(gofmt -s -l $(GO_FILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "$(RED)Go files need formatting:$(NC)"; \
		echo "$$unformatted"; \
		echo "Run: make fmt"; \
		exit 1; \
	fi
	@echo "$(GREEN)Formatting check passed!$(NC)"

## check-fmt-staged: Verify staged Go files are gofmt-formatted
check-fmt-staged:
	@echo "$(GREEN)Checking staged Go formatting...$(NC)"
	@if [ -n "$(STAGED_GO_FILES)" ]; then \
		unformatted="$$(gofmt -s -l $(STAGED_GO_FILES))"; \
		if [ -n "$$unformatted" ]; then \
			echo "$(RED)Staged Go files need formatting:$(NC)"; \
			echo "$$unformatted"; \
			echo "Run: gofmt -s -w $$unformatted"; \
			exit 1; \
		fi; \
	else \
		echo "$(YELLOW)No staged Go files to check.$(NC)"; \
	fi
	@echo "$(GREEN)Staged formatting check passed!$(NC)"

## lint: Run golangci-lint (pinned version, same as CI) using the repo config
lint:
	@echo "$(GREEN)Running golangci-lint $(GOLANGCI_LINT_VERSION)...$(NC)"
	$(GOLANGCI_LINT) run --timeout=5m
	@echo "$(GREEN)Lint complete!$(NC)"

## vet: Run go vet
vet:
	@echo "$(GREEN)Running go vet...$(NC)"
	$(GOVET) ./...
	@echo "$(GREEN)Vet complete!$(NC)"

## deps: Download and tidy dependencies
deps:
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	$(GOMOD) download
	@echo "$(GREEN)Tidying dependencies...$(NC)"
	$(GOMOD) tidy
	@echo "$(GREEN)Dependencies updated!$(NC)"

## pre-commit-local: Run local pre-commit checks without modifying files
pre-commit-local: check-fmt-staged lint vet
	@echo "$(GREEN)Running tests...$(NC)"
	$(GOTEST) ./...
	@echo "$(GREEN)Pre-commit checks passed!$(NC)"

## pre-commit: Run full pre-commit checks (format, lint, vet, race tests)
pre-commit: fmt lint vet test
	@echo "$(GREEN)Pre-commit checks passed!$(NC)"

## release: Build release binaries for multiple platforms
release:
	@echo "$(GREEN)Building release binaries...$(NC)"
	@mkdir -p dist
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/itemize
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/itemize
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/itemize
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/itemize
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/itemize
	@echo "$(GREEN)Release binaries built in dist/$(NC)"

## version: Display version information
version:
	@echo "$(GREEN)Version information:$(NC)"
	@go version
	@echo "Module: $$(go list -m)"
	@echo "Git commit: $$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
	@echo "Build date: $$(date -u +%Y-%m-%d_%H:%M:%S)"

## run-costco: Run Costco sync in dry-run mode (safe testing)
run-costco:
	@echo "$(GREEN)Running Costco sync (dry-run)...$(NC)"
	$(GOCMD) run ./cmd/itemize costco -dry-run -verbose

## install-hooks: Install versioned git hooks for this checkout
install-hooks:
	@echo "$(GREEN)Installing git hooks...$(NC)"
	@git config core.hooksPath .githooks
	@chmod +x .githooks/pre-commit scripts/pre-commit.sh
	@echo "$(GREEN)Git hooks installed. Use ITEMIZE_SKIP_PRECOMMIT=1 to bypass once.$(NC)"

## install-tools: Install development tools (goimports, golangci-lint)
install-tools:
	@echo "$(GREEN)Installing development tools...$(NC)"
	@echo "Installing goimports..."
	@go install golang.org/x/tools/cmd/goimports@latest
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION) (optional: make lint runs the pinned version itself)..."
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo "$(GREEN)Tools installation complete!$(NC)"

# Default target
.DEFAULT_GOAL := help
