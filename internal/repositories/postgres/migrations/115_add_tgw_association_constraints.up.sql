-- Prevent cross-TGW associations by ensuring both sides share the same transit_gateway_id
ALTER TABLE transit_gateway_rt_associations
ADD CONSTRAINT chk_same_transit_gateway
CHECK (
    (SELECT transit_gateway_id FROM transit_gateway_route_tables WHERE id = transit_gateway_rt_associations.route_table_id) =
    (SELECT transit_gateway_id FROM transit_gateway_attachments WHERE id = transit_gateway_rt_associations.attachment_id)
);