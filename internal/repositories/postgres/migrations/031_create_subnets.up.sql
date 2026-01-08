-- +goose Up
CREATE TABLE subnets (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    vpc_id UUID NOT NULL REFERENCES vpcs(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    cidr_block CIDR NOT NULL,
    availability_zone VARCHAR(64),
    gateway_ip INET,
    arn VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(vpc_id, name),
    UNIQUE(vpc_id, cidr_block)
);

CREATE INDEX idx_subnets_vpc ON subnets(vpc_id);
CREATE INDEX idx_subnets_user ON subnets(user_id);

-- +goose Down
DROP TABLE IF EXISTS subnets;
