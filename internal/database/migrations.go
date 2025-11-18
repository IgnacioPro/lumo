package database

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// RunMigrations runs all pending database migrations
func RunMigrations(db *sql.DB, logger *logrus.Logger) error {
	if logger == nil {
		logger = logrus.New()
	}

	// Set goose to use the embedded filesystem
	goose.SetBaseFS(embedMigrations)

	// Set the dialect to postgres
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	logger.Info("Running database migrations...")

	// Run migrations
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Get current version
	version, err := goose.GetDBVersion(db)
	if err != nil {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	logger.WithField("version", version).Info("Database migrations completed successfully")
	return nil
}

// RollbackMigration rolls back the last migration
func RollbackMigration(db *sql.DB, logger *logrus.Logger) error {
	if logger == nil {
		logger = logrus.New()
	}

	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	logger.Info("Rolling back last migration...")

	if err := goose.Down(db, "migrations"); err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	version, err := goose.GetDBVersion(db)
	if err != nil {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	logger.WithField("version", version).Info("Migration rollback completed")
	return nil
}

// MigrationStatus returns the current migration status
func MigrationStatus(db *sql.DB) (int64, error) {
	if err := goose.SetDialect("postgres"); err != nil {
		return 0, fmt.Errorf("failed to set goose dialect: %w", err)
	}

	version, err := goose.GetDBVersion(db)
	if err != nil {
		return 0, fmt.Errorf("failed to get migration version: %w", err)
	}

	return version, nil
}
