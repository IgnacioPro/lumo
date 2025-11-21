package repository

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewJobRepository(t *testing.T) {
	t.Run("ValidDB", func(t *testing.T) {
		// Create a mock DB (not connected)
		db := &sql.DB{}
		repo := NewJobRepository(db)

		assert.NotNil(t, repo)
		assert.NotNil(t, repo.db)
	})

	t.Run("NilDB", func(t *testing.T) {
		repo := NewJobRepository(nil)

		assert.NotNil(t, repo)
		assert.Nil(t, repo.db)
	})
}

func TestNewAgentRepository(t *testing.T) {
	t.Run("ValidDB", func(t *testing.T) {
		db := &sql.DB{}
		repo := NewAgentRepository(db)

		assert.NotNil(t, repo)
		assert.NotNil(t, repo.db)
	})
}

func TestNewAPIKeyRepository(t *testing.T) {
	t.Run("ValidDB", func(t *testing.T) {
		db := &sql.DB{}
		repo := NewAPIKeyRepository(db)

		assert.NotNil(t, repo)
		assert.NotNil(t, repo.db)
	})
}

// Note: Full repository tests require database integration tests
// These should be run with `go test -tags=integration` against a test database
// Current tests only verify constructor behavior
