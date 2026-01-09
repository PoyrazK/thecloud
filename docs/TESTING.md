# 🧪 Testing Guide

## Overview

The Cloud has comprehensive test coverage across all layers, ensuring reliability and maintainability of the codebase.

## Current Coverage Status

### Overall Project Coverage: **59.7%**

| Module | Coverage | Status |
|--------|----------|--------|
| **pkg/sdk** | 80.1% | ✅ Excellent |
| **internal/repositories/postgres** | 70.1% | ✅ Good |
| **pkg/audit** | 100% | ✅ Perfect |
| **pkg/ratelimit** | 100% | ✅ Perfect |
| **pkg/util** | 85.7% | ✅ Very Good |
| **pkg/crypto** | 75.8% | ✅ Good |
| **internal/core/services** | 71.5% | ✅ Good |
| **internal/handlers** | 65.8% | ✅ Acceptable |
| **pkg/httputil** | 65.3% | ✅ Acceptable |

## Running Tests

### Quick Commands

**Run all tests:**
```bash
go test ./...
```

**Run tests with coverage:**
```bash
go test -coverprofile=coverage.out ./...
```

**View coverage in browser:**
```bash
go tool cover -html=coverage.out
```

**View coverage summary:**
```bash
go tool cover -func=coverage.out | grep total
```

**Run tests for specific package:**
```bash
# Test SDK only
go test ./pkg/sdk/...

# Test repositories only
go test ./internal/repositories/postgres/...

# Test services only
go test ./internal/core/services/...
```

### Using Make Commands

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run benchmarks
make benchmark
```

## Test Organization

### 1. Unit Tests

Located alongside source code with `*_test.go` suffix.

**SDK Tests** (`pkg/sdk/*_test.go`):
- Test SDK client methods using httptest
- Mock HTTP responses for all API operations
- No external dependencies required
- Coverage: 80.1%

Example test structure:
```go
func TestClient_CreateQueue(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Mock server response
    }))
    defer server.Close()
    
    client := NewClient(server.URL, "test-api-key")
    result, err := client.CreateQueue(...)
    
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

**Service Tests** (`internal/core/services/*_test.go`):
- Use mock repositories and dependencies
- Test business logic in isolation
- Shared mocks defined in `shared_test.go`
- Coverage: 71.5%

Example test structure:
```go
func TestQueueService_CreateQueue(t *testing.T) {
    mockRepo := new(MockQueueRepository)
    mockEventSvc := new(MockEventService)
    svc := NewQueueService(mockRepo, mockEventSvc, ...)
    
    mockRepo.On("Create", ctx, mock.Anything).Return(nil)
    
    result, err := svc.CreateQueue(ctx, "test-queue", nil)
    
    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
}
```

**Handler Tests** (`internal/handlers/*_test.go`):
- Test HTTP handlers using httptest
- Mock service layer
- Validate HTTP responses and status codes
- Coverage: 65.8%

### 2. Integration Tests

**Repository Tests** (`internal/repositories/postgres/*_test.go`):
- Use pgxmock for database mocking
- Test SQL queries and data mapping
- No real database required for unit tests
- Coverage: 70.1%

Example test structure:
```go
func TestQueueRepository_Create(t *testing.T) {
    mock, err := pgxmock.NewPool()
    assert.NoError(t, err)
    defer mock.Close()
    
    repo := NewQueueRepository(mock)
    
    mock.ExpectExec("INSERT INTO queues").
        WithArgs(queue.ID, queue.Name, ...).
        WillReturnResult(pgxmock.NewResult("INSERT", 1))
    
    err = repo.Create(ctx, queue)
    assert.NoError(t, err)
}
```

### 3. Test Utilities

**Shared Mocks** (`internal/core/services/shared_test.go`):
- Centralized mock implementations
- Reusable across service tests
- Mock interfaces for:
  - Repositories (Queue, Cache, Instance, VPC, etc.)
  - Services (Event, Audit, Compute)
  - External dependencies (Docker, Libvirt)

**Test Helpers:**
```go
// Creating test context with user ID
ctx := appcontext.WithUserID(context.Background(), uuid.New())

// Setting up mock expectations
mock.On("MethodName", arg1, arg2).Return(result, nil)
mock.On("MethodName", mock.Anything).Return(result, nil)
mock.On("MethodName", mock.MatchedBy(func(x Type) bool {
    return x.Field == expectedValue
})).Return(result, nil)
```

## Writing New Tests

### Best Practices

1. **Test naming convention:**
   - Function: `TestFunctionName_Scenario`
   - Example: `TestCreateQueue_Success`, `TestCreateQueue_Unauthorized`

2. **Table-driven tests for multiple scenarios:**
```go
func TestCreateQueue(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"success", "valid-name", false},
        {"empty name", "", true},
        {"too long", strings.Repeat("a", 300), true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := CreateQueue(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.NotNil(t, result)
            }
        })
    }
}
```

3. **Always clean up mocks:**
```go
defer mockRepo.AssertExpectations(t)
defer server.Close()
```

4. **Test both success and error cases:**
   - Happy path (success scenarios)
   - Error handling (validation, not found, unauthorized, etc.)
   - Edge cases (boundary conditions, nil values, etc.)

5. **Use meaningful assertions:**
```go
// Good - specific assertions
assert.Equal(t, expected, actual)
assert.Contains(t, err.Error(), "expected message")

// Avoid - too generic
assert.NotNil(t, result)
```

## Coverage Goals

### Current Targets (Achieved ✅)
- **Overall Project**: 59.7% ✅ (Target: 55%+)
- **SDK**: 80.1% ✅ (Target: 70%+)
- **Repositories**: 70.1% ✅ (Target: 70%+)
- **Services**: 71.5% ✅ (Target: 65%+)
- **Handlers**: 65.8% ✅ (Target: 60%+)

### Future Targets
- **Overall Project**: 65%+
- **Critical Packages**: 80%+
- **All Core Logic**: 75%+

## Continuous Integration

Tests are automatically run on:
- Every push to any branch
- Pull requests
- Before deployments

See `.github/workflows/` for CI configuration.

### GitHub Actions Workflow

```yaml
- name: Run tests
  run: go test -race -coverprofile=coverage.out ./...

- name: Check coverage
  run: |
    coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
    echo "Coverage: $coverage%"
    if (( $(echo "$coverage < 55" | bc -l) )); then
      echo "Coverage is below 55%"
      exit 1
    fi
```

## Benchmarks

Run performance benchmarks:

```bash
# Run all benchmarks
go test -bench=. ./internal/core/services/

# Run specific benchmark
go test -bench=BenchmarkInstanceService ./internal/core/services/

# With memory allocation stats
go test -bench=. -benchmem ./internal/core/services/
```

## Debugging Tests

### Verbose Output

```bash
go test -v ./pkg/sdk/...
```

### Run Specific Test

```bash
go test -v -run TestCreateQueue ./pkg/sdk/
```

### Debug with Delve

```bash
dlv test ./pkg/sdk/ -- -test.run TestCreateQueue
```

### Print Debug Information

```go
func TestDebug(t *testing.T) {
    t.Logf("Debug info: %+v", someValue)
    // Test continues...
}
```

## Common Testing Patterns

### 1. HTTP Handler Testing

```go
func TestHandler(t *testing.T) {
    req := httptest.NewRequest(http.MethodPost, "/api/resource", body)
    w := httptest.NewRecorder()
    
    handler.ServeHTTP(w, req)
    
    assert.Equal(t, http.StatusOK, w.Code)
}
```

### 2. Context Testing

```go
func TestWithContext(t *testing.T) {
    userID := uuid.New()
    ctx := appcontext.WithUserID(context.Background(), userID)
    
    // Use ctx in test
}
```

### 3. Time-based Testing

```go
func TestTimeDependent(t *testing.T) {
    now := time.Now()
    // Mock time or use time.Now() consistently
    
    // For mocking time, use interface:
    mockClock := new(MockClock)
    mockClock.On("Now").Return(now)
}
```

## Troubleshooting

### Tests Failing Intermittently
- Check for race conditions: `go test -race`
- Ensure proper cleanup of resources
- Check for test interdependencies

### Mock Expectations Not Met
```go
// Add this to see what calls were made
defer mockRepo.AssertExpectations(t)

// Debug with:
mockRepo.AssertCalled(t, "MethodName", arg1, arg2)
```

### Coverage Not Updating
```bash
# Clear test cache
go clean -testcache

# Re-run with coverage
go test -coverprofile=coverage.out ./...
```

## Resources

- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Testify Documentation](https://github.com/stretchr/testify)
- [pgxmock Documentation](https://github.com/pashagolub/pgxmock)
- [httptest Documentation](https://golang.org/pkg/net/http/httptest/)

## Contributing

When adding new features:
1. Write tests first (TDD approach recommended)
2. Ensure coverage doesn't drop below current levels
3. Add both positive and negative test cases
4. Update this documentation if adding new testing patterns
