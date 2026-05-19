ALTER TABLE functions ADD COLUMN max_concurrent_invocations INTEGER NOT NULL DEFAULT 0;
ALTER TABLE functions ADD COLUMN max_queue_depth INTEGER NOT NULL DEFAULT 0;