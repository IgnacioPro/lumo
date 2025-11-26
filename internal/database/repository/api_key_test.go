package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ignacio/lumo/internal/database/models"
)

func TestAPIKeyRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewAPIKeyRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		key := &models.APIKey{
			Name: "test-key",
		}

		mock.ExpectQuery("INSERT INTO api_keys").
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), key.Name, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(uuid.New(), time.Now()))

		err := repo.Create(ctx, key)
		assert.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, key.ID)
	})

	t.Run("Duplicate", func(t *testing.T) {
		key := &models.APIKey{Name: "duplicate"}

		mock.ExpectQuery("INSERT INTO api_keys").
			WillReturnError(&pq.Error{Code: "23505"})

		err := repo.Create(ctx, key)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestAPIKeyRepository_GetByHash(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewAPIKeyRepository(db)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		hash := "some-hash"
		expectedID := uuid.New()

		rows := sqlmock.NewRows([]string{"id", "key_hash", "name", "scopes", "created_at", "last_used_at", "expires_at", "revoked", "metadata"}).
			AddRow(expectedID, hash, "test", []uint8("{}"), time.Now(), nil, nil, false, []byte("{}"))

		mock.ExpectQuery("SELECT .* FROM api_keys WHERE key_hash =").
			WithArgs(hash).
			WillReturnRows(rows)

		key, err := repo.GetByHash(ctx, hash)
		assert.NoError(t, err)
		assert.Equal(t, expectedID, key.ID)
	})

	t.Run("NotFound", func(t *testing.T) {
		mock.ExpectQuery("SELECT .* FROM api_keys WHERE key_hash =").
			WillReturnError(sqlmock.ErrCancelled) // Simulating not found or other error

		_, err := repo.GetByHash(ctx, "missing")
		assert.Error(t, err)
	})
}
