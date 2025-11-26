package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*RedisClient, *miniredis.Miniredis, func()) {
	t.Helper()

	// Create miniredis server
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Create config
	config := &RedisConfig{
		Enabled:    true,
		RedisURL:   "redis://" + mr.Addr(),
		MaxRetries: 3,
		PoolSize:   10,
		TTL:        5 * time.Minute,
	}

	// Create logger (silent for tests)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Create client
	client, err := NewRedisClient(config, logger)
	require.NoError(t, err)

	cleanup := func() {
		if client != nil {
			_ = client.Close()
		}
		mr.Close()
	}

	return client, mr, cleanup
}

func TestNewRedisClient(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		client, _, cleanup := setupTestRedis(t)
		defer cleanup()

		assert.NotNil(t, client)
		assert.NotNil(t, client.client)
		assert.NotNil(t, client.config)
		assert.NotNil(t, client.logger)
	})

	t.Run("NilConfig", func(t *testing.T) {
		client, err := NewRedisClient(nil, logrus.New())
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("DisabledRedis", func(t *testing.T) {
		config := &RedisConfig{
			Enabled: false,
		}
		client, err := NewRedisClient(config, logrus.New())
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "disabled")
	})

	t.Run("InvalidURL", func(t *testing.T) {
		config := &RedisConfig{
			Enabled:  true,
			RedisURL: "invalid-url",
		}
		client, err := NewRedisClient(config, logrus.New())
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "failed to parse")
	})

	t.Run("ConnectionFailure", func(t *testing.T) {
		config := &RedisConfig{
			Enabled:  true,
			RedisURL: "redis://localhost:9999", // Non-existent server
		}
		client, err := NewRedisClient(config, logrus.New())
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "failed to ping")
	})

	t.Run("NilLogger", func(t *testing.T) {
		mr, err := miniredis.Run()
		require.NoError(t, err)
		defer mr.Close()

		config := &RedisConfig{
			Enabled:  true,
			RedisURL: "redis://" + mr.Addr(),
		}
		client, err := NewRedisClient(config, nil)
		require.NoError(t, err)
		defer func() { _ = client.Close() }()

		assert.NotNil(t, client.logger) // Should create default logger
	})
}

func TestRedisClient_Set(t *testing.T) {
	client, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		err := client.Set(ctx, "test-key", "test-value")
		assert.NoError(t, err)

		// Verify value was set
		val, err := mr.Get("test-key")
		assert.NoError(t, err)
		assert.Equal(t, "test-value", val)
	})

	t.Run("WithTTL", func(t *testing.T) {
		err := client.Set(ctx, "ttl-key", "ttl-value")
		assert.NoError(t, err)

		// Check TTL was set
		ttl := mr.TTL("ttl-key")
		assert.Greater(t, ttl, time.Duration(0))
		assert.LessOrEqual(t, ttl, client.config.TTL)
	})

	t.Run("OverwriteExisting", func(t *testing.T) {
		err := client.Set(ctx, "overwrite-key", "value1")
		assert.NoError(t, err)

		err = client.Set(ctx, "overwrite-key", "value2")
		assert.NoError(t, err)

		val, err := mr.Get("overwrite-key")
		assert.NoError(t, err)
		assert.Equal(t, "value2", val)
	})
}

func TestRedisClient_SetWithTTL(t *testing.T) {
	client, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("CustomTTL", func(t *testing.T) {
		customTTL := 10 * time.Second
		err := client.SetWithTTL(ctx, "custom-ttl-key", "value", customTTL)
		assert.NoError(t, err)

		// Check custom TTL was set
		ttl := mr.TTL("custom-ttl-key")
		assert.Greater(t, ttl, time.Duration(0))
		assert.LessOrEqual(t, ttl, customTTL)
	})

	t.Run("ZeroTTL", func(t *testing.T) {
		err := client.SetWithTTL(ctx, "no-ttl-key", "value", 0)
		assert.NoError(t, err)

		// Key should exist but have no expiration
		val, err := mr.Get("no-ttl-key")
		assert.NoError(t, err)
		assert.Equal(t, "value", val)
	})
}

func TestRedisClient_Get(t *testing.T) {
	client, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		_ = mr.Set("existing-key", "existing-value")

		val, err := client.Get(ctx, "existing-key")
		assert.NoError(t, err)
		assert.Equal(t, "existing-value", val)
	})

	t.Run("KeyNotFound", func(t *testing.T) {
		val, err := client.Get(ctx, "non-existent-key")
		assert.Error(t, err)
		assert.Empty(t, val)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("GetAfterSet", func(t *testing.T) {
		err := client.Set(ctx, "set-get-key", "set-get-value")
		require.NoError(t, err)

		val, err := client.Get(ctx, "set-get-key")
		assert.NoError(t, err)
		assert.Equal(t, "set-get-value", val)
	})
}

func TestRedisClient_Delete(t *testing.T) {
	client, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("SingleKey", func(t *testing.T) {
		_ = mr.Set("delete-key", "delete-value")

		err := client.Delete(ctx, "delete-key")
		assert.NoError(t, err)

		// Verify key was deleted
		exists := mr.Exists("delete-key")
		assert.False(t, exists)
	})

	t.Run("MultipleKeys", func(t *testing.T) {
		_ = mr.Set("key1", "value1")
		_ = mr.Set("key2", "value2")
		_ = mr.Set("key3", "value3")

		err := client.Delete(ctx, "key1", "key2", "key3")
		assert.NoError(t, err)

		assert.False(t, mr.Exists("key1"))
		assert.False(t, mr.Exists("key2"))
		assert.False(t, mr.Exists("key3"))
	})

	t.Run("NonExistentKey", func(t *testing.T) {
		err := client.Delete(ctx, "non-existent")
		assert.NoError(t, err) // Redis doesn't error on deleting non-existent keys
	})
}

func TestRedisClient_Exists(t *testing.T) {
	client, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("SingleKeyExists", func(t *testing.T) {
		_ = mr.Set("exists-key", "value")

		count, err := client.Exists(ctx, "exists-key")
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("SingleKeyNotExists", func(t *testing.T) {
		count, err := client.Exists(ctx, "non-existent")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("MultipleKeys", func(t *testing.T) {
		_ = mr.Set("key1", "value1")
		_ = mr.Set("key2", "value2")

		count, err := client.Exists(ctx, "key1", "key2", "key3")
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count) // 2 out of 3 exist
	})
}

func TestRedisClient_Health(t *testing.T) {
	t.Run("Healthy", func(t *testing.T) {
		client, _, cleanup := setupTestRedis(t)
		defer cleanup()

		err := client.Health(context.Background())
		assert.NoError(t, err)
	})

	t.Run("HealthyWithNilContext", func(t *testing.T) {
		client, _, cleanup := setupTestRedis(t)
		defer cleanup()

		err := client.Health(nil) //nolint:staticcheck
		assert.NoError(t, err)
	})

	t.Run("Unhealthy", func(t *testing.T) {
		client, mr, cleanup := setupTestRedis(t)
		defer cleanup()

		// Close the miniredis server to simulate unhealthy state
		mr.Close()

		err := client.Health(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "health check failed")
	})
}

func TestRedisClient_Close(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		client, mr, _ := setupTestRedis(t)
		defer mr.Close()

		err := client.Close()
		assert.NoError(t, err)

		// Verify connection is closed by attempting an operation
		err = client.Health(context.Background())
		assert.Error(t, err)
	})
}

func TestRedisClient_GetClient(t *testing.T) {
	client, _, cleanup := setupTestRedis(t)
	defer cleanup()

	underlyingClient := client.GetClient()
	assert.NotNil(t, underlyingClient)
	assert.Equal(t, client.client, underlyingClient)
}

func TestRedisClient_ContextCancellation(t *testing.T) {
	client, _, cleanup := setupTestRedis(t)
	defer cleanup()

	t.Run("SetWithCancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := client.Set(ctx, "cancel-key", "value")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("GetWithCancelledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := client.Get(ctx, "cancel-key")
		assert.Error(t, err)
	})
}

func TestRedisClient_IntegrationWorkflow(t *testing.T) {
	client, _, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	// Set a value
	err := client.Set(ctx, "workflow-key", "workflow-value")
	require.NoError(t, err)

	// Verify it exists
	count, err := client.Exists(ctx, "workflow-key")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Get the value
	val, err := client.Get(ctx, "workflow-key")
	require.NoError(t, err)
	assert.Equal(t, "workflow-value", val)

	// Update the value
	err = client.Set(ctx, "workflow-key", "updated-value")
	require.NoError(t, err)

	// Verify update
	val, err = client.Get(ctx, "workflow-key")
	require.NoError(t, err)
	assert.Equal(t, "updated-value", val)

	// Delete the key
	err = client.Delete(ctx, "workflow-key")
	require.NoError(t, err)

	// Verify deletion
	count, err = client.Exists(ctx, "workflow-key")
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}
