# 🗺️ Mini AWS: Future Roadmap for Agents

This document provides a detailed technical roadmap for the remaining phases of the Mini AWS project.

## 🏁 Current State
- **Phase 5 (Console)**: Backend foundation is complete.
  - Resource aggregation service implemented.
  - Real-time streaming (SSE & WebSockets) ready.
  - Dashboard API verified with 100% test coverage.
  - Infrastructure: Postgres moved to port 5433, Auto-migrations added.

---

## 📅 Upcoming Sprints

### Phase 5: The Console (Sprint 3)
**Objective**: Build the visual interface.
- [ ] **Frontend**: Initialize Next.js 14 in `/frontend`.
- [ ] **Components**: Build `ResourceCard`, `ActivityFeed`, and `RealtimeChart` (using Chart.js or Recharts).
- [ ] **Streaming**: Connect frontend to `/api/dashboard/stream` (SSE) and WebSocket events.
- [ ] **CLI**: Add `cloud dashboard open` to launch the local web server.

### Phase 6: The Elastic Cloud (Sprints 4-6)
**Objective**: Load Balancing and Auto-Scaling.
- **Sprint 4 (LB Core)**: Implement HAProxy-based load balancer service.
- **Sprint 5 (Health Checks)**: Background workers for monitoring target health.
- **Sprint 6 (Auto-Scaling)**: Dynamic instance spawning based on CPU usage metrics fetched from Docker stats.

### Phase 7: The Managed Cloud (Sprints 7-9)
**Objective**: Managed DBs, Caching, and Queues.
- **Sprint 7 (RDS-lite)**: Pre-configured Postgres/MySQL containers with automated backup logic.
- **Sprint 8 (Snapshots)**: Volume snapshotting and point-in-time recovery.
- **Sprint 9 (Managed Services)**: **CloudCache** (Redis) and **CloudQueue** (NATS/RabbitMQ).

---

## 🛠️ Technical Context for Next Agent
- **Port Strategy**: Always check `internal/platform/config.go`. Port `5433` is the standard for DB.
- **Identity**: All non-public routes require the `Authorization: miniaws_<key>` header.
- **WebSocket**: Handshake requires `?api_key=...` in the query string.
- **Migrations**: New `.up.sql` files in `internal/repositories/postgres/migrations/` are auto-applied on startup.

## 🧪 Verification Standard
- Every feature MUST have a corresponding unit test in `*_test.go`.
- Integration tests (using `-tags=integration`) are required for Docker and SQL operations.
