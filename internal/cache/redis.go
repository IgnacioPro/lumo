package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// RedisConfig contains Redis connection configuration
type RedisConfig struct {
	Enabled    bool
	RedisURL   string
	Password   string
	MaxRetries int
	PoolSize   int
	TTL        time.Duration
}

// RedisClient wraps the Redis client with additional functionality
type RedisClient struct {
	client *redis.Client
	config *RedisConfig
	logger *logrus.Logger
}

// NewRedisClient creates a new Redis client connection
func NewRedisClient(config *RedisConfig, logger *logrus.Logger) (*RedisClient, error) {
	if config == nil {
		return nil, fmt.Errorf("redis config cannot be nil")
	}

	if !config.Enabled {
		return nil, fmt.Errorf("redis is disabled in configuration")
	}

	if logger == nil {
		logger = logrus.New()
	}

	// Parse Redis URL and create options
	opts, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	// Override with config values
	if config.Password != "" {
		opts.Password = config.Password
	}
	opts.MaxRetries = config.MaxRetries
	opts.PoolSize = config.PoolSize

	// Create Redis client
	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	rc := &RedisClient{
		client: client,
		config: config,
		logger: logger,
	}

	logger.WithFields(logrus.Fields{
		"url":         config.RedisURL,
		"pool_size":   config.PoolSize,
		"max_retries": config.MaxRetries,
	}).Info("Redis connection established")

	return rc, nil
}

// Close closes the Redis client connection
func (rc *RedisClient) Close() error {
	rc.logger.Info("Closing Redis connection")
	if err := rc.client.Close(); err != nil {
		return fmt.Errorf("failed to close redis: %w", err)
	}
	return nil
}

// Health checks the Redis connection health
func (rc *RedisClient) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rc.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}
	return nil
}

// Set sets a key-value pair with the configured TTL
func (rc *RedisClient) Set(ctx context.Context, key string, value interface{}) error {
	return rc.client.Set(ctx, key, value, rc.config.TTL).Err()
}

// SetWithTTL sets a key-value pair with a custom TTL
func (rc *RedisClient) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return rc.client.Set(ctx, key, value, ttl).Err()
}

// Get retrieves a value by key
func (rc *RedisClient) Get(ctx context.Context, key string) (string, error) {
	val, err := rc.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("key not found: %s", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to get key %s: %w", key, err)
	}
	return val, nil
}

// Delete deletes a key
func (rc *RedisClient) Delete(ctx context.Context, keys ...string) error {
	return rc.client.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func (rc *RedisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return rc.client.Exists(ctx, keys...).Result()
}

// GetClient returns the underlying Redis client for advanced operations
func (rc *RedisClient) GetClient() *redis.Client {
	return rc.client
}
