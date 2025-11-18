package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/lib/pq"
)

// JobRepository handles database operations for jobs
type JobRepository struct {
	db *sql.DB
}

// NewJobRepository creates a new job repository
func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

// Create creates a new job in the database
func (r *JobRepository) Create(ctx context.Context, job *models.Job) error {
	query := `
		INSERT INTO jobs (id, type, status, target, created_at, created_by, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	// Generate UUID if not provided
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}

	// Set default status if not provided
	if job.Status == "" {
		job.Status = models.JobStatusPending
	}

	// Set created_at if not provided
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}

	err := r.db.QueryRowContext(
		ctx,
		query,
		job.ID,
		job.Type,
		job.Status,
		job.Target,
		job.CreatedAt,
		job.CreatedBy,
		job.Metadata,
	).Scan(&job.ID, &job.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}

// GetByID retrieves a job by its ID
func (r *JobRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	query := `
		SELECT id, type, status, target, created_at, started_at, completed_at,
		       created_by, result, error, metadata
		FROM jobs
		WHERE id = $1
	`

	job := &models.Job{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID,
		&job.Type,
		&job.Status,
		&job.Target,
		&job.CreatedAt,
		&job.StartedAt,
		&job.CompletedAt,
		&job.CreatedBy,
		&job.Result,
		&job.Error,
		&job.Metadata,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return job, nil
}

// ListOptions contains options for listing jobs
type ListOptions struct {
	Limit     int
	Offset    int
	Status    *models.JobStatus
	Type      *models.JobType
	Target    string
	CreatedBy string
	SortBy    string
	SortOrder string
}

// List retrieves jobs with pagination and filtering
func (r *JobRepository) List(ctx context.Context, opts ListOptions) ([]*models.Job, int, error) {
	// Build WHERE clause
	where := "WHERE 1=1"
	args := []interface{}{}
	argPos := 1

	if opts.Status != nil {
		where += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, *opts.Status)
		argPos++
	}

	if opts.Type != nil {
		where += fmt.Sprintf(" AND type = $%d", argPos)
		args = append(args, *opts.Type)
		argPos++
	}

	if opts.Target != "" {
		where += fmt.Sprintf(" AND target = $%d", argPos)
		args = append(args, opts.Target)
		argPos++
	}

	if opts.CreatedBy != "" {
		where += fmt.Sprintf(" AND created_by = $%d", argPos)
		args = append(args, opts.CreatedBy)
		argPos++
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM jobs " + where
	var total int
	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	// Build ORDER BY clause
	sortBy := "created_at"
	if opts.SortBy != "" {
		// Whitelist allowed sort columns
		allowedSorts := map[string]bool{
			"created_at":   true,
			"started_at":   true,
			"completed_at": true,
			"status":       true,
			"type":         true,
		}
		if allowedSorts[opts.SortBy] {
			sortBy = opts.SortBy
		}
	}

	sortOrder := "DESC"
	if opts.SortOrder == "ASC" || opts.SortOrder == "asc" {
		sortOrder = "ASC"
	}

	// Set defaults
	limit := 50
	if opts.Limit > 0 && opts.Limit <= 100 {
		limit = opts.Limit
	}

	offset := 0
	if opts.Offset > 0 {
		offset = opts.Offset
	}

	// Build final query
	query := fmt.Sprintf(`
		SELECT id, type, status, target, created_at, started_at, completed_at,
		       created_by, result, error, metadata
		FROM jobs
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, where, sortBy, sortOrder, argPos, argPos+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	jobs := make([]*models.Job, 0, limit)
	for rows.Next() {
		job := &models.Job{}
		err := rows.Scan(
			&job.ID,
			&job.Type,
			&job.Status,
			&job.Target,
			&job.CreatedAt,
			&job.StartedAt,
			&job.CompletedAt,
			&job.CreatedBy,
			&job.Result,
			&job.Error,
			&job.Metadata,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, job)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating jobs: %w", err)
	}

	return jobs, total, nil
}

// UpdateStatus updates the status of a job
func (r *JobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.JobStatus) error {
	now := time.Now()
	query := `
		UPDATE jobs
		SET status = $1,
		    started_at = CASE WHEN $2 = 'running' AND started_at IS NULL THEN $3 ELSE started_at END,
		    completed_at = CASE WHEN $2 IN ('completed', 'failed', 'cancelled') THEN $3 ELSE completed_at END
		WHERE id = $4
	`

	result, err := r.db.ExecContext(ctx, query, status, status, now, id)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}

// UpdateResult updates the result of a completed job
func (r *JobRepository) UpdateResult(ctx context.Context, id uuid.UUID, result []byte) error {
	query := `
		UPDATE jobs
		SET result = $1,
		    completed_at = COALESCE(completed_at, $2)
		WHERE id = $3
	`

	_, err := r.db.ExecContext(ctx, query, result, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update job result: %w", err)
	}

	return nil
}

// UpdateError updates the error message of a failed job
func (r *JobRepository) UpdateError(ctx context.Context, id uuid.UUID, errorMsg string) error {
	query := `
		UPDATE jobs
		SET error = $1,
		    status = 'failed',
		    completed_at = COALESCE(completed_at, $2)
		WHERE id = $3
	`

	_, err := r.db.ExecContext(ctx, query, errorMsg, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to update job error: %w", err)
	}

	return nil
}

// Delete deletes a job by ID
func (r *JobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM jobs WHERE id = $1"

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		// Check for foreign key constraint violations
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23503" { // foreign_key_violation
				return fmt.Errorf("cannot delete job: referenced by other records")
			}
		}
		return fmt.Errorf("failed to delete job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job not found: %s", id)
	}

	return nil
}
