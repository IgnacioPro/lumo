-- +goose Up
-- Incidents table: stores correlated incidents for real-time processing
-- Incidents persist events, AI analysis, and postmortems across the lifecycle

CREATE TABLE IF NOT EXISTS incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID,
    
    -- Classification
    category VARCHAR(50) NOT NULL,  -- memory, crash, image, storage, node, scheduling, deployment, network, unknown
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    status VARCHAR(20) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'analyzing', 'resolved', 'suppressed')),
    is_critical BOOLEAN NOT NULL DEFAULT FALSE,  -- True for critical services (label: critical=true)
    
    -- Identification
    title VARCHAR(500),
    correlation_key VARCHAR(255) NOT NULL,
    correlation_reason TEXT,
    
    -- Analysis (incremental, appended as insights come)
    root_cause TEXT,
    summary TEXT,
    ai_analysis JSONB DEFAULT '{}'::jsonb,  -- Structured AIAnalysisResult
    postmortem TEXT,  -- Generated when incident closes
    
    -- Affected resources (denormalized for quick access)
    affected_resources JSONB DEFAULT '[]'::jsonb,
    primary_resource JSONB,
    
    -- Context
    cluster_name VARCHAR(255),
    node_name VARCHAR(255),
    namespace VARCHAR(255),
    context JSONB DEFAULT '{}'::jsonb,  -- IncidentContext (logs, metrics, k8s events)
    
    -- Timeline and metadata
    timeline JSONB DEFAULT '[]'::jsonb,
    metadata JSONB DEFAULT '{}'::jsonb,
    
    -- Timestamps
    first_event_at TIMESTAMP NOT NULL,
    last_event_at TIMESTAMP NOT NULL,
    opened_at TIMESTAMP NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMP,
    analyzed_at TIMESTAMP,
    notified_at TIMESTAMP,
    last_health_check_at TIMESTAMP,  -- For auto-close health monitoring
    
    -- Notification tracking
    notification_channels TEXT[],
    notification_count INTEGER DEFAULT 0,  -- How many notifications sent (progress updates)
    last_notification_at TIMESTAMP,
    
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Junction table: events belonging to incidents
CREATE TABLE IF NOT EXISTS incident_events (
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    added_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (incident_id, event_id)
);

-- Incident analysis log: tracks incremental AI analysis updates
CREATE TABLE IF NOT EXISTS incident_analysis_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    analysis_type VARCHAR(50) NOT NULL,  -- initial, update, insight, root_cause, postmortem
    content TEXT NOT NULL,
    notified BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for incidents
CREATE INDEX IF NOT EXISTS idx_incidents_tenant_id ON incidents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_incidents_status ON incidents(status);
CREATE INDEX IF NOT EXISTS idx_incidents_is_critical ON incidents(is_critical);
CREATE INDEX IF NOT EXISTS idx_incidents_category ON incidents(category);
CREATE INDEX IF NOT EXISTS idx_incidents_severity ON incidents(severity);
CREATE INDEX IF NOT EXISTS idx_incidents_correlation_key ON incidents(correlation_key);
CREATE INDEX IF NOT EXISTS idx_incidents_namespace ON incidents(namespace);
CREATE INDEX IF NOT EXISTS idx_incidents_opened_at ON incidents(opened_at DESC);
CREATE INDEX IF NOT EXISTS idx_incidents_closed_at ON incidents(closed_at DESC);

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_incidents_status_opened ON incidents(status, opened_at DESC) WHERE status = 'open';
CREATE INDEX IF NOT EXISTS idx_incidents_critical_open ON incidents(is_critical, status) WHERE is_critical = TRUE AND status = 'open';
CREATE INDEX IF NOT EXISTS idx_incidents_tenant_status ON incidents(tenant_id, status);

-- Indexes for incident_events
CREATE INDEX IF NOT EXISTS idx_incident_events_incident ON incident_events(incident_id);
CREATE INDEX IF NOT EXISTS idx_incident_events_event ON incident_events(event_id);

-- Indexes for analysis log
CREATE INDEX IF NOT EXISTS idx_incident_analysis_log_incident ON incident_analysis_log(incident_id);
CREATE INDEX IF NOT EXISTS idx_incident_analysis_log_type ON incident_analysis_log(analysis_type);

-- Add incident_id to events table for reverse lookup
ALTER TABLE events ADD COLUMN IF NOT EXISTS incident_id UUID REFERENCES incidents(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_events_incident_id ON events(incident_id);

-- Comments
COMMENT ON TABLE incidents IS 'Correlated incidents grouping related Kubernetes events for AI analysis';
COMMENT ON COLUMN incidents.is_critical IS 'True if incident affects critical services (bypasses debouncing)';
COMMENT ON COLUMN incidents.status IS 'Lifecycle: open (collecting), analyzing (AI processing), resolved (closed), suppressed (duplicate)';
COMMENT ON COLUMN incidents.postmortem IS 'Auto-generated postmortem when incident closes';
COMMENT ON COLUMN incidents.notification_count IS 'Number of progress notifications sent during incident';
COMMENT ON COLUMN incidents.last_health_check_at IS 'Last time health was checked for auto-close';
COMMENT ON TABLE incident_events IS 'Junction table linking events to their parent incident';
COMMENT ON TABLE incident_analysis_log IS 'Incremental AI analysis updates and insights';

-- +goose Down
DROP INDEX IF EXISTS idx_events_incident_id;
ALTER TABLE events DROP COLUMN IF EXISTS incident_id;
DROP INDEX IF EXISTS idx_incident_analysis_log_type;
DROP INDEX IF EXISTS idx_incident_analysis_log_incident;
DROP INDEX IF EXISTS idx_incident_events_event;
DROP INDEX IF EXISTS idx_incident_events_incident;
DROP INDEX IF EXISTS idx_incidents_tenant_status;
DROP INDEX IF EXISTS idx_incidents_critical_open;
DROP INDEX IF EXISTS idx_incidents_status_opened;
DROP INDEX IF EXISTS idx_incidents_closed_at;
DROP INDEX IF EXISTS idx_incidents_opened_at;
DROP INDEX IF EXISTS idx_incidents_namespace;
DROP INDEX IF EXISTS idx_incidents_correlation_key;
DROP INDEX IF EXISTS idx_incidents_severity;
DROP INDEX IF EXISTS idx_incidents_category;
DROP INDEX IF EXISTS idx_incidents_is_critical;
DROP INDEX IF EXISTS idx_incidents_status;
DROP INDEX IF EXISTS idx_incidents_tenant_id;
DROP TABLE IF EXISTS incident_analysis_log;
DROP TABLE IF EXISTS incident_events;
DROP TABLE IF EXISTS incidents;
