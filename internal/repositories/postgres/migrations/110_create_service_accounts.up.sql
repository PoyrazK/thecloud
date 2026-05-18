-- +goose Up
-- Service accounts for machine-to-machine authentication
CREATE TABLE service_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    role VARCHAR(100) NOT NULL DEFAULT 'service',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Client secrets (hashed, similar to api_keys)
CREATE TABLE service_account_secrets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    service_account_id UUID NOT NULL REFERENCES service_accounts(id) ON DELETE CASCADE,
    secret_hash VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL DEFAULT 'default',
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    UNIQUE(service_account_id, name)
);

-- SA policies junction (mirrors user_policies)
CREATE TABLE service_account_policies (
    service_account_id UUID NOT NULL REFERENCES service_accounts(id) ON DELETE CASCADE,
    policy_id UUID NOT NULL REFERENCES policies(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    attached_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (service_account_id, policy_id)
);

CREATE INDEX idx_sa_tenant ON service_accounts(tenant_id);
CREATE INDEX idx_sas_sa ON service_account_secrets(service_account_id);
CREATE INDEX idx_sap_sa ON service_account_policies(service_account_id);
CREATE INDEX idx_sap_tenant ON service_account_policies(tenant_id);
