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
| `--api-key` | `-k` | API key for authentication | `-k thecloud_abc123...` |
| `--api-url` | | API server URL | `--api-url http://localhost:8080` |
| `--tenant` | | Tenant ID to use for requests | `--tenant uuid` |
| `--output` | `-o` | Output format (table, json, yaml) | `--output json` |
| `--json` | `-j` | Output in JSON format (deprecated, use --output=json) | `-j` |
| `--debug` | `-d` | Enable debug mode | `-d` |
| `--help` | `-h` | Show command help | `-h` |

## Configuration

The CLI stores configuration in `~/.cloud/config.json`:

```json
{
  "api_key": "thecloud_xxxxx",
  "api_url": "http://localhost:8080",
  "output": "table",
  "tenant": "",
  "debug": false
}
```

### `config` Command

Manage CLI configuration:

```bash
# Show current configuration
cloud config show

# Set a config value
cloud config set api-url http://localhost:8080
cloud config set output table

# Unset a config value
cloud config unset tenant
```

---

## Pagination

List commands support `--limit` and `--offset` flags for pagination:

```bash
# Get first 10 results
cloud instance list --limit 10

# Get results with offset
cloud instance list --limit 10 --offset 10

# Works with all list commands
cloud function list --limit 20
cloud container list --limit 50 --offset 100
cloud storage list --limit 25
```

When paginated results are available, the CLI shows:
```
Showing 10 of 50 total (more available)
```

---

## Shell Completion

Generate shell completion scripts:

```bash
# Bash
cloud completion bash > /etc/bash/completion.d/cloud

# Zsh
cloud completion zsh > ~/.zsh/completions/_cloud

# Fish
cloud completion fish > ~/.config/fish/completions/cloud.fish
```

---

## Authentication Commands

### `auth register`

Register a new user account.

```bash
cloud auth register <email> <password> <name>
```

### `auth login-user`

Login with email and password to receive and save an API key.

```bash
cloud auth login-user <email> <password>
```

### `auth whoami`

Show current session information (user ID, email, role, default tenant).

```bash
cloud auth whoami
```

### `auth list-keys`

List your active API keys.

```bash
cloud auth list-keys
```

### `auth revoke-key <id>`

Revoke an API key.

```bash
cloud auth revoke-key <id>
```

### `auth rotate-key <id>`

Rotate an API key (replaces it with a new one).

```bash
cloud auth rotate-key <id>
```

### `auth create-demo <name>`

Create a demo user and save API key (development only).

```bash
cloud auth create-demo my-user
```

---

## Tenant Commands 🆕

### `tenant list`

List all organizations (tenants) you belong to.

```bash
cloud tenant list
```

### `tenant create <name> <slug>`

Create a new organization.

```bash
cloud tenant create "My Org" my-org
```

### `tenant switch <id>`

Switch your default tenant for future CLI operations.

```bash
cloud tenant switch <id>
```

---

## Instance Commands

Manage compute instances (containers or VMs).

### `instance list`

List all instances in the current tenant.

```bash
cloud instance list
cloud instance list --json  # JSON output
cloud instance list --limit 10 --offset 0  # Pagination
```

**Flags**:
| Flag | Default | Description |
|------|---------|-------------|
| `--limit` | `0` | Maximum number of results (0 = use server default) |
| `--offset` | `0` | Number of results to skip |

**Standard Output**:
```
ID        NAME        IMAGE          STATUS    ACCESS
a1b2c3d4  my-server   nginx:alpine   RUNNING   localhost:8080->80
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
| `--type` | `-t` | No | `basic-2` | Instance type (e.g., basic-1, standard-1) |
| `--port` | `-p` | No | - | Port mapping (host:container) |
| `--vpc` | `-v` | No | - | VPC ID or name |
| `--subnet` | `-s` | No | - | Subnet ID or name |
| `--volume` | `-V` | No | - | Volume attachment (vol-id:/path) |
| `--ssh-key` | | No | - | SSH Key ID to inject |
| `--wait` | `-w` | No | `false` | Wait for instance to reach RUNNING state |

**Examples**:
```bash
# Basic instance
cloud instance launch --name web --image nginx:alpine --type basic-2 --wait

# With port mapping
cloud instance launch --name api --image node:20 --port 3000:3000 --type standard-1

# With VPC and Subnet
cloud instance launch --name db --image postgres:16 \
  --vpc vpc-uuid \
  --subnet subnet-uuid
```

### `instance stop <id>`

Stop a running instance.

```bash
cloud instance stop my-server
```

### `instance logs <id>`

View real-time instance logs.

```bash
cloud instance logs my-server
```

### `instance show <id>`

Show detailed instance information.

```bash
cloud instance show my-server
```

### `instance rm <id>`

Terminate and remove an instance. (Alias: `delete`)

```bash
cloud instance rm my-server
```

### `instance resize <id> <instance-type>`

Resize an instance to a new instance type.

```bash
cloud instance resize my-server basic-4
```

**Examples**:
```bash
# Resize to a larger instance type
cloud instance resize my-server standard-2

# Resize to a smaller instance type
cloud instance resize my-server basic-1
```

### `instance stats <id>`

Show real-time resource usage.

```bash
cloud instance stats my-server
```

---

## Container Commands

Manage container deployments.

### `container list`

List all container deployments.

```bash
cloud container list
cloud container list --limit 10 --offset 0  # Pagination
```

**Flags**:
| Flag | Default | Description |
|------|---------|-------------|
| `--limit` | `0` | Maximum number of results (0 = use server default) |
| `--offset` | `0` | Number of results to skip |

### `container deploy <name> <image>`

Create a new container deployment.

```bash
cloud container deploy my-app nginx:latest --replicas 3 --ports 80:80
```

**Flags**:
| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--replicas` | `-r` | `1` | Number of replicas |
| `--ports` | `-p` | - | Ports to expose (e.g., 80:80) |

### `container scale <id> <replicas>`

Scale a deployment.

```bash
cloud container scale dep-uuid 5
```

### `container rm <id>`

Delete a container deployment.

```bash
cloud container rm dep-uuid
```

---

## SSH Key Commands 🆕

### `ssh-key register <name> <file>`

Register a public SSH key for use with instances.

```bash
cloud ssh-key register my-laptop ~/.ssh/id_rsa.pub
```

### `ssh-key list`

List all registered SSH keys.

```bash
cloud ssh-key list
cloud ssh-key list --limit 10 --offset 0  # Pagination
```

**Flags**:
| Flag | Default | Description |
|------|---------|-------------|
| `--limit` | `0` | Maximum number of results (0 = use server default) |
| `--offset` | `0` | Number of results to skip |

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
| Flag | Short | Required | Default | Description |
|------|-------|----------|---------|-------------|
| `--name` | `-n` | Yes | - | Volume name |
| `--size` | `-s` | No | `1` | Size in GB |

### `volume rm <id>`

Delete a volume. (Alias: `delete`)

```bash
cloud volume rm my-data
```

### `volume resize <id/name> <sizeGB>`

Resize a volume to a larger size.

```bash
cloud volume resize my-data 50
```

---

## Snapshot Commands 🆕

Manage volume snapshots.

### `snapshot list`

List all snapshots.

```bash
cloud snapshot list
```

### `snapshot create <volume-id>`

Create a snapshot from a volume.

```bash
cloud snapshot create vol-uuid --desc "Backup before upgrade"
```

### `snapshot restore <snapshot-id>`

Restore a snapshot to a new volume.

```bash
cloud snapshot restore snap-uuid --name restored-vol
```

### `snapshot rm <id>`

Delete a snapshot. (Alias: `delete`)

```bash
cloud snapshot rm snap-uuid
```

---

## VPC Commands

Manage Virtual Private Clouds (network isolation).

### `vpc list`

List all VPCs.

```bash
cloud vpc list
```

### `vpc create <name>`

Create a new VPC.

```bash
cloud vpc create my-network --cidr-block 10.0.0.0/16
```

### `vpc show <id>`

Show VPC details.

```bash
cloud vpc show my-network
```

### `vpc rm <id>`

Delete a VPC. (Alias: `delete`)

```bash
cloud vpc rm my-network
```
 
 ---
 
 ## VPC Peering Commands 🆕
 
 Manage private connectivity between VPCs.
 
 ### `vpc-peering list`
 
 List all VPC peering connections.
 
 ```bash
 cloud vpc-peering list
 ```
 
 ### `vpc-peering create`
 
 Initiate a peering request between two VPCs.
 
 ```bash
 cloud vpc-peering create --requester-vpc <vpc1-id> --accepter-vpc <vpc2-id>
 ```
 
 ### `vpc-peering accept <id>`
 
 Accept a pending peering request.
 
 ```bash
 cloud vpc-peering accept peering-uuid
 ```
 
 ### `vpc-peering reject <id>`
 
 Reject a pending peering request.
 
 ```bash
 cloud vpc-peering reject peering-uuid
 ```
 
 ### `vpc-peering rm <id>`
 
 Delete a peering connection. (Alias: `delete`)
 
 ```bash
 cloud vpc-peering rm peering-uuid
```

---

## Route Table Commands 🆕

Manage VPC route tables.

### `route-table list [vpc-id]`

List all route tables in a VPC.

```bash
cloud route-table list vpc-uuid
```

### `route-table create [vpc-id] [name]`

Create a new route table in a VPC.

```bash
cloud route-table create vpc-uuid my-rt --main
```

**Flags:**
| Flag | Short | Required | Default | Description |
|------|-------|----------|---------|-------------|
| `--main` | | No | false | Set as the main route table for the VPC |

### `route-table rm <id>`

Delete a route table. Cannot delete the main route table.

```bash
cloud route-table rm rt-uuid
```

### `route-table add-route <rt-id> [dest-cidr] [target-type]`

Add a route to a route table.

```bash
cloud route-table add-route rt-uuid 0.0.0.0/0 igw --target-id igw-uuid
cloud route-table add-route rt-uuid 10.0.1.0/24 nat --target-id nat-uuid
cloud route-table add-route rt-uuid 10.0.2.0/24 peering --target-id peering-uuid
```

**Target Types:** `local`, `igw`, `nat`, `peering`, `instance`

### `route-table remove-route <rt-id> <route-id>`

Remove a route from a route table.

```bash
cloud route-table remove-route rt-uuid route-uuid
```

### `route-table associate <rt-id> <subnet-id>`

Associate a subnet with a route table.

```bash
cloud route-table associate rt-uuid subnet-uuid
```

### `route-table disassociate <rt-id> <subnet-id>`

Disassociate a subnet from a route table.

```bash
cloud route-table disassociate rt-uuid subnet-uuid
```

---

## Internet Gateway Commands 🆕

Manage internet gateways for VPC routing.

### `igw create`

Create a new internet gateway.

```bash
cloud igw create
```

### `igw attach <igw-id> <vpc-id>`

Attach an internet gateway to a VPC. This allows instances in private subnets to access the internet via the IGW.

```bash
cloud igw attach igw-uuid vpc-uuid
```

### `igw detach <igw-id>`

Detach an internet gateway from its VPC. Must remove all routes referencing the IGW first.

```bash
cloud igw detach igw-uuid
```

### `igw list`

List all internet gateways.

```bash
cloud igw list
```

### `igw rm <id>`

Delete an internet gateway. Must be detached first.

```bash
cloud igw rm igw-uuid
```

---

## NAT Gateway Commands 🆕

Manage NAT gateways for private subnet internet access.

### `nat-gateway create [subnet-id] [eip-id]`

Create a NAT gateway in a public subnet with an allocated elastic IP.

```bash
cloud nat-gateway create subnet-uuid eip-uuid
```

**Flags:**
| Flag | Short | Required | Default | Description |
|------|-------|----------|---------|-------------|
| `--name` | `-n` | No | - | NAT gateway name |

### `nat-gateway list [vpc-id]`

List all NAT gateways, optionally filtered by VPC.

```bash
cloud nat-gateway list
cloud nat-gateway list vpc-uuid
```

### `nat-gateway rm <id>`

Delete a NAT gateway and release the associated elastic IP.

```bash
cloud nat-gateway rm nat-uuid
```

---

## Subnet Commands

Manage VPC subnets.

### `subnet list <vpc-id>`

List all subnets in a VPC.

```bash
cloud subnet list vpc-uuid
```

### `subnet create <vpc-id> <name> <cidr>`

Create a new subnet in a VPC.

```bash
cloud subnet create vpc-uuid my-private-subnet 10.0.1.0/24 --az us-east-1a
```

### `subnet rm <id>`

Delete a subnet. (Alias: `delete`)

```bash
cloud subnet rm subnet-uuid
```

---

## Security Group Commands

### `sg list`

List all security groups in a VPC.

```bash
cloud sg list --vpc-id <vpc-id>
```

### `sg get <sg-id>`

Get details and rules for a security group.

```bash
cloud sg get sg-uuid
```

### `sg create <name>`

Create a new security group.

```bash
cloud sg create my-sg --vpc-id vpc-uuid --description "Web servers"
```

### `sg rm <sg-id>`

Delete a security group. (Alias: `delete`)

```bash
cloud sg rm sg-uuid
```

### `sg add-rule <sg-id>`

Add a rule to a security group.

```bash
cloud sg add-rule sg-uuid --direction ingress --protocol tcp --port-min 80 --port-max 80 --cidr 0.0.0.0/0
```

---

## Storage Commands

Manage distributed object storage.

### `storage mb <name>`

Make bucket (Create a new bucket). (Alias: `create-bucket`)

```bash
cloud storage mb my-bucket --public
```

### `storage rb <name>`

Remove bucket. (Alias: `delete-bucket`)

```bash
cloud storage rb my-bucket --force
```

### `storage upload <bucket> <file>`

Upload a file to object storage.

```bash
cloud storage upload my-bucket ./local-file.txt --key remote-key.txt
```

### `storage list [bucket]`

List all buckets, or objects within a specific bucket.

```bash
cloud storage list
cloud storage list my-bucket
cloud storage list --limit 20 --offset 0  # Pagination (buckets only)
```

**Flags** (for bucket listing):
| Flag | Default | Description |
|------|---------|-------------|
| `--limit` | `0` | Maximum number of results (0 = use server default) |
| `--offset` | `0` | Number of results to skip |

### `storage download <bucket> <key> <dest>`

Download an object.

```bash
cloud storage download my-bucket remote-key.txt ./local-file.txt
```

### `storage rm <bucket> <key>`

Delete an object. (Alias: `delete`)

```bash
cloud storage rm my-bucket my-file.txt
```

### `storage lifecycle`

Manage bucket lifecycle rules (expiration).

```bash
# List rules
cloud storage lifecycle list my-bucket

# Set expiration rule
cloud storage lifecycle set my-bucket --prefix logs/ --days 30

# Remove rule
cloud storage lifecycle rm my-bucket rule-uuid
```

---

## IAM Commands (Policies) 🆕

Manage granular IAM policies.

### `iam list`

List all platform IAM policies.

```bash
cloud iam list
```

### `iam create <name> <json-file>`

Create a new policy from a JSON file.

```bash
cloud iam create ReadOnlyS3 ./policy.json
```

### `iam attach <user-id> <policy-id>`

Attach a policy to a user.

```bash
cloud iam attach user-uuid policy-uuid
```

### `iam detach <user-id> <policy-id>`

Detach a policy from a user.

```bash
cloud iam detach user-uuid policy-uuid
```

---

## Audit & Billing Commands 🆕

### `audit list`

List recent platform audit logs.

```bash
cloud audit list --limit 20
```

### `billing summary`

Get billing summary for the current period.

```bash
cloud billing summary
```

### `billing usage`

List detailed usage records.

```bash
cloud billing usage
```

---

## Database Commands (RDS)

Manage managed databases.

### `db list`

List all database instances.

```bash
cloud db list
```

### `db create`

Create a new managed database.

```bash
cloud db create --name my-db --engine postgres --version 16 --size 20
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--name` | `-n` | | Database name (required) |
| `--engine` | `-e` | `postgres` | Database engine (postgres/mysql) |
| `--version` | `-v` | `16` | Engine version |
| `--vpc` | `-V` | | VPC ID to attach to |
| `--size` | | `10` | Allocated storage in GB (minimum 10GB) |

### `db connection <id>`

Get database connection string.

```bash
cloud db connection db-uuid
```

### `db show <id>`

Show detailed database information.

```bash
cloud db show db-uuid
```

### `db rm <id>`

Delete a database instance. (Alias: `delete`)

```bash
cloud db rm db-uuid
```

### `db resize <id> <sizeGB>`

Resize the allocated storage for a database instance.

```bash
cloud db resize db-uuid 100
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
cloud cache create --name my-redis --memory 256 --wait
```

### `cache rm <id>`

Delete a cache instance. (Alias: `delete`)

```bash
cloud cache rm redis-uuid
```

### `cache resize <id> <memoryMB>`

Resize a cache instance's memory allocation (requires docker compute backend).

```bash
cloud cache resize redis-uuid 512
```

---

## Kubernetes Commands (KaaS)

Manage managed Kubernetes clusters.

### `k8s list`

List all Kubernetes clusters.

```bash
cloud k8s list
```

### `k8s create`

Create a new Kubernetes cluster.

```bash
cloud k8s create --name my-cluster --vpc vpc-uuid --ha --workers 3
```

### `k8s kubeconfig <id>`

Get kubeconfig for a cluster.

```bash
cloud k8s kubeconfig cluster-uuid > kubeconfig.yaml
```

### `k8s rm <id>`

Delete a Kubernetes cluster. (Alias: `delete`)

```bash
cloud k8s rm cluster-uuid
```

---

## Cloud Gateway Commands 🆕

Manage API gateway routes.

### `gateway list-routes`

List all registered routes.

```bash
cloud gateway list-routes
```

### `gateway create-route <name> <pattern> <target>`

Create a new gateway route.

```bash
cloud gateway create-route my-api "/users/{id}" http://my-instance:8080 --strip --methods GET,POST
```

### `gateway rm-route <id>`

Delete a route. (Alias: `delete`)

```bash
cloud gateway rm-route route-uuid
```

---

## Cloud Functions Commands 🆕

Manage serverless functions.

### `function list`

List all functions.

```bash
cloud function list
cloud function list --limit 20 --offset 0  # Pagination
```

**Flags**:
| Flag | Default | Description |
|------|---------|-------------|
| `--limit` | `0` | Maximum number of results (0 = use server default) |
| `--offset` | `0` | Number of results to skip |

### `function create`

Create a new function.

```bash
cloud function create --name my-func --runtime nodejs20 --code ./code.zip --handler index.handler
```

### `function invoke <id>`

Invoke a function.

```bash
cloud function invoke my-func --payload '{"key":"value"}'
```

### `function rm <id>`

Delete a function. (Alias: `delete`)

```bash
cloud function rm my-func
```

### `function update <id>`

Update a function's configuration.

```bash
# Update timeout and memory
cloud function update my-func --timeout 300 --memory 256

# Update handler
cloud function update my-func --handler newhandler.js

# Set environment variables
cloud function update my-func --env FOO=bar --env DEBUG=true
```

**Flags:**
- `--handler`, `-H`: Handler name
- `--timeout`: Timeout in seconds (1-900)
- `--memory`: Memory in MB (64-10240)
- `--env`: Environment variable `KEY=VALUE` or `KEY=@secretname` for secrets (can be repeated)

### `fn-schedule create`

Create a scheduled function invocation.

```bash
cloud fn-schedule create --name nightly --function my-func --schedule "0 2 * * *"
```

### `fn-schedule list`

List all function schedules.

```bash
cloud fn-schedule list
```

### `fn-schedule pause <id>`

Pause a schedule to temporarily prevent invocations.

```bash
cloud fn-schedule pause [schedule-id]
```

### `fn-schedule resume <id>`

Resume a paused schedule.

```bash
cloud fn-schedule resume [schedule-id]
```

### `fn-schedule logs <id>`

View run history for a schedule.

```bash
cloud fn-schedule logs [schedule-id]
```

### `fn-schedule rm <id>`

Delete a function schedule.

```bash
cloud fn-schedule rm [schedule-id]
```

---

## CloudNotify Commands

Manage pub/sub messaging topics and subscriptions.

### `notify create-topic <name>`

Create a new notification topic.

```bash
cloud notify create-topic my-topic
```

### `notify list-topics`

List all notification topics.

```bash
cloud notify list-topics
```

### `notify subscribe <topic-id>`

Subscribe to a topic to receive notifications.

```bash
cloud notify subscribe topic-uuid --endpoint https://example.com/webhook
```

**Flags**:
| Flag | Short | Required | Default | Description |
|------|-------|----------|---------|-------------|
| `--protocol` | `-p` | No | `webhook` | Protocol (webhook or queue) |
| `--endpoint` | `-e` | Yes | - | Endpoint URL or queue ID |

### `notify unsubscribe <subscription-id>`

Unsubscribe from a topic.

```bash
cloud notify unsubscribe subscription-uuid
```

### `notify publish <topic-id> <message>`

Publish a message to a topic.

```bash
cloud notify publish topic-uuid "Hello, world!"
```

---

## Tips & Tricks

### Standardized IDs
All CLI table outputs truncate UUIDs to the first **8 characters** for readability. You can use either the full UUID or the truncated version in commands.

### JSON Output
Use `--json` (or `-j`) with any list/get command for programmatic integration:
```bash
cloud instance list --json | jq '.[].id'
```

### Environment Variables
- `CLOUD_API_KEY`: Default API key for all commands.
- `CLOUD_TENANT_ID`: Default tenant context.
- `CLOUD_API_URL`: Override API server URL.

### Aliases
Add to your shell config for faster access:
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
