# Detailed Implementation Plan: Firecracker MicroVM Backend

## Objective
Implement a new `ComputeBackend` adapter using Amazon Firecracker to support lightweight, high-density MicroVMs.

## Implementation Status (PR #567 — Feature Parity)

### Completed

- [x] Analyze `ports.ComputeBackend` interface.
- [x] Add SDK dependency (`firecracker-go-sdk`).
- [x] Create `FirecrackerAdapter` struct.
- [x] Implement `LaunchInstanceWithOptions` (via firecracker-go-sdk).
- [x] Implement `StartInstance` / `StopInstance` / `DeleteInstance`.
- [x] Implement `GetInstanceLogs` (multi-path search).
- [x] Implement `GetInstanceStats` (reads `/proc/{pid}/status` and `/proc/{pid}/stat`).
- [x] Implement `GetInstancePort` (port mapping lookup).
- [x] Implement `GetInstanceIP` (ARP lookup via `ip neigh show` + `/proc/net/arp`).
- [x] Implement `CreateNetwork` / `DeleteNetwork` (TAP device via `ip tuntap`).
- [x] Implement `CreateSnapshot` / `RestoreSnapshot` / `DeleteSnapshot` (via qemu-img + tar wrappers).
- [x] Implement `ResizeInstance` — returns `ErrNotSupported` (Firecracker SDK lacks VM config update API; cold resize via stop→restart not implemented).
- [x] Return proper error types for unsupported operations (`PauseInstance`, `ResumeInstance`, `GetConsoleURL`, `Exec`, `DetachVolume`).
- [x] Unit tests and E2E coverage.

### Known Limitations

- **ResizeInstance returns ErrNotSupported** — Firecracker SDK lacks VM config update API (`PutMachineConfiguration`). True online resize requires Firecracker v1.0+ support or VM recreation (stop → recreate with new config → start).
- **AttachVolume returns NotImplemented** — Firecracker does not support hot-attach of drives after VM start.
- **PauseInstance / ResumeInstance** — Firecracker has no pause/resume support. These return `ErrInstanceNotPausable` / `ErrInstanceNotResumable`.
- **GetConsoleURL / Exec** — Firecracker has no VNC console or guest agent; these return structured `NotImplemented` errors.
- **Root/CAP_NET_ADMIN required** — TAP device creation and deletion require host privileges.

### Not Yet Implemented

- [ ] True online `ResizeInstance` via Firecracker config update API
- [ ] `AttachVolume` via VM recreation (stop → recreate with extra drive → start)
- [ ] `DetachVolume`
- [ ] `Exec` via guest agent (Firecracker does not provide an agent mechanism)

## Subtasks

### 1. Scout & Setup ✅
- [x] Analyze `ports.ComputeBackend` interface.
- [x] Research `firecracker-go-sdk` usage.
- [x] Add SDK dependency to `go.mod`.

### 2. Core Adapter Implementation (`internal/repositories/firecracker/`) ✅
- [x] Create `FirecrackerAdapter` struct.
- [x] Implement `LaunchInstanceWithOptions`:
    - Configure Machine (VCPU, Memory).
    - Configure Kernel (`vmlinux` path).
    - Configure Drives (RootFS).
    - Configure Network (TAP device).
- [x] Implement Lifecycle Methods:
    - `StartInstance` / `StopInstance` / `DeleteInstance`.
    - `GetInstanceStatus` / `GetInstanceLogs` / `GetInstanceStats`.
- [ ] Implement `Exec` (requires guest agent or serial console interaction) — not supported by Firecracker.

### 3. Networking Logic ✅
- [x] Implement TAP device management helper (`CreateNetwork` / `DeleteNetwork`).
- [ ] Integration with host bridge if applicable — out of scope for now.

### 4. Integration ✅
- [x] Modify `internal/api/setup/infrastructure.go` to support `COMPUTE_BACKEND=firecracker`.
- [x] Add configuration fields for Firecracker binary, kernel path, and rootfs templates.

### 5. Testing & Validation ✅
- [x] Create `adapter_test.go`.
- [x] Mock Firecracker binary execution for unit tests.
- [x] Verify interface compliance.
- [x] E2E tests (`tests/firecracker_e2e_test.go`).

## Challenges
- **Root Privileges:** Firecracker and network setup (TAP) typically require root.
- **Host Dependencies:** Binary presence of `firecracker` and `jailer`.
- **Kernel/RootFS:** Need compatible artifacts available on the host.
- **Firecracker Limitations:** No pause/resume, no VNC console, no guest agent, no hot-attach. These are architectural constraints of Firecracker itself.

## Success Criteria
- [x] `internal/repositories/firecracker` implements the `ComputeBackend` interface.
- [x] System boots with `COMPUTE_BACKEND=firecracker`.
- [x] Unit tests pass with mocked SDK/Execution.
- [ ] True online resize support (pending Firecracker SDK / v1.0 API).
- [ ] `AttachVolume` via VM recreation (requires careful handling to avoid data loss).
