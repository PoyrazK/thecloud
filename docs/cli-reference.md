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
cloud compute launch --name my-server --image nginx:alpine
```
| Flag | Default | Description |
|------|---------|-------------|
| `-n, --name` | (required) | Instance name |
| `-i, --image` | `alpine` | Docker image |
| `-p, --port` | | Port mapping (host:container) |

### `compute stop <id>`
Stop an instance.
```bash
cloud compute stop a1b2c3d4
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
