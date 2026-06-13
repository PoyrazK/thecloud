-- Add foreign key constraint on vpc_id to prevent orphaned attachments
ALTER TABLE transit_gateway_attachments
ADD CONSTRAINT fk_tgw_attachment_vpc
FOREIGN KEY (vpc_id) REFERENCES vpcs(id) ON DELETE CASCADE;

-- Add unique constraint to prevent duplicate VPC attachments to same TGW
CREATE UNIQUE INDEX idx_tgw_attachments_unique_vpc
ON transit_gateway_attachments(transit_gateway_id, vpc_id);
