# Database Guide

This document covers the Data Layer of The Cloud.

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

### `vpcs` Table
Stores Virtual Private Cloud networks.
```sql
CREATE TABLE vpcs (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    cidr_block VARCHAR(18) NOT NULL,
    network_id VARCHAR(255) NOT NULL,
    gateway_id VARCHAR(255)
);
```

### `load_balancers` Table
```sql
CREATE TABLE load_balancers (
    id UUID PRIMARY KEY,
    vpc_id UUID REFERENCES vpcs(id) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) DEFAULT 'HTTP',
    status VARCHAR(50),
    listener_port INT NOT NULL,
    container_id VARCHAR(255)
);
```

### `scaling_groups` Table
```sql
CREATE TABLE scaling_groups (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    vpc_id UUID REFERENCES vpcs(id),
    min_instances INT NOT NULL,
    max_instances INT NOT NULL,
    desired_count INT NOT NULL,
    current_count INT NOT NULL DEFAULT 0,
    status VARCHAR(50) DEFAULT 'ACTIVE',
    failure_count INT DEFAULT 0,
    last_failure_at TIMESTAMPTZ
);
```

### `databases` Table
Stores managed database instance metadata.
```sql
CREATE TABLE databases (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vpc_id UUID REFERENCES vpcs(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    engine VARCHAR(50) NOT NULL, -- 'postgres', 'mysql'
    version VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    host VARCHAR(255),
    port INT,
    username VARCHAR(255),
    password TEXT, -- Plaintext for demo, should be encrypted
    container_id VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### `secrets` Table
Stores encrypted user secrets.
```sql
CREATE TABLE secrets (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    encrypted_value TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_accessed_at TIMESTAMPTZ,
    CONSTRAINT secrets_name_user_key UNIQUE (name, user_id)
);
```

### `metrics_history` Table
Stores time-series data for instances.
```sql
CREATE TABLE metrics_history (
    id UUID PRIMARY KEY,
    instance_id UUID NOT NULL,
    cpu_percent DOUBLE PRECISION,
    memory_usage_bytes BIGINT,
    recorded_at TIMESTAMPTZ DEFAULT NOW()
);
```

## Migration Strategy
- **Mechanism**: Embedded Go Filesystem (`embed`)
- **Location**: `internal/repositories/postgres/migrations/`
- **Execution**: Migrations run automatically on API startup.
- **CI/CD / Manual**: Use the `-migrate-only` flag to run migrations and exit:
    ```bash
    go run cmd/api/main.go -migrate-only
    ```

## Connection Details
The default connection string for local development is:
`postgres://cloud:cloud@localhost:5433/thecloud`

Note: The port was changed from `5432` to **`5433`** to avoid conflicts with system-level PostgreSQL installations.

## Schema Integrity
Every migration includes a `.up.sql` and a `.down.sql` file. We maintain a strict parity between them to ensure local environments can be reset cleanly for testing.

## Repository Pattern
We use interfaces to decouple database from business logic:
```go
type InstanceRepository interface {
    Create(ctx context.Context, i *domain.Instance) error
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error)
    // ...
}
```
