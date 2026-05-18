-- +goose Up
ALTER TABLE lifecycle_rules ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_lifecycle_rules_tenant ON lifecycle_rules(tenant_id);

-- Backfill existing data using default_tenant_id from users table
UPDATE lifecycle_rules lr SET tenant_id = u.default_tenant_id FROM users u WHERE lr.user_id = u.id AND lr.tenant_id IS NULL;
