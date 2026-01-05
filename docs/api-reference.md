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

  "expires_in": 86400
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

### POST /instances
Launch a new instance.
```json
{
  "name": "web-01",
  "image": "nginx",
  "vpc_id": "vpc-uuid"
}
```

### GET /instances/:id
Get details of a specific instance.

### PUT /instances/:id
Update instance (e.g., status).

### DELETE /instances/:id
Terminate an instance.

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

---

## Load Balancers

**Headers Required:** `X-API-Key: <your-api-key>`

### GET /loadbalancers
List load balancers.

### POST /loadbalancers
Create a load balancer.

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
Register a new route.
```json
{
  "name": "auth-service",
  "prefix": "/auth",
  "target": "http://auth-service:8080",
  "strip_prefix": true
}
```

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
