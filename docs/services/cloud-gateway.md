CloudGateway provides entry-point routing, rate limiting, JWT authentication, mTLS, and observability for your cloud infrastructure.

## Implementation
- **Core Engine**: Built using Go's standard `net/http/httputil.ReverseProxy` with a custom routing engine.
- **Advanced Path Matching**: Supports pattern-based routing with parameter extraction (regex-backed).
- **Dynamic Routing**: Routes are stored in PostgreSQL and cached in-memory. The service pre-compiles route patterns for sub-millisecond matching.
- **Path Stripping**: Optional prefix stripping (e.g., `/gw/v1/users` -> target: `/users`).
- **Rate Limiting**: Per-route rate limiting enforced at the gateway layer using token-bucket algorithm.
- **Circuit Breaker & Retry**: Per-route circuit breaker with configurable threshold and timeout. Automatic retry with exponential backoff for idempotent methods on `502`, `503`, `504`, `429`.
- **JWT Authentication**: JWKS-backed validation with issuer/audience verification. Claims propagated to upstream via `X-JWT-Claim-*` headers.
- **mTLS**: Configurable client certificates and CA certs for backend TLS verification.
- **CORS**: Per-route CORS configuration with origins, methods, headers, exposed headers, and max-age.
- **Compression**: Gzip response compression when client advertises support.
- **Observability**: Prometheus metrics for upstream latency, retry totals, circuit breaker state, and JWKS fetch operations.

## Route Configuration

Routes are created via `POST /gateway/routes` with the following configurable fields:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Human-readable route name |
| `path_prefix` | string | Path pattern to match (e.g., `/api/v1`, `/users/{id}`) |
| `target_url` | string | Backend URL to proxy to |
| `methods` | []string | Allowed HTTP methods (empty = all) |
| `strip_prefix` | bool | Strip matched prefix before forwarding |
| `rate_limit` | int | Max requests per second per client |
| `max_body_size` | int64 | Max request body in bytes |
| `dial_timeout` | int64 | TCP dial timeout in milliseconds |
| `idle_conn_timeout` | int64 | Idle connection timeout in milliseconds |
| `tls_skip_verify` | bool | Skip TLS verification for backend |
| `require_tls` | bool | Force HTTPS for backend |
| `allowed_cidrs` | []string | IP allowlist (empty = all allowed) |
| `blocked_cidrs` | []string | IP blocklist (checked before allowlist) |
| `circuit_breaker_threshold` | int | Failures to trip circuit breaker (0 = disabled) |
| `circuit_breaker_timeout` | int64 | Milliseconds in open state before half-open |
| `max_retries` | int | Max retry attempts (0 = disabled) |
| `retry_timeout` | int64 | Total retry window in milliseconds |
| `allowed_origins` | []string | CORS allowed origins |
| `allowed_methods` | []string | CORS allowed methods |
| `allowed_headers` | []string | CORS allowed headers |
| `expose_headers` | []string | CORS exposed headers |
| `max_age` | int | CORS preflight cache duration in seconds |
| `strip_response_headers` | []string | Headers to remove from upstream responses |
| `compression` | string | `gzip`, `br`, or `deflate` (empty = disabled) |
| `jwt_issuer` | string | Expected JWT issuer claim |
| `jwt_jwks_url` | string | JWKS endpoint for public key fetching |
| `jwt_audience` | string | Expected JWT audience claim |
| `client_cert` | string | PEM-encoded client certificate for mTLS |
| `client_key` | string | PEM-encoded private key for mTLS |
| `ca_cert` | string | PEM-encoded CA certificate for backend verification |

### Dry-Run Validation

Create routes with `?dry_run=true` to validate configuration without persisting:

```bash
curl -X POST /gateway/routes?dry_run=true \
  -H "Content-Type: application/json" \
  -d '{"name":"test","path_prefix":"/api","target_url":"http://backend:8080"}'
```

Returns `{ "dry_run": true, "valid": true }` or `{ "dry_run": true, "valid": false, "errors": [...] }`.

## JWT Authentication

When a route has `jwt_jwks_url` configured:

1. Gateway extracts `Authorization: Bearer <token>` header
2. Fetches and caches JWKS from the configured URL (5-minute TTL)
3. Validates token signature against fetched public keys
4. Verifies `iss` and `aud` claims match configuration
5. On success, propagates claims as `X-JWT-Claim-{key}` headers to upstream

JWKS fetches are deduplicated via singleflight to prevent thundering herd. A circuit breaker (3 failures, 30s reset) protects against unhealthy JWKS endpoints. Metrics exported: `thecloud_jwks_fetch_total` (success/error/circuit_open), `thecloud_jwks_breaker_state`.

## Pattern Matching Syntax

CloudGateway supports powerful pattern-based routing:

| Pattern Type | Syntax | Example Path | Extracted Params |
|--------------|--------|--------------|------------------|
| **Wildcard** | `/api/v1/*` | `/api/v1/users/list` | None |
| **Parameter** | `/users/{id}` | `/users/123` | `id=123` |
| **Regex Param**| `/id/{id:[0-9]+}` | `/id/456` | `id=456` |
| **Extension** | `/files/*.{ext}` | `/files/img.png` | `ext=png` |

### Routing Priority

Routes are evaluated based on **Specificity Scoring**:
1. Exact path matches are prioritized.
2. Longest pattern matches win tie-breaks.
3. Explicit `priority` field can be used for manual overrides.

## Observability Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `thecloud_gateway_upstream_latency_seconds` | histogram | route_id, method, path | Upstream request duration |
| `thecloud_gateway_retry_total` | counter | route_id, status | Retry attempts |
| `thecloud_gateway_circuit_breaker_state` | gauge | route_id | Circuit breaker state (0=closed, 1=open, 2=half-open) |
| `thecloud_jwks_fetch_total` | counter | status | JWKS fetch attempts |
| `thecloud_jwks_breaker_state` | gauge | - | JWKS circuit breaker state |
| `thecloud_http_requests_total` | counter | method, path, status | Total HTTP requests |
| `thecloud_http_request_duration_seconds` | histogram | method, path | HTTP request duration |

## CLI Usage

```bash
# Basic prefix mapping
cloud gateway create-route identity /auth http://identity-service:8080 --strip

# Parameterized routing
cloud gateway create-route user-api "/users/{id}" http://user-service:8080

# Constrained parameter matching (numbers only)
cloud gateway create-route post-api "/posts/{pid:[0-9]+}" http://posts-service:8080

# Route with JWT authentication
cloud gateway create-route api "/api" http://backend:8080 \
  --jwt-issuer "https://auth.example.com" \
  --jwt-jwks-url "https://auth.example.com/.well-known/jwks.json" \
  --jwt-audience "my-api"

# Route with mTLS
cloud gateway create-route secure-api "/secure" http://backend:8080 \
  --client-cert @/path/to/cert.pem \
  --client-key @/path/to/key.pem \
  --ca-cert @/path/to/ca.pem

# Route with circuit breaker and retry
cloud gateway create-route resilient-api "/resilient" http://backend:8080 \
  --circuit-breaker-threshold 5 \
  --circuit-breaker-timeout 30000 \
  --max-retries 3 \
  --retry-timeout 5000

# Route with rate limiting (100 req/sec, burst 200)
cloud gateway create-route limited-api "/limited" http://backend:8080 \
  --rate-limit 100

# Route with CORS
cloud gateway create-route cors-api "/cors" http://backend:8080 \
  --allowed-origins "https://app.example.com" \
  --allowed-methods "GET,POST,PUT" \
  --allowed-headers "Authorization,Content-Type" \
  --max-age 3600

# Dry-run validation
cloud gateway create-route --dry-run ...

# List all active patterns
cloud gateway list-routes
```
