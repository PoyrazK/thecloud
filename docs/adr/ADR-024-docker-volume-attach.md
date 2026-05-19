# ADR-024: Docker Volume Attach/Detach via Container Recreation

**Status**: Accepted
**Date**: 2026-04-14
**Deciders**: Platform Team

---

## Context

The Cloud platform supports multiple compute backends (Docker, Libvirt/KVM, Firecracker) via a `ComputeBackend` interface. The `AttachVolume` and `DetachVolume` methods on this interface are used to attach persistent storage to compute instances.

**Problem**: Docker does not support hot-attaching bind mounts to running containers. When `AttachVolume` was called on a Docker container, it returned "not implemented", making volume attachment unavailable for Docker-based instances.

**Requirements**:
- Docker instances must support volume attach/detach
- Accept brief container downtime during attach/detach (short-term solution)
- Container ID changes after attach/detach must be tracked
- Subsequent operations (stop, delete, exec) must use the new container ID

---

## Decision

We implement volume attach/detach for Docker via a **stop → recreate → start** cycle:

1. **Stop**: Gracefully stop the running container (30s timeout)
2. **Inspect**: Retrieve current container configuration (image, env, binds, networking)
3. **Create**: Create new container with updated `HostConfig.Binds`
4. **Start**: Start the new container
5. **Cleanup**: Remove the old container (orphaned containers are eventually cleaned up)
6. **Track**: Return the new container ID so the caller can update the instance record

### Interface Changes

The `ComputeBackend` interface signatures were updated to return the new container ID:

```go
// Before
AttachVolume(ctx context.Context, id string, volumePath string) error
DetachVolume(ctx context.Context, id string, volumePath string) error

// After
AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error)
//                                                              ↑↑↑
//                                                        (devicePath, newContainerID, error)
DetachVolume(ctx context.Context, id string, volumePath string) (string, error)
//                                                   ↑
//                                           (newContainerID, error)
```

### Per-Container Locking

Concurrent attach/detach operations on the same container are prevented via a `sync.Map` of per-container mutexes in the `DockerAdapter`:

```go
type DockerAdapter struct {
    cli    dockerClient
    logger *slog.Logger
    containerLocks sync.Map // map[string]*sync.Mutex
}

func (a *DockerAdapter) getContainerLock(containerID string) *sync.Mutex {
    lock, _ := a.containerLocks.LoadOrStore(containerID, &sync.Mutex{})
    return lock.(*sync.Mutex)
}
```

### Rollback Handling

| Failure Point | Action |
|--------------|--------|
| Stop fails | Return error, container still running, no state change |
| Create fails | Restart original container, return error |
| Start fails | Remove new container, restart original, return error |
| Remove old container fails | Log warning only (new container already running) |

### VolumeService Integration

The `VolumeService` was updated to:
1. Call `storage.AttachVolume` (LVM/physical) to get device path
2. Call `instanceRepo.GetByID` to find current `ContainerID`
3. Call `compute.AttachVolume` to add the bind mount (which returns new container ID)
4. Call `instanceRepo.Update` to persist the new `ContainerID`

---

## Consequences

### Positive
- Docker instances can now use persistent volume storage
- Container ID tracking ensures subsequent operations work correctly
- Rollback handling prevents inconsistent state on failures
- Per-container locking prevents race conditions

### Negative
- Brief downtime (~1-2 seconds) during attach/detach operations
- Container ID changes, requiring instance record updates
- Not a true hot-attach solution (acceptable for short-term)

### Neutral
- Libvirt backend returns empty string for new container ID (supports true hot-attach via `virDomainAttachDeviceFlags`)
- Firecracker backend returns `NotImplemented` — Firecracker does not support hot-attach; full support would require VM recreation (stop → recreate with extra drive → start)
- The `VolumeService` now has additional dependencies (`Compute`, `InstanceRepo`)

---

## Implementation Details

### Files Modified

| File | Change |
|------|--------|
| `internal/core/ports/compute.go` | Updated `AttachVolume`/`DetachVolume` signatures |
| `internal/repositories/docker/adapter.go` | Implemented stop→recreate→start cycle |
| `internal/repositories/libvirt/adapter.go` | Updated signatures (returns empty string) |
| `internal/repositories/firecracker/adapter.go` | Updated signatures |
| `internal/repositories/noop/adapters.go` | Updated signatures |
| `internal/core/services/volume.go` | Added `Compute` and `InstanceRepo` dependencies |
| `internal/core/services/volume_unit_test.go` | Added integration tests |
| `docs/architecture.md` | Updated interface documentation |

### Key Patterns Reused

- `InstanceService.finalizeProvision` - pattern for updating `ContainerID` via repo
- Existing `ContainerStop`/`ContainerStart`/`ContainerInspect` methods
- `container.HostConfig{Binds}` pattern from `LaunchInstanceWithOptions`
- Rollback patterns from `AttachVolume` error handling

---

## Alternatives Considered

### 1. nsenter Bind Mount (No Restart)
Use `docker exec` + `nsenter` to enter the container's mount namespace and perform a bind mount without restarting.

**Rejected**: Requires privileged container, security risks, complex namespace management.

### 2. Volume Plugin Architecture
Create a Docker volume plugin that handles attach/detach semantics.

**Rejected**: Significant complexity, requires plugin distribution.

### 3. FUSE/passthrough Filesystem
Use FUSE to present a volume as a filesystem mounted into the container.

**Rejected**: Performance overhead, complex setup.

---

## References

- `internal/repositories/docker/adapter.go` - Docker adapter implementation
- `internal/core/services/volume.go` - VolumeService integration
- `internal/core/ports/compute.go` - ComputeBackend interface
- `ADR-015-kubernetes-csi-driver.md` - CSI driver architecture