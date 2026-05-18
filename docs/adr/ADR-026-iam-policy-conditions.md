# ADR 026: IAM Policy Condition Evaluation

## Status
Accepted

## Date
2026-05-12

## Context

IAM policies support statements with `Effect` (Allow/Deny), `Action` patterns, and `Resource` patterns. However, the original implementation had no way to evaluate dynamic context such as:
- Source IP address (e.g., allow only from corporate network)
- Current time (e.g., allow during business hours)
- User or tenant attributes (e.g., restrict to specific tenant)

Without conditions, policies could only be binary on/off for matching action/resource.

## Decision

We implemented **condition evaluation** in the IAM policy evaluator, following AWS IAM conventions.

### Condition Structure

Conditions are stored in policy statements as a map:

```json
{
  "effect": "Allow",
  "action": ["instance:*"],
  "resource": ["*"],
  "condition": {
    "IpAddress": {"aws:SourceIp": ["192.168.1.0/24"]},
    "StringEquals": {"thecloud:TenantId": "tenant-123"}
  }
}
```

### Supported Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `IpAddress` | Source IP is in CIDR range | `{"aws:SourceIp": ["192.168.1.0/24"]}` |
| `NotIpAddress` | Source IP is NOT in CIDR | `{"NotIpAddress": {"aws:SourceIp": ["10.0.0.0/8"]}}` |
| `StringEquals` | Exact string match | `{"StringEquals": {"thecloud:TenantId": "tenant-123"}}` |
| `StringNotEquals` | String not equal | `{"StringNotEquals": {"thecloud:TenantId": "other"}}` |
| `StringLike` | Wildcard pattern match | `{"StringLike": {"aws:UserId": "user-*"}}` |
| `StringNotLike` | Wildcard pattern not match | - |
| `DateGreaterThan` | Time after threshold | `{"DateGreaterThan": {"aws:CurrentTime": "2024-01-01T00:00:00Z"}}` |
| `DateLessThan` | Time before threshold | `{"DateLessThan": {"aws:CurrentTime": "2099-01-01T00:00:00Z"}}` |
| `DateEquals` | Time equals threshold | - |
| `Bool` | Boolean match | `{"Bool": {"thecloud:IsAdmin": true}}` |
| `Null` | Key exists check | `{"Null": {"thecloud:SomeKey": "true"}}` (key must NOT exist) |

### Evaluation Context

The `evalCtx` map is built in `RBACService.buildEvalCtx()` with:
- `aws:SourceIp` - Client IP from request
- `aws:UserId` - User UUID
- `aws:CurrentTime` - Current UTC time
- `thecloud:TenantId` - Active tenant UUID

### Evaluation Logic

1. If statement has conditions, ALL must pass for the statement to match
2. If conditions fail, statement is skipped (no effect)
3. If conditions pass, normal Allow/Deny evaluation applies
4. Explicit Deny still wins over Allow

## Consequences

### Positive
- Enables fine-grained access control based on context
- Follows AWS IAM conventions for familiarity
- Supports IP-based restrictions, time-based access, attribute matching
- Existing policies without conditions continue to work unchanged

### Negative
- Evaluation context must be populated at authorization time
- IP spoofing risk via X-Forwarded-For (mitigated with documentation)
- More complex policy evaluation (performance impact negligible)

### Neutral
- Condition keys are extensible for future use cases
- Implementation in separate `evaluateCondition()` method keeps code modular

## Security Considerations

1. **IP Spoofing**: `c.ClientIP()` respects `X-Forwarded-For` header which clients can spoof. Production deployments should configure trusted proxies in Gin engine.
2. **Time-based conditions**: Rely on server time, vulnerable to clock skew
3. **Condition validation**: Invalid condition operators or formats fail gracefully (statement skipped)

## Files Changed

- `internal/core/domain/condition.go` - Condition operators and keys
- `internal/core/services/iam_evaluator.go` - `evaluateCondition()`, `evalIP()`, etc.
- `internal/core/services/rbac.go` - `buildEvalCtx()` for context population
- `pkg/httputil/auth.go` - Source IP capture in middleware
- `internal/core/context/context.go` - `WithSourceIP()` / `SourceIPFromContext()`