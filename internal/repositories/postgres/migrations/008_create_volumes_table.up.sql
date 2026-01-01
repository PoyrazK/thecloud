CREATE TABLE IF NOT EXISTS volumes (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    size_gb INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(50) NOT NULL DEFAULT 'AVAILABLE',
    instance_id UUID REFERENCES instances(id) ON DELETE SET NULL,
    mount_path VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_volumes_status ON volumes(status);
CREATE INDEX idx_volumes_instance_id ON volumes(instance_id);
