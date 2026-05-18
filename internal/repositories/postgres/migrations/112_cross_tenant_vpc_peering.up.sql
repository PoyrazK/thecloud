-- Add cross-tenant peering support: split tenant_id into requester_tenant_id and accepter_tenant_id
ALTER TABLE vpc_peerings ADD COLUMN requester_tenant_id UUID;
ALTER TABLE vpc_peerings ADD COLUMN accepter_tenant_id UUID;
UPDATE vpc_peerings SET requester_tenant_id = tenant_id, accepter_tenant_id = tenant_id WHERE tenant_id IS NOT NULL;
ALTER TABLE vpc_peerings ALTER COLUMN requester_tenant_id SET NOT NULL;
ALTER TABLE vpc_peerings ALTER COLUMN accepter_tenant_id SET NOT NULL;