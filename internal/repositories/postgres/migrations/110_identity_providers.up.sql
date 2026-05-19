-- Identity Providers table
CREATE TABLE identity_providers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(255) NOT NULL,
    type            VARCHAR(10) NOT NULL CHECK (type IN ('oidc', 'saml')),
    scope           VARCHAR(10) NOT NULL CHECK (scope IN ('global', 'tenant')),
    tenant_id       UUID REFERENCES tenants(id) ON DELETE CASCADE,
    client_id       VARCHAR(255),
    client_secret   TEXT,
    issuer_url      VARCHAR(2048),
    discovery_url   VARCHAR(2048),
    entity_id       VARCHAR(2048),
    sso_url         VARCHAR(2048),
    metadata_url    VARCHAR(2048),
    certificate     TEXT,
    scopes          TEXT[] DEFAULT ARRAY['openid', 'profile', 'email'],
    redirect_uris   TEXT[] NOT NULL DEFAULT '{}',
    enabled         BOOLEAN DEFAULT true,
    default_role    VARCHAR(50) DEFAULT 'developer',
    group_mappings  JSONB DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_oidc_requires_issuer CHECK (
        (type = 'oidc' AND issuer_url IS NOT NULL) OR
        (type = 'saml')
    )
);

-- Federated Identities table
CREATE TABLE federated_identities (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    idp_id            UUID NOT NULL REFERENCES identity_providers(id) ON DELETE CASCADE,
    subject           VARCHAR(1024) NOT NULL,
    email             VARCHAR(255) NOT NULL,
    email_verified    BOOLEAN DEFAULT false,
    groups            TEXT[] DEFAULT '{}',
    last_login_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (idp_id, subject),
    UNIQUE (idp_id, user_id)
);

-- Indexes
CREATE INDEX idx_idp_tenant ON identity_providers(tenant_id) WHERE tenant_id IS NOT NULL;
CREATE INDEX idx_idp_scope_enabled ON identity_providers(scope, enabled);
CREATE INDEX idx_fed_id_user ON federated_identities(user_id);
CREATE INDEX idx_fed_id_subject ON federated_identities(idp_id, subject);