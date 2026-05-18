-- Add per-route resilience columns to gateway_routes
ALTER TABLE gateway_routes ADD COLUMN IF NOT EXISTS circuit_breaker_threshold INT NOT NULL DEFAULT 5;
ALTER TABLE gateway_routes ADD COLUMN IF NOT EXISTS circuit_breaker_timeout BIGINT NOT NULL DEFAULT 30000;
ALTER TABLE gateway_routes ADD COLUMN IF NOT EXISTS max_retries INT NOT NULL DEFAULT 2;
ALTER TABLE gateway_routes ADD COLUMN IF NOT EXISTS retry_timeout BIGINT NOT NULL DEFAULT 5000;