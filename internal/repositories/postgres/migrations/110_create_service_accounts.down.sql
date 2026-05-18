-- +goose Down
DROP TABLE IF EXISTS service_account_policies;
DROP TABLE IF EXISTS service_account_secrets;
DROP TABLE IF EXISTS service_accounts;
