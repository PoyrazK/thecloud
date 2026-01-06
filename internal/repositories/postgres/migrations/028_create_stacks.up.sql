CREATE TABLE stacks (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    template TEXT NOT NULL,
    parameters JSONB,
    status TEXT NOT NULL,
    status_reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, name)
);

CREATE INDEX idx_stacks_status ON stacks(status);

CREATE TABLE stack_resources (
    id UUID PRIMARY KEY,
    stack_id UUID NOT NULL REFERENCES stacks(id) ON DELETE CASCADE,
    logical_id TEXT NOT NULL,
    physical_id TEXT,
    resource_type TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(stack_id, logical_id)
);
