# Database Guide

This document covers the Data Layer of Mini AWS.

## Schema Design

### `instances` Table
Stores compute resource metadata.
```sql
CREATE TABLE instances (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    image VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    container_id VARCHAR(255),
    ports VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
```

### `api_keys` Table
Stores authentication keys.
```sql
CREATE TABLE api_keys (
    id UUID PRIMARY KEY,
    user_id UUID,
    key VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_used TIMESTAMPTZ
);
```

### `objects` Table
Stores object storage metadata (file bytes are on disk).
```sql
CREATE TABLE objects (
    id UUID PRIMARY KEY,
    arn VARCHAR(512) NOT NULL UNIQUE,
    bucket VARCHAR(255) NOT NULL,
    key VARCHAR(512) NOT NULL,
    size_bytes BIGINT NOT NULL,
    content_type VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    UNIQUE (bucket, key)
);
```

## Migration Strategy
- **Tool**: Manual SQL (golang-migrate planned)
- **Location**: `internal/repositories/postgres/migrations/`
- **Rules**:
  1. Never modify committed migrations
  2. Run in transactions
  3. Use `IF NOT EXISTS` for idempotency

## Repository Pattern
We use interfaces to decouple database from business logic:
```go
type InstanceRepository interface {
    Create(ctx context.Context, i *domain.Instance) error
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error)
    // ...
}
```
