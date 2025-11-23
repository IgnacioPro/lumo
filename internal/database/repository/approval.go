package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ignacio/lumo/internal/database/models"
)

// ApprovalRepository handles database operations for approvals
type ApprovalRepository struct {
	db *sql.DB
}

// NewApprovalRepository creates a new approval repository
func NewApprovalRepository(db *sql.DB) *ApprovalRepository {
	return &ApprovalRepository{db: db}
}

// Create creates a new approval request
func (r *ApprovalRepository) Create(ctx context.Context, approval *models.Approval) error {
	// Generate UUID if not provided
	if approval.ID == uuid.Nil {
		approval.ID = uuid.New()
	}

	// Set timestamp
	approval.RequestedAt = time.Now()

	query := `
		INSERT INTO approvals (
			id, job_id, action_id, action_name, action_category, description,
			risk_level, is_reversible, estimated_impact, target, status,
			requested_by, requested_at, expires_at, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, requested_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		approval.ID,
		approval.JobID,
		approval.ActionID,
		approval.ActionName,
		approval.ActionCategory,
		approval.Description,
		approval.RiskLevel,
		approval.IsReversible,
		approval.EstimatedImpact,
		approval.Target,
		approval.Status,
		approval.RequestedBy,
		approval.RequestedAt,
		approval.ExpiresAt,
		approval.Metadata,
	).Scan(&approval.ID, &approval.RequestedAt)

	if err != nil {
		return fmt.Errorf("failed to create approval: %w", err)
	}

	return nil
}

// GetByID retrieves an approval by ID
func (r *ApprovalRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Approval, error) {
	query := `
		SELECT id, job_id, action_id, action_name, action_category, description,
			   risk_level, is_reversible, estimated_impact, target, status,
			   requested_by, requested_at, reviewed_by, reviewed_at, reason,
			   expires_at, metadata
		FROM approvals
		WHERE id = $1
	`

	approval := &models.Approval{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&approval.ID,
		&approval.JobID,
		&approval.ActionID,
		&approval.ActionName,
		&approval.ActionCategory,
		&approval.Description,
		&approval.RiskLevel,
		&approval.IsReversible,
		&approval.EstimatedImpact,
		&approval.Target,
		&approval.Status,
		&approval.RequestedBy,
		&approval.RequestedAt,
		&approval.ReviewedBy,
		&approval.ReviewedAt,
		&approval.Reason,
		&approval.ExpiresAt,
		&approval.Metadata,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("approval not found: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get approval: %w", err)
	}

	return approval, nil
}

// GetByJobID retrieves all approvals for a specific job
func (r *ApprovalRepository) GetByJobID(ctx context.Context, jobID uuid.UUID) ([]*models.Approval, error) {
	query := `
		SELECT id, job_id, action_id, action_name, action_category, description,
			   risk_level, is_reversible, estimated_impact, target, status,
			   requested_by, requested_at, reviewed_by, reviewed_at, reason,
			   expires_at, metadata
		FROM approvals
		WHERE job_id = $1
		ORDER BY requested_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get approvals by job ID: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	approvals := []*models.Approval{}
	for rows.Next() {
		approval := &models.Approval{}
		err := rows.Scan(
			&approval.ID,
			&approval.JobID,
			&approval.ActionID,
			&approval.ActionName,
			&approval.ActionCategory,
			&approval.Description,
			&approval.RiskLevel,
			&approval.IsReversible,
			&approval.EstimatedImpact,
			&approval.Target,
			&approval.Status,
			&approval.RequestedBy,
			&approval.RequestedAt,
			&approval.ReviewedBy,
			&approval.ReviewedAt,
			&approval.Reason,
			&approval.ExpiresAt,
			&approval.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan approval: %w", err)
		}
		approvals = append(approvals, approval)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate approvals: %w", err)
	}

	return approvals, nil
}

// List retrieves approvals with optional filtering
func (r *ApprovalRepository) List(ctx context.Context, filters map[string]interface{}) ([]*models.Approval, error) {
	query := `
		SELECT id, job_id, action_id, action_name, action_category, description,
			   risk_level, is_reversible, estimated_impact, target, status,
			   requested_by, requested_at, reviewed_by, reviewed_at, reason,
			   expires_at, metadata
		FROM approvals
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 1

	// Apply filters
	if status, ok := filters["status"].(models.ApprovalStatus); ok {
		query += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	if riskLevel, ok := filters["risk_level"].(models.ApprovalRiskLevel); ok {
		query += fmt.Sprintf(" AND risk_level = $%d", argCount)
		args = append(args, riskLevel)
		argCount++
	}

	if target, ok := filters["target"].(string); ok && target != "" {
		query += fmt.Sprintf(" AND target = $%d", argCount)
		args = append(args, target)
		argCount++
	}

	if requestedBy, ok := filters["requested_by"].(string); ok && requestedBy != "" {
		query += fmt.Sprintf(" AND requested_by = $%d", argCount)
		args = append(args, requestedBy)
		argCount++
	}

	// Exclude expired approvals if requested
	if excludeExpired, ok := filters["exclude_expired"].(bool); ok && excludeExpired {
		query += " AND (expires_at IS NULL OR expires_at > NOW())"
	}

	// Sorting
	if sort, ok := filters["sort"].(string); ok && sort != "" {
		switch sort {
		case "requested_at", "risk_level", "status":
			query += " ORDER BY " + sort + " DESC"
		default:
			query += " ORDER BY requested_at DESC"
		}
	} else {
		query += " ORDER BY requested_at DESC"
	}

	// Pagination
	if limit, ok := filters["limit"].(int); ok && limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
		argCount++
	}

	if offset, ok := filters["offset"].(int); ok && offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list approvals: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	approvals := []*models.Approval{}
	for rows.Next() {
		approval := &models.Approval{}
		err := rows.Scan(
			&approval.ID,
			&approval.JobID,
			&approval.ActionID,
			&approval.ActionName,
			&approval.ActionCategory,
			&approval.Description,
			&approval.RiskLevel,
			&approval.IsReversible,
			&approval.EstimatedImpact,
			&approval.Target,
			&approval.Status,
			&approval.RequestedBy,
			&approval.RequestedAt,
			&approval.ReviewedBy,
			&approval.ReviewedAt,
			&approval.Reason,
			&approval.ExpiresAt,
			&approval.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan approval: %w", err)
		}
		approvals = append(approvals, approval)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate approvals: %w", err)
	}

	return approvals, nil
}

// Approve marks an approval as approved
func (r *ApprovalRepository) Approve(ctx context.Context, id uuid.UUID, reviewedBy, reason string) error {
	query := `
		UPDATE approvals
		SET status = $1, reviewed_by = $2, reviewed_at = $3, reason = $4
		WHERE id = $5 AND status = $6
	`

	now := time.Now()
	result, err := r.db.ExecContext(
		ctx,
		query,
		models.ApprovalStatusApproved,
		reviewedBy,
		now,
		reason,
		id,
		models.ApprovalStatusPending,
	)

	if err != nil {
		return fmt.Errorf("failed to approve: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("approval not found or already resolved")
	}

	return nil
}

// Reject marks an approval as rejected
func (r *ApprovalRepository) Reject(ctx context.Context, id uuid.UUID, reviewedBy, reason string) error {
	query := `
		UPDATE approvals
		SET status = $1, reviewed_by = $2, reviewed_at = $3, reason = $4
		WHERE id = $5 AND status = $6
	`

	now := time.Now()
	result, err := r.db.ExecContext(
		ctx,
		query,
		models.ApprovalStatusRejected,
		reviewedBy,
		now,
		reason,
		id,
		models.ApprovalStatusPending,
	)

	if err != nil {
		return fmt.Errorf("failed to reject: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("approval not found or already resolved")
	}

	return nil
}

// MarkExpired marks expired pending approvals as expired
func (r *ApprovalRepository) MarkExpired(ctx context.Context) (int64, error) {
	query := `
		UPDATE approvals
		SET status = $1
		WHERE status = $2 AND expires_at IS NOT NULL AND expires_at < NOW()
	`

	result, err := r.db.ExecContext(ctx, query, models.ApprovalStatusExpired, models.ApprovalStatusPending)
	if err != nil {
		return 0, fmt.Errorf("failed to mark expired approvals: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}

// Delete deletes an approval by ID
func (r *ApprovalRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM approvals WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete approval: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("approval not found")
	}

	return nil
}

// CountByStatus returns the count of approvals by status
func (r *ApprovalRepository) CountByStatus(ctx context.Context) (map[models.ApprovalStatus]int, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM approvals
		GROUP BY status
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count approvals by status: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	counts := make(map[models.ApprovalStatus]int)
	for rows.Next() {
		var status models.ApprovalStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan status count: %w", err)
		}
		counts[status] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate status counts: %w", err)
	}

	return counts, nil
}

// CountByRiskLevel returns the count of approvals by risk level
func (r *ApprovalRepository) CountByRiskLevel(ctx context.Context) (map[models.ApprovalRiskLevel]int, error) {
	query := `
		SELECT risk_level, COUNT(*) as count
		FROM approvals
		WHERE status = $1
		GROUP BY risk_level
	`

	rows, err := r.db.QueryContext(ctx, query, models.ApprovalStatusPending)
	if err != nil {
		return nil, fmt.Errorf("failed to count approvals by risk level: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	counts := make(map[models.ApprovalRiskLevel]int)
	for rows.Next() {
		var riskLevel models.ApprovalRiskLevel
		var count int
		if err := rows.Scan(&riskLevel, &count); err != nil {
			return nil, fmt.Errorf("failed to scan risk level count: %w", err)
		}
		counts[riskLevel] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate risk level counts: %w", err)
	}

	return counts, nil
}
