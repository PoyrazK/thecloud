# 🗺️ The Cloud: Future Roadmap for Agents

This document provides a detailed technical roadmap for the remaining phases of the The Cloud project.

## 🏁 Current State
- **Phase 1-6 (Core & Services)**: ✅ Completed.
  - Compute, Networking, Storage, LB, Auto-Scaling.
  - Managed Services: RDS, Cache, Queue, Notify, Cron, Gateway.
- **Phase 7 (Console)**: 🚧 In Progress.
  - Backend foundation is complete (Resource aggregation, Streaming).
  - **Frontend**: Next.js dashboard needs implementation.

---

## 📅 Upcoming Sprints

### Phase 7: The Console (Sprint 3)
**Objective**: Build the visual interface.
- [ ] **Frontend**: Initialize Next.js 14 in `/frontend`.
- [ ] **Components**: Build `ResourceCard`, `ActivityFeed`, and `RealtimeChart` (using Chart.js or Recharts).
- [ ] **Streaming**: Connect frontend to `/api/dashboard/stream` (SSE) and WebSocket events.
- [ ] **CLI**: Add `cloud dashboard open` to launch the local web server.

### Phase 8: The Marketplace (Sprints 8-10)
**Objective**: One-click deployments and templates.
- **Sprint 8 (Templates)**: YAML-based CloudFormation-like definitions for stacks (e.g., WordPress = Instance + RDS).
- **Sprint 9 (Registry)**: A public/private registry for sharing templates.
- **Sprint 10 (Billing)**: Simulated billing metrics and usage reports.

---

## 🛠️ Technical Context for Next Agent
- **Port Strategy**: Always check `internal/platform/config.go`. Port `5433` is the standard for DB.
- **Identity**: All non-public routes require the `Authorization: thecloud_<key>` header.
- **WebSocket**: Handshake requires `?api_key=...` in the query string.
- **Migrations**: New `.up.sql` files in `internal/repositories/postgres/migrations/` are auto-applied on startup.

## 🧪 Verification Standard
- Every feature MUST have a corresponding unit test in `*_test.go`.
- Integration tests (using `-tags=integration`) are required for Docker and SQL operations.
