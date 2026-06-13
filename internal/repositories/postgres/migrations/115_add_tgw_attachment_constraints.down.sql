-- Drop unique constraint
DROP INDEX IF EXISTS idx_tgw_attachments_unique_vpc;

-- Drop foreign key constraint
ALTER TABLE transit_gateway_attachments
DROP CONSTRAINT IF EXISTS fk_tgw_attachment_vpc;
