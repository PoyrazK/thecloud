CREATE TABLE audit_logs (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    action VARCHAR(255) NOT NULL,
    resource_type VARCHAR(255),
    resource_id VARCHAR(255),
    details JSONB,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
