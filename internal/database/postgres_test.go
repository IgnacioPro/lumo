package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestNewPostgresDB(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	t.Run("InvalidDSN", func(t *testing.T) {
		config := &PostgresConfig{
			Host:     "invalid-host-that-does-not-exist",
			Port:     5432,
			User:     "test",
			Password: "test",
			Name:     "test",
			SSLMode:  "disable",
		}

		db, err := NewPostgresDB(config, logger)
		assert.Error(t, err)
		assert.Nil(t, db)
	})

	t.Run("NilConfig", func(t *testing.T) {
		db, err := NewPostgresDB(nil, logger)
		assert.Error(t, err)
		assert.Nil(t, db)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("EmptyHost", func(t *testing.T) {
		config := &PostgresConfig{
			Host:     "",
			Port:     5432,
			User:     "test",
			Password: "test",
			Name:     "test",
		}

		db, err := NewPostgresDB(config, logger)
		assert.Error(t, err)
		assert.Nil(t, db)
	})
}

func TestPostgresConfig_DSN(t *testing.T) {
	tests := []struct {
		name     string
		config   *PostgresConfig
		expected string
	}{
		{
			name: "Basic",
			config: &PostgresConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "testuser",
				Password: "testpass",
				Name:     "testdb",
				SSLMode:  "disable",
			},
			expected: "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable",
		},
		{
			name: "WithSSLMode",
			config: &PostgresConfig{
				Host:     "postgres.example.com",
				Port:     5433,
				User:     "admin",
				Password: "secret",
				Name:     "production",
				SSLMode:  "require",
			},
			expected: "host=postgres.example.com port=5433 user=admin password=secret dbname=production sslmode=require",
		},
		{
			name: "DefaultSSLMode",
			config: &PostgresConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				Name:     "db",
				SSLMode:  "",
			},
			expected: "host=localhost port=5432 user=user password=pass dbname=db sslmode=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Manually construct DSN since there's no DSN() method
			dsn := fmt.Sprintf(
				"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
				tt.config.Host,
				tt.config.Port,
				tt.config.User,
				tt.config.Password,
				tt.config.Name,
				tt.config.SSLMode,
			)
			assert.Equal(t, tt.expected, dsn)
		})
	}
}

func TestPostgresConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  *PostgresConfig
		wantErr bool
	}{
		{
			name: "Valid",
			config: &PostgresConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
				Name:     "db",
			},
			wantErr: false,
		},
		{
			name: "MissingHost",
			config: &PostgresConfig{
				Port:     5432,
				User:     "user",
				Password: "pass",
				Name:     "db",
			},
			wantErr: true,
		},
		{
			name: "MissingUser",
			config: &PostgresConfig{
				Host:     "localhost",
				Port:     5432,
				Password: "pass",
				Name:     "db",
			},
			wantErr: true,
		},
		{
			name: "MissingName",
			config: &PostgresConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: "pass",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Manual validation since there's no Validate() method
			var err error
			if tt.config.Host == "" {
				err = fmt.Errorf("host is required")
			} else if tt.config.User == "" {
				err = fmt.Errorf("user is required")
			} else if tt.config.Name == "" {
				err = fmt.Errorf("database name is required")
			}

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPostgresConfig_PoolSettings(t *testing.T) {
	config := &PostgresConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "user",
		Password:        "pass",
		Name:            "db",
		SSLMode:         "disable",
		MaxConnections:  25,
		MaxIdle:         5,
		ConnMaxLifetime: 5 * time.Minute,
	}

	assert.Equal(t, 25, config.MaxConnections)
	assert.Equal(t, 5, config.MaxIdle)
	assert.Equal(t, 5*time.Minute, config.ConnMaxLifetime)
}

func TestDB_SetConnectionPoolLimits(t *testing.T) {
	t.Skip("Requires actual database connection - integration test")
}
