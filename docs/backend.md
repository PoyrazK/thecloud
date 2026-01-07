# Backend Guide

This document explains the Go backend implementation for The Cloud platform.

## Project Structure

The codebase follows **Hexagonal Architecture** (Clean Architecture):

```text
internal/
├── core/
│   ├── context/       # Custom context utilities (UserID injection)
│   ├── domain/        # Pure data structs (Instance, VPC, Event, etc.)
│   ├── ports/         # Interface definitions (contracts)
│   └── services/      # Business logic (55.4% test coverage)
│       ├── *_test.go          # Unit tests
│       └── shared_test.go     # Mock definitions
├── handlers/          # HTTP Handlers (52.8% test coverage)
│   ├── *_handler.go           # REST endpoints
│   ├── *_test.go              # Handler tests
│   └── ws/                    # Real-time WebSocket Hub
├── repositories/      # Infrastructure Adapters
│   ├── docker/        # Docker SDK (Compute, Networks, Volumes)
│   ├── libvirt/       # KVM/QEMU virtualization
│   ├── postgres/      # PostgreSQL repositories (57.5% test coverage)
│   │   ├── *_repo.go          # Repository implementations
│   │   ├── *_test.go          # Integration tests
│   │   └── migrations/        # Database migrations
│   └── filesystem/    # Local file storage
├── platform/          # Cross-cutting concerns
│   ├── metrics.go     # Prometheus metrics
│   ├── logger.go      # Structured logging
│   └── database.go    # Database connection pooling
└── errors/            # Custom error types (100% test coverage)
```

## Architecture Principles

### 1. Dependency Inversion
- Core domain has no external dependencies
- Services depend on interfaces (ports), not implementations
- Repositories implement the interfaces defined in ports

### 2. Separation of Concerns
- **Domain**: Pure business entities
- **Services**: Business logic and orchestration
- **Handlers**: HTTP transport layer
- **Repositories**: Data persistence

### 3. Testability
- **51.3% overall test coverage** (Services: 55.4%, Handlers: 52.8%, Repos: 57.5%)
- Dependency injection enables easy mocking
- Unit tests run without external dependencies
- Integration tests verify real database interactions

## Key Components

### Services Layer

**Core Services:**
- `InstanceService` - Compute instance lifecycle management
- `VolumeService` - Block storage operations
- `VPCService` - Network isolation and management
- `LoadBalancerService` - Layer 7 traffic distribution
- `AutoScalingService` - Dynamic resource scaling
- `RBACService` - Role-based access control (100% coverage)
- `FunctionService` - Serverless execution
- `NotifyService` - Pub/Sub messaging
- `QueueService` - Message queue management
- `CronService` - Scheduled task execution

**Background Workers:**
- `AutoScalingWorker` - Evaluates policies and scales groups
- `ContainerWorker` - Monitors and heals container deployments
- `CronWorker` - Executes scheduled tasks
- `MetricCollector` - Collects instance statistics

### Handlers Layer

**REST API Endpoints:**
- `/instances` - Compute management
- `/volumes` - Block storage
- `/vpcs` - Networking
- `/loadbalancers` - Load balancing
- `/autoscaling` - Auto-scaling groups
- `/functions` - Serverless functions
- `/queues` - Message queues
- `/topics` - Pub/Sub topics
- `/cron` - Scheduled tasks
- `/rbac` - Role management

**WebSocket Endpoints:**
- `/ws/logs/:id` - Real-time log streaming
- `/ws/stats/:id` - Real-time metrics streaming
- `/ws/events` - System event notifications

### Repository Layer

**PostgreSQL Repositories:**
- User, Role, APIKey management
- Instance, Volume, VPC metadata
- Load Balancer, Auto-Scaling configuration
- Function, Queue, Topic state
- Audit logs and events

**Docker/Libvirt Adapters:**
- Container/VM lifecycle operations
- Network creation and management
- Volume attachment and snapshots
- Resource monitoring

## API Design Standards

### Authentication
All protected endpoints require `X-API-Key` header:
```bash
curl -H "X-API-Key: your-key-here" http://localhost:8080/instances
```

### JSON Response Format
Current format (v0.3.0):
```json
{
  "id": "uuid",
  "name": "instance-1",
  "status": "running"
}
```

Future format (v2.0):
```json
{
  "data": { "id": "uuid", "name": "instance-1" },
  "error": null,
  "meta": { "request_id": "..." }
}
```

### Context Usage
Every service and repository method accepts `context.Context` as the first argument:
```go
func (s *InstanceService) Launch(ctx context.Context, req LaunchRequest) (*domain.Instance, error)
```

This enables:
- Request cancellation
- Timeout handling
- User ID propagation
- Distributed tracing

### Error Handling
We use a custom error package (`internal/errors`) with:
- Error codes (NotFound, Unauthorized, Internal, etc.)
- Error wrapping for context
- Automatic HTTP status code mapping in handlers

Example:
```go
if instance == nil {
    return errors.New(errors.NotFound, "instance not found")
}
```

## Testing

### Test Coverage (51.3% Overall)

| Layer | Current | Target | Test Type |
|-------|---------|--------|-----------|
| Services | 55.4% | 80% | Unit Tests |
| Handlers | 52.8% | 80% | Unit Tests |
| Repositories | 57.5% | 75% | Integration Tests |
| Errors | 100% | 100% | Unit Tests |
| Context | 100% | 100% | Unit Tests |

### Test Organization

**Unit Tests** (`*_test.go` files):
```
internal/core/services/
├── function_test.go           # FunctionService tests
├── autoscaling_worker_test.go # AutoScaling worker tests
├── container_test.go          # ContainerService tests
├── cron_test.go               # CronService tests
├── rbac_test.go               # RBACService tests
├── notify_test.go             # NotifyService tests
└── shared_test.go             # Mock definitions
```

**Integration Tests** (require `//go:build integration` tag):
```
internal/repositories/postgres/
├── rbac_repo_test.go          # RBAC repository tests
├── function_repo_test.go      # Function repository tests
├── storage_repo_test.go       # Storage repository tests
├── secret_repo_test.go        # Secret repository tests
├── identity_repo_test.go      # Identity repository tests
└── helper_test.go             # Test utilities
```

### Running Tests

```bash
# Unit tests only (fast, no database)
go test ./...

# Integration tests (requires PostgreSQL)
docker compose up -d postgres
go test -tags=integration ./...

# Specific package
go test ./internal/core/services/...

# With coverage
go test -tags=integration -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Verbose output
go test -v ./internal/handlers/...
```

### Mocking Strategy

We use `testify/mock` for all mocks:

**Mock Definitions** (`shared_test.go`):
```go
type MockInstanceRepo struct {
    mock.Mock
}

func (m *MockInstanceRepo) Create(ctx context.Context, inst *domain.Instance) error {
    args := m.Called(ctx, inst)
    return args.Error(0)
}
```

**Using Mocks in Tests**:
```go
func TestInstanceService_Launch(t *testing.T) {
    repo := new(MockInstanceRepo)
    svc := services.NewInstanceService(repo)
    
    // Setup expectations
    repo.On("Create", ctx, mock.Anything).Return(nil).Once()
    
    // Execute
    _, err := svc.Launch(ctx, req)
    
    // Verify
    require.NoError(t, err)
    repo.AssertExpectations(t)
}
```

### Integration Test Pattern

```go
//go:build integration

func TestRBACRepository_Integration(t *testing.T) {
    db := setupDB(t)
    defer db.Close()
    repo := NewRBACRepository(db)
    ctx := setupTestUser(t, db)
    
    // Cleanup
    _, _ = db.Exec(ctx, "DELETE FROM roles")
    
    t.Run("CreateRole", func(t *testing.T) {
        role := &domain.Role{...}
        err := repo.CreateRole(ctx, role)
        require.NoError(t, err)
    })
}
```

## How to Add a New Feature

### 1. Define Domain Model
Add structs to `internal/core/domain/`:
```go
type MyResource struct {
    ID        uuid.UUID
    UserID    uuid.UUID
    Name      string
    Status    string
    CreatedAt time.Time
}
```

### 2. Define Port Interface
Add methods to `internal/core/ports/`:
```go
type MyResourceService interface {
    Create(ctx context.Context, req CreateRequest) (*domain.MyResource, error)
    Get(ctx context.Context, id uuid.UUID) (*domain.MyResource, error)
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### 3. Implement Service
Create `internal/core/services/myresource.go`:
```go
type myResourceService struct {
    repo ports.MyResourceRepository
}

func NewMyResourceService(repo ports.MyResourceRepository) ports.MyResourceService {
    return &myResourceService{repo: repo}
}

func (s *myResourceService) Create(ctx context.Context, req CreateRequest) (*domain.MyResource, error) {
    // Business logic here
    return s.repo.Create(ctx, resource)
}
```

### 4. Write Service Tests
Create `internal/core/services/myresource_test.go`:
```go
func TestMyResourceService_Create(t *testing.T) {
    repo := new(MockMyResourceRepo)
    svc := services.NewMyResourceService(repo)
    
    repo.On("Create", ctx, mock.Anything).Return(nil).Once()
    
    _, err := svc.Create(ctx, req)
    require.NoError(t, err)
    repo.AssertExpectations(t)
}
```

### 5. Implement Repository
Create `internal/repositories/postgres/myresource_repo.go`:
```go
type myResourceRepository struct {
    db *pgxpool.Pool
}

func (r *myResourceRepository) Create(ctx context.Context, res *domain.MyResource) error {
    query := `INSERT INTO my_resources (id, user_id, name) VALUES ($1, $2, $3)`
    _, err := r.db.Exec(ctx, query, res.ID, res.UserID, res.Name)
    return err
}
```

### 6. Write Integration Tests
Create `internal/repositories/postgres/myresource_repo_test.go`:
```go
//go:build integration

func TestMyResourceRepository_Integration(t *testing.T) {
    db := setupDB(t)
    defer db.Close()
    repo := NewMyResourceRepository(db)
    // ... tests
}
```

### 7. Create Handler
Create `internal/handlers/myresource_handler.go`:
```go
type MyResourceHandler struct {
    svc ports.MyResourceService
}

func (h *MyResourceHandler) Create(c *gin.Context) {
    var req CreateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        httputil.Error(c, errors.New(errors.InvalidInput, err.Error()))
        return
    }
    
    resource, err := h.svc.Create(c.Request.Context(), req)
    if err != nil {
        httputil.Error(c, err)
        return
    }
    
    httputil.Success(c, http.StatusCreated, resource)
}
```

### 8. Write Handler Tests
Create `internal/handlers/myresource_handler_test.go`:
```go
func TestMyResourceHandler_Create(t *testing.T) {
    gin.SetMode(gin.TestMode)
    svc := new(MockMyResourceService)
    handler := NewMyResourceHandler(svc)
    
    svc.On("Create", mock.Anything, mock.Anything).Return(&domain.MyResource{}, nil)
    
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    // ... test
}
```

### 9. Wire Up in Main
Update `cmd/api/main.go`:
```go
// Repository
myResourceRepo := postgres.NewMyResourceRepository(db)

// Service
myResourceSvc := services.NewMyResourceService(myResourceRepo)

// Handler
myResourceHandler := handlers.NewMyResourceHandler(myResourceSvc)

// Routes
api.POST("/myresources", myResourceHandler.Create)
api.GET("/myresources/:id", myResourceHandler.Get)
```

### 10. Add Database Migration
Create `internal/repositories/postgres/migrations/XXX_create_my_resources.sql`:
```sql
-- +goose Up
CREATE TABLE my_resources (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE my_resources;
```

## Observability

### Metrics (Prometheus)
All services expose metrics:
```go
platform.InstancesTotal.WithLabelValues("running").Inc()
platform.APIRequestDuration.WithLabelValues("/instances", "GET").Observe(duration)
```

View metrics: `http://localhost:8080/metrics`

### Logging (slog)
Structured logging throughout:
```go
logger.Info("instance launched", "id", inst.ID, "user", userID)
logger.Error("failed to create instance", "error", err)
```

### Audit Logging
All mutations are audited:
```go
auditSvc.Log(ctx, userID, "instance.launch", "instance", instID, metadata)
```

## Performance Considerations

- **Connection Pooling**: PostgreSQL uses pgxpool with configurable pool size
- **Context Timeouts**: All operations respect context deadlines
- **Lazy Loading**: Relationships loaded on-demand
- **Caching**: Redis integration for frequently accessed data
- **Metrics**: Prometheus for performance monitoring

## Security

- **API Key Authentication**: All endpoints protected
- **RBAC**: Fine-grained permission system
- **SQL Injection**: Parameterized queries only
- **Secrets**: Encrypted storage with rotation support
- **Audit Logs**: Complete audit trail

## Next Steps

- Increase test coverage to 80%
- Add E2E tests for critical flows
- Implement distributed tracing (OpenTelemetry)
- Add performance benchmarks
- Enhance error messages with request IDs
