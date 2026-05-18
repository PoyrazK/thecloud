# ADR-027: Volume, Database, and Cache Resize

**Status**: Accepted
**Date**: 2026-05-18
**Deciders**: Platform Team

---

## Context

Issue #442 requested CLI resize commands for volume, database, and cache. Instance resize already existed (`ADR-025`). This ADR documents the design decisions for resizing the three remaining resource types.

**Forces at play:**
- Volume backend (LVM) supports online expansion via `lvresize`
- Database resize reuses the volume service since RDS storage is backed by volumes
- Cache (Redis) resize requires container restart — no live resize possible for Docker

---

## Decision

### Volume Resize

**Backend already existed** at `ports.VolumeService.ResizeVolume`. This PR wires up the handler, SDK, and CLI:

- **Handler**: `POST /volumes/:id/resize` with `{ new_size_gb: int }` payload
- **SDK**: `ResizeVolume(idOrName, newSizeGB)` with name→ID resolution
- **CLI**: `cloud volume resize [id/name] [sizeGB]`

The backend `volumeSvc.ResizeVolume()` calls the LVM resize via `exec.Command("lvresize", ...)`. Volume resize is the simplest of the three since the backend already existed.

### Database Resize

Database storage is backed by LVM volumes managed via `volumeSvc`. Resize delegates to the existing volume service:

```
DatabaseService.ResizeDatabase → volumeSvc.ResizeVolume → LVM lvresize
```

- **Handler**: `POST /databases/:id/resize` with `{ allocated_storage: int }` payload
- **SDK**: `ResizeDatabase(id, newSizeGB)`
- **CLI**: `cloud db resize [id] [sizeGB]`
- **Validation**: `newSizeGB > db.AllocatedStorage` (cannot shrink storage)
- **Audit**: Logs `database.resize` with old/new size

### Cache Resize

Cache resize is the most complex. Redis does not support live memory hot-reconfiguration, so resize requires a **stop-delete-relaunch** cycle:

1. **Validate** `newMemoryMB > cache.MemoryMB`
2. **Check backend** `compute.Type() == "docker"` (libvirt not supported)
3. **Stop and delete** old container
4. **Relaunch** container with new `--memory` limit via `launchCacheContainer`
5. **Update** `MemoryMB` in DB record

**Rollback on relaunch failure:** If `launchCacheContainer` fails after the old container is already deleted, the service attempts to restart the old container before propagating the error. This prevents inconsistent state where the cache record shows a new memory size but no container exists.

- **Handler**: `POST /caches/:id/resize` with `{ memory_mb: int }` payload (`min=64`)
- **SDK**: `ResizeCache(idOrName, newMemoryMB)` with name→ID resolution
- **CLI**: `cloud cache resize [id] [memoryMB]`
- **Audit**: Logs `cache.resize` with old/new memory

---

## Consequences

### Positive
- Unified CLI experience across all three resource types
- Volume resize is immediate (LVM online expansion)
- Cache rollback prevents inconsistent state on relaunch failure

### Negative
- Cache resize causes brief downtime (container stop→start cycle)
- Libvirt compute backend not supported for cache resize (docker-only)
- Volume resize cannot shrink — only expand (LVM constraint)

### Neutral
- Database resize uses existing volumeSvc — consistent with ModifyDatabase pattern
- Audit logging captures all resize operations

---

## Alternatives Considered

### Alternative 1: Live Memory Resize via Redis CONFIG SET
**Why rejected:** `CONFIG SET maxmemory` requires `CONFIG REWRITE` to persist, and the container would need restart anyway to pick up the new memory limit. The stop-delete-relaunch approach is more reliable.

### Alternative 2: Resize Database by Creating New Volume and Migrating
**Why rejected:** Unnecessarily complex. RDS volumes can be expanded in-place via LVM.

### Alternative 3: Allow Cache Resize on Libvirt Backend
**Why rejected:** Libvirt-based cache instances run Redis in QEMU VMs, not Docker containers. The container restart approach requires Docker. A separate VM-based resize path would require significant additional implementation.