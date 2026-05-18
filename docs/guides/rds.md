# Managed Databases (RDS)

The Cloud provides managed database instances, allowing you to launch PostgreSQL or MySQL containers with a single command. The system automatically handles credential generation, network isolation (when attached to a VPC), and port mapping.

## Overview

- **Engines Supported**: PostgreSQL, MySQL.
- **Isolation**: Supports attachment to a VPC for private networking.
- **Port Mapping**: Automatically assigns a dynamic host port if not in a VPC, or uses standard ports if internal.
- **Credentials**: Automatically generates a secure random password if not provided.
- **High Availability**: Supports primary-replica replication with automated failover.

## CLI Usage

### 1. Create a Database

To create a new PostgreSQL 16 instance:

```bash
cloud db create --name my-postgres --engine postgres --version 16 --size 20
```

| Flag | Description |
|------|-------------|
| `--size` | Storage in GB (default 10, minimum 10GB) |

Output:
```
[SUCCESS] Database my-postgres created successfully!
ID:       550e8400-e29b-41d4-a716-446655440000
Username: cloud_user
Password: generated-password-here (SAVE THIS!)
```

### 2. List Databases

```bash
cloud db list
```

### 3. Get Details & Connection String

To see detailed info:
```bash
cloud db show <id>
```

To get a ready-to-use connection string:
```bash
cloud db connection <id>
```

### 4. Credential Rotation 🆕

Regenerate the database password and update it in both the database engine and Vault automatically:

```bash
cloud db rotate-credentials <id>
```

### 5. High Availability & Replication

#### Create a Read Replica
Launch a new read-only replica from an existing primary:
```bash
cloud db create-replica --primary <primary-id> --name my-replica
```

#### Manual Promotion
Promote a replica to become a standalone primary:
```bash
cloud db promote <replica-id>
```

### 6. Delete a Database

```bash
cloud db rm <id>
```

## Internal Architecture

1. **Service Layer**: Handles Docker container lifecycle and credential generation.
2. **Docker Adapter**: Pulls the appropriate image (e.g., `postgres:16-alpine`) and launches it with environment variables for database setup.
3. **Replication**: Replicas are provisioned with engine-specific environment variables (`PRIMARY_HOST` for Postgres, `MYSQL_MASTER_HOST` for MySQL) to establish streaming replication.
4. **Failover Worker**: An automated background worker monitors Primary instances via TCP health checks. If a Primary fails, it automatically promotes the first available healthy replica.
5. **Storage**: Currently uses ephemeral storage within the container. (Future plan: Persistent Volume attachment).
6. **Networking**: Uses bridge network or VPC-defined network.

## Roadmap

- [ ] **Snapshots**: Backup and restore database state.
- [ ] **Volume Persistence**: Attach persistent volumes for data durability.
- [x] **Read Replicas**: Support for horizontal read scaling.
- [x] **Volume Expansion**: Dynamic storage scaling for existing instances.
- [ ] **Custom Config**: Support for custom `postgresql.conf` or `my.cnf`.
