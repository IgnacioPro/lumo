package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/stretchr/testify/assert"
)

func TestJobRepository_Create(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewJobRepository(db)
	ctx := context.Background()

	job := &models.Job{
		Type:      models.JobTypeDiagnostic,
		Status:    models.JobStatusPending,
		Target:    "localhost",
		CreatedBy: "user1",
	}

	// Expect transaction
	mock.ExpectQuery("INSERT INTO jobs").
		WithArgs(sqlmock.AnyArg(), job.Type, job.Status, job.Target, sqlmock.AnyArg(), job.CreatedBy, job.Metadata).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
			AddRow(uuid.New(), time.Now()))

	err = repo.Create(ctx, job)
	assert.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, job.ID)
	assert.False(t, job.CreatedAt.IsZero())
}

func TestJobRepository_GetByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	repo := NewJobRepository(db)
	ctx := context.Background()
	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery("SELECT id, type").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"id", "type", "status", "target", "created_at", "started_at", "completed_at", "created_by", "result", "error", "metadata"}).
			AddRow(id, models.JobTypeDiagnostic, models.JobStatusPending, "localhost", now, nil, nil, "user1", []byte("{}"), nil, nil))

	job, err := repo.GetByID(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, id, job.ID)
	assert.Equal(t, models.JobTypeDiagnostic, job.Type)
}
