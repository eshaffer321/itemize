# Walmart-Monarch Money Sync Backend

[![CI](https://github.com/eshaffer321/monarchmoney-walmart-server/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/eshaffer321/monarchmoney-walmart-server/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/eshaffer321/monarchmoney-walmart-server/graph/badge.svg)](https://codecov.io/gh/eshaffer321/monarchmoney-walmart-server)
[![Go Report Card](https://goreportcard.com/badge/github.com/eshaffer321/monarchmoney-walmart-server)](https://goreportcard.com/report/github.com/eshaffer321/monarchmoney-walmart-server)

A Go backend server that receives Walmart order data from a Chrome extension and syncs it with Monarch Money for intelligent transaction categorization and splitting.

## Overview

This backend is part of a larger system that automatically transforms single Walmart transactions in Monarch Money into properly categorized, split transactions that accurately reflect what was purchased.

**Example**: A $150 Walmart transaction becomes:
- $50 - Groceries (milk, bread, eggs)
- $30 - Home & Garden (cleaning supplies) 
- $40 - Electronics (phone charger, batteries)
- $30 - Personal Care (shampoo, toothpaste)

## Architecture

```
Chrome Extension → Go Backend → LLM API → Monarch Money API
     ↓                ↓            ↓            ↓
Scrape Orders    Process      Categorize    Split & Update
from Walmart      Orders        Items        Transactions
```

## Current Status

✅ **Phase 1 Implementation Complete**:
- Health check endpoint with monitoring
- Order reception from Chrome extension  
- Request authentication with X-Extension-Key
- Comprehensive order validation
- Sentry error tracking and monitoring
- 60%+ test coverage with TDD methodology
- golangci-lint passing with 0 issues
- Full CI/CD pipeline with GitHub Actions
- Cross-platform binary builds
- Security scanning integration

🚧 **Next: Monarch Money Integration**

## Quick Start

```bash
# Install dependencies and development tools
make deps
make install-tools

# Copy environment variables
cp .env.example .env
# Edit .env with your API keys

# Run tests (TDD workflow)
make test
# Or with coverage
make coverage

# Check code quality
make check  # Runs fmt, vet, lint, test

# Build and run
make build
make run
# Or for development with hot reload
make run-watch
```

## Development Workflow

### Available Make Commands

```bash
# Development
make help           # Show all available commands
make deps           # Download and tidy dependencies
make install-tools  # Install development tools (golangci-lint, etc.)

# Code Quality
make fmt           # Format code with gofmt and goimports
make fmt-check     # Check if code is formatted
make vet           # Run go vet
make lint          # Run golangci-lint
make check         # Run all checks (fmt, vet, lint, test)

# Testing
make test          # Run all tests
make test-short    # Run tests in short mode
make coverage      # Run tests with coverage report
make benchmark     # Run benchmarks

# Building
make build         # Build binary
make release       # Build release binaries for all platforms
make clean         # Clean build artifacts

# Running
make run           # Run the application
make run-watch     # Run with hot reload (requires air)

# Docker
make docker-build  # Build Docker image
make docker-run    # Run Docker container

# CI/CD
make pre-commit    # Run pre-commit checks
make ci            # Run full CI pipeline locally
```

### TDD Workflow

This project follows strict Test-Driven Development:

```bash
# 1. Write test first
# 2. Run test - watch it fail
make test

# 3. Implement feature
# 4. Run test - watch it pass
make test

# 5. Check all code quality
make check
```

## API Endpoints

- `GET /health` - Health check
- `POST /api/walmart/orders` - Receive Walmart orders (requires `X-Extension-Key` header)

See `/docs/api.md` for full API documentation.

## Testing

```bash
# Test health endpoint
curl http://localhost:8080/health

# Test order endpoint
curl -X POST http://localhost:8080/api/walmart/orders \
  -H "Content-Type: application/json" \
  -H "X-Extension-Key: test-secret" \
  -d @testdata/sample_order.json
```

## Documentation

- `/docs/progress.md` - Development progress tracking
- `/docs/api.md` - API documentation
- `/docs/testing.md` - Testing strategy
- `/docs/setup.md` - Setup instructions
- `/docs/bug-fixes.md` - Bug fix log

## Current Status

✅ Phase 1 MVP Complete:
- Health check endpoint
- Walmart order receive endpoint
- Authentication middleware
- Sentry error tracking integration
- Configuration management system
- 77.4% test coverage for handlers
- All tests passing (9/9)

## Next Steps

1. Integrate Monarch Money SDK (currently blocked - package unavailable)
2. Add transaction matching logic
3. Implement LLM categorization (Ollama for free local option)
4. Add transaction splitting functionality

## CI/CD Pipeline

### GitHub Actions

- **Continuous Integration**: Runs on every push/PR to main/develop
  - Linting with golangci-lint
  - Testing on multiple Go versions (1.21, 1.22, 1.23)
  - Security scanning with Gosec and Trivy
  - Docker image building
  - Coverage reporting to Codecov

- **Automated Releases**: Triggers on version tags (v*)
  - Cross-platform binary builds (Linux, macOS, Windows)
  - Docker image publishing to Docker Hub & GitHub Container Registry
  - GitHub release creation with assets

### Pre-commit Hooks

```bash
# Install pre-commit (requires Python)
pip install pre-commit

# Install hooks
pre-commit install

# Run manually
pre-commit run --all-files
```

## Tech Stack

- **Framework**: Gin
- **Error Tracking**: Sentry
- **Testing**: Testify
- **Code Quality**: golangci-lint, gosec, pre-commit
- **CI/CD**: GitHub Actions, Docker
- **Configuration**: godotenv + custom config package
- **Future**: Monarch SDK, Ollama/OpenAI, PostgreSQL, Redis