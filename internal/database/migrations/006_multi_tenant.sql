-- +goose Up
-- +goose StatementBegin

-- ============================================
-- TENANT MANAGEMENT (public schema)
-- ============================================

-- Core tenant table
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    
    -- Plan & Subscription
    plan VARCHAR(50) NOT NULL DEFAULT 'trial',
    plan_started_at TIMESTAMP WITH TIME ZONE,
    plan_expires_at TIMESTAMP WITH TIME ZONE,
    stripe_customer_id VARCHAR(255),
    stripe_subscription_id VARCHAR(255),
    
    -- Limits (based on plan)
    max_agents INTEGER NOT NULL DEFAULT 5,
    max_events_per_day INTEGER NOT NULL DEFAULT 10000,
    max_users INTEGER NOT NULL DEFAULT 3,
    ai_analysis_enabled BOOLEAN DEFAULT true,
    retention_days INTEGER NOT NULL DEFAULT 30,
    
    -- Isolation tier
    isolation_tier VARCHAR(50) NOT NULL DEFAULT 'shared',
    dedicated_namespace VARCHAR(255),
    dedicated_db_host VARCHAR(255),
    
    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    suspended_reason TEXT,
    
    -- Metadata
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT tenants_plan_check CHECK (plan IN ('trial', 'starter', 'pro', 'enterprise')),
    CONSTRAINT tenants_status_check CHECK (status IN ('active', 'suspended', 'cancelled', 'pending')),
    CONSTRAINT tenants_isolation_tier_check CHECK (isolation_tier IN ('shared', 'dedicated'))
);

CREATE INDEX idx_tenants_slug ON tenants(slug);
CREATE INDEX idx_tenants_status ON tenants(status);
CREATE INDEX idx_tenants_plan ON tenants(plan);
CREATE INDEX idx_tenants_stripe_customer ON tenants(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;

-- Tenant API Keys (for agent provisioning)
CREATE TABLE IF NOT EXISTS tenant_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(64) NOT NULL,
    key_prefix VARCHAR(12) NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT ARRAY['agent:register', 'agent:read'],
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_by UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT tenant_api_keys_unique_prefix UNIQUE (key_prefix)
);

CREATE INDEX idx_tenant_api_keys_tenant ON tenant_api_keys(tenant_id);
CREATE INDEX idx_tenant_api_keys_prefix ON tenant_api_keys(key_prefix);
CREATE INDEX idx_tenant_api_keys_not_revoked ON tenant_api_keys(tenant_id) WHERE revoked_at IS NULL;

-- Tenant Users (for portal access)
CREATE TABLE IF NOT EXISTS tenant_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255),
    name VARCHAR(255),
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    
    -- SSO
    sso_provider VARCHAR(50),
    sso_id VARCHAR(255),
    
    -- MFA
    mfa_enabled BOOLEAN DEFAULT false,
    mfa_secret VARCHAR(255),
    
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT tenant_users_role_check CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    CONSTRAINT tenant_users_status_check CHECK (status IN ('active', 'inactive', 'pending')),
    CONSTRAINT tenant_users_unique_email_tenant UNIQUE (tenant_id, email)
);

CREATE INDEX idx_tenant_users_tenant ON tenant_users(tenant_id);
CREATE INDEX idx_tenant_users_email ON tenant_users(email);

-- ============================================
-- USAGE TRACKING
-- ============================================

-- Daily usage aggregation per tenant
CREATE TABLE IF NOT EXISTS tenant_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    
    -- Counts
    agent_count INTEGER DEFAULT 0,
    event_count INTEGER DEFAULT 0,
    ai_analysis_count INTEGER DEFAULT 0,
    notification_count INTEGER DEFAULT 0,
    
    -- Storage (bytes)
    storage_used_bytes BIGINT DEFAULT 0,
    
    -- Computed metrics
    peak_agents INTEGER DEFAULT 0,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT tenant_usage_unique_date UNIQUE (tenant_id, date)
);

CREATE INDEX idx_tenant_usage_tenant_date ON tenant_usage(tenant_id, date DESC);

-- ============================================
-- AUDIT LOG
-- ============================================

CREATE TABLE IF NOT EXISTS audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE SET NULL,
    user_id UUID,
    agent_id UUID,
    
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    
    ip_address INET,
    user_agent TEXT,
    
    details JSONB,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_audit_log_tenant ON audit_log(tenant_id, created_at DESC);
CREATE INDEX idx_audit_log_action ON audit_log(action, created_at DESC);
CREATE INDEX idx_audit_log_created ON audit_log(created_at DESC);

-- ============================================
-- ADD TENANT REFERENCE TO EXISTING TABLES
-- ============================================

-- Add tenant_id to agents table
ALTER TABLE agents ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_agents_tenant_id ON agents(tenant_id);

-- Add tenant_id to events table
ALTER TABLE events ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_events_tenant_id ON events(tenant_id);

-- Add tenant_id to jobs table
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_jobs_tenant_id ON jobs(tenant_id);

-- Add tenant_id to approvals table
ALTER TABLE approvals ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_approvals_tenant_id ON approvals(tenant_id);

-- ============================================
-- DEFAULT TENANT FOR BACKWARDS COMPATIBILITY
-- ============================================

-- Create a default tenant for existing data (single-tenant mode)
INSERT INTO tenants (id, name, slug, display_name, plan, status, max_agents, max_events_per_day, max_users, isolation_tier)
VALUES (
    '00000000-0000-0000-0000-000000000000',
    'default',
    'default',
    'Default Tenant',
    'enterprise',
    'active',
    999999,
    999999999,
    999999,
    'shared'
) ON CONFLICT (slug) DO NOTHING;

-- Update existing records to belong to default tenant
UPDATE agents SET tenant_id = '00000000-0000-0000-0000-000000000000' WHERE tenant_id IS NULL;
UPDATE events SET tenant_id = '00000000-0000-0000-0000-000000000000' WHERE tenant_id IS NULL;
UPDATE jobs SET tenant_id = '00000000-0000-0000-0000-000000000000' WHERE tenant_id IS NULL;
UPDATE approvals SET tenant_id = '00000000-0000-0000-0000-000000000000' WHERE tenant_id IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove tenant_id from existing tables
ALTER TABLE agents DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE events DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE jobs DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE approvals DROP COLUMN IF EXISTS tenant_id;

-- Drop new tables
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS tenant_usage;
DROP TABLE IF EXISTS tenant_users;
DROP TABLE IF EXISTS tenant_api_keys;
DROP TABLE IF EXISTS tenants;

-- +goose StatementEnd
