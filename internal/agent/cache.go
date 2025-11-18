package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// CacheEntry represents a cached diagnostic result
type CacheEntry struct {
	Timestamp time.Time     `json:"timestamp"`
	Data      interface{}   `json:"data"`
	TTL       time.Duration `json:"ttl"`
}

// Cache provides local storage for diagnostic results
// Enables offline mode when API is unavailable
type Cache struct {
	path    string
	maxSize int64
	ttl     time.Duration
	mu      sync.RWMutex
	logger  *logrus.Logger
}

// NewCache creates a new cache instance
func NewCache(path string, maxSize int64, ttl time.Duration, logger *logrus.Logger) (*Cache, error) {
	// Ensure cache directory exists
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Cache{
		path:    path,
		maxSize: maxSize,
		ttl:     ttl,
		logger:  logger,
	}, nil
}

// Set stores data in the cache with a key
func (c *Cache) Set(key string, data interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry := CacheEntry{
		Timestamp: time.Now(),
		Data:      data,
		TTL:       c.ttl,
	}

	// Serialize entry to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	// Write to file
	filePath := c.getFilePath(key)
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"key":  key,
		"size": len(jsonData),
	}).Debug("Cache entry saved")

	// Check and enforce size limit
	if err := c.enforceMaxSize(); err != nil {
		c.logger.WithError(err).Warn("Failed to enforce cache size limit")
	}

	return nil
}

// Get retrieves data from the cache
func (c *Cache) Get(key string) (interface{}, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	filePath := c.getFilePath(key)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, false, nil
	}

	// Read file
	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read cache file: %w", err)
	}

	// Deserialize entry
	var entry CacheEntry
	if err := json.Unmarshal(jsonData, &entry); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal cache entry: %w", err)
	}

	// Check if entry has expired
	if time.Since(entry.Timestamp) > entry.TTL {
		c.logger.WithField("key", key).Debug("Cache entry expired")
		// Remove expired entry
		if err := os.Remove(filePath); err != nil {
			c.logger.WithError(err).Debug("Failed to remove expired cache entry")
		}
		return nil, false, nil
	}

	c.logger.WithField("key", key).Debug("Cache entry found")
	return entry.Data, true, nil
}

// Delete removes a cache entry
func (c *Cache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	filePath := c.getFilePath(key)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file: %w", err)
	}

	c.logger.WithField("key", key).Debug("Cache entry deleted")
	return nil
}

// Clear removes all cache entries
func (c *Cache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, err := os.ReadDir(c.path)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			filePath := filepath.Join(c.path, entry.Name())
			if err := os.Remove(filePath); err != nil {
				c.logger.WithError(err).WithField("file", entry.Name()).Warn("Failed to delete cache file")
			}
		}
	}

	c.logger.Info("Cache cleared")
	return nil
}

// Size returns the current cache size in bytes
func (c *Cache) Size() (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sizeNoLock()
}

// sizeNoLock returns the current cache size without acquiring lock (internal use only)
func (c *Cache) sizeNoLock() (int64, error) {
	var totalSize int64
	entries, err := os.ReadDir(c.path)
	if err != nil {
		return 0, fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			totalSize += info.Size()
		}
	}

	return totalSize, nil
}

// enforceMaxSize removes oldest entries if cache exceeds max size
func (c *Cache) enforceMaxSize() error {
	currentSize, err := c.sizeNoLock()
	if err != nil {
		return err
	}

	if currentSize <= c.maxSize {
		return nil
	}

	// Get all cache files sorted by modification time
	type fileInfo struct {
		path    string
		modTime time.Time
		size    int64
	}

	var files []fileInfo
	entries, err := os.ReadDir(c.path)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			files = append(files, fileInfo{
				path:    filepath.Join(c.path, entry.Name()),
				modTime: info.ModTime(),
				size:    info.Size(),
			})
		}
	}

	// Sort by modification time (oldest first)
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].modTime.After(files[j].modTime) {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	// Remove oldest files until we're under the limit
	for _, file := range files {
		if currentSize <= c.maxSize {
			break
		}

		if err := os.Remove(file.path); err != nil {
			c.logger.WithError(err).WithField("file", file.path).Warn("Failed to remove old cache file")
			continue
		}

		currentSize -= file.size
		c.logger.WithFields(logrus.Fields{
			"file": filepath.Base(file.path),
			"size": file.size,
		}).Debug("Removed old cache entry")
	}

	return nil
}

// getFilePath returns the full file path for a cache key
func (c *Cache) getFilePath(key string) string {
	// Use a simple hash to avoid special characters in filenames
	// In production, consider using a proper hash function
	safeKey := filepath.Base(key)
	return filepath.Join(c.path, fmt.Sprintf("%s.json", safeKey))
}

// CleanExpired removes all expired cache entries
func (c *Cache) CleanExpired() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entries, err := os.ReadDir(c.path)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(c.path, entry.Name())
		jsonData, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var cacheEntry CacheEntry
		if err := json.Unmarshal(jsonData, &cacheEntry); err != nil {
			continue
		}

		// Remove if expired
		if time.Since(cacheEntry.Timestamp) > cacheEntry.TTL {
			if err := os.Remove(filePath); err == nil {
				removed++
			}
		}
	}

	if removed > 0 {
		c.logger.WithField("count", removed).Info("Removed expired cache entries")
	}

	return nil
}
