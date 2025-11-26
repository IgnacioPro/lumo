// Package integration provides integration test utilities using testcontainers.
package integration

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ignacio/lumo/internal/api"
	"github.com/ignacio/lumo/internal/api/auth"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestEnv holds the test environment configuration
type TestEnv struct {
	DB          *database.DB
	Config      *config.Config
	JWTManager  *auth.JWTManager
	Logger      *logrus.Logger
	PostgresCtr testcontainers.Container
	ctx         context.Context
}

// Migrations contains the SQL statements to set up the test database
var Migrations = []string{
	// 001_init_schema.sql - Jobs table
	`CREATE TABLE IF NOT EXISTS jobs (
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
	)`,
	`CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status)`,
	`CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type)`,
	`CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at DESC)`,

	// 002_api_keys.sql
	`CREATE TABLE IF NOT EXISTS api_keys (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		key_hash VARCHAR(64) UNIQUE NOT NULL,
		name VARCHAR(255) NOT NULL,
		scopes TEXT[] NOT NULL DEFAULT '{}',
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		last_used_at TIMESTAMP,
		expires_at TIMESTAMP,
		revoked BOOLEAN DEFAULT FALSE,
		metadata JSONB DEFAULT '{}'::jsonb
	)`,
	`CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash)`,

	// 003_agents.sql
	`CREATE TABLE IF NOT EXISTS agents (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(255) NOT NULL,
		hostname VARCHAR(255) NOT NULL,
		ip_address VARCHAR(45),
		platform VARCHAR(50) NOT NULL CHECK (platform IN ('linux', 'darwin', 'windows', 'kubernetes')),
		architecture VARCHAR(50) NOT NULL,
		version VARCHAR(50) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'offline', 'error')),
		capabilities TEXT[] NOT NULL DEFAULT '{}',
		labels JSONB DEFAULT '{}'::jsonb,
		kubernetes_metadata JSONB,
		last_heartbeat_at TIMESTAMP NOT NULL DEFAULT NOW(),
		registered_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
		UNIQUE(hostname)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status)`,
	`CREATE INDEX IF NOT EXISTS idx_agents_hostname ON agents(hostname)`,

	// 004_approvals.sql
	`CREATE TABLE IF NOT EXISTS approvals (
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
		metadata JSONB DEFAULT '{}'::jsonb
	)`,
	`CREATE INDEX IF NOT EXISTS idx_approvals_job_id ON approvals(job_id)`,
	`CREATE INDEX IF NOT EXISTS idx_approvals_status ON approvals(status)`,

	// 005_events.sql
	`CREATE TABLE IF NOT EXISTS events (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
		event_type VARCHAR(100) NOT NULL,
		severity VARCHAR(20) NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
		resource_kind VARCHAR(50) NOT NULL,
		resource_name VARCHAR(255) NOT NULL,
		resource_uid VARCHAR(255),
		namespace VARCHAR(255),
		message TEXT NOT NULL,
		metadata JSONB DEFAULT '{}'::jsonb,
		event_timestamp TIMESTAMP NOT NULL,
		ai_analysis TEXT,
		ai_analyzed_at TIMESTAMP,
		notification_sent BOOLEAN DEFAULT FALSE,
		notification_sent_at TIMESTAMP,
		notification_channels TEXT[],
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	)`,
	`CREATE INDEX IF NOT EXISTS idx_events_agent_id ON events(agent_id)`,
	`CREATE INDEX IF NOT EXISTS idx_events_severity ON events(severity)`,
}

// NewTestEnv creates a new test environment with PostgreSQL container
func NewTestEnv(ctx context.Context) (*TestEnv, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Start PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("lumo_test"),
		postgres.WithUsername("lumo"),
		postgres.WithPassword("lumo_test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	// Get connection string
	host, err := pgContainer.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	mappedPort, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	// Create database config
	dbConfig := &database.PostgresConfig{
		Host:            host,
		Port:            mappedPort.Int(),
		Name:            "lumo_test",
		User:            "lumo",
		Password:        "lumo_test",
		SSLMode:         "disable",
		MaxConnections:  10,
		MaxIdle:         5,
		ConnMaxLifetime: 5 * time.Minute,
	}

	// Connect to database
	db, err := database.NewPostgresDB(dbConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migrations
	for _, migration := range Migrations {
		if _, err := db.Exec(migration); err != nil {
			return nil, fmt.Errorf("failed to run migration: %w", err)
		}
	}

	// Create config
	cfg := config.DefaultConfig()
	cfg.API.JWTSecret = "test-secret-for-integration-tests-32!"
	cfg.API.JWTExpiration = 24 * time.Hour
	cfg.API.JWTIssuer = "lumo-test"
	cfg.API.RateLimitEnabled = false // Disable for tests
	cfg.AI.Enabled = false           // Disable AI for basic tests
	cfg.Notifications.Enabled = false

	// Create JWT manager
	jwtManager, err := auth.NewJWTManager(cfg.API.JWTSecret, cfg.API.JWTExpiration, cfg.API.JWTIssuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT manager: %w", err)
	}

	return &TestEnv{
		DB:          db,
		Config:      cfg,
		JWTManager:  jwtManager,
		Logger:      logger,
		PostgresCtr: pgContainer,
		ctx:         ctx,
	}, nil
}

// CreateRouter creates a new API router for testing
func (env *TestEnv) CreateRouter() *chi.Mux {
	return api.NewRouter(env.DB, env.Config, env.JWTManager, env.Logger)
}

// CreateAPIKey creates a test API key and returns the raw key
func (env *TestEnv) CreateAPIKey(name string, scopes []string) (string, error) {
	rawKey := fmt.Sprintf("test-key-%s-%d", name, time.Now().UnixNano())
	keyHash := models.HashAPIKey(rawKey)

	scopesArray := "{" + joinScopes(scopes) + "}"

	_, err := env.DB.Exec(`
		INSERT INTO api_keys (key_hash, name, scopes)
		VALUES ($1, $2, $3)
	`, keyHash, name, scopesArray)
	if err != nil {
		return "", fmt.Errorf("failed to create API key: %w", err)
	}

	return rawKey, nil
}

// CreateTestAgent creates a test agent and returns its ID
func (env *TestEnv) CreateTestAgent(name, hostname string) (string, error) {
	var agentID string
	err := env.DB.QueryRow(`
		INSERT INTO agents (name, hostname, platform, architecture, version, capabilities)
		VALUES ($1, $2, 'linux', 'amd64', '1.0.0', '{"cpu","memory","disk"}')
		RETURNING id
	`, name, hostname).Scan(&agentID)
	if err != nil {
		return "", fmt.Errorf("failed to create test agent: %w", err)
	}
	return agentID, nil
}

// CreateTestJob creates a test job and returns its ID
func (env *TestEnv) CreateTestJob(jobType, status, target string) (string, error) {
	var jobID string
	err := env.DB.QueryRow(`
		INSERT INTO jobs (type, status, target, created_by, result, metadata)
		VALUES ($1, $2, $3, 'test-user', '{}', '{}')
		RETURNING id
	`, jobType, status, target).Scan(&jobID)
	if err != nil {
		return "", fmt.Errorf("failed to create test job: %w", err)
	}
	return jobID, nil
}

// CleanupTables truncates all tables for test isolation
func (env *TestEnv) CleanupTables() error {
	tables := []string{"events", "approvals", "jobs", "agents", "api_keys"}
	for _, table := range tables {
		if _, err := env.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)); err != nil {
			return fmt.Errorf("failed to truncate %s: %w", table, err)
		}
	}
	return nil
}

// Close cleans up the test environment
func (env *TestEnv) Close() error {
	if env.DB != nil {
		_ = env.DB.Close()
	}
	if env.PostgresCtr != nil {
		return env.PostgresCtr.Terminate(env.ctx)
	}
	return nil
}

// GetDB returns the underlying sql.DB for direct queries
func (env *TestEnv) GetDB() *sql.DB {
	return env.DB.DB
}

func joinScopes(scopes []string) string {
	if len(scopes) == 0 {
		return ""
	}
	result := ""
	for i, s := range scopes {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}
