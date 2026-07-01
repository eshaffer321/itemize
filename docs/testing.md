# Testing Strategy and Guidelines

## Test-Driven Development (TDD) Methodology

This project strictly follows TDD principles:

1. **Write the test first** - Define expected behavior
2. **Run test and watch it fail** - Verify test is actually testing something
3. **Write minimal code** - Just enough to make test pass
4. **Run test and watch it pass** - Verify implementation works
5. **Refactor** - Clean up while keeping tests green

## Test Coverage Goals
- Minimum 80% code coverage
- 100% coverage for critical paths (handlers, services)
- All edge cases covered

## Running Tests

### Run all tests
```bash
go test ./... -v
```

### Run specific package tests
```bash
go test ./handlers -v
go test ./models -v
go test ./services -v
```

### Run specific test
```bash
go test -run TestHealthCheck -v
go test -run TestReceiveOrders -v
```

### Check test coverage
```bash
go test ./... -cover
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out  # View in browser
```

### Check for race conditions
```bash
go test ./... -race
```

## Test Structure

### Naming Convention
- Test files: `<name>_test.go`
- Test functions: `Test<FunctionName>_<Scenario>`
- Example: `TestReceiveOrders_InvalidJSON`

### Test Organization
Each test follows the AAA pattern:
```go
func TestFunctionName_Scenario(t *testing.T) {
    // Arrange - Set up test data and dependencies
    
    // Act - Execute the function being tested
    
    // Assert - Verify the results
}
```

## Current Test Suite

### Handler Tests

#### Health Check (`handlers/health_test.go`)
- `TestHealthCheck` - Verifies health endpoint returns OK status

#### Walmart Orders (`handlers/walmart_test.go`)
- `TestReceiveOrders_Success` - Valid order processing
- `TestReceiveOrders_InvalidJSON` - Malformed JSON handling
- `TestReceiveOrders_MissingAuth` - Authentication validation
- `TestReceiveOrders_EmptyOrder` - Empty order validation
- `TestReceiveOrders_InvalidOrderTotal` - Negative total validation

## Testing Best Practices

### 1. Test Independence
- Each test must be independent
- No shared state between tests
- Use `t.Run()` for subtests when appropriate

### 2. Test Data
- Use realistic test data
- Store complex test data in `testdata/` directory
- Use table-driven tests for multiple scenarios

### 3. Mocking
- Mock external dependencies (APIs, databases)
- Use interfaces for easy mocking
- Use `github.com/stretchr/testify/mock` for complex mocks

### 4. Error Testing
- Test both success and failure paths
- Verify error messages are meaningful
- Test edge cases and boundary conditions

## Integration Testing (Future)

### Database Tests
- Use test database or in-memory database
- Clean up after each test
- Test transactions and rollbacks

### API Integration Tests
- Mock external APIs during unit tests
- Create separate integration test suite
- Use environment variables for test configuration

## Performance Testing (Future)

### Benchmarks
```go
func BenchmarkReceiveOrders(b *testing.B) {
    for i := 0; i < b.N; i++ {
        // Benchmark code
    }
}
```

Run benchmarks:
```bash
go test -bench=. -benchmem
```

## Continuous Integration

### Pre-commit Checks
1. Run all tests: `go test ./...`
2. Check coverage: `go test ./... -cover`
3. Run linter: `golangci-lint run`
4. Check formatting: `go fmt ./...`

### CI Pipeline (Future)
- Run tests on every push
- Block merge if tests fail
- Generate coverage reports
- Run security scanning

## Test Documentation

### Bug Fix Tests
When fixing a bug:
1. Write a test that reproduces the bug
2. Fix the bug
3. Document in `/docs/bug-fixes.md`:
   - Bug description
   - Test case that caught it
   - Fix applied
   - Date and commit reference

### Test Maintenance
- Review and update tests when requirements change
- Remove obsolete tests
- Keep test code clean and readable
- Document complex test scenarios

## Current Test Status

✅ **All tests passing** (6/6)
- Health check: 1 test passing
- Walmart orders: 5 tests passing
- Coverage: 100% for implemented handlers

## Next Testing Priorities

1. Add service layer tests
2. Add Monarch SDK integration tests (mocked)
3. Add configuration tests
4. Add main.go integration tests
5. Add performance benchmarks
6. Set up CI/CD pipeline