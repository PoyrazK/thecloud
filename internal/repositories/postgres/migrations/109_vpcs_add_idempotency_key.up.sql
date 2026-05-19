-- Add idempotency_key column to vpcs table for idempotent create operations
ALTER TABLE vpcs ADD COLUMN idempotency_key VARCHAR(64) UNIQUE;
