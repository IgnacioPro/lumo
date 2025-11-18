-- +goose Up
-- Agents table: stores agent registration and metadata

CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    ip_address VARCHAR(45),  -- IPv4 or IPv6
    platform VARCHAR(50) NOT NULL CHECK (platform IN ('linux', 'darwin', 'windows', 'kubernetes')),
    architecture VARCHAR(50) NOT NULL,  -- amd64, arm64, etc.
    version VARCHAR(50) NOT NULL,  -- Agent version
    status VARCHAR(20) NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'offline', 'error')),
    capabilities TEXT[] NOT NULL DEFAULT '{}',  -- Available checkers
    labels JSONB DEFAULT '{}'::jsonb,  -- User-defined labels
    kubernetes_metadata JSONB,  -- cluster, namespace, node_name, pod_name
    last_heartbeat_at TIMESTAMP NOT NULL DEFAULT NOW(),
    registered_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(hostname)
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_platform ON agents(platform);
CREATE INDEX IF NOT EXISTS idx_agents_last_heartbeat ON agents(last_heartbeat_at DESC);
CREATE INDEX IF NOT EXISTS idx_agents_registered_at ON agents(registered_at DESC);
CREATE INDEX IF NOT EXISTS idx_agents_hostname ON agents(hostname);

-- GIN index for JSONB columns (for efficient querying of labels and k8s metadata)
CREATE INDEX IF NOT EXISTS idx_agents_labels ON agents USING GIN (labels);
CREATE INDEX IF NOT EXISTS idx_agents_k8s_metadata ON agents USING GIN (kubernetes_metadata);

-- Comment on tables and columns
COMMENT ON TABLE agents IS 'Stores agent registration information and metadata';
COMMENT ON COLUMN agents.name IS 'Friendly name for the agent';
COMMENT ON COLUMN agents.hostname IS 'Agent hostname (unique)';
COMMENT ON COLUMN agents.ip_address IS 'Agent IP address (IPv4 or IPv6)';
COMMENT ON COLUMN agents.platform IS 'Operating system/platform';
COMMENT ON COLUMN agents.architecture IS 'CPU architecture (amd64, arm64, etc.)';
COMMENT ON COLUMN agents.version IS 'Agent version';
COMMENT ON COLUMN agents.status IS 'Agent status (online, offline, error)';
COMMENT ON COLUMN agents.capabilities IS 'Array of available diagnostic checkers';
COMMENT ON COLUMN agents.labels IS 'User-defined labels for organization';
COMMENT ON COLUMN agents.kubernetes_metadata IS 'Kubernetes-specific metadata (cluster, namespace, node, pod)';
COMMENT ON COLUMN agents.last_heartbeat_at IS 'Timestamp of last heartbeat';
COMMENT ON COLUMN agents.registered_at IS 'Timestamp when agent was registered';
COMMENT ON COLUMN agents.updated_at IS 'Timestamp of last update';

-- +goose Down
DROP INDEX IF EXISTS idx_agents_k8s_metadata;
DROP INDEX IF EXISTS idx_agents_labels;
DROP INDEX IF EXISTS idx_agents_hostname;
DROP INDEX IF EXISTS idx_agents_registered_at;
DROP INDEX IF EXISTS idx_agents_last_heartbeat;
DROP INDEX IF EXISTS idx_agents_platform;
DROP INDEX IF EXISTS idx_agents_status;
DROP TABLE IF EXISTS agents;
