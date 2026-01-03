# Next Steps and Refactor Ideas

This document proposes actionable next steps based on recent work (mostly expanded service-level tests) and the current roadmap. It is meant to guide what to add or refactor next.

## What Changed Recently
- Added broad unit test coverage across services (cache, instance, autoscaling, functions, database, event, identity, VPC, auth, secrets, load balancer).
- Removed internal comments in tests; overall test suite is growing and more systematic.

## Suggested Additions (Feature Work)
1. RBAC (Role-Based Access Control)
   - Define roles, scopes, and permissions model.
   - Add CLI flags for role assignment and introspection.
   - Update API middleware to enforce authorization.
2. Web Dashboard (Next.js)
   - Read-only views first (instances, logs, metrics, events).
   - Auth with API keys or session-bound tokens.
3. Database-as-a-Service (RDS)
   - Instance lifecycle API parity with compute (create/list/show/delete/logs).
   - Connection string/credentials management, rotation policy.
4. CLI Quality-of-Life
   - Output formatting options (json/table), consistent error codes.
   - Shell autocomplete, config profiles, and env overrides.

## Refactor Opportunities
1. Test Harness Consistency
   - Consolidate repeated setup/fixtures into shared helpers under `internal/core/services/testhelpers`.
   - Standardize table-driven patterns and error assertions.
2. Service Interface Alignment
   - Ensure services expose a consistent CRUD and "show/logs/stats" shape where applicable.
   - Normalize pagination parameters (limit/offset) across list endpoints.
3. Error Model
   - Introduce typed errors for not-found, validation, and provider failures.
   - Map typed errors to CLI exit codes and API HTTP status consistently.
4. Eventing and Observability
   - Centralize event emission around resource lifecycle hooks.
   - Add structured logging fields (resource_id, tenant_id, op, duration_ms).

## Testing and Validation
- Add integration tests for CLI commands that wrap core services.
- Add end-to-end tests for multi-step flows (create -> logs -> stats -> delete).
- Add contract tests for API routes (request/response schema).
- Add negative tests for authorization and forbidden access.

## Documentation Follow-ups
- Add "Testing Strategy" doc: how unit/integration/e2e are organized.
- Update `docs/roadmap.md` with timelines for RBAC, Dashboard, RDS.
- Add developer runbook for common local setups (docker, env vars).

## Recommended Short-Term Plan (2-4 weeks)
1. Unify test helpers and error typing.
2. Ship RBAC core model with minimal CLI support.
3. Add integration tests for top 5 CLI commands.
4. Draft dashboard read-only routes (instances/events).

## Tailored Plans (RBAC, Dashboard, RDS)

### RBAC (Role-Based Access Control)
**Goal:** enforce consistent, auditable authorization across API and CLI.

**Scope and model**
- Roles: `owner`, `admin`, `developer`, `viewer`.
- Resources: `instances`, `databases`, `functions`, `secrets`, `networks`, `storage`, `events`, `auth`.
- Actions: `create`, `read`, `update`, `delete`, `execute` (for logs/stats).
- Use a `role -> permissions[]` mapping and a `principal -> role` assignment.

**Deliverables**
- Authorization middleware enforcing `resource:action`.
- CLI support for role assignment and inspection.
- API endpoints for listing roles and permissions.
- Audit events for role changes.

**Tests**
- Unit: permission matrix coverage, deny-by-default, invalid role/action.
- Integration: CLI role assignment, forbidden access returns correct status/exit code.

**Docs**
- Add a short RBAC guide and update `docs/cli-reference.md`.

### Dashboard (Next.js)
**Goal:** read-only operational visibility first; iterate to management actions later.

**Phase 1 (Read-only)**
- Pages: Instances, Logs, Metrics, Events, Networks.
- Data sources: existing API endpoints, default to paginated lists.
- Auth: API key input + local storage (short-lived) or session token.

**Phase 2 (Light control)**
- Actions: stop/start instance, view details, copy connection strings.
- Confirmation dialogs and safe defaults.

**Tests**
- API contract tests for list endpoints used by UI.
- Basic UI smoke tests for routing and data fetch.

**Docs**
- Add `docs/guides/dashboard.md` with setup and auth flow.

### RDS (Database-as-a-Service)
**Goal:** parity with compute lifecycle + secure credential handling.

**Phase 1 (Core lifecycle)**
- Create/list/show/delete database instances.
- Connection string retrieval.
- Basic logs endpoint (if available).

**Phase 2 (Security and ops)**
- Credential rotation and revocation.
- Backup/restore hooks (design only if implementation is heavy).

**Tests**
- Unit: validation, not-found, credentials formatting.
- Integration: create -> get -> connection string -> delete.

**Docs**
- Update `docs/database.md` and add CLI examples to `docs/cli-reference.md`.

## Feature Work Done (Best Practices Checklist)
Use this checklist when declaring a feature "done".

- Scope, API, and CLI changes documented in a brief spec or PRD.
- Unit tests added for all new service logic, with edge cases.
- Integration tests added for the primary CLI/API flows.
- Error handling uses typed errors and consistent status/exit codes.
- Logging/events include resource IDs and action context.
- Backward compatibility checked; migrations or deprecations noted.
- Documentation updated (guides, CLI reference, roadmap if needed).
