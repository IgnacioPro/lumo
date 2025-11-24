-- +goose Up
-- Events table: stores Kubernetes events reported by agents for AI analysis and notifications

CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,  -- pod-failed, oom-killed, crash-loop, deployment-failed, etc.
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    resource_kind VARCHAR(50) NOT NULL,  -- Pod, Deployment, StatefulSet, Node, PVC, etc.
    resource_name VARCHAR(255) NOT NULL,  -- Name of the affected resource
    resource_uid VARCHAR(255),  -- Kubernetes UID for grouping related events
    namespace VARCHAR(255),  -- Kubernetes namespace (nullable for cluster-scoped resources)
    message TEXT NOT NULL,  -- Human-readable event description
    metadata JSONB DEFAULT '{}'::jsonb,  -- Additional event data (labels, annotations, conditions, etc.)
    event_timestamp TIMESTAMP NOT NULL,  -- When the event occurred in the cluster
    ai_analysis TEXT,  -- AI-generated analysis (populated by API after submission)
    ai_analyzed_at TIMESTAMP,  -- When AI analysis was performed
    notification_sent BOOLEAN DEFAULT FALSE,  -- Whether notification was sent
    notification_sent_at TIMESTAMP,  -- When notification was sent
    notification_channels TEXT[],  -- Which channels received notification (slack, email, etc.)
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),  -- When recorded in database
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_events_agent_id ON events(agent_id);
CREATE INDEX IF NOT EXISTS idx_events_event_type ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_severity ON events(severity);
CREATE INDEX IF NOT EXISTS idx_events_resource_kind ON events(resource_kind);
CREATE INDEX IF NOT EXISTS idx_events_namespace ON events(namespace);
CREATE INDEX IF NOT EXISTS idx_events_resource_uid ON events(resource_uid);
CREATE INDEX IF NOT EXISTS idx_events_event_timestamp ON events(event_timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_notification_sent ON events(notification_sent) WHERE notification_sent = FALSE;

-- Composite indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_events_agent_timestamp ON events(agent_id, event_timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_severity_timestamp ON events(severity, event_timestamp DESC) WHERE severity IN ('high', 'critical');
CREATE INDEX IF NOT EXISTS idx_events_namespace_timestamp ON events(namespace, event_timestamp DESC);

-- GIN index for JSONB metadata (for efficient querying of event details)
CREATE INDEX IF NOT EXISTS idx_events_metadata ON events USING GIN (metadata);

-- Comment on tables and columns
COMMENT ON TABLE events IS 'Kubernetes events reported by agents for centralized AI analysis and notifications';
COMMENT ON COLUMN events.agent_id IS 'Agent that detected and reported the event';
COMMENT ON COLUMN events.event_type IS 'Type of event (pod-failed, oom-killed, crash-loop, etc.)';
COMMENT ON COLUMN events.severity IS 'Event severity level (low, medium, high, critical)';
COMMENT ON COLUMN events.resource_kind IS 'Kubernetes resource kind (Pod, Deployment, Node, etc.)';
COMMENT ON COLUMN events.resource_name IS 'Name of the affected Kubernetes resource';
COMMENT ON COLUMN events.resource_uid IS 'Kubernetes UID for grouping related events';
COMMENT ON COLUMN events.namespace IS 'Kubernetes namespace (null for cluster-scoped resources)';
COMMENT ON COLUMN events.message IS 'Human-readable event description';
COMMENT ON COLUMN events.metadata IS 'Additional event data in JSON format';
COMMENT ON COLUMN events.event_timestamp IS 'When the event occurred in the cluster';
COMMENT ON COLUMN events.ai_analysis IS 'AI-generated analysis and recommendations';
COMMENT ON COLUMN events.ai_analyzed_at IS 'Timestamp when AI analysis was performed';
COMMENT ON COLUMN events.notification_sent IS 'Whether notification was sent to configured channels';
COMMENT ON COLUMN events.notification_sent_at IS 'Timestamp when notification was sent';
COMMENT ON COLUMN events.notification_channels IS 'Array of notification channels that received the event';
COMMENT ON COLUMN events.created_at IS 'Timestamp when event was recorded in database';
COMMENT ON COLUMN events.updated_at IS 'Timestamp of last update';

-- +goose Down
DROP INDEX IF EXISTS idx_events_metadata;
DROP INDEX IF EXISTS idx_events_namespace_timestamp;
DROP INDEX IF EXISTS idx_events_severity_timestamp;
DROP INDEX IF EXISTS idx_events_agent_timestamp;
DROP INDEX IF EXISTS idx_events_notification_sent;
DROP INDEX IF EXISTS idx_events_created_at;
DROP INDEX IF EXISTS idx_events_event_timestamp;
DROP INDEX IF EXISTS idx_events_resource_uid;
DROP INDEX IF EXISTS idx_events_namespace;
DROP INDEX IF EXISTS idx_events_resource_kind;
DROP INDEX IF EXISTS idx_events_severity;
DROP INDEX IF EXISTS idx_events_event_type;
DROP INDEX IF EXISTS idx_events_agent_id;
DROP TABLE IF EXISTS events;
