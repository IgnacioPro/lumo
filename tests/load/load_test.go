package load

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestLoadHealthCheck performs a basic load test on the health check endpoint
func TestLoadHealthCheck(t *testing.T) {
	// Skip if not running in integration mode or if server URL is not set
	// For now, we assume the server is running locally on port 8080 for this test
	// In a real CI environment, we would start the server or have it running

	// This test is designed to be run manually or in a specific CI stage
	// t.Skip("Skipping load test in normal test run")

	baseURL := "http://127.0.0.1:8080/api/v1"
	concurrency := 50
	requestsPerWorker := 20

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	// Check if server is reachable first
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		t.Skipf("Server not reachable at %s, skipping load test: %v", baseURL, err)
		return
	}
	resp.Body.Close()

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(concurrency)

	errorCount := 0
	var errorMu sync.Mutex

	fmt.Printf("Starting load test: %d workers, %d requests each...\n", concurrency, requestsPerWorker)

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
				resp.Body.Close()
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
