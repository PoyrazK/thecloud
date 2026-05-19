-- +goose Up
-- Allow IAM policies to be attached to RBAC roles
CREATE TABLE IF NOT EXISTS role_policies (
    role_name VARCHAR(255) NOT NULL,
    policy_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    attached_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (role_name, policy_id),
    FOREIGN KEY (policy_id) REFERENCES policies(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_role_policies_role_name ON role_policies(role_name);
CREATE INDEX IF NOT EXISTS idx_role_policies_tenant_id ON role_policies(tenant_id);