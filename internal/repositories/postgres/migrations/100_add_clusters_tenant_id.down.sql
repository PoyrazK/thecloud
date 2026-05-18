-- Remove tenant_id column from clusters table
DO $$
DECLARE
    lock_id bigint := hashtext('100_add_clusters_tenant_id')::bigint;
BEGIN
    -- Acquire advisory lock to prevent concurrent migration runs
    PERFORM pg_advisory_lock(lock_id);

    -- Remove tenant_id column (skip if already removed)
    IF EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'clusters' AND column_name = 'tenant_id') THEN
        ALTER TABLE clusters DROP COLUMN tenant_id;
    END IF;

    -- Restore original index
    DROP INDEX IF EXISTS idx_clusters_tenant_id;
    DROP INDEX IF EXISTS idx_clusters_tenant_user;
    CREATE INDEX IF NOT EXISTS idx_clusters_user_id ON clusters(user_id);

    -- Release advisory lock
    PERFORM pg_advisory_unlock(lock_id);
END $$;
