package load

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/api"
	"github.com/ignacio/lumo/internal/api/auth"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database"
	"github.com/ignacio/lumo/tests/testutil"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestServer creates a real router with a real DB connection (if available)
// and returns a test server.
func setupTestServer(t *testing.T) (*httptest.Server, func()) {
	// Load configuration
	cfg := config.DefaultConfig()

	// Configure for testing
	cfg.Logging.Level = "error"                              // Reduce noise
	cfg.API.JWTSecret = testutil.TestJWTSecret // Required for JWT Manager

	// Disable rate limiting for load testing stability, or set very high
	// The user wants to test rate limiting, but for a "pass/fail" load test
	// that expects 0 errors, we usually want to ensure we don't hit limits.
	// We'll set high limits to test DB performance primarily.
	cfg.API.RateLimitEnabled = false

	// Initialize logger
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Initialize database
	// We use the default config (localhost:5432, user=lumo, pass=lumo_dev)
	// This requires Postgres to be running.
	postgresConfig := &database.PostgresConfig{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		Name:            cfg.Database.Name,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		SSLMode:         cfg.Database.SSLMode,
		MaxConnections:  cfg.Database.MaxConnections,
		MaxIdle:         cfg.Database.MaxIdle,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}

	db, err := database.NewPostgresDB(postgresConfig, logger)
	if err != nil {
		t.Skipf("Skipping load test: failed to connect to database: %v", err)
		return nil, nil
	}

	// Initialize JWT manager
	jwtManager, err := auth.NewJWTManager(cfg.API.JWTSecret, cfg.API.JWTExpiration, cfg.API.JWTIssuer)
	if err != nil {
		t.Fatalf("Failed to create JWT manager: %v", err)
	}

	// Create router
	router := api.NewRouter(db, cfg, jwtManager, logger)

	// Create test server
	ts := httptest.NewServer(router)

	cleanup := func() {
		ts.Close()
		_ = db.Close()
	}

	return ts, cleanup
}

// TestLoadHealthCheck performs a basic load test on the health check endpoint
func TestLoadHealthCheck(t *testing.T) {
	// Setup server
	ts, cleanup := setupTestServer(t)
	if ts == nil {
		return // Skipped
	}
	defer cleanup()

	baseURL := ts.URL + "/api/v1"
	concurrency := 50
	requestsPerWorker := 20

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(concurrency)

	errorCount := 0
	var errorMu sync.Mutex

	fmt.Printf("Starting load test against %s: %d workers, %d requests each...\n", baseURL, concurrency, requestsPerWorker)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				resp, err := client.Get(baseURL + "/health")
				if err != nil {
					errorMu.Lock()
					if errorCount == 0 {
						fmt.Printf("First error: %v\n", err)
					}
					errorCount++
					errorMu.Unlock()
					continue
				}
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					errorMu.Lock()
					if errorCount == 0 {
						fmt.Printf("First error status: %d\n", resp.StatusCode)
					}
					errorCount++
					errorMu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)
	totalRequests := concurrency * requestsPerWorker

	fmt.Printf("Load test complete in %v\n", duration)
	fmt.Printf("Total requests: %d\n", totalRequests)
	fmt.Printf("Error count: %d\n", errorCount)
	fmt.Printf("RPS: %.2f\n", float64(totalRequests)/duration.Seconds())

	assert.Equal(t, 0, errorCount, "Expected 0 errors during load test")
}

// TestRateLimiting verifies that rate limiting actually kicks in under heavy load
// This addresses the user's request to test rate limiting.
func TestRateLimiting(t *testing.T) {
	// Manually setup server with strict rate limits
	cfg := config.DefaultConfig()
	cfg.Logging.Level = "error"
	cfg.API.JWTSecret = "test-secret-for-load-testing-12345"
	cfg.API.RateLimitEnabled = true
	cfg.API.RateLimitRequestsPerMin = 10 // Very low limit
	cfg.API.RateLimitBurstSize = 5

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	postgresConfig := &database.PostgresConfig{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		Name:            cfg.Database.Name,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		SSLMode:         cfg.Database.SSLMode,
		MaxConnections:  cfg.Database.MaxConnections,
		MaxIdle:         cfg.Database.MaxIdle,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}

	db, err := database.NewPostgresDB(postgresConfig, logger)
	if err != nil {
		t.Skipf("Skipping rate limit test: failed to connect to database: %v", err)
		return
	}
	defer func() { _ = db.Close() }()

	jwtManager, err := auth.NewJWTManager(cfg.API.JWTSecret, cfg.API.JWTExpiration, cfg.API.JWTIssuer)
	require.NoError(t, err)

	router := api.NewRouter(db, cfg, jwtManager, logger)
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := ts.Client()

	// Send enough requests to trigger rate limit
	// Burst is 5, limit is 10/min.
	// Sending 20 requests quickly should trigger 429.

	hitRateLimit := false
	for i := 0; i < 20; i++ {
		resp, err := client.Get(ts.URL + "/api/v1/health")
		require.NoError(t, err)
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			hitRateLimit = true
			break
		}
	}

	assert.True(t, hitRateLimit, "Expected to hit rate limit (429) but did not")
}
