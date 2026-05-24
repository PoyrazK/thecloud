-- Drop the trigger and function
DROP TRIGGER IF EXISTS trg_chk_tgw_association_match ON transit_gateway_rt_associations;
DROP FUNCTION IF EXISTS chk_tgw_association_match();