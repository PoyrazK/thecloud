# ADR 029: Gateway Security and Resilience

## Status
Accepted

## Date
2026-05-19

## Context

PR #596 extends the API gateway with security (JWT, mTLS), resilience (circuit breaker, retry), and observability features. These require non-trivial decisions about how external trust is established, how failures are handled, and how observability data is exposed.

## Decision

### JWT Authentication with JWKS

The gateway validates JWTs by fetching public keys from a JWKS endpoint. Because multiple routes may share the same JWKS URL (or different routes may have different JWKS URLs), we use a **per-route JWKS cache** with a 5-minute TTL. JWKS fetches are deduplicated using `singleflight.Group` so concurrent requests for the same keyset result in a single HTTP call.

**JWKS circuit breaker**: When the JWKS endpoint returns errors, a circuit breaker (`Threshold: 3`, `ResetTimeout: 30s`) prevents further fetch attempts until the half-open probe succeeds. Metrics exported: `JWKSBreakerState` (gauge 0/1/2) and `JWKSFetchTotal` (counter with labels: `success`, `error`, `circuit_open`).

**RSA key parsing**: Keys are parsed from JWK `n` (modulus) and `e` (exponent) using `math/big.Int`. An exponent `>= 1<<30` is rejected as a defensive measure against overflow.

**Claim propagation**: Validated JWT claims are forwarded to the upstream service as `X-JWT-Claim-{key}` headers.

### mTLS Configuration

Routes can be configured with `client_cert` + `client_key` (PEM) for client certificates and `ca_cert` for backend certificate verification. Both client cert and key must be provided together; CA cert is optional. `buildTLSConfig` returns descriptive errors on malformed certs/keys rather than silently ignoring them.

### Circuit Breaker and Retry

Each route can have `circuit_breaker_threshold` (consecutive failures to trip) and `circuit_breaker_timeout` (ms in open before half-open). Retry behavior is controlled by `max_retries` and `retry_timeout`. Retries are only attempted for idempotent methods (GET, HEAD, PUT, DELETE, OPTIONS) and only on retryable status codes (502, 503, 504, 429) or retryable errors (connection refused, timeout, reset by peer, broken pipe).

### CORS

Per-route CORS uses `allowed_origins`, `allowed_methods`, `allowed_headers`, `expose_headers`, and `max_age`. When `allowed_origins` includes `"*"` with credentials, the response sets `Access-Control-Allow-Credentials: true`.

## Consequences

### Positive
- JWT authentication enables stateless auth with upstream services trusting gateway-validated tokens
- JWKS singleflight deduplication prevents thundering herd on shared JWKS endpoints
- Circuit breaker protects gateway from cascading failures when backends are unhealthy
- mTLS enables secure service-to-service communication within the cluster

### Negative
- JWKS caching introduces staleness window (up to 5 minutes for rotated keys)
- Circuit breaker state is in-memory only — restarts reset all circuits
- Retry behavior increases latency on failure and may amplify load on unhealthy backends

### Neutral
- CORS headers are only injected when `allowed_origins` is non-empty
- TLS settings (skip verify, require TLS, client certs) are independent concerns

## Alternatives Considered

### Alternative 1: Opaque JWT validation without JWKS caching
**Why rejected:** Would require fetching JWKS on every request, defeating the purpose of JWT's stateless design and creating unnecessary latency.

### Alternative 2: Global circuit breaker for all backends
**Why rejected:** Per-route circuit breakers provide finer-grained isolation — one unhealthy backend shouldn't trip the breaker for unrelated routes.

### Alternative 3: Retry on any HTTP error
**Why rejected:** Non-idempotent methods (POST, PATCH) must not be retried automatically as that could cause duplicate operations. The current implementation limits retries to idempotent methods and specific error codes.