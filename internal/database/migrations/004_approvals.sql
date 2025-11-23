-- +goose Up
-- Approvals table: stores approval requests for remediation actions

CREATE TABLE IF NOT EXISTS approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    action_id VARCHAR(255) NOT NULL,
    action_name VARCHAR(255) NOT NULL,
    action_category VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    risk_level VARCHAR(20) NOT NULL CHECK (risk_level IN ('safe', 'moderate', 'critical')),
    is_reversible BOOLEAN NOT NULL DEFAULT false,
    estimated_impact TEXT NOT NULL,
    target VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
    requested_by VARCHAR(255) NOT NULL,
    requested_at TIMESTAMP NOT NULL DEFAULT NOW(),
    reviewed_by VARCHAR(255),
    reviewed_at TIMESTAMP,
    reason TEXT,
    expires_at TIMESTAMP,
    metadata JSONB DEFAULT '{}'::jsonb,
    CONSTRAINT approvals_reviewed_check CHECK (
        (status = 'pending' AND reviewed_by IS NULL AND reviewed_at IS NULL) OR
        (status IN ('approved', 'rejected', 'expired') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
    )
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_approvals_job_id ON approvals(job_id);
CREATE INDEX IF NOT EXISTS idx_approvals_status ON approvals(status);
CREATE INDEX IF NOT EXISTS idx_approvals_risk_level ON approvals(risk_level);
CREATE INDEX IF NOT EXISTS idx_approvals_requested_at ON approvals(requested_at DESC);
CREATE INDEX IF NOT EXISTS idx_approvals_expires_at ON approvals(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_approvals_requested_by ON approvals(requested_by);
CREATE INDEX IF NOT EXISTS idx_approvals_target ON approvals(target);

-- Composite index for common query pattern: pending approvals ordered by risk
CREATE INDEX IF NOT EXISTS idx_approvals_pending_risk ON approvals(status, risk_level, requested_at DESC) WHERE status = 'pending';

-- GIN index for JSONB metadata
CREATE INDEX IF NOT EXISTS idx_approvals_metadata ON approvals USING GIN (metadata);

-- Comment on tables and columns
COMMENT ON TABLE approvals IS 'Stores approval requests for remediation actions';
COMMENT ON COLUMN approvals.job_id IS 'Reference to the job this approval belongs to';
COMMENT ON COLUMN approvals.action_id IS 'Unique identifier for the action';
COMMENT ON COLUMN approvals.action_name IS 'Human-readable name of the action';
COMMENT ON COLUMN approvals.action_category IS 'Category of action (service, disk, process, etc.)';
COMMENT ON COLUMN approvals.description IS 'Detailed description of what the action will do';
COMMENT ON COLUMN approvals.risk_level IS 'Risk level (safe, moderate, critical)';
COMMENT ON COLUMN approvals.is_reversible IS 'Whether the action can be rolled back';
COMMENT ON COLUMN approvals.estimated_impact IS 'Estimated impact of the action';
COMMENT ON COLUMN approvals.target IS 'Target system for the action';
COMMENT ON COLUMN approvals.status IS 'Approval status (pending, approved, rejected, expired)';
COMMENT ON COLUMN approvals.requested_by IS 'User or agent who requested the approval';
COMMENT ON COLUMN approvals.requested_at IS 'Timestamp when approval was requested';
COMMENT ON COLUMN approvals.reviewed_by IS 'User who reviewed the approval';
COMMENT ON COLUMN approvals.reviewed_at IS 'Timestamp when approval was reviewed';
COMMENT ON COLUMN approvals.reason IS 'Reason for approval/rejection';
COMMENT ON COLUMN approvals.expires_at IS 'Timestamp when approval request expires';
COMMENT ON COLUMN approvals.metadata IS 'Additional metadata for the approval';

-- +goose Down
DROP INDEX IF EXISTS idx_approvals_metadata;
DROP INDEX IF EXISTS idx_approvals_pending_risk;
DROP INDEX IF EXISTS idx_approvals_target;
DROP INDEX IF EXISTS idx_approvals_requested_by;
DROP INDEX IF EXISTS idx_approvals_expires_at;
DROP INDEX IF EXISTS idx_approvals_requested_at;
DROP INDEX IF EXISTS idx_approvals_risk_level;
DROP INDEX IF EXISTS idx_approvals_status;
DROP INDEX IF EXISTS idx_approvals_job_id;
DROP TABLE IF EXISTS approvals;
