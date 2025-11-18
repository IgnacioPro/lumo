package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	// Create temporary directory for cache
	tempDir := t.TempDir()
	logger := logrus.New()
	logger.SetOutput(os.Stderr)

	cache, err := NewCache(tempDir, 1024*1024, 1*time.Hour, logger)
	require.NoError(t, err)

	t.Run("SetAndGet", func(t *testing.T) {
		data := map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		}

		err := cache.Set("test-key", data)
		assert.NoError(t, err)

		retrieved, found, err := cache.Get("test-key")
		assert.NoError(t, err)
		assert.True(t, found)
		assert.NotNil(t, retrieved)
	})

	t.Run("GetNonExistent", func(t *testing.T) {
		retrieved, found, err := cache.Get("nonexistent")
		assert.NoError(t, err)
		assert.False(t, found)
		assert.Nil(t, retrieved)
	})

	t.Run("Delete", func(t *testing.T) {
		err := cache.Set("delete-test", "value")
		assert.NoError(t, err)

		err = cache.Delete("delete-test")
		assert.NoError(t, err)

		_, found, _ := cache.Get("delete-test")
		assert.False(t, found)
	})

	t.Run("Size", func(t *testing.T) {
		err := cache.Clear()
		assert.NoError(t, err)

		err = cache.Set("size-test", "test data")
		assert.NoError(t, err)

		size, err := cache.Size()
		assert.NoError(t, err)
		assert.Greater(t, size, int64(0))
	})

	t.Run("Clear", func(t *testing.T) {
		err := cache.Set("clear-test-1", "value1")
		assert.NoError(t, err)
		err = cache.Set("clear-test-2", "value2")
		assert.NoError(t, err)

		err = cache.Clear()
		assert.NoError(t, err)

		_, found1, _ := cache.Get("clear-test-1")
		_, found2, _ := cache.Get("clear-test-2")
		assert.False(t, found1)
		assert.False(t, found2)
	})

	t.Run("ExpiredEntry", func(t *testing.T) {
		// Create a cache with very short TTL
		shortTTLCache, err := NewCache(
			filepath.Join(tempDir, "short-ttl"),
			1024*1024,
			1*time.Millisecond,
			logger,
		)
		require.NoError(t, err)

		err = shortTTLCache.Set("expire-test", "value")
		assert.NoError(t, err)

		// Wait for entry to expire
		time.Sleep(10 * time.Millisecond)

		_, found, err := shortTTLCache.Get("expire-test")
		assert.NoError(t, err)
		assert.False(t, found)
	})
}
