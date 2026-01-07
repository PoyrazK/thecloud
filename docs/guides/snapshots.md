# Volume Snapshots

This guide documents snapshot operations for block volumes: create, list, restore, and delete.

## Concepts

- Snapshot: point-in-time copy of a volume (QCOW2 snapshot for libvirt or logical snapshot for Docker volumes)
- Snapshot lifecycle: create -> list -> restore -> delete

## Quick CLI Examples

Create a snapshot:

```bash
cloud volumes snapshot create --volume-id vol-1234 --name backup-2026-01-07
```

List snapshots for a volume:

```bash
cloud volumes snapshot list --volume-id vol-1234
```

Restore a snapshot:

```bash
cloud volumes snapshot restore --snapshot-id snap-1234 --target-volume vol-1234
```

Delete a snapshot:

```bash
cloud volumes snapshot delete --snapshot-id snap-1234
```

## API Endpoints

- `POST /snapshots` — Create snapshot
- `GET /snapshots?volume_id=...` — List snapshots
- `POST /snapshots/:id/restore` — Restore snapshot
- `DELETE /snapshots/:id` — Delete snapshot

## Notes & Limitations

- Libvirt backend supports efficient QCOW2 snapshots; ensure the storage pool has enough space
- Snapshot restore may require instance restart if a volume is attached

## Troubleshooting

- Snapshot creation fails with disk space error: verify storage pool availability
- Restores not applied: ensure instance detach/reattach and check volume mount state

## Next Steps

- Add automatic retention policies
- Implement snapshot schedules via CloudCron
