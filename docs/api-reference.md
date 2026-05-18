# API Reference

## Authentication

> **Rate Limiting**: Auth endpoints are rate-limited to 5 requests per minute per IP to prevent brute-force attacks.

### POST /auth/register
Register a new user account.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "password",
  "name": "User Name"
}
```

**Response:**
```json
{
  "message": "user created successfully",
  "user_id": "uuid"
}
```

### POST /auth/login
Login to obtain an API Key.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "password"
}
```

**Response:**
```json
{
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "User Name"
  },
  "api_key": "thecloud_xxxxx"
}
```

### POST /auth/forgot-password
Request a password reset token (rate limited: 5 requests/minute).

**Request:**
```json
{
  "email": "user@example.com"
}
```

**Response:**
```json
{
  "message": "If the email exists, a reset token has been sent."
}
```

### POST /auth/reset-password
Reset password using a valid token (rate limited: 5 requests/minute).

**Request:**
```json
{
  "token": "reset-token-from-email",
  "new_password": "new-secure-password"
}
```

**Response:**
```json
{
  "message": "password updated successfully"
}
```

---

## API Key Management

**Headers Required:** `X-API-Key: <your-api-key>`

### POST /auth/keys
Create a new API key.

**Request:**
```json
{
  "name": "Production Key"
}
```

**Response:**
```json
{
  "id": "uuid",
  "user_id": "user-uuid",
  "name": "Production Key",
  "key": "thecloud_xxxxx",
  "created_at": "2026-01-05T23:00:00Z",
  "last_used": "2026-01-05T23:00:00Z"
}
```

### GET /auth/keys
List all API keys for the authenticated user.

### DELETE /auth/keys/:id
Revoke an API key.

### POST /auth/keys/:id/rotate
Rotate an API key (creates new key, deletes old one).

### POST /auth/keys/:id/regenerate
Regenerate an API key (alias for rotate).

---

## Tenants & Organizations 🆕

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /tenants
List all tenants (organizations) the authenticated user belongs to.

### POST /tenants
Create a new tenant.
**Request:**
```json
{
  "name": "Acme Corp",
  "slug": "acme"
}
```

### POST /tenants/:id/switch
Switch the user's active/default tenant. This affects which resources are visible in subsequent requests.

### POST /tenants/:id/members
Add a new member to a tenant.
**Request:**
```json
{
  "user_id": "uuid",
  "role": "member"
}
```

---

## System Health

### GET /health/live
Liveness probe. Returns 200 OK if the process is running.
**Response:** `{"status": "ok"}`

### GET /health/ready
Readiness probe. Checks connections to Database and Docker daemon.
**Response (200 OK):**
```json
{
  "status": "UP",
  "checks": { "database": "CONNECTED", "docker": "CONNECTED" },
  "time": "..."
}
```
**Response (503 Service Unavailable):**
```json
{
  "status": "DEGRADED",
  "checks": { "database": "DISCONNECTED", ... }
}
```

---

## Compute Instances

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /instances
List all instances owned by the authenticated user.

**Query Parameters:**
- `tag` (string, repeatable): Filter instances by label. Supports `key:value` format for exact match or bare key for existence check. Example: `?tag=env:prod&tag=team:backend`

**Response:**
```json
{
  "data": [
    {
      "id": "inst-uuid",
      "name": "web-01",
      "status": "RUNNING",
      "labels": {
        "env": "prod",
        "team": "backend"
      }
    }
  ]
}
```

### GET /instances/:id/tags
Get all labels for an instance.

**Response:**
```json
{
  "data": {
    "env": "prod",
    "team": "backend"
  }
}
```

### POST /instances/:id/tags
Add or update one or more labels on an instance. Merges with existing labels.

**Request:**
```json
{
  "tags": {
    "env": "prod",
    "team": "backend"
  }
}
```

**Response:** Updated label map (same format as GET /instances/:id/tags).

### DELETE /instances/:id/tags/:key
Remove a label key from an instance.

**Response:** `204 No Content`

### POST /instances
Launch a new instance.
```json
{
  "name": "web-01",
  "image": "nginx",
  "instance_type": "basic-2",
  "vpc_id": "vpc-uuid",
  "subnet_id": "subnet-uuid",
  "ports": "80:80",
  "volumes": [
    { "volume_id": "vol-uuid", "mount_path": "/data" }
  ]
}
```

### GET /instance-types
List all available instance types.
**Response:**
```json
[
  {
    "id": "basic-2",
    "name": "Basic 2",
    "vcpus": 1,
    "memory_mb": 1024,
    "disk_gb": 10,
    "network_mbps": 1000,
    "price_per_hour": 0.02,
    "category": "general-purpose"
  }
]
```

### GET /instances/:id
Get details of a specific instance.

### PUT /instances/:id
Update instance (e.g., status).

### DELETE /instances/:id
Terminate an instance.

### POST /instances/:id/resize
Resize an instance to a different instance type (CPU/memory).

**Request:**
```json
{
  "instance_type": "basic-4"
}
```

**Response:**
```json
{
  "message": "instance resized"
}
```

**Error Responses:**
- `400` — Invalid input (bad instance ID, empty instance type, invalid type)
- `404` — Instance not found
- `403` — Insufficient quota for the requested type

### GET /instances/:id/console
Get the VNC console URL for the instance.
**Response:**
```json
{
  "console_url": "vnc://127.0.0.1:5901"
}
```

### GET /instances/:id/stats
Get real-time resource usage stats for an instance (CPU, memory, network I/O, disk I/O). Returns a wrapped response via `httputil.Success`.

**Response:**
```json
{
  "data": {
    "cpu_percentage": 25.5,
    "memory_usage_bytes": 524288000,
    "memory_limit_bytes": 1073741824,
    "memory_percentage": 48.8,
    "network_rx_bytes": 1024000,
    "network_tx_bytes": 512000,
    "disk_read_bytes": 10240000,
    "disk_write_bytes": 5120000,
    "cpu_time_nanoseconds": 5000000000
  }
}
```

**Fields:**
- `cpu_percentage` (float): CPU usage as percentage of available
- `memory_usage_bytes` (float): Current memory usage in bytes
- `memory_limit_bytes` (float): Memory limit in bytes
- `memory_percentage` (float): Memory usage as percentage of limit
- `network_rx_bytes` (uint64 | null): Total network bytes received; omitted when unavailable
- `network_tx_bytes` (uint64 | null): Total network bytes transmitted; omitted when unavailable
- `disk_read_bytes` (uint64 | null): Total disk bytes read; omitted when unavailable
- `disk_write_bytes` (uint64 | null): Total disk bytes written; omitted when unavailable
- `cpu_time_nanoseconds` (uint64 | null, optional): Cumulative CPU time in nanoseconds (Libvirt backend); omitted when unavailable

**Error Responses:**
- `404` — Instance not found
- `500` — Backend stats unavailable (internal error)

### POST /instances/:id/pause
Pause a running instance (freezes CPU, retains memory/network).

**Prerequisites:** Instance must be in `RUNNING` state.

**Response:**
```json
{
  "message": "instance paused"
}
```

**Error Responses:**
- `400` — Instance not in RUNNING state (returned as `CONFLICT`)
- `404` — Instance not found
- `403` — Insufficient permissions

### POST /instances/:id/resume
Resume a paused instance back to running state.

**Prerequisites:** Instance must be in `PAUSED` state.

**Response:**
```json
{
  "message": "instance resumed"
}
```

**Error Responses:**
- `400` — Instance not in PAUSED state (returned as `CONFLICT`)
- `404` — Instance not found
- `403` — Insufficient permissions

---

## Images

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /images
List all images available to the authenticated user (own + public).

**Response:**
```json
[
  {
    "id": "uuid",
    "name": "ubuntu-22.04",
    "description": "Ubuntu 22.04 LTS",
    "os": "linux",
    "version": "22.04",
    "format": "qcow2",
    "size_gb": 2,
    "is_public": false,
    "status": "ACTIVE",
    "created_at": "2026-04-21T10:00:00Z"
  }
]
```

### POST /images
Register a new image (metadata only; upload the file separately via `POST /images/:id/upload`).

**Request:**
```json
{
  "name": "my-custom-image",
  "description": "My custom OS image",
  "os": "linux",
  "version": "22.04",
  "is_public": false
}
```

### GET /images/:id
Get details of a specific image.

### DELETE /images/:id
Delete an image and its associated file from storage.

### POST /images/:id/upload
Upload the qcow2/image file for a registered image (multipart/form-data).

**Form field:** `file` — the image binary file.

### POST /images/import
Import an image from a remote URL. The file is downloaded and stored automatically.

**Request:**
```json
{
  "name": "ubuntu-22.04-cloud",
  "url": "https://cloud-images.ubuntu.com/releases/22.04/release/ubuntu-22.04-server-cloudimg-amd64.img",
  "description": "Ubuntu 22.04 LTS cloud image",
  "os": "linux",
  "version": "22.04",
  "is_public": false
}
```

**Response:** `202 Accepted` — image metadata is returned immediately. The image status transitions from `PENDING` to `ACTIVE` once the download completes.

---

## Networks (VPC)

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /vpcs
List all VPCs.

### POST /vpcs
Create a new VPC.
```json
{
  "name": "prod-vpc"
}
```

### DELETE /vpcs/:id
Delete a VPC.
 
 ---
 
 ## VPC Peering 🆕
 
 **Headers Required:** `X-API-Key: <your-api-key>`
 
 ### GET /vpc-peerings
 List all VPC peering connections for the tenant.
 
 ### POST /vpc-peerings
 Initiate a new VPC peering request.
 ```json
 {
   "requester_vpc_id": "uuid",
   "accepter_vpc_id": "uuid"
 }
 ```
 
 **Response (201 Created):**
 ```json
 {
   "id": "uuid",
   "status": "pending-acceptance",
   "requester_vpc_id": "uuid",
   "accepter_vpc_id": "uuid",
   "arn": "arn:thecloud:vpc-peering:..."
 }
 ```
 
 ### GET /vpc-peerings/:id
 Get details of a specific peering connection.
 
 ### POST /vpc-peerings/:id/accept
 Accept a pending peering request. This activates cross-bridge routing via OVS.
 
 ### POST /vpc-peerings/:id/reject
 Reject a pending peering request.
 
 ### DELETE /vpc-peerings/:id
 Delete a peering connection and remove all associated network routes.

 ---

## Route Tables 🆕

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /route-tables
List all route tables for a VPC.
Query params: `?vpc_id=<vpc-uuid>` (required)

### POST /route-tables
Create a new custom route table.
```json
{
  "vpc_id": "vpc-uuid",
  "name": "custom-rt",
  "is_main": false
}
```

### GET /route-tables/:id
Get details of a specific route table including its routes.

### DELETE /route-tables/:id
Delete a custom route table. Cannot delete the main route table of a VPC.

### POST /route-tables/:id/routes
Add a route to a route table.
```json
{
  "destination_cidr": "0.0.0.0/0",
  "target_type": "igw",
  "target_id": "igw-uuid"
}
```

**Target Types:**
- `local` - Traffic within VPC CIDR
- `igw` - Internet Gateway
- `nat` - NAT Gateway
- `peering` - VPC Peering connection

### DELETE /route-tables/:id/routes?route_id=<route_id>
Remove a route from a route table. Query param `route_id` is required.

### POST /route-tables/:id/associate
Associate a subnet with a route table.
```json
{
  "subnet_id": "subnet-uuid"
}
```

### POST /route-tables/:id/disassociate
Disassociate a subnet from a route table.
```json
{
  "subnet_id": "subnet-uuid"
}
```

---

## Internet Gateways 🆕

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /internet-gateways
List all internet gateways for the tenant.

### POST /internet-gateways
Create a new internet gateway.
**Response (201 Created):**
```json
{
  "id": "uuid",
  "status": "detached",
  "arn": "arn:thecloud:vpc:local:tenant:igw/uuid"
}
```

### GET /internet-gateways/:id
Get details of a specific internet gateway.

### POST /internet-gateways/:id/attach
Attach an internet gateway to a VPC.
```json
{
  "vpc_id": "vpc-uuid"
}
```

### POST /internet-gateways/:id/detach
Detach an internet gateway from its VPC. Must remove all routes referencing this IGW first.

### DELETE /internet-gateways/:id
Delete an internet gateway. Must be detached first.

---

## NAT Gateways 🆕

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /nat-gateways
List all NAT gateways for a VPC.
Query params: `?vpc_id=<vpc-uuid>` (required)

### POST /nat-gateways
Create a new NAT gateway in a subnet with an allocated Elastic IP.
```json
{
  "subnet_id": "subnet-uuid",
  "eip_id": "eip-uuid"
}
```
**Response (201 Created):**
```json
{
  "id": "uuid",
  "vpc_id": "vpc-uuid",
  "subnet_id": "subnet-uuid",
  "elastic_ip_id": "eip-uuid",
  "status": "active",
  "private_ip": "10.0.1.100",
  "arn": "arn:thecloud:vpc:local:tenant:nat-gateway/uuid"
}
```

### GET /nat-gateways/:id
Get details of a specific NAT gateway.

### DELETE /nat-gateways/:id
Delete a NAT gateway and release the associated Elastic IP back to allocated state.

---

## Security Groups

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /security-groups
List all security groups.
Query params: `?vpc_id=<vpc-uuid>` (required)

### POST /security-groups
Create a new security group.
```json
{
  "vpc_id": "vpc-uuid",
  "name": "web-tier",
  "description": "Port 80 and 443 allowed"
}
```

### GET /security-groups/:id
Get details and rules for a security group.

### DELETE /security-groups/:id
Delete a security group.

### POST /security-groups/:id/rules
Add a firewall rule.
```json
{
  "direction": "ingress",
  "protocol": "tcp",
  "port_min": 80,
  "port_max": 80,
  "cidr": "0.0.0.0/0",
  "priority": 100
}
```

### DELETE /security-groups/rules/:rule_id
Remove a firewall rule.

### POST /security-groups/attach
Attach a security group to an instance.
```json
{
  "instance_id": "inst-uuid",
  "group_id": "sg-uuid"
}
```

### POST /security-groups/detach
Detach a security group from an instance.
```json
{
  "instance_id": "inst-uuid",
  "group_id": "sg-uuid"
}
```

---

## Subnets

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /vpcs/:vpc_id/subnets
List all subnets in a VPC.

### POST /vpcs/:vpc_id/subnets
Create a new subnet.
```json
{
  "name": "private-subnet-1",
  "cidr_block": "10.0.1.0/24",
  "availability_zone": "us-east-1a"
}
```

### GET /subnets/:id
Get details of a specific subnet.

### DELETE /subnets/:id
Delete a subnet.

---

## Elastic IPs (Static IPs) 🆕

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /elastic-ips
List all allocated elastic IPs for the tenant.

### POST /elastic-ips
Allocate a new elastic IP. No body required.
**Response:**
```json
{
  "id": "uuid",
  "public_ip": "100.64.x.y",
  "status": "allocated",
  "arn": "arn:thecloud:vpc:local:tenant:eip/uuid"
}
```

### GET /elastic-ips/:id
Get details of a specific elastic IP.

### DELETE /elastic-ips/:id
Release an elastic IP back to the pool. Fails if still associated.

### POST /elastic-ips/:id/associate
Associate an elastic IP with a compute instance.
**Request:**
```json
{
  "instance_id": "inst-uuid"
}
```

### POST /elastic-ips/:id/disassociate
Disassociate an elastic IP from its current instance.

---

## Pipelines (CI/CD) 🆕

**Headers Required:** `X-API-Key: <your-api-key>` for protected endpoints.

### GET /pipelines
List pipelines for the authenticated user.

### POST /pipelines
Create a pipeline definition.

```json
{
  "name": "lint-thecloud",
  "repository_url": "https://github.com/poyrazK/thecloud.git",
  "branch": "main",
  "webhook_secret": "your-secret",
  "config": {
    "stages": [
      {
        "name": "lint",
        "steps": [
          {
            "name": "golangci",
            "image": "golang:1.24",
            "commands": [
              "git clone https://github.com/poyrazK/thecloud.git /workspace/thecloud",
              "cd /workspace/thecloud",
              "go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.60.3",
              "/go/bin/golangci-lint run ./..."
            ]
          }
        ]
      }
    ]
  }
}
```

### GET /pipelines/:id
Get a pipeline by ID.

### PUT /pipelines/:id
Update mutable fields of a pipeline.

### DELETE /pipelines/:id
Delete a pipeline.

### POST /pipelines/:id/runs
Trigger a manual run.

```json
{
  "commit_hash": "abc123",
  "trigger_type": "MANUAL"
}
```

### GET /pipelines/:id/runs
List runs for a pipeline.

### GET /pipelines/runs/:buildID
Get run details.

### GET /pipelines/runs/:buildID/steps
List step results for a run.

### GET /pipelines/runs/:buildID/logs?limit=200
List logs for a run.

### POST /pipelines/:id/webhook/:provider
Public webhook trigger endpoint.

- `provider`: `github` or `gitlab`
- No API key required (validated by webhook secret/signature).

#### GitHub headers
- `X-GitHub-Event` (supported: `push`)
- `X-Hub-Signature-256` (HMAC SHA-256)
- `X-GitHub-Delivery` (used for idempotency)

#### GitLab headers
- `X-Gitlab-Event` (supported: `Push Hook`)
- `X-Gitlab-Token`
- `X-Gitlab-Event-UUID` (used for idempotency)

#### Webhook response behavior
- `202 accepted` with build payload when a run is queued.
- `202 accepted` with `{ "status": "ignored" }` for duplicate/non-matching events.

---

## Cloud Storage (S3-Compatible)

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /storage/buckets
List all buckets owned by or accessible to the tenant.

### POST /storage/buckets
Create a new storage bucket.
```json
{
  "name": "my-assets",
  "is_public": false
}
```

### DELETE /storage/buckets/:bucket
Delete a bucket. Fails if the bucket is not empty unless `?force=true` is provided.

### PATCH /storage/buckets/:bucket/versioning
Enable or disable versioning for a bucket.
```json
{
  "enabled": true
}
```

### GET /storage/:bucket
List all objects in a bucket (latest versions only).

### PUT /storage/:bucket/*key
Upload an object. The request body is the raw file data.
**Response:**
```json
{
  "id": "uuid",
  "bucket": "my-bucket",
  "key": "test.txt",
  "size_bytes": 1024,
  "content_type": "text/plain",
  "checksum": "sha256-hex-hash",
  "upload_status": "AVAILABLE",
  "version_id": "null",
  "created_at": "2026-03-04T17:00:00Z"
}
```

### GET /storage/:bucket/*key
Download an object (latest version).

### DELETE /storage/:bucket/*key
Soft-delete an object (latest version).

### GET /storage/versions/:bucket/*key
List all versions of a specific object.

### GET /storage/cluster/status
Get health and node status of the distributed storage cluster.

---

## Multipart Uploads

### POST /storage/multipart/init/:bucket/*key
Initiate a multipart upload session.
**Response:** `{"id": "upload-uuid", ...}`

### PUT /storage/multipart/upload/:id/parts?part=1
Upload a part for a multipart session.

### POST /storage/multipart/complete/:id
Finalize a multipart upload and assemble the object.

### DELETE /storage/multipart/abort/:id
Abort a multipart upload and clean up uploaded parts.

---

## Lifecycle Rules

### GET /storage/buckets/:bucket/lifecycle
List lifecycle rules for a bucket.

### POST /storage/buckets/:bucket/lifecycle
Create a new lifecycle rule.
```json
{
  "prefix": "logs/",
  "expiration_days": 30
}
```

### DELETE /storage/buckets/:bucket/lifecycle/:id
Delete a lifecycle rule.

---

### DELETE /elastic-ips/:id
Release an elastic IP back to the pool. Fails if still associated.

### POST /elastic-ips/:id/associate
Associate an elastic IP with a compute instance.
**Request:**
```json
{
  "instance_id": "inst-uuid"
}
```

### POST /elastic-ips/:id/disassociate
Disassociate an elastic IP from its current instance.

---

## Volumes

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /volumes
List all volumes.

### POST /volumes
Create a new volume.
```json
{
  "name": "data-vol",
  "size_gb": 10
}
```

### POST /volumes/:id/resize
Resize a volume to a larger capacity.

**Request:**
```json
{
  "new_size_gb": 50
}
```

**Response:**
```json
{
  "message": "volume resized",
  "new_size_gb": 50
}
```

**Error Responses:**
- `400` — Invalid input (size must be at least 1)
- `404` — Volume not found

---

## Managed Databases (RDS)

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /databases
List all managed databases.

### POST /databases
Provision a new primary database.
```json
{
  "name": "prod-db",
  "engine": "postgres",
  "version": "16",
  "vpc_id": "vpc-uuid",
  "allocated_storage": 20,
  "pooling_enabled": true,
  "metrics_enabled": true,
  "parameters": {
    "max_connections": "100"
  }
}
```

### GET /databases/:id
Get details of a specific database.

### PATCH /databases/:id
Modify an existing database configuration.
```json
{
  "allocated_storage": 40,
  "pooling_enabled": false,
  "metrics_enabled": true,
  "parameters": {
    "max_connections": "200"
  }
}
```

### DELETE /databases/:id
Terminate a database instance.

### GET /databases/:id/connection
Get the connection string for the database.

### POST /databases/:id/replicas
Create a read-replica for a primary database.
```json
{
  "name": "prod-db-replica-1"
}
```

### POST /databases/:id/promote
Promote a replica to a standalone primary database.

### POST /databases/:id/rotate-credentials
Regenerate the database password and update it in both the database engine and Vault.
- **Security**: Ensures credentials are never stored in plain text in the primary metadata store.
- **Workflow**: Automated update of database users and sidecar (pooler) reloads.

**Response Example**:
```json
{
  "message": "database credentials rotated successfully",
  "data": {
    "message": "database credentials rotated successfully"
  }
}
```

### POST /databases/:id/stop
Stop a running database instance. The data volume is retained.
- **Constraints**: Cannot stop replicas (must promote first) or databases in CREATING/DELETING state.
- **Response**:
```json
{
  "message": "database stopped"
}
```

### POST /databases/:id/start
Start a stopped database instance.
- **Constraints**: Database must be in STOPPED state.
- **Workflow**: Starts container, waits for readiness, starts sidecars if enabled.
- **Response**:
```json
{
  "message": "database started"
}
```

### POST /databases/:id/resize
Resize the allocated storage for a database instance.

**Request:**
```json
{
  "allocated_storage": 100
}
```

**Response:**
```json
{
  "message": "database resized",
  "allocated_storage": 100
}
```

**Error Responses:**
- `400` — Invalid input (size must be larger than current allocated storage)
- `404` — Database not found

---

## Global Load Balancers 🆕

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /global-lb
List all global load balancers owned by the authenticated user.

### POST /global-lb
Create a new global load balancer.
```json
{
  "name": "api-global",
  "hostname": "api.myapp.com",
  "policy": "LATENCY",
  "health_check": {
    "protocol": "HTTP",
    "port": 80,
    "path": "/health"
  }
}
```

### GET /global-lb/:id
Get details of a GLB including its endpoints.

### DELETE /global-lb/:id
Delete a global load balancer and its DNS records. Only the owner can delete the resource.

### POST /global-lb/:id/endpoints
Add a regional endpoint to the GLB.
```json
{
  "region": "us-east-1",
  "target_type": "IP",
  "target_ip": "1.2.3.4",
  "weight": 100
}
```

### DELETE /global-lb/:id/endpoints/:epID
Remove an endpoint from the GLB.

---

## Auto-Scaling Groups

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /autoscaling/groups
List auto-scaling groups.

### POST /autoscaling/groups
Create an ASG.

---

## Cloud Gateway

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /routes
List all registered routes.

### POST /routes
Register a new gateway route. Supports advanced pattern matching and HTTP method filtering.

**Request:**
```json
{
  "name": "users-api",
  "path_prefix": "/users/{id}",
  "target_url": "http://user-service:8080",
  "methods": ["GET", "PUT"],
  "strip_prefix": true,
  "priority": 10
}
```

**Fields:**
- `path_prefix`: The pattern to match (e.g., `/api/*`, `/users/{id}`, `/id/{id:[0-9]+}`).
- `methods`: Array of allowed HTTP methods (empty/null = all).
- `priority`: Higher values take precedence when multiple patterns match.
- `strip_prefix`: If true, the matched part of the path is removed before forwarding.

**Extracted Parameters:**
Matched parameters like `{id}` are made available to downstream services as headers (in the future) and are currently injected into the gateway context.

### DELETE /routes/:id
Remove a route.

---

## Cloud Functions

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /functions
List all deployed functions.

### POST /functions
Create a new function.
```json
{
  "name": "hello-world",
  "runtime": "nodejs20",
  "code_zip": "<base64>"
}
```

### POST /functions/:id/invoke
Invoke a function.
```json
{
  "payload": { "foo": "bar" },
  "async": false
}
```

### GET /functions/:id/logs
Get execution logs.

### PATCH /functions/:id
Update a function's configuration (timeout, memory, handler, environment variables).

**Environment variables** — each entry can be either a plain-text value or a secret reference, but not both:
```json
{
  "handler": "newhandler.js",
  "timeout": 300,
  "memory_mb": 256,
  "env_vars": [
    { "key": "FOO", "value": "bar" },
    { "key": "API_KEY", "secret_ref": "@my-api-key" }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `key` | string | Env var name |
| `value` | string | Plain-text value (mutually exclusive with `secret_ref`) |
| `secret_ref` | string | Secret name reference with `@` prefix (e.g. `"@my-api-key"`) |

### GET /function-schedules
List all function schedules.

### POST /function-schedules
Create a scheduled function invocation.
```json
{
  "function_id": "uuid",
  "name": "nightly-processing",
  "schedule": "0 2 * * *",
  "payload": {}
}
```

### GET /function-schedules/:id
Get a specific schedule.

### DELETE /function-schedules/:id
Delete a schedule.

### POST /function-schedules/:id/pause
Pause a schedule.

### POST /function-schedules/:id/resume
Resume a paused schedule.

### GET /function-schedules/:id/runs
Get run history for a schedule.

---

## CloudLogs (Persistent Logs) 🆕

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /logs
Search and filter historical platform logs.

**Query Parameters:**
- `resource_id`: Filter by specific resource UUID.
- `resource_type`: Filter by type (`instance`, `function`).
- `level`: Filter by severity (`INFO`, `WARN`, `ERROR`).
- `search`: Keyword search in log messages.
- `start_time`: RFC3339 start timestamp.
- `end_time`: RFC3339 end timestamp.
- `limit`: Max results (default 100).
- `offset`: Pagination offset.

### GET /logs/:resource_id
Get historical logs for a specific resource.

**Parameters:**
| Name | In | Type | Required | Description |
|------|----|------|----------|-------------|
| `resource_id` | path | string | Yes | The UUID of the resource (instance or function) |
| `limit` | query | integer | No | Max logs to return |

**Example:**
```bash
curl -H "X-API-Key: $API_KEY" http://api.thecloud.local/logs/a1b2c3d4-5678-90ab-cdef-1234567890ab?limit=10
```

---

## Cloud Cache (Redis)

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /caches
List all cache instances.

### POST /caches
Provision a new Redis cache.
```json
{
  "name": "my-cache",
  "memory_mb": 256
}
```

### DELETE /caches/:id
Terminate a cache instance.

### POST /caches/:id/resize
Resize a cache instance's memory allocation (requires docker compute backend).

**Request:**
```json
{
  "memory_mb": 512
}
```

**Response:**
```json
{
  "message": "cache resized",
  "memory_mb": 512
}
```

**Error Responses:**
- `400` — Invalid input (memory must be larger than current, docker backend required)
- `404` — Cache not found

---

## Cloud Queue

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /queues
List all queues.

### POST /queues
Create a new message queue.
```json
{
  "name": "task-queue",
  "visibility_timeout": 30
}
```

### POST /queues/:id/messages
Send a message.
```json
{
  "body": "payload-data"
}
```

### GET /queues/:id/messages
Receive messages.
Query params: `?count=1`

---

## Cloud Notify (Pub/Sub)

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /topics
List topics.

### POST /topics
Create a new topic.

### POST /subscriptions
Subscribe to a topic.
```json
{
  "topic_id": "uuid",
  "protocol": "webhook",
  "endpoint": "http://my-api/hook"
}
```

---

## Cloud Cron

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /cron
List scheduled jobs.

### POST /cron
Create a scheduled job.
```json
{
  "name": "daily-cleanup",
  "schedule": "0 0 * * *",
  "target_url": "http://my-service/cleanup"
}
```

### POST /cron/:id/pause
Pause a job.

### POST /cron/:id/resume
Resume a job.

---

## Cloud IAM (Policies) 🆕

**Headers Required:** `X-API-Key: <your-api-key>`
**Constraint**: These endpoints require `PermissionFullAccess` (Admin).

### GET /iam/policies
List all IAM policies.

### POST /iam/policies
Create a new granular IAM policy.
```json
{
  "name": "ReadOnlyS3",
  "statements": [
    {
      "effect": "Allow",
      "action": ["storage:list", "storage:get"],
      "resource": ["*"]
    }
  ]
}
```

### GET /iam/policies/:id
Get details of a specific policy.

### DELETE /iam/policies/:id
Delete a policy.

### POST /iam/users/:userId/policies/:policyId
Attach a policy to a specific user.

### DELETE /iam/users/:userId/policies/:policyId
Detach a policy from a user.

### GET /iam/users/:userId/policies
List all policies attached to a specific user.

---

## Accounting & Billing 🆕

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /billing/summary
Get billing summary for the current period.

**Response:**
```json
{
  "period_start": "2026-01-01T00:00:00Z",
  "period_end": "2026-01-31T23:59:59Z",
  "total_amount": 150.25,
  "currency": "USD",
  "usage_by_type": {
    "compute": 45.50,
    "storage": 12.75,
    "database": 92.00
  }
}
```

### GET /billing/usage
List detailed usage records.

---

## Audit Logs 🆕

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /audit
List platform audit logs for the authenticated user/tenant.

**Query Parameters:**
- `limit`: Max results (default 50).

**Response:**
```json
[
  {
    "id": "uuid",
    "timestamp": "2026-01-05T23:00:00Z",
    "actor": "user@example.com",
    "action": "INSTANCE_LAUNCH",
    "resource": "instance/a1b2c3d4",
    "status": "success",
    "ip_address": "1.2.3.4"
  }
]
```

---

## Managed Kubernetes (KaaS)

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /clusters
List all Kubernetes clusters.

### POST /clusters
Create a new Kubernetes cluster (Asynchronous).
```json
{
  "name": "my-cluster",
  "vpc_id": "vpc-uuid",
  "version": "v1.29.0",
  "workers": 3,
  "ha": true
}
```

### GET /clusters/:id
Get detailed information and status.

### DELETE /clusters/:id
Delete a cluster (Asynchronous).

### GET /clusters/:id/kubeconfig
Download the admin kubeconfig.

### GET /clusters/:id/health
Get operational health (nodes ready, api server reachability).

### POST /clusters/:id/upgrade
Upgrade cluster version (Asynchronous).
```json
{
  "version": "v1.30.0"
}
```

### POST /clusters/:id/scale
Scale worker nodes.
```json
{
  "workers": 5
}
```

### POST /clusters/:id/repair
Re-run bootstrap scripts on nodes.

### POST /clusters/:id/backups
Trigger an etcd backup.

### POST /clusters/:id/restore
Restore etcd state from path.
```json
{
  "backup_path": "/backups/etcd-snapshot.db"
}
```

## Error Codes

| Status Code | Description |
|-------------|-------------|
| 200/201 | Success |
| 400 | Bad Request (Invalid input) |
| 401 | Unauthorized (Missing/Invalid API Key) |
| 403 | Forbidden (Access denied to resource) |
| 404 | Not Found |
| 429 | Too Many Requests (Rate limit exceeded) |
| 500 | Internal Server Error |
