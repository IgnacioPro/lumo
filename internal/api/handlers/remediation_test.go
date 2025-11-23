package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ignacio/lumo/internal/api/middleware"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockJobRepository is a mock implementation of JobRepository for testing
type MockRemediationJobRepository struct {
	mock.Mock
}

func (m *MockRemediationJobRepository) Create(ctx context.Context, job *models.Job) error {
	args := m.Called(ctx, job)
	// Set a mock ID for the job
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	job.CreatedAt = time.Now()
	return args.Error(0)
}

// MockApprovalRepository is a mock implementation of ApprovalRepository for testing
type MockApprovalRepository struct {
	mock.Mock
}

func (m *MockApprovalRepository) Create(ctx context.Context, approval *models.Approval) error {
	args := m.Called(ctx, approval)
	// Set a mock ID for the approval
	if approval.ID == uuid.Nil {
		approval.ID = uuid.New()
	}
	return args.Error(0)
}

func (m *MockApprovalRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Approval, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Approval), args.Error(1)
}

func (m *MockRemediationJobRepository) Get(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Job), args.Error(1)
}

func (m *MockRemediationJobRepository) List(ctx context.Context, options repository.ListOptions) ([]*models.Job, int, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*models.Job), args.Int(1), args.Error(2)
}

func (m *MockRemediationJobRepository) Update(ctx context.Context, job *models.Job) error {
	args := m.Called(ctx, job)
	return args.Error(0)
}

func (m *MockRemediationJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.JobStatus) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockRemediationJobRepository) UpdateResult(ctx context.Context, id uuid.UUID, result []byte) error {
	args := m.Called(ctx, id, result)
	return args.Error(0)
}

func (m *MockRemediationJobRepository) UpdateError(ctx context.Context, id uuid.UUID, errorMsg string) error {
	args := m.Called(ctx, id, errorMsg)
	return args.Error(0)
}

func (m *MockRemediationJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestRemediationHandler_Run_ValidRequest(t *testing.T) {
	// Setup
	mockRepo := new(MockRemediationJobRepository)
	mockApprovalRepo := new(MockApprovalRepository)
	cfg := &config.Config{
		SSH: config.SSHConfig{
			Port:    22,
			Timeout: 30,
		},
	}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs during testing

	handler := NewRemediationHandler(mockRepo, mockApprovalRepo, cfg, logger)

	// Create test request
	reqBody := RemediationRequest{
		Target:      "localhost",
		AutoApprove: true,
		DryRun:      true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/remediation", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// Mock API key in context
	apiKey := &models.APIKey{
		ID:   uuid.New(),
		Name: "test-key",
	}
	ctx := middleware.SetAPIKeyInContext(req.Context(), apiKey)
	req = req.WithContext(ctx)

	// Expect job creation
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Job")).Return(nil)
	// Expect async status updates (might happen during test execution)
	mockRepo.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("UpdateResult", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("UpdateError", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	// Execute
	handler.Run(rec, req)

	// Assert
	assert.Equal(t, http.StatusCreated, rec.Code)

	var wrappedResp struct {
		Success bool                `json:"success"`
		Data    RemediationResponse `json:"data"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &wrappedResp)
	assert.NoError(t, err)
	assert.True(t, wrappedResp.Success)
	resp := wrappedResp.Data

	assert.NotEmpty(t, resp.JobID)
	assert.Equal(t, "localhost", resp.Target)
	assert.Equal(t, "pending", resp.Status)

	mockRepo.AssertExpectations(t)

	// Give some time for the background goroutine to start
	time.Sleep(100 * time.Millisecond)
}

func TestRemediationHandler_Run_MissingTarget(t *testing.T) {
	// Setup
	mockRepo := new(MockRemediationJobRepository)
	mockApprovalRepo := new(MockApprovalRepository)
	cfg := &config.Config{}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewRemediationHandler(mockRepo, mockApprovalRepo, cfg, logger)

	// Create test request without target
	reqBody := RemediationRequest{
		DryRun: true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/remediation", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// Mock API key in context
	apiKey := &models.APIKey{
		ID:   uuid.New(),
		Name: "test-key",
	}
	ctx := middleware.SetAPIKeyInContext(req.Context(), apiKey)
	req = req.WithContext(ctx)

	// Execute
	handler.Run(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRemediationHandler_Run_InvalidJSON(t *testing.T) {
	// Setup
	mockRepo := new(MockRemediationJobRepository)
	mockApprovalRepo := new(MockApprovalRepository)
	cfg := &config.Config{}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewRemediationHandler(mockRepo, mockApprovalRepo, cfg, logger)

	// Create invalid JSON request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/remediation", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()

	// Mock API key in context
	apiKey := &models.APIKey{
		ID:   uuid.New(),
		Name: "test-key",
	}
	ctx := middleware.SetAPIKeyInContext(req.Context(), apiKey)
	req = req.WithContext(ctx)

	// Execute
	handler.Run(rec, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRemediationHandler_Run_NoAuthentication(t *testing.T) {
	// Setup
	mockRepo := new(MockRemediationJobRepository)
	mockApprovalRepo := new(MockApprovalRepository)
	cfg := &config.Config{}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewRemediationHandler(mockRepo, mockApprovalRepo, cfg, logger)

	// Create test request
	reqBody := RemediationRequest{
		Target: "localhost",
		DryRun: true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/remediation", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// No API key in context

	// Execute
	handler.Run(rec, req)

	// Assert
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRemediationHandler_Run_WithSkipCategories(t *testing.T) {
	// Setup
	mockRepo := new(MockRemediationJobRepository)
	mockApprovalRepo := new(MockApprovalRepository)
	cfg := &config.Config{
		SSH: config.SSHConfig{
			Port:    22,
			Timeout: 30,
		},
	}
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	handler := NewRemediationHandler(mockRepo, mockApprovalRepo, cfg, logger)

	// Create test request with skip categories
	reqBody := RemediationRequest{
		Target:         "localhost",
		AutoApprove:    false,
		SkipCategories: []string{"service", "network"},
		DryRun:         true,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/remediation", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	// Mock API key in context
	apiKey := &models.APIKey{
		ID:   uuid.New(),
		Name: "test-key",
	}
	ctx := middleware.SetAPIKeyInContext(req.Context(), apiKey)
	req = req.WithContext(ctx)

	// Expect job creation
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Job")).Return(nil)
	// Expect async status updates (might happen during test execution)
	mockRepo.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("UpdateResult", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.On("UpdateError", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	// Execute
	handler.Run(rec, req)

	// Assert
	assert.Equal(t, http.StatusCreated, rec.Code)

	var wrappedResp struct {
		Success bool                `json:"success"`
		Data    RemediationResponse `json:"data"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &wrappedResp)
	assert.NoError(t, err)
	assert.True(t, wrappedResp.Success)
	resp := wrappedResp.Data

	assert.NotEmpty(t, resp.JobID)
	mockRepo.AssertExpectations(t)

	// Give some time for the background goroutine to start
	time.Sleep(100 * time.Millisecond)
}

func TestIsLocalhost(t *testing.T) {
	tests := []struct {
		hostname string
		expected bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"0.0.0.0", true},
		{"LOCALHOST", true}, // Case insensitive
		{"example.com", false},
		{"192.168.1.1", false},
		{"server.local", false},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			result := isLocalhost(tt.hostname)
			assert.Equal(t, tt.expected, result)
		})
	}
}
