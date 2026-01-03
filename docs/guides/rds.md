# Managed Databases (RDS)

The Cloud provides managed database instances, allowing you to launch PostgreSQL or MySQL containers with a single command. The system automatically handles credential generation, network isolation (when attached to a VPC), and port mapping.

## Overview

- **Engines Supported**: PostgreSQL, MySQL.
- **Isolation**: Supports attachment to a VPC for private networking.
- **Port Mapping**: Automatically assigns a dynamic host port if not in a VPC, or uses standard ports if internal.
- **Credentials**: Automatically generates a secure random password if not provided.

## CLI Usage

### 1. Create a Database

To create a new PostgreSQL 16 instance:

```bash
cloud db create --name my-postgres --engine postgres --version 16
```

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

### 4. View Logs

```bash
cloud db logs <id>
```

### 5. Delete a Database

```bash
cloud db rm <id>
```

## Internal Architecture

1. **Service Layer**: Handles Docker container lifecycle and credential generation.
2. **Docker Adapter**: Pulls the appropriate image (e.g., `postgres:16-alpine`) and launches it with environment variables for database setup.
3. **Storage**: Currently uses ephemeral storage within the container. (Future plan: Persistent Volume attachment).
4. **Networking**: Uses bridge network or VPC-defined network.

## Roadmap

- [ ] **Snapshots**: Backup and restore database state.
- [ ] **Volume Persistence**: Attach persistent volumes for data durability.
- [ ] **Read Replicas**: Support for horizontal read scaling.
- [ ] **Custom Config**: Support for custom `postgresql.conf` or `my.cnf`.
