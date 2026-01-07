# Cloud Infrastructure Guide

This document covers the infrastructure and DevOps aspects of The Cloud.

## The Docker Adapter (`internal/repositories/docker`)

The "Compute" service acts as a hypervisor. Instead of launching VMs (KVM/QEMU), it launches **Docker Containers** that act as instances.

### How it works
1.  **Pull**: Logic uses `client.ImagePull` to ensure the requested image (e.g., `nginx`) exists.
2.  **Create**: Uses `client.ContainerCreate`.
3.  **Start**: Uses `client.ContainerStart`.

### Current Limitations
- **Isolation**: No CPU/Memory limits are enforced yet for standard instances (though CloudFunctions do enforce them). This will be added in a future update.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | API Server Port | `:8080` |
| `DB_DSN` | Postgres Connection String | `host=localhost ...` |
| `GODAEMON` | (Internal) Docker Socket | `/var/run/docker.sock` |

## Deployment Strategy

### Docker Compose
We use `docker-compose.yml` to orchestrate the control plane.
- **Service**: `postgres` (State)
-- **Service**: `api` (HTTP server / control plane)

### Mounting the Socket
To let the `api` container launch *sibling* containers, we mount the host Docker socket:
```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
```
*Note: This is a security risk in production but acceptable for a local learning simulator.*

## Network Architecture (Planned)

```mermaid
graph LR
    API[Compute API] --(Control Plane Network)--> DB[(PostgreSQL)]
    
    subgraph "User Space (VPC)"
        App1[User Instance 1]
        App2[User Instance 2]
    end

    API -.->|Docker Socket| App1
    API -.->|Docker Socket| App2
```
