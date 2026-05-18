# ADR 028: Gateway Per-Route Rate Limiting

## Status
Accepted

## Date
2026-05-15

## Context
The API gateway previously supported only a single global rate limit applied to all proxied requests. As the gateway evolved to serve multiple independent consumers with different rate requirements, a single global limit became insufficient. Some routes needed stricter limits to protect backend services, while others needed higher limits for trusted clients.

## Decision
We introduced per-route rate limiting that supplements the existing global rate limit. The implementation uses the existing `IPRateLimiter` token bucket with a per-route variant: `GetRouteLimiter(routeID, key, rate, burst)`.

**Implementation:**

1. **Per-route limiter lookup** — `Proxy()` checks if `route.RateLimit > 0` and calls `h.rateLimiter.GetRouteLimiter(route.ID, key, rate.Limit(route.RateLimit), burst)` where `burst = route.RateLimit * 2`.

2. **Two-phase handler initialization** — `GatewayHandler` is set to `nil` in `InitHandlers` and re-initialized in `SetupRouter` after the global limiter is created, allowing the handler to access the limiter for per-route operations.

3. **Key selection** — Same as global: API Key header (masked to first 5 chars) or client IP, enabling per-client tracking within each route.

4. **Burst sizing** — Burst is set to `rateLimit * 2` to allow brief bursting while maintaining the sustained rate floor.

## Consequences

### Positive
- Routes can have independent rate limits without affecting global gateway throughput
- Per-client tracking within routes enables fair sharing among consumers of the same route
- Existing global rate limit continues to protect gateway infrastructure

### Negative
- Increased memory per route (a `map[uuid.UUID]map[string]*rate.Limiter` instead of a flat map)
- Two-phase init required because `IPRateLimiter` is created in `SetupRouter` after handlers are initialized
- Deterministic testing of rate-limit-exceeded paths requires a mock-able `RateLimiter` interface (not yet introduced)

### Neutral
- Rate limit applies only to proxied requests, not management API calls (create/delete routes)
- Burst calculation (`rateLimit * 2`) is heuristic; production tuning may be needed

## Alternatives Considered

### Alternative 1: Separate limiter pool per route
**Why rejected:** Requires managing lifecycle of many independent limiters. The `IPRateLimiter` already supports route-scoped entries via `GetRouteLimiter`.

### Alternative 2: Third-party rate limiting middleware (e.g., ulule/limiter)
**Why rejected:** Introduces new dependency; the existing `golang.org/x/time/rate` meets requirements with minimal added code.

### Alternative 3: Global limiter with route-specific token deductions
**Why rejected:** Complex to reason about; one route's burst could starve another. Independent limiters per route provide cleaner semantics.
