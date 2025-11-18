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

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		DB:     sqlDB,
		config: config,
		logger: logger,
	}

	logger.WithFields(logrus.Fields{
		"host":            config.Host,
		"port":            config.Port,
		"database":        config.Name,
		"max_connections": config.MaxConnections,
		"max_idle":        config.MaxIdle,
	}).Info("PostgreSQL connection established")

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
