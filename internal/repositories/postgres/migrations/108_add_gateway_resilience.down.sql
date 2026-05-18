ALTER TABLE gateway_routes DROP COLUMN IF EXISTS circuit_breaker_threshold;
ALTER TABLE gateway_routes DROP COLUMN IF EXISTS circuit_breaker_timeout;
ALTER TABLE gateway_routes DROP COLUMN IF EXISTS max_retries;
ALTER TABLE gateway_routes DROP COLUMN IF EXISTS retry_timeout;