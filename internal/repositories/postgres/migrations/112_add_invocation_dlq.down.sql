ALTER TABLE invocations DROP COLUMN IF EXISTS retry_count;
ALTER TABLE invocations DROP COLUMN IF EXISTS max_retries;