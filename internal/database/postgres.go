package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/sirupsen/logrus"
)

// PostgresConfig contains PostgreSQL connection configuration
type PostgresConfig struct {
	Host            string
	Port            int
	Name            string
	User            string
	Password        string
	SSLMode         string
	MaxConnections  int
	MaxIdle         int
	ConnMaxLifetime time.Duration
}

// DB wraps the SQL database connection with additional context
type DB struct {
	*sql.DB
	config *PostgresConfig
	logger *logrus.Logger
}

// NewPostgresDB creates a new PostgreSQL database connection with connection pooling
func NewPostgresDB(config *PostgresConfig, logger *logrus.Logger) (*DB, error) {
	if config == nil {
		return nil, fmt.Errorf("database config cannot be nil")
	}

	if logger == nil {
		logger = logrus.New()
	}

	// Build connection string
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host,
		config.Port,
		config.User,
		config.Password,
		config.Name,
		config.SSLMode,
	)

	// Open database connection
	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(config.MaxConnections)
	sqlDB.SetMaxIdleConns(config.MaxIdle)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute) // Close idle connections after 10 minutes

	// Log pool configuration
	logger.WithFields(logrus.Fields{
		"host":              config.Host,
		"port":              config.Port,
		"database":          config.Name,
		"max_open_conns":    config.MaxConnections,
		"max_idle_conns":    config.MaxIdle,
		"conn_max_lifetime": config.ConnMaxLifetime,
	}).Info("Database connection pool configured")

	// Validate pool settings and log warnings
	if config.MaxIdle > config.MaxConnections {
		logger.Warn("max_idle > max_connections - this is unusual and may cause issues")
	}
	if config.MaxConnections > 200 {
		logger.Warn("max_connections > 200 - ensure PostgreSQL max_connections is increased server-side (edit postgresql.conf)")
	}
	if config.MaxConnections < 10 {
		logger.Warn("max_connections < 10 - this is very low and may cause connection bottlenecks")
	}

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		if closeErr := sqlDB.Close(); closeErr != nil {
			logger.WithError(closeErr).Warn("Failed to close database connection after ping failure")
		}
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		DB:     sqlDB,
		config: config,
		logger: logger,
	}

	logger.Info("PostgreSQL connection established successfully")

	return db, nil
}

// Close closes the database connection and logs the action
func (db *DB) Close() error {
	db.logger.Info("Closing database connection")
	if err := db.DB.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	return nil
}

// Health checks the database connection health
func (db *DB) Health() error {
	if err := db.Ping(); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}
	return nil
}

// Stats returns database connection pool statistics
func (db *DB) Stats() sql.DBStats {
	return db.DB.Stats()
}

// GetPoolStats returns detailed connection pool statistics as a map
func (db *DB) GetPoolStats() map[string]interface{} {
	stats := db.DB.Stats()

	utilization := float64(0)
	if stats.MaxOpenConnections > 0 {
		utilization = float64(stats.InUse) / float64(stats.MaxOpenConnections) * 100
	}

	return map[string]interface{}{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount, // Total # of connections waited for
		"wait_duration_ms":     stats.WaitDuration.Milliseconds(),
		"max_idle_closed":      stats.MaxIdleClosed,     // Closed due to max idle
		"max_lifetime_closed":  stats.MaxLifetimeClosed, // Closed due to max lifetime
		"utilization_percent":  utilization,
	}
}

// LogPoolStats logs current connection pool statistics
// Call this periodically to monitor pool health
func (db *DB) LogPoolStats() {
	stats := db.GetPoolStats()
	utilizationPercent, ok := stats["utilization_percent"].(float64)

	// Warn if pool is saturated
	if ok && utilizationPercent > 90 {
		db.logger.WithFields(logrus.Fields(stats)).Warn("Connection pool saturation detected (>90% utilization) - consider increasing max_connections")
	} else if ok && utilizationPercent > 75 {
		db.logger.WithFields(logrus.Fields(stats)).Warn("Connection pool utilization high (>75%)")
	} else {
		db.logger.WithFields(logrus.Fields(stats)).Info("Database connection pool stats")
	}
}

// SetConnectionPoolLimits updates connection pool settings dynamically
// Useful for auto-scaling based on runtime metrics
func (db *DB) SetConnectionPoolLimits(maxOpen, maxIdle int, lifetime time.Duration) {
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(lifetime)

	db.config.MaxConnections = maxOpen
	db.config.MaxIdle = maxIdle
	db.config.ConnMaxLifetime = lifetime

	db.logger.WithFields(logrus.Fields{
		"max_open":      maxOpen,
		"max_idle":      maxIdle,
		"conn_lifetime": lifetime,
	}).Info("Updated database connection pool settings")
}
