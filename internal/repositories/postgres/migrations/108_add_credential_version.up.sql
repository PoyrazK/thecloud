-- +goose Up

ALTER TABLE databases ADD COLUMN credential_version INT DEFAULT 1 NOT NULL;

-- +goose Down

ALTER TABLE databases DROP COLUMN credential_version;
