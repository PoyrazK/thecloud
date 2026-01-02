# 🚀 Mini AWS - Master Task Breakdown (Handoff)

This is a copy of the active task state from the previous machine.

## 📋 Phase 5: The Console (Sprints 1-3)

### Sprint 1: Backend Foundation
- [x] **Architect**: Define `domain/dashboard.go` (ResourceSummary, MetricPoint)
- [x] **Architect**: Define `domain/ws_event.go` (WSEventType, WSEvent struct)
- [x] **Architect**: Create ADR-006: Real-time Communication Strategy
- [x] **Backend**: Implement `services/dashboard.go` (GetSummary, CountResources)
- [x] 🧪 **QA**: Unit tests for DashboardService (≥90% coverage)
- [x] **Backend**: Add `GET /api/dashboard/summary` endpoint
- [x] 🧪 **QA**: API tests for dashboard/summary endpoint
- [x] **Infra**: Create `migrations/009_metrics_history.sql`
- [ ] 🧪 **QA**: Migration rollback test

### Sprint 2: Real-time Streaming
- [x] **Infra**: Implement `docker/stats.go` (real-time container metrics)
- [x] 🧪 **QA**: Integration test for Docker stats adapter
- [x] **Backend**: Implement SSE endpoint `GET /api/dashboard/stream`
- [x] 🧪 **QA**: SSE connection and event delivery test
- [x] **Backend**: Create WebSocket hub `handlers/ws/hub.go`
- [x] 🧪 **QA**: WebSocket lifecycle test (connect/message/disconnect)
- [x] **Security**: Add WS handshake authentication middleware
- [x] 🧪 **QA**: Auth rejection test for invalid API keys
- [ ] **Security**: Configure CORS for frontend origin
- [ ] **Platform**: Add `mini_aws_ws_connections_active` gauge
- [ ] 🧪 **QA**: Metrics endpoint validation

### Sprint 3: Frontend Dashboard (NEXT UP)
- [ ] **Frontend**: Initialize Next.js 14 project in `/frontend`
- [ ] **Frontend**: Create dashboard layout with sidebar navigation
- [ ] 🧪 **QA**: Component snapshot tests
- [ ] **Frontend**: Build ResourceCard components (Instances, Volumes, VPCs)
- [ ] 🧪 **QA**: Unit tests for ResourceCard props
- [ ] **Frontend**: Implement real-time metrics charts (CPU/Memory)
- [ ] 🧪 **QA**: Chart rendering with mock data
- [ ] **Frontend**: Build Activity Feed (audit logs stream)
- [ ] **CLI**: Add `cloud dashboard open` command
- [ ] 🧪 **QA**: CLI command execution test
- [ ] **Docs**: Create `docs/guides/console.md`
- [ ] 🧪 **QA**: Playwright E2E tests for full dashboard flow
