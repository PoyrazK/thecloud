-- Prevent cross-TGW associations using a trigger
-- The trigger validates that route_table and attachment belong to the same transit_gateway

-- Create trigger function
CREATE OR REPLACE FUNCTION chk_tgw_association_match()
RETURNS TRIGGER AS $$
DECLARE
    rt_tgw_id UUID;
    att_tgw_id UUID;
BEGIN
    SELECT transit_gateway_id INTO rt_tgw_id
    FROM transit_gateway_route_tables
    WHERE id = NEW.route_table_id;

    SELECT transit_gateway_id INTO att_tgw_id
    FROM transit_gateway_attachments
    WHERE id = NEW.attachment_id;

    IF rt_tgw_id IS NULL OR att_tgw_id IS NULL OR rt_tgw_id != att_tgw_id THEN
        RAISE EXCEPTION 'route table and attachment must belong to the same transit gateway'
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger
CREATE TRIGGER trg_chk_tgw_association_match
    BEFORE INSERT OR UPDATE ON transit_gateway_rt_associations
    FOR EACH ROW
    EXECUTE FUNCTION chk_tgw_association_match();