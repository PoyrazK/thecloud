-- Create transit_gateways table
CREATE TABLE IF NOT EXISTS transit_gateways (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    owner_tenant_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    arn VARCHAR(500) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transit_gateways_tenant ON transit_gateways(owner_tenant_id);
CREATE INDEX idx_transit_gateways_status ON transit_gateways(status);

-- Create transit_gateway_attachments table
CREATE TABLE IF NOT EXISTS transit_gateway_attachments (
    id UUID PRIMARY KEY,
    transit_gateway_id UUID NOT NULL REFERENCES transit_gateways(id) ON DELETE CASCADE,
    vpc_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    attachment_type VARCHAR(50) NOT NULL DEFAULT 'vpc',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tgw_attachments_tgw ON transit_gateway_attachments(transit_gateway_id);
CREATE INDEX idx_tgw_attachments_vpc ON transit_gateway_attachments(vpc_id);
CREATE INDEX idx_tgw_attachments_tenant ON transit_gateway_attachments(tenant_id);

-- Create transit_gateway_route_tables table
CREATE TABLE IF NOT EXISTS transit_gateway_route_tables (
    id UUID PRIMARY KEY,
    transit_gateway_id UUID NOT NULL REFERENCES transit_gateways(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    default_route_table BOOLEAN NOT NULL DEFAULT FALSE,
    propagation_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tgw_rt_tgw ON transit_gateway_route_tables(transit_gateway_id);

-- Create transit_gateway_routes table
CREATE TABLE IF NOT EXISTS transit_gateway_routes (
    id UUID PRIMARY KEY,
    transit_gateway_rt_id UUID NOT NULL REFERENCES transit_gateway_route_tables(id) ON DELETE CASCADE,
    destination_cidr VARCHAR(50) NOT NULL,
    target_type VARCHAR(50) NOT NULL,
    target_id UUID,
    target_name VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tgw_routes_rt ON transit_gateway_routes(transit_gateway_rt_id);

-- Create transit_gateway_rt_associations for linking attachments to route tables
CREATE TABLE IF NOT EXISTS transit_gateway_rt_associations (
    route_table_id UUID NOT NULL REFERENCES transit_gateway_route_tables(id) ON DELETE CASCADE,
    attachment_id UUID NOT NULL REFERENCES transit_gateway_attachments(id) ON DELETE CASCADE,
    propagation_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (route_table_id, attachment_id)
);