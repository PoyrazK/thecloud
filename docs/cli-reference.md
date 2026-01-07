# CLI Reference

Complete command-line reference for the `cloud` CLI tool.

## Installation

```bash
# Build from source
make build

# The binary will be in ./bin/cloud
./bin/cloud --help

# Add to PATH (optional)
export PATH=$PATH:$(pwd)/bin
```

## Global Flags

Available for all commands:

| Flag | Short | Description | Example |
|------|-------|-------------|---------|
| `--api-key` | `-k` | API key for authentication | `-k sk_abc123...` |
| `--json` | `-j` | Output in JSON format | `-j` |
| `--help` | `-h` | Show command help | `-h` |

## Configuration

The CLI stores configuration in `~/.cloud/config.yaml`:

```yaml
api_key: sk_abc123...
api_url: http://localhost:8080
```

---

## Authentication Commands

### `auth register`

Register a new user account.

```bash
cloud auth register
# Interactive prompts for email, password, name
```

**Flags**:
| Flag | Description |
|------|-------------|
| `--email` | Email address |
| `--password` | Password (min 8 chars) |
| `--name` | Display name |

**Example**:
```bash
cloud auth register --email user@example.com --password SecurePass123! --name "John Doe"
```

### `auth login`

Login and save API key.

```bash
cloud auth login
# Interactive prompts for email and password
```

**Example**:
```bash
cloud auth login --email user@example.com --password SecurePass123!
```

### `auth create-demo <name>`

Create a demo user and save API key (development only).

```bash
cloud auth create-demo my-user
```

---

## Instance Commands

Manage compute instances (containers or VMs).

### `instance list`

List all instances.

```bash
cloud instance list
cloud instance list --json  # JSON output
```

**Output**:
```
ID                                   NAME        IMAGE          STATUS    CREATED
a1b2c3d4-5678-90ab-cdef-1234567890ab my-server   nginx:alpine   running   2h ago
```

### `instance launch`

Launch a new compute instance.

```bash
cloud instance launch --name my-server --image nginx:alpine
```

**Flags**:
| Flag | Short | Required | Default | Description |
|------|-------|----------|---------|-------------|
| `--name` | `-n` | Yes | - | Instance name |
| `--image` | `-i` | No | `alpine` | Docker image or VM template |
| `--port` | `-p` | No | - | Port mapping (host:container) |
| `--vpc` | `-v` | No | - | VPC ID or name |
| `--volume` | `-V` | No | - | Volume attachment (vol-name:/path) |
| `--env` | `-e` | No | - | Environment variable (KEY=VALUE) |
| `--backend` | | No | `docker` | Backend (docker/libvirt) |

**Examples**:
```bash
# Basic instance
cloud instance launch --name web --image nginx:alpine

# With port mapping
cloud instance launch --name api --image node:20 --port 3000:3000

# With VPC and volume
cloud instance launch --name db --image postgres:16 \
  --vpc my-network \
  --volume db-data:/var/lib/postgresql/data

# With environment variables
cloud instance launch --name app --image myapp:latest \
  --env DATABASE_URL=postgres://... \
  --env API_KEY=secret

# Using KVM backend
cloud instance launch --name vm --image ubuntu-22.04 --backend libvirt
```

### `instance stop <id>`

Stop a running instance.

```bash
cloud instance stop my-server
cloud instance stop a1b2c3d4  # By ID
```

### `instance start <id>`

Start a stopped instance.

```bash
cloud instance start my-server
```

### `instance restart <id>`

Restart an instance.

```bash
cloud instance restart my-server
```

### `instance rm <id>`

Terminate and remove an instance.

```bash
cloud instance rm my-server
cloud instance rm my-server --force  # Skip confirmation
```

### `instance logs <id>`

View instance logs.

```bash
cloud instance logs my-server
cloud instance logs my-server --follow  # Stream logs
cloud instance logs my-server --tail 100  # Last 100 lines
```

### `instance show <id>`

Show detailed instance information.

```bash
cloud instance show my-server
```

**Output**:
```yaml
ID: a1b2c3d4-5678-90ab-cdef-1234567890ab
Name: my-server
Image: nginx:alpine
Status: running
Ports: 8080:80
VPC: my-network
Created: 2024-01-07 10:30:00
```

### `instance stats <id>`

Show real-time resource usage.

```bash
cloud instance stats my-server
```

**Output**:
```
CPU: 15.3%
Memory: 256MB / 512MB (50%)
Network: ↓ 1.2 MB/s ↑ 0.8 MB/s
```

---

## Volume Commands

Manage persistent block storage.

### `volume list`

List all volumes.

```bash
cloud volume list
```

### `volume create`

Create a new volume.

```bash
cloud volume create --name my-data --size 10
```

**Flags**:
| Flag | Required | Default | Description |
|------|----------|---------|-------------|
| `--name` | Yes | - | Volume name |
| `--size` | No | `1` | Size in GB |

### `volume attach <volume-id> <instance-id>`

Attach volume to instance.

```bash
cloud volume attach my-data my-server --mount /data
```

### `volume detach <volume-id>`

Detach volume from instance.

```bash
cloud volume detach my-data
```

### `volume snapshot <volume-id>`

Create a snapshot of a volume.

```bash
cloud volume snapshot my-data --name backup-2024-01-07
```

### `volume rm <id>`

Delete a volume.

```bash
cloud volume rm my-data
```

---

## VPC Commands

Manage Virtual Private Clouds (network isolation).

### `vpc list`

List all VPCs.

```bash
cloud vpc list
```

### `vpc create`

Create a new VPC.

```bash
cloud vpc create --name my-network --cidr 10.0.0.0/16
```

**Flags**:
| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | VPC name |
| `--cidr` | `10.0.0.0/16` | CIDR block |

### `vpc show <id>`

Show VPC details.

```bash
cloud vpc show my-network
```

### `vpc rm <id>`

Delete a VPC.

```bash
cloud vpc rm my-network
```

---

## Load Balancer Commands

Manage Layer 7 load balancers.

### `lb list`

List all load balancers.

```bash
cloud lb list
```

### `lb create`

Create a new load balancer.

```bash
cloud lb create --name my-lb --vpc my-network --port 8080
```

**Flags**:
| Flag | Required | Description |
|------|----------|-------------|
| `--name` | Yes | Load balancer name |
| `--vpc` | Yes | VPC ID or name |
| `--port` | Yes | Listener port |
| `--type` | No | Type (HTTP/TCP) |

### `lb add-target <lb-id> <instance-id>`

Register instance as target.

```bash
cloud lb add-target my-lb my-server --port 80
```

### `lb remove-target <lb-id> <instance-id>`

Deregister instance.

```bash
cloud lb remove-target my-lb my-server
```

### `lb show <id>`

Show load balancer details and health.

```bash
cloud lb show my-lb
```

### `lb rm <id>`

Delete a load balancer.

```bash
cloud lb rm my-lb
```

---

## Auto-Scaling Commands

Manage auto-scaling groups.

### `autoscaling list`

List all scaling groups.

```bash
cloud autoscaling list
```

### `autoscaling create`

Create a new auto-scaling group.

```bash
cloud autoscaling create \
  --name web-asg \
  --vpc my-network \
  --image nginx:alpine \
  --ports 80:80 \
  --min 1 --max 5 --desired 2
```

**Flags**:
| Flag | Required | Description |
|------|----------|-------------|
| `--name` | Yes | Group name |
| `--vpc` | Yes | VPC ID |
| `--image` | Yes | Docker image |
| `--ports` | No | Port mappings |
| `--min` | Yes | Minimum instances |
| `--max` | Yes | Maximum instances |
| `--desired` | Yes | Desired count |

### `autoscaling add-policy <id>`

Add a scaling policy.

```bash
cloud autoscaling add-policy web-asg \
  --name cpu-policy \
  --metric cpu \
  --threshold 70 \
  --adjustment 1
```

**Metrics**: `cpu`, `memory`, `requests`

### `autoscaling show <id>`

Show group details and instances.

```bash
cloud autoscaling show web-asg
```

### `autoscaling rm <id>`

Delete scaling group and terminate instances.

```bash
cloud autoscaling rm web-asg
```

---

## Storage Commands

Manage S3-compatible object storage.

### `storage upload <bucket> <file>`

Upload a file to object storage.

```bash
cloud storage upload my-bucket README.md
cloud storage upload my-bucket ./data.json --key custom-name.json
```

**Flags**:
| Flag | Description |
|------|-------------|
| `--key` | Custom object key (default: filename) |

### `storage list <bucket>`

List objects in a bucket.

```bash
cloud storage list my-bucket
```

### `storage download <bucket> <key> <dest>`

Download an object.

```bash
cloud storage download my-bucket file.txt ./local.txt
```

### `storage delete <bucket> <key>`

Delete an object.

```bash
cloud storage delete my-bucket file.txt
```

---

## Database Commands (RDS)

Manage managed database instances.

### `db list`

List all database instances.

```bash
cloud db list
```

### `db create`

Create a new managed database.

```bash
cloud db create --name my-db --engine postgres --version 16
```

**Flags**:
| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | Database name |
| `--engine` | `postgres` | Engine (postgres/mysql) |
| `--version` | `16` | Engine version |
| `--vpc` | - | VPC ID |
| `--storage` | `10` | Storage size in GB |

**Supported Engines**:
- PostgreSQL: `14`, `15`, `16`
- MySQL: `8.0`, `8.2`

### `db connection <id>`

Get database connection string.

```bash
cloud db connection my-db
```

**Output**:
```
postgresql://admin:password@db-host:5432/mydb
```

### `db show <id>`

Show database details.

```bash
cloud db show my-db
```

### `db rm <id>`

Delete a database instance.

```bash
cloud db rm my-db
```

---

## Cache Commands (Redis)

Manage managed Redis instances.

### `cache list`

List all cache instances.

```bash
cloud cache list
```

### `cache create`

Create a new Redis cache.

```bash
cloud cache create --name my-redis --memory 256
```

**Flags**:
| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | Cache name |
| `--version` | `7.2` | Redis version |
| `--memory` | `128` | Memory limit (MB) |
| `--vpc` | - | VPC ID |
| `--wait` | `false` | Wait for ready |

### `cache connection <id>`

Get Redis connection string.

```bash
cloud cache connection my-redis
```

### `cache stats <id>`

Show cache statistics.

```bash
cloud cache stats my-redis
```

### `cache flush <id>`

Flush all keys (dangerous!).

```bash
cloud cache flush my-redis --yes
```

### `cache rm <id>`

Delete a cache instance.

```bash
cloud cache rm my-redis
```

---

## Secrets Commands

Manage encrypted secrets.

### `secrets list`

List all secrets (values redacted).

```bash
cloud secrets list
```

### `secrets create`

Store a new encrypted secret.

```bash
cloud secrets create --name api-key --value sk_12345
cloud secrets create --name db-password --value "$(cat password.txt)"
```

### `secrets get <id>`

Decrypt and show secret value.

```bash
cloud secrets get api-key
```

### `secrets update <id>`

Update a secret value.

```bash
cloud secrets update api-key --value new_value
```

### `secrets rm <id>`

Delete a secret.

```bash
cloud secrets rm api-key
```

---

## Function Commands (Serverless)

Manage CloudFunctions.

### `function list`

List all functions.

```bash
cloud function list
```

### `function create`

Create a new function.

```bash
cloud function create \
  --name my-function \
  --runtime nodejs20 \
  --code ./function.zip \
  --handler index.handler
```

**Supported Runtimes**:
- `nodejs18`, `nodejs20`
- `python3.9`, `python3.10`, `python3.11`
- `go1.21`

### `function invoke <id>`

Invoke a function.

```bash
cloud function invoke my-function --payload '{"key":"value"}'
```

### `function logs <id>`

Get function execution logs.

```bash
cloud function logs my-function --limit 50
```

### `function rm <id>`

Delete a function.

```bash
cloud function rm my-function
```

---

## Queue Commands

Manage message queues.

### `queue create <name>`

Create a new queue.

```bash
cloud queue create my-queue
```

### `queue list`

List all queues.

```bash
cloud queue list
```

### `queue send <id> <message>`

Send a message to queue.

```bash
cloud queue send my-queue "Hello, World!"
```

### `queue receive <id>`

Receive messages from queue.

```bash
cloud queue receive my-queue --max 10
```

### `queue rm <id>`

Delete a queue.

```bash
cloud queue rm my-queue
```

---

## Notify Commands (Pub/Sub)

Manage topics and subscriptions.

### `notify create-topic <name>`

Create a notification topic.

```bash
cloud notify create-topic my-updates
```

### `notify list-topics`

List all topics.

```bash
cloud notify list-topics
```

### `notify subscribe <topic-id>`

Subscribe to a topic.

```bash
cloud notify subscribe my-updates \
  --protocol webhook \
  --endpoint https://example.com/hook
```

**Protocols**: `webhook`, `queue`

### `notify publish <topic-id> <message>`

Publish message to all subscribers.

```bash
cloud notify publish my-updates "System update complete"
```

### `notify rm-topic <id>`

Delete a topic.

```bash
cloud notify rm-topic my-updates
```

---

## Cron Commands (Scheduled Tasks)

Manage scheduled tasks.

### `cron create <name> <schedule> <url>`

Create a scheduled task.

```bash
cloud cron create cleanup "0 0 * * *" https://api.example.com/cleanup
```

**Schedule Format**: Standard cron syntax
- `* * * * *` - Every minute
- `0 * * * *` - Every hour
- `0 0 * * *` - Daily at midnight
- `0 0 * * 0` - Weekly on Sunday

### `cron list`

List all scheduled tasks.

```bash
cloud cron list
```

### `cron pause <id>`

Pause a task.

```bash
cloud cron pause cleanup
```

### `cron resume <id>`

Resume a paused task.

```bash
cloud cron resume cleanup
```

### `cron rm <id>`

Delete a scheduled task.

```bash
cloud cron rm cleanup
```

---

## Gateway Commands (API Gateway)

Manage API gateway routes.

### `gateway create-route <name> <prefix> <target>`

Create a new route.

```bash
cloud gateway create-route my-api /v1 http://my-instance:8080 --strip
```

**Flags**:
| Flag | Default | Description |
|------|---------|-------------|
| `--strip` | `true` | Strip prefix before forwarding |
| `--rate-limit` | `100` | Requests per second |

**Access**: Routes available at `http://api-host/gw/<prefix>/...`

### `gateway list-routes`

List all routes.

```bash
cloud gateway list-routes
```

### `gateway rm-route <id>`

Delete a route.

```bash
cloud gateway rm-route my-api
```

---

## Container Commands (Orchestration)

Manage container deployments with auto-healing.

### `container deploy <name> <image>`

Create a new deployment.

```bash
cloud container deploy my-web nginx:latest --replicas 3 --ports 80:80
```

**Flags**:
| Flag | Default | Description |
|------|---------|-------------|
| `--replicas` | `1` | Number of instances |
| `--ports` | - | Port mappings |
| `--env` | - | Environment variables |

### `container list`

List all deployments.

```bash
cloud container list
```

### `container scale <id> <replicas>`

Scale a deployment.

```bash
cloud container scale my-web 5
```

### `container rm <id>`

Delete deployment and all containers.

```bash
cloud container rm my-web
```

---

## RBAC Commands

Manage roles and permissions.

### `roles list`

List all roles.

```bash
cloud roles list
```

### `roles create <name>`

Create a new role.

```bash
cloud roles create developer --permissions "instance:read,instance:launch,volume:create"
```

**Available Permissions**:
- `instance:read`, `instance:launch`, `instance:stop`, `instance:terminate`
- `volume:read`, `volume:create`, `volume:delete`
- `vpc:read`, `vpc:create`, `vpc:delete`
- `full_access` - All permissions

### `roles bind <email> <role>`

Assign role to user.

```bash
cloud roles bind user@example.com developer
```

### `roles unbind <email> <role>`

Remove role from user.

```bash
cloud roles unbind user@example.com developer
```

### `roles list-bindings`

List all role bindings.

```bash
cloud roles list-bindings
```

### `roles delete <name>`

Delete a role.

```bash
cloud roles delete developer
```

---

## Events Commands

View system events and audit logs.

### `events list`

List recent events.

```bash
cloud events list
cloud events list --limit 100
cloud events list --type INSTANCE_LAUNCHED
```

---

## Tips & Tricks

### Using JSON Output

All list commands support `--json` for programmatic access:

```bash
cloud instance list --json | jq '.[] | select(.status=="running")'
```

### Environment Variables

Set default API key:

```bash
export CLOUD_API_KEY=sk_abc123...
cloud instance list  # No need for --api-key flag
```

### Batch Operations

```bash
# Stop all running instances
cloud instance list --json | jq -r '.[] | select(.status=="running") | .id' | \
  xargs -I {} cloud instance stop {}
```

### Aliases

Add to your shell config:

```bash
alias ci='cloud instance'
alias cv='cloud volume'
alias cdb='cloud db'
```

---

## Exit Codes

- `0` - Success
- `1` - General error
- `2` - Invalid arguments
- `3` - Authentication error
- `4` - Resource not found
- `5` - Permission denied

---

## Further Reading

- [Backend Guide](backend.md) - API implementation
- [Architecture Guide](architecture.md) - System design
- [Development Guide](development.md) - Setup and testing
