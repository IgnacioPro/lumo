// Package testutil provides shared testing utilities and constants.
package testutil

// Test constants - use these instead of hardcoding in tests
const (
	// TestJWTSecret is a test secret for JWT generation in tests.
	// DO NOT use in production.
	TestJWTSecret = "test-secret-for-testing-only-32chars!"

	// TestAPIKey is a test API key for authentication tests.
	TestAPIKey = "test-api-key-for-testing"

	// TestDatabasePassword is a test database password.
	TestDatabasePassword = "test-db-password"
)
