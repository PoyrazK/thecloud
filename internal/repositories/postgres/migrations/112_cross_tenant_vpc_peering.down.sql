-- Rollback cross-tenant peering: merge requester_tenant_id and accepter_tenant_id back to tenant_id
ALTER TABLE vpc_peerings ADD COLUMN tenant_id UUID NOT NULL DEFAULT requester_tenant_id;
UPDATE vpc_peerings SET tenant_id = requester_tenant_id WHERE requester_tenant_id IS NOT NULL;
ALTER TABLE vpc_peerings ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE vpc_peerings DROP COLUMN accepter_tenant_id;
ALTER TABLE vpc_peerings DROP COLUMN requester_tenant_id;