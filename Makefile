# Variables
BINARY_NAME=monarch-sync
GO_FILES=$(shell find . -name '*.go' -type f -not -path "./vendor/*")
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

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
NC=\033[0m # No Color

.PHONY: all build clean test coverage fmt vet deps help install-tools pre-commit release version run-costco

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
	$(GOBUILD) -v -o $(BINARY_NAME) ./cmd/monarch-sync
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

## pre-commit: Run pre-commit checks (fmt, vet, test)
pre-commit: fmt vet test
	@echo "$(GREEN)Pre-commit checks passed!$(NC)"

## release: Build release binaries for multiple platforms
release:
	@echo "$(GREEN)Building release binaries...$(NC)"
	@mkdir -p dist
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/monarch-sync
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/monarch-sync
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/monarch-sync
	GOOS=linux GOARCH=arm64 $(GOBUILD) -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/monarch-sync
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/monarch-sync
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
	$(GOCMD) run ./cmd/monarch-sync costco -dry-run -verbose

## install-tools: Install development tools (goimports)
install-tools:
	@echo "$(GREEN)Installing development tools...$(NC)"
	@echo "Installing goimports..."
	@go install golang.org/x/tools/cmd/goimports@latest
	@echo "$(GREEN)Tools installation complete!$(NC)"

# Default target
.DEFAULT_GOAL := help
