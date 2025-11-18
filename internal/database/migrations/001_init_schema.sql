-- +goose Up
-- Initial schema for Lumo API server

-- Jobs table: stores diagnostic and remediation jobs
CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(50) NOT NULL CHECK (type IN ('diagnostic', 'remediation')),
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    target VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_by VARCHAR(255),
    result JSONB,
    error TEXT,
    metadata JSONB DEFAULT '{}'::jsonb
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type);
CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_jobs_target ON jobs(target);
CREATE INDEX IF NOT EXISTS idx_jobs_created_by ON jobs(created_by);

-- Comment on tables and columns
COMMENT ON TABLE jobs IS 'Stores diagnostic and remediation job information';
COMMENT ON COLUMN jobs.type IS 'Job type: diagnostic or remediation';
COMMENT ON COLUMN jobs.status IS 'Current job status';
COMMENT ON COLUMN jobs.target IS 'Target hostname or IP address';
COMMENT ON COLUMN jobs.result IS 'Job result in JSON format';
COMMENT ON COLUMN jobs.metadata IS 'Additional job metadata and context';

-- +goose Down
DROP INDEX IF EXISTS idx_jobs_created_by;
DROP INDEX IF EXISTS idx_jobs_target;
DROP INDEX IF EXISTS idx_jobs_created_at;
DROP INDEX IF EXISTS idx_jobs_type;
DROP INDEX IF EXISTS idx_jobs_status;
DROP TABLE IF EXISTS jobs;
