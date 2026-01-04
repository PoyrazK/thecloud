-- CloudGateway: API Gateway Service
CREATE TABLE gateway_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    path_prefix VARCHAR(255) NOT NULL,
    target_url TEXT NOT NULL,
    strip_prefix BOOLEAN NOT NULL DEFAULT TRUE,
    rate_limit INT NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, name),
    UNIQUE(path_prefix)
);

CREATE INDEX idx_gateway_routes_user_id ON gateway_routes(user_id);
CREATE INDEX idx_gateway_routes_path_prefix ON gateway_routes(path_prefix);
