-- Rollback cross-tenant peering: drop the new columns added by this migration
ALTER TABLE vpc_peerings DROP COLUMN IF EXISTS requester_tenant_id;
ALTER TABLE vpc_peerings DROP COLUMN IF EXISTS accepter_tenant_id;
