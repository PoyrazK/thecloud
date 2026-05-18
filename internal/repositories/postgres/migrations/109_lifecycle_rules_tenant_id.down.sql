-- +goose Down
ALTER TABLE lifecycle_rules DROP COLUMN IF EXISTS tenant_id;
