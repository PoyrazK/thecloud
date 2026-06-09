# API Gateway Improvement Roadmap

Prioritized by: Impact × Effort

---

## Phase 1: Quick Wins (Low Effort, High Impact)

### 1.1 Add Upstream Timeouts
**Impact:** Prevents gateway hangs on backend issues
**Effort:** Low
**Status:** ✅ Shipped

- Add `DialTimeout`, `ResponseHeaderTimeout`, `IdleConnTimeout` to `ReverseProxy` transport
- Add `ProxyTimeout` to `Config` (default 30s)
- File: `internal/core/services/gateway.go`

### 1.2 Per-Route Rate Limiting
**Impact:** Currently global only; per-route gives finer control
**Effort:** Low
**Status:** ✅ Shipped in PR #570

- Modify `limiter.go` to support route-key limiting
- Add `RateLimit` field usage from `GatewayRoute` (already exists in domain)
- File: `pkg/ratelimit/limiter.go`, `internal/handlers/gateway_handler.go`

### 1.3 IP Allowlist/Denylist
**Impact:** Basic security hardening
**Effort:** Low
**Status:** ✅ Shipped

- Add `AllowedCIDRs`, `BlockedCIDRs` fields to `GatewayRoute`
- Implement CIDR matching middleware
- File: `internal/core/domain/gateway.go`, `internal/handlers/gateway_handler.go`

### 1.4 Request Size Limits
**Impact:** DoS prevention
**Effort:** Low
**Status:** ✅ Shipped

- Add `MaxBodySize` field to `GatewayRoute`
- Apply `maxBytes.ReadLimit` in proxy handler
- File: `internal/core/domain/gateway.go`, `internal/handlers/gateway_handler.go`

### 1.5 TLS Configuration for Backends
**Impact:** Secure internal HTTPS communication
**Effort:** Low
**Status:** ✅ Shipped

- Add `TLSSkipVerify`, `RequireTLS` fields to `GatewayRoute`
- Configure `httputil.ReverseProxy` with custom TLS config
- File: `internal/core/domain/gateway.go`, `internal/core/services/gateway.go`

### 1.6 Trace Header Propagation
**Impact:** Distributed tracing works end-to-end
**Effort:** Low
**Status:** ✅ Shipped

- Inject W3C TraceContext headers (`traceparent`, `tracestate`) in proxy handler
- Ensure `X-Request-ID` is passed to backends
- File: `internal/handlers/gateway_handler.go`

---

## Phase 2: Resilience (High Impact)

### 2.1 Circuit Breaker Per Route
**Impact:** Prevents cascading failures
**Effort:** Medium
**Status:** ✅ Shipped in PR #395

- Wire up existing `platform/circuit_breaker.go` to gateway routes
- Add `CircuitBreakerThreshold`, `CircuitBreakerTimeout` to `GatewayRoute` or config
- Track CB state in `GatewayService`
- File: `internal/core/services/gateway.go`, `internal/platform/circuit_breaker.go`

### 2.2 Retry on Upstream Failure
**Impact:** Handle transient 502/503/504 errors
**Effort:** Medium
**Status:** ✅ Shipped in PR #395

- Use existing `platform/retry.go` utilities
- Add `MaxRetries`, `RetryTimeout` to `GatewayRoute`
- Retry on idempotent methods (GET, HEAD, PUT, DELETE)
- File: `internal/core/services/gateway.go`, `internal/platform/retry.go`

### 2.3 Connection Pooling
**Impact:** Better HTTP performance
**Effort:** Low
**Status:** ✅ Shipped

- Create shared `http.Transport` with connection pooling config
- Configure `MaxIdleConns`, `MaxIdleConnsPerHost`, `IdleConnTimeout`
- File: `internal/core/services/gateway.go`

### 2.4 Health Check & Auto-Failover
**Impact:** Automatic recovery from backend failures
**Effort:** High

- Add health check configuration to `GatewayRoute` (`HealthCheckPath`, `HealthCheckInterval`)
- Background worker to check upstream health
- Mark unhealthy routes and fallback if multiple targets exist
- File: `internal/core/services/gateway.go`, `internal/workers/`

---

## Phase 3: Traffic Management (Medium Impact)

### 3.1 Canary Deployments
**Impact:** Safe production rollouts
**Effort:** High

- Add `Weight` field to routes (0-100)
- Support multiple routes with same pattern, different weights
- Traffic splitter logic in `GetProxy`
- File: `internal/core/domain/gateway.go`, `internal/core/services/gateway.go`

### 3.2 Blue-Green Deployments
**Impact:** Instant rollback capability
**Effort:** High

- Add `RouteGroup` concept (routes belong to groups)
- Add `Active` boolean to routes
- Atomic switch via management API
- File: `internal/core/domain/gateway.go`, `internal/handlers/gateway_handler.go`

### 3.3 Traffic Mirroring
**Impact:** Test against real traffic without impact
**Effort:** Medium

- Add `MirrorTarget` to `GatewayRoute`
- Fork requests to mirror target (async, don't wait for response)
- File: `internal/handlers/gateway_handler.go`

---

## Phase 4: Security (High Impact)

### 4.1 CORS Fine-Tuning Per Route
**Impact:** Proper CORS instead of wildcard
**Effort:** Low
**Status:** ✅ Shipped

- Add `AllowedOrigins`, `AllowedMethods`, `AllowedHeaders`, `ExposeHeaders`, `MaxAge` to `GatewayRoute`
- Replace global CORS middleware with route-level
- File: `internal/core/domain/gateway.go`, `internal/handlers/gateway_handler.go`

### 4.2 Header Filtering
**Impact:** Prevent information leakage
**Effort:** Low
**Status:** ✅ Shipped

- Add `StripResponseHeaders` list to `GatewayRoute`
- Remove headers like `X-Powered-By`, `Server`, `X-AspNet-Version`
- File: `internal/core/domain/gateway.go`, `internal/handlers/gateway_handler.go`

### 4.3 JWT Validation
**Impact:** Auth at gateway level
**Effort:** Medium
**Status:** ✅ Shipped

- Add `JWTIssuer`, `JWTAudience`, `JWTJwksURL` to `GatewayRoute`
- Validate JWT before proxying, inject claims as headers
- File: `internal/core/domain/gateway.go`, `internal/handlers/gateway_handler.go`, `internal/core/services/gateway.go`

### 4.4 mTLS for Backend Communication
**Impact:** Secure service-to-service communication
**Effort:** Medium
**Status:** ✅ Shipped

- Add `ClientCert`, `ClientKey`, `CACert` fields for mTLS
- Configure TLS handshake with client certificates in `buildTLSConfig()`
- File: `internal/core/domain/gateway.go`, `internal/core/services/gateway.go`

---

## Phase 5: Observability (High Impact)

### 5.1 Per-Route Metrics
**Impact:** Debug which route is slow/failing
**Effort:** Low
**Status:** ✅ Shipped

- Add `route_id` label to existing `HTTPRequestsTotal` and `HTTPRequestDuration`
- Track upstream response codes per route
- File: `internal/handlers/gateway_handler.go`, `pkg/httputil/middleware.go`

### 5.2 Upstream Latency Metrics
**Impact:** Know when backends are slow
**Effort:** Low
**Status:** ✅ Shipped

- Time the proxy roundtrip
- Export `gateway_upstream_latency_seconds` histogram
- File: `internal/handlers/gateway_handler.go`

### 5.3 Circuit Breaker State Metrics
**Impact:** Monitor resilience state
**Effort:** Low
**Status:** ✅ Shipped

- Export CB state as Prometheus gauge (0=closed, 1=open, 2=half-open)
- Per-route labeled
- File: `internal/core/services/gateway.go`, `internal/platform/circuit_breaker.go`

### 5.4 Retry Attempt Metrics
**Impact:** Visibility into retry operations
**Effort:** Low
**Status:** ✅ Shipped

- Counter for `gateway_retry_total` with `route_id`, `status` labels
- File: `internal/core/services/gateway.go`

---

## Phase 6: Protocol Support (Medium Impact)

### 6.1 WebSocket Path Routing
**Impact:** Consistent routing for all protocols
**Effort:** Medium

- Integrate WebSocket into same pattern matching system
- Remove separate `ws/handler.go` routing
- File: `internal/handlers/gateway_handler.go`, `internal/handlers/ws/handler.go`

### 6.2 SSE Support
**Impact:** Proxy Server-Sent Events
**Effort:** Low

- Ensure `Transfer-Encoding: chunked` is handled properly
- Add `EnableSSE` flag if needed
- File: `internal/handlers/gateway_handler.go`

### 6.3 gRPC Gateway
**Impact:** Expose gRPC services
**Effort:** High

- Add `Protocol` field to `GatewayRoute` ("http", "grpc")
- Use `grpc/grpc-go` for gRPC proxying
- Handle HTTP/2 and gRPC framing
- File: `internal/core/domain/gateway.go`, `internal/handlers/gateway_handler.go`

---

## Phase 7: Configuration & Management (Medium Impact)

### 7.1 Route Versioning
**Impact:** Change history and rollback
**Effort:** Medium

- Add `Version` field to routes
- Store route history in separate table
- Management API to list versions and rollback
- File: `internal/repositories/postgres/gateway_repo.go`

### 7.2 Dry-Run Mode
**Impact:** Test routes before applying
**Effort:** Low
**Status:** ✅ Shipped

- Add `?dry_run=true` to `POST /gateway/routes`
- Validate route config without persisting
- File: `internal/handlers/gateway_handler.go`

### 7.3 YAML/JSON Config Import
**Impact:** Configuration-as-code
**Effort:** Medium

- Add `POST /gateway/routes/import` endpoint
- Accept batch route definitions
- Validate and apply in transaction
- File: `internal/handlers/gateway_handler.go`

### 7.4 Route Health Dashboard
**Impact:** Visual health overview
**Effort:** Medium

- Add `GET /gateway/health` endpoint
- Return per-route health status from background checker
- File: `internal/handlers/gateway_handler.go`

---

## Phase 8: Performance (Low-Medium Impact)

### 8.1 Response Caching
**Impact:** Reduce backend load for immutable responses
**Effort:** High

- Add `CacheTTL`, `CacheKeyHeaders` to `GatewayRoute`
- In-memory or Redis cache for GET responses
- Vary header support
- File: `internal/core/services/gateway.go`, `internal/handlers/gateway_handler.go`

### 8.2 Compression
**Impact:** Reduce bandwidth
**Effort:** Low
**Status:** ✅ Shipped

- Add `Compression` field (gzip, br, deflate)
- Compress responses if client accepts encoding
- File: `internal/handlers/gateway_handler.go`

### 8.3 Request Coalescing (Singleflight)
**Impact:** Prevent thundering herd
**Effort:** Medium

- Use `golang.org/x/sync/singleflight` for identical concurrent requests
- Key by route + path + query
- File: `internal/handlers/gateway_handler.go`

### 8.4 Route Preload on Startup
**Impact:** No cold start latency
**Effort:** Low

- Eager load all routes on service start
- Remove lazy loading in `GetProxy`
- File: `internal/core/services/gateway.go`

---

## Implementation Order

```
Phase 1 (Quick Wins)
├── 1.1 Add Upstream Timeouts ✅
├── 1.2 Per-Route Rate Limiting ✅
├── 1.3 IP Allowlist/Denylist ✅
├── 1.4 Request Size Limits ✅
├── 1.5 TLS Configuration ✅
└── 1.6 Trace Header Propagation ✅

Phase 2 (Resilience)
├── 2.1 Circuit Breaker Per Route ✅
├── 2.2 Retry on Upstream Failure ✅
├── 2.3 Connection Pooling ✅
└── 2.4 Health Check & Auto-Failover

Phase 5 (Observability - can parallelize with Phase 2)
├── 5.1 Per-Route Metrics ✅
├── 5.2 Upstream Latency Metrics ✅
├── 5.3 Circuit Breaker State Metrics ✅
└── 5.4 Retry Attempt Metrics ✅

Phase 4 (Security - can parallelize with Phase 2)
├── 4.1 CORS Fine-Tuning ✅
├── 4.2 Header Filtering ✅
├── 4.3 JWT Validation ✅
└── 4.4 mTLS for Backends ✅

Phase 3 (Traffic Management)
├── 3.1 Canary Deployments
├── 3.2 Blue-Green Deployments
└── 3.3 Traffic Mirroring

Phase 6 (Protocol)
├── 6.1 WebSocket Path Routing
├── 6.2 SSE Support
└── 6.3 gRPC Gateway

Phase 7 (Config & Management)
├── 7.1 Route Versioning
├── 7.2 Dry-Run Mode ✅
├── 7.3 YAML/JSON Import
└── 7.4 Route Health Dashboard

Phase 8 (Performance)
├── 8.1 Response Caching
├── 8.2 Compression ✅
├── 8.3 Request Coalescing
└── 8.4 Route Preload
```

---

## Success Metrics

| Phase | Metric |
|-------|--------|
| Phase 1 | Zero gateway hangs on slow backends |
| Phase 2 | ✅ 0 cascade failures in故障 scenarios |
| Phase 3 | <1% failed deployments via canary |
| Phase 4 | Zero unauthorized IP access |
| Phase 5 | p99 latency visible per route |
| Phase 6 | gRPC support shipped |
| Phase 7 | Route changes auditable |
| Phase 8 | p95 latency reduced by 20% |