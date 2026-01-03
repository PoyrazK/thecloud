# CLI Reference

Complete reference for `cloud` CLI commands.

## Global Flags
| Flag | Description |
|------|-------------|
| `-k, --api-key` | API key for authentication |
| `-j, --json` | Output in JSON format |

---

## auth
Manage authentication.

### `auth create-demo <name>`
Generate a demo API key and save it.
```bash
cloud auth create-demo my-user
```

### `auth login <key>`
Save an existing API key.
```bash
cloud auth login sk_abc123...
```

### `auth roles`
List available roles.
```bash
cloud auth roles
```

### `auth role get [user-id|me]`
Get a user's role (defaults to current user).
```bash
cloud auth role get
cloud auth role get me
cloud auth role get 550e8400-e29b-41d4-a716-446655440000
```

### `auth role set <user-id> <role>`
Update a user's role.
```bash
cloud auth role set 550e8400-e29b-41d4-a716-446655440000 admin
```

---

## compute
Manage compute instances.

### `compute list`
List all instances.
```bash
cloud compute list
```

### `compute launch`
Launch a new instance.
```bash
cloud compute launch --name my-server --image nginx:alpine --port 8080:80 --volume data:/var/lib/data
```
| Flag | Default | Description |
|------|---------|-------------|
| `-n, --name` | (required) | Instance name |
| `-i, --image` | `alpine` | Docker image |
| `-p, --port` | | Port mapping (host:container) |
| `-v, --vpc` | | VPC ID or Name |
| `-V, --volume` | | Volume attachment (vol-name:/path) |

### `compute stop <id>`
Stop an instance.
```bash
cloud compute stop a1b2c3d4
```

### `compute rm <id>`
Terminate and remove an instance.
```bash
cloud compute rm my-server
```

### `compute logs <id>`
View instance logs (supports ID or Name).
```bash
cloud compute logs my-server
```

### `compute show <id>`
Show detailed instance information.
```bash
cloud compute show my-server
```

### `compute stats <id>`
Show instance CPU and Memory usage.
```bash
cloud compute stats my-server
```

---

## volume
Manage block storage volumes.

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
| Flag | Default | Description |
|------|---------|-------------|
| `-n, --name` | (required) | Volume name |
| `-s, --size` | `1` | Size in GB |

### `volume rm <id>`
Delete a volume.
```bash
cloud volume rm my-data
```

---

## vpc
Manage Virtual Private Clouds.

### `vpc list`
List all VPCs.
```bash
cloud vpc list
```

### `vpc create`
Create a new VPC.
```bash
cloud vpc create --name my-network
```

### `vpc rm <id>`
Delete a VPC.
```bash
cloud vpc rm my-network
```

---

## events
View system event logs.

### `events list`
List recent events (audit log).
```bash
cloud events list
```

---

## storage
Manage object storage.

### `storage upload <bucket> <file>`
Upload a file.
```bash
cloud storage upload my-bucket README.md
```
| Flag | Description |
|------|-------------|
| `--key` | Custom key (default: filename) |

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

## lb
Manage Load Balancers.

### `lb list`
List all Load Balancers.
```bash
cloud lb list
```

### `lb create`
Create a new Load Balancer.
```bash
cloud lb create --name my-lb --vpc <vpc-id> --port 8080
```
| Flag | Description |
|------|-------------|
| `-n, --name` | (required) Name of the load balancer |
| `-v, --vpc` | (required) VPC ID |
| `-p, --port` | (required) Port to listen on |
| `-t, --type` | Type (default: HTTP) |

### `lb rm <id>`
Delete a Load Balancer.
```bash
cloud lb rm <lb-id>
```

### `lb add-target <lb-id> <instance-id>`
Register an instance with the load balancer.
```bash
cloud lb add-target   --instance <inst-id>
```

### `lb remove-target <lb-id> <instance-id>`
Deregister an instance.
```bash
cloud lb remove-target   --instance <inst-id>
```

---

## autoscaling
Manage Auto-Scaling Groups.

### `autoscaling list`
List all scaling groups.
```bash
cloud autoscaling list
```

### `autoscaling create`
Create a new scaling group.
```bash
cloud autoscaling create \
  --name web-asg \
  --vpc <vpc-id> \
  --image nginx:alpine \
  --ports 80:80 \
  --min 1 --max 5 --desired 2
```

### `autoscaling rm <id>`
Delete a scaling group and terminate its instances.
```bash
cloud autoscaling rm <asg-id>
```

### `autoscaling add-policy <id>`
Add a scaling policy.
```bash
cloud autoscaling add-policy <asg-id> \
  --name cpu-policy \
  --metric cpu \
  --target 50
```

---

## cache
Manage managed Redis caches.

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
| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | Cache name |
| `--version` | `7.2` | Redis version |
| `--memory` | `128` | Memory limit in MB |
| `--vpc` | | VPC ID |
| `--wait` | `false` | Wait for instance to be ready |

### `cache connection <id>`
Get the connection string for a cache (includes password).
```bash
cloud cache connection my-redis
```

### `cache show <id>`
Show detailed cache information.
```bash
cloud cache show my-redis
```

### `cache stats <id>`
Show cache statistics (Memory, Clients, Keys).
```bash
cloud cache stats my-redis
```

### `cache flush <id>`
Flush all keys from the cache (Dangerous!).
```bash
cloud cache flush my-redis --yes
```

### `cache rm <id>`
Delete a cache instance.
```bash
cloud cache rm my-redis
```

---

## db
Manage managed database instances (RDS).

### `db list`
List all database instances.
```bash
cloud db list
```

### `db create`
Create a new managed database instance.
```bash
cloud db create --name my-db --engine postgres --version 16
```
| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | Database name |
| `--engine` | `postgres` | Engine (postgres/mysql) |
| `--version` | `16` | Engine version |
| `--vpc` | | VPC ID |

### `db connection <id>`
Get database connection string.
```bash
cloud db connection <db-id>
```

### `db logs <id>`
Get database logs.
```bash
cloud db logs <db-id>
```

### `db show <id>`
Show detailed database information.
```bash
cloud db show <db-id>
```

### `db rm <id>`
Remove a database instance.
```bash
cloud db rm <db-id>
```

---

## secrets
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
```

### `secrets get <id>`
Decrypt and show a secret value.
```bash
cloud secrets get <id>
```

### `secrets rm <id>`
Remove a secret.
```bash
cloud secrets rm <id>
```

---

## fn
Manage CloudFunctions (Serverless).

### `fn list`
List all functions.
```bash
cloud fn list
```

### `fn create`
Create a new function.
```bash
cloud fn create --name my-fn --runtime nodejs20 --code ./func.zip
```
| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | Function name |
| `--code` | (required) | Path to zip file |
| `--runtime` | `nodejs20` | Runtime environment |
| `--handler` | `index.handler` | Entry point |

### `fn invoke <id>`
Invoke a function.
```bash
cloud fn invoke my-fn --payload '{"foo":"bar"}'
```

### `fn logs <id>`
Get recent logs for a function.
```bash
cloud fn logs my-fn
```

### `fn rm <id>`
Remove a function.
```bash
cloud fn rm my-fn
```
