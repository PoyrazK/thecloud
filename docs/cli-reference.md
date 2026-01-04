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

---

## instance
Manage compute instances.

### `instance list`
List all instances.
```bash
cloud instance list
```

### `instance launch`
Launch a new instance.
```bash
cloud instance launch --name my-server --image nginx:alpine --port 8080:80 --volume data:/var/lib/data
```
| Flag | Default | Description |
|------|---------|-------------|
| `-n, --name` | (required) | Instance name |
| `-i, --image` | `alpine` | Docker image |
| `-p, --port` | | Port mapping (host:container) |
| `-v, --vpc` | | VPC ID or Name |
| `-V, --volume` | | Volume attachment (vol-name:/path) |

### `instance stop <id>`
Stop an instance.
```bash
cloud instance stop a1b2c3d4
```

### `instance rm <id>`
Terminate and remove an instance.
```bash
cloud instance rm my-server
```

### `instance logs <id>`
View instance logs (supports ID or Name).
```bash
cloud instance logs my-server
```

### `instance show <id>`
Show detailed instance information.
```bash
cloud instance show my-server
```

### `instance stats <id>`
Show instance CPU and Memory usage.
```bash
cloud instance stats my-server
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

## function
Manage CloudFunctions (Serverless).

### `function list`
List all functions.
```bash
cloud function list
```

### `function create`
Create a new function.
```bash
cloud function create --name my-function --runtime nodejs20 --code ./func.zip
```
| Flag | Default | Description |
|------|---------|-------------|
| `--name` | (required) | Function name |
| `--code` | (required) | Path to zip file |
| `--runtime` | `nodejs20` | Runtime environment |
| `--handler` | `index.handler` | Entry point |

### `function invoke <id>`
Invoke a function.
```bash
cloud function invoke my-function --payload '{"foo":"bar"}'
```

### `function logs <id>`
Get recent logs for a function.
```bash
cloud function logs my-function
```

### `function rm <id>`
Remove a function.
```bash
cloud function rm my-function
```

---

## notify
Manage CloudNotify (Pub/Sub topics and subscriptions).

### `notify create-topic <name>`
Create a new notification topic.
```bash
cloud notify create-topic my-updates
```

### `notify list-topics`
List all notification topics.
```bash
cloud notify list-topics
```

### `notify subscribe <topic-id>`
Subscribe to a topic.
```bash
cloud notify subscribe <topic-id> --protocol webhook --endpoint https://example.com/hook
```
| Flag | Default | Description |
|------|---------|-------------|
| `-p, --protocol` | `webhook` | Protocol (webhook/queue) |
| `-e, --endpoint` | (required) | Target URL or Queue ID |

### `notify publish <topic-id> <message>`
Publish a message to all subscribers.
```bash
cloud notify publish <topic-id> "System update complete"
```

---

## cron
Manage CloudCron (Scheduled Tasks).

### `cron create <name> <schedule> <url>`
Create a new scheduled task.
```bash
cloud cron create cleanup "0 0 * * *" https://api.example.com/cleanup -X POST -d '{"force": true}'
```
| Flag | Default | Description |
|------|---------|-------------|
| `-X, --method` | `POST` | HTTP method |
| `-d, --payload` | | Request payload |

### `cron list`
List all scheduled tasks.
```bash
cloud cron list
```

### `cron pause <id>`
Pause a scheduled task.
```bash
cloud cron pause <job-id>
```

### `cron resume <id>`
Resume a paused scheduled task.
```bash
cloud cron resume <job-id>
```

### `cron rm <id>`
Delete a scheduled task.
```bash
cloud cron rm <job-id>
```
