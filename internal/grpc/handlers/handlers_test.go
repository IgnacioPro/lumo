package handlers

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database/models"
	"github.com/ignacio/lumo/internal/database/repository"
)

// Mock repository interfaces
type mockJobRepository struct {
	mu   sync.RWMutex
	jobs map[uuid.UUID]*models.Job
}

func newMockJobRepository() *mockJobRepository {
	return &mockJobRepository{
		jobs: make(map[uuid.UUID]*models.Job),
	}
}

func (m *mockJobRepository) Create(ctx context.Context, job *models.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
	return nil
}

func (m *mockJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.jobs[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return job, nil
}

func (m *mockJobRepository) List(ctx context.Context, opts repository.ListOptions) ([]*models.Job, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	jobs := make([]*models.Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	return jobs, len(jobs), nil
}

func (m *mockJobRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.JobStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return sql.ErrNoRows
	}
	job.Status = status
	return nil
}

func (m *mockJobRepository) UpdateResult(ctx context.Context, id uuid.UUID, result []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return sql.ErrNoRows
	}
	job.Result = result
	return nil
}

func (m *mockJobRepository) UpdateError(ctx context.Context, id uuid.UUID, errorMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[id]
	if !ok {
		return sql.ErrNoRows
	}
	job.Error = &errorMsg
	return nil
}

func (m *mockJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, id)
	return nil
}

type mockAgentRepository struct {
	agents map[uuid.UUID]*models.Agent
}

func newMockAgentRepository() *mockAgentRepository {
	return &mockAgentRepository{
		agents: make(map[uuid.UUID]*models.Agent),
	}
}

func (m *mockAgentRepository) Create(ctx context.Context, agent *models.Agent) error {
	m.agents[agent.ID] = agent
	return nil
}

func (m *mockAgentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	agent, ok := m.agents[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return agent, nil
}

func (m *mockAgentRepository) GetByHostname(ctx context.Context, hostname string) (*models.Agent, error) {
	for _, agent := range m.agents {
		if agent.Hostname == hostname {
			return agent, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *mockAgentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*models.Agent, error) {
	agents := make([]*models.Agent, 0, len(m.agents))
	for _, agent := range m.agents {
		agents = append(agents, agent)
	}
	return agents, nil
}

func (m *mockAgentRepository) Update(ctx context.Context, agent *models.Agent) error {
	if _, ok := m.agents[agent.ID]; !ok {
		return sql.ErrNoRows
	}
	m.agents[agent.ID] = agent
	return nil
}

func (m *mockAgentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.AgentStatus) error {
	agent, ok := m.agents[id]
	if !ok {
		return sql.ErrNoRows
	}
	agent.Status = status
	return nil
}

func (m *mockAgentRepository) MarkStaleAgentsOffline(ctx context.Context, threshold time.Duration) (int64, error) {
	count := int64(0)
	for _, agent := range m.agents {
		if agent.Status == models.AgentStatusOnline && agent.TimeSinceHeartbeat() > threshold {
			agent.Status = models.AgentStatusOffline
			count++
		}
	}
	return count, nil
}

func (m *mockAgentRepository) UpdateHeartbeat(ctx context.Context, id uuid.UUID) error {
	agent, ok := m.agents[id]
	if !ok {
		return sql.ErrNoRows
	}
	now := time.Now()
	agent.LastHeartbeatAt = now
	return nil
}

func (m *mockAgentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	delete(m.agents, id)
	return nil
}

func (m *mockAgentRepository) CountByStatus(ctx context.Context) (map[models.AgentStatus]int, error) {
	counts := make(map[models.AgentStatus]int)
	for _, agent := range m.agents {
		counts[agent.Status]++
	}
	return counts, nil
}

// TestHealthHandler tests the health service handler
func TestHealthHandler_Check(t *testing.T) {
	cfg := &config.Config{}
	handler := NewHealthHandler(cfg, nil)

	ctx := context.Background()
	req := &lumov1.HealthCheckRequest{}

	resp, err := handler.Check(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, lumov1.HealthCheckResponse_SERVING_STATUS_SERVING, resp.Status)
	assert.NotEmpty(t, resp.Version)
	assert.NotNil(t, resp.Components)
}

func TestHealthHandler_Ready(t *testing.T) {
	cfg := &config.Config{}
	handler := NewHealthHandler(cfg, nil)

	ctx := context.Background()
	req := &lumov1.ReadyRequest{}

	resp, err := handler.Ready(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Ready)
}

func TestHealthHandler_Live(t *testing.T) {
	cfg := &config.Config{}
	handler := NewHealthHandler(cfg, nil)

	ctx := context.Background()
	req := &lumov1.LiveRequest{}

	resp, err := handler.Live(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Alive)
	assert.NotNil(t, resp.Timestamp)
}

// TestAgentsHandler tests the agents service handler
func TestAgentsHandler_RegisterAgent(t *testing.T) {
	repo := newMockAgentRepository()
	handler := NewAgentsHandler(repo)

	tests := []struct {
		name    string
		req     *lumov1.RegisterAgentRequest
		wantErr bool
		errCode codes.Code
	}{
		{
			name: "valid registration",
			req: &lumov1.RegisterAgentRequest{
				Name:         "test-agent",
				Hostname:     "test-host",
				Platform:     "linux",
				Architecture: "amd64",
				Version:      "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			req: &lumov1.RegisterAgentRequest{
				Hostname: "test-host",
			},
			wantErr: true,
			errCode: codes.InvalidArgument,
		},
		{
			name: "missing hostname",
			req: &lumov1.RegisterAgentRequest{
				Name: "test-agent",
			},
			wantErr: true,
			errCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			resp, err := handler.RegisterAgent(ctx, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.errCode, st.Code())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.AgentId)
				assert.NotNil(t, resp.RegisteredAt)

				// Verify agent was stored
				agentID, err := uuid.Parse(resp.AgentId)
				require.NoError(t, err)
				agent, err := repo.GetByID(ctx, agentID)
				require.NoError(t, err)
				assert.Equal(t, tt.req.Name, agent.Name)
				assert.Equal(t, tt.req.Hostname, agent.Hostname)
			}
		})
	}
}

func TestAgentsHandler_SendHeartbeat(t *testing.T) {
	repo := newMockAgentRepository()
	handler := NewAgentsHandler(repo)

	// Create a test agent
	ctx := context.Background()
	agent := &models.Agent{
		ID:              uuid.New(),
		Name:            "test-agent",
		Hostname:        "test-host",
		Platform:        "linux",
		Status:          "online",
		LastHeartbeatAt: time.Now(),
		RegisteredAt:    time.Now(),
	}
	err := repo.Create(ctx, agent)
	require.NoError(t, err)

	tests := []struct {
		name    string
		agentID string
		wantErr bool
		errCode codes.Code
	}{
		{
			name:    "valid heartbeat",
			agentID: agent.ID.String(),
			wantErr: false,
		},
		{
			name:    "invalid agent ID format",
			agentID: "invalid-uuid",
			wantErr: true,
			errCode: codes.InvalidArgument,
		},
		{
			name:    "non-existent agent",
			agentID: uuid.New().String(),
			wantErr: true,
			errCode: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &lumov1.SendHeartbeatRequest{
				AgentId: tt.agentID,
			}

			resp, err := handler.SendHeartbeat(ctx, req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.errCode, st.Code())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.True(t, resp.Acknowledged)
				assert.NotNil(t, resp.ServerTime)
			}
		})
	}
}

func TestAgentsHandler_ListAgents(t *testing.T) {
	repo := newMockAgentRepository()
	handler := NewAgentsHandler(repo)

	ctx := context.Background()

	// Create test agents
	for i := 0; i < 5; i++ {
		agent := &models.Agent{
			ID:              uuid.New(),
			Name:            "test-agent",
			Hostname:        "test-host",
			Platform:        "linux",
			Status:          "online",
			RegisteredAt:    time.Now(),
			LastHeartbeatAt: time.Now(),
		}
		err := repo.Create(ctx, agent)
		require.NoError(t, err)
	}

	req := &lumov1.ListAgentsRequest{
		Limit:  10,
		Offset: 0,
	}

	resp, err := handler.ListAgents(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 5, len(resp.Agents))
	assert.Equal(t, int32(5), resp.TotalCount)
}

func TestAgentsHandler_GetAgent(t *testing.T) {
	repo := newMockAgentRepository()
	handler := NewAgentsHandler(repo)

	ctx := context.Background()

	// Create a test agent
	agent := &models.Agent{
		ID:              uuid.New(),
		Name:            "test-agent",
		Hostname:        "test-host",
		Platform:        "linux",
		Status:          "online",
		RegisteredAt:    time.Now(),
		LastHeartbeatAt: time.Now(),
	}
	err := repo.Create(ctx, agent)
	require.NoError(t, err)

	tests := []struct {
		name    string
		agentID string
		wantErr bool
	}{
		{
			name:    "existing agent",
			agentID: agent.ID.String(),
			wantErr: false,
		},
		{
			name:    "invalid ID format",
			agentID: "invalid-uuid",
			wantErr: true,
		},
		{
			name:    "non-existent agent",
			agentID: uuid.New().String(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &lumov1.GetAgentRequest{
				AgentId: tt.agentID,
			}

			resp, err := handler.GetAgent(ctx, req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.Agent)
				assert.Equal(t, agent.Name, resp.Agent.Name)
			}
		})
	}
}

func TestAgentsHandler_DeleteAgent(t *testing.T) {
	repo := newMockAgentRepository()
	handler := NewAgentsHandler(repo)

	ctx := context.Background()

	// Create a test agent
	agent := &models.Agent{
		ID:              uuid.New(),
		Name:            "test-agent",
		Hostname:        "test-host",
		Platform:        "linux",
		Status:          "online",
		RegisteredAt:    time.Now(),
		LastHeartbeatAt: time.Now(),
	}
	err := repo.Create(ctx, agent)
	require.NoError(t, err)

	req := &lumov1.DeleteAgentRequest{
		AgentId: agent.ID.String(),
	}

	resp, err := handler.DeleteAgent(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, resp.Success)

	// Verify agent was deleted
	_, err = repo.GetByID(ctx, agent.ID)
	assert.Error(t, err)
}

func TestAgentsHandler_GetAgentStats(t *testing.T) {
	repo := newMockAgentRepository()
	handler := NewAgentsHandler(repo)

	ctx := context.Background()

	// Create test agents
	for i := 0; i < 3; i++ {
		agent := &models.Agent{
			ID:              uuid.New(),
			Name:            "test-agent",
			Hostname:        "test-host",
			Platform:        "linux",
			Status:          "online",
			RegisteredAt:    time.Now(),
			LastHeartbeatAt: time.Now(),
		}
		err := repo.Create(ctx, agent)
		require.NoError(t, err)
	}

	req := &lumov1.GetAgentStatsRequest{}

	resp, err := handler.GetAgentStats(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int32(3), resp.TotalAgents)
	assert.Equal(t, int32(3), resp.OnlineAgents)
}

// TestDiagnosticsHandler tests the diagnostics service handler
func TestDiagnosticsHandler_RunDiagnostics(t *testing.T) {
	repo := newMockJobRepository()
	cfg := &config.Config{}
	logger := logrus.New()
	handler := NewDiagnosticsHandler(repo, cfg, logger)

	tests := []struct {
		name    string
		req     *lumov1.RunDiagnosticsRequest
		wantErr bool
		errCode codes.Code
	}{
		{
			name: "valid request",
			req: &lumov1.RunDiagnosticsRequest{
				Target: "localhost",
				Checks: []string{"cpu", "memory"},
			},
			wantErr: false,
		},
		{
			name: "missing target",
			req: &lumov1.RunDiagnosticsRequest{
				Checks: []string{"cpu"},
			},
			wantErr: true,
			errCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			resp, err := handler.RunDiagnostics(ctx, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
				st, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.errCode, st.Code())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.JobId)
				assert.Equal(t, lumov1.JobStatus_JOB_STATUS_PENDING, resp.Status)
				assert.NotNil(t, resp.CreatedAt)

				// Verify job was created
				jobID, err := uuid.Parse(resp.JobId)
				require.NoError(t, err)
				job, err := repo.GetByID(ctx, jobID)
				require.NoError(t, err)
				assert.Equal(t, tt.req.Target, job.Target)
			}
		})
	}
}

func TestDiagnosticsHandler_GetDiagnosticsResult(t *testing.T) {
	repo := newMockJobRepository()
	cfg := &config.Config{}
	logger := logrus.New()
	handler := NewDiagnosticsHandler(repo, cfg, logger)

	ctx := context.Background()

	// Create a test job
	job := &models.Job{
		ID:        uuid.New(),
		Type:      models.JobTypeDiagnostic,
		Status:    models.JobStatusCompleted,
		Target:    "localhost",
		CreatedAt: time.Now(),
	}
	err := repo.Create(ctx, job)
	require.NoError(t, err)

	tests := []struct {
		name    string
		jobID   string
		wantErr bool
	}{
		{
			name:    "existing job",
			jobID:   job.ID.String(),
			wantErr: false,
		},
		{
			name:    "invalid ID format",
			jobID:   "invalid-uuid",
			wantErr: true,
		},
		{
			name:    "non-existent job",
			jobID:   uuid.New().String(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &lumov1.GetDiagnosticsResultRequest{
				JobId: tt.jobID,
			}

			resp, err := handler.GetDiagnosticsResult(ctx, req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, job.ID.String(), resp.JobId)
				assert.Equal(t, lumov1.JobStatus_JOB_STATUS_COMPLETED, resp.Status)
			}
		})
	}
}

func TestDiagnosticsHandler_ListDiagnostics(t *testing.T) {
	repo := newMockJobRepository()
	cfg := &config.Config{}
	logger := logrus.New()
	handler := NewDiagnosticsHandler(repo, cfg, logger)

	ctx := context.Background()

	// Create test jobs
	for i := 0; i < 5; i++ {
		job := &models.Job{
			ID:        uuid.New(),
			Type:      models.JobTypeDiagnostic,
			Status:    models.JobStatusCompleted,
			Target:    "localhost",
			CreatedAt: time.Now(),
		}
		err := repo.Create(ctx, job)
		require.NoError(t, err)
	}

	req := &lumov1.ListDiagnosticsRequest{
		Limit:  10,
		Offset: 0,
	}

	resp, err := handler.ListDiagnostics(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 5, len(resp.Jobs))
	assert.Equal(t, int32(5), resp.TotalCount)
}

// Test helper functions
func TestToProtoJobStatus(t *testing.T) {
	tests := []struct {
		input    models.JobStatus
		expected lumov1.JobStatus
	}{
		{models.JobStatusPending, lumov1.JobStatus_JOB_STATUS_PENDING},
		{models.JobStatusRunning, lumov1.JobStatus_JOB_STATUS_RUNNING},
		{models.JobStatusCompleted, lumov1.JobStatus_JOB_STATUS_COMPLETED},
		{models.JobStatusFailed, lumov1.JobStatus_JOB_STATUS_FAILED},
		{models.JobStatusCancelled, lumov1.JobStatus_JOB_STATUS_CANCELLED},
		{"unknown", lumov1.JobStatus_JOB_STATUS_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := toProtoJobStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToProtoJobType(t *testing.T) {
	tests := []struct {
		input    models.JobType
		expected lumov1.JobType
	}{
		{models.JobTypeDiagnostic, lumov1.JobType_JOB_TYPE_DIAGNOSTIC},
		{models.JobTypeRemediation, lumov1.JobType_JOB_TYPE_REMEDIATION},
		{"unknown", lumov1.JobType_JOB_TYPE_UNSPECIFIED},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := toProtoJobType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToProtoAgent(t *testing.T) {
	ipAddr := "192.168.1.100"
	labels, _ := models.MapToJSONB(map[string]string{"env": "test"})

	agent := &models.Agent{
		ID:              uuid.New(),
		Name:            "test-agent",
		Hostname:        "test-host",
		IPAddress:       &ipAddr,
		Platform:        models.AgentPlatformLinux,
		Architecture:    "amd64",
		Version:         "1.0.0",
		Status:          models.AgentStatusOnline,
		Capabilities:    []string{"cpu", "memory"},
		Labels:          labels,
		LastHeartbeatAt: time.Now(),
		RegisteredAt:    time.Now(),
	}

	proto := toProtoAgent(agent)

	assert.Equal(t, agent.ID.String(), proto.Id)
	assert.Equal(t, agent.Name, proto.Name)
	assert.Equal(t, agent.Hostname, proto.Hostname)
	assert.Equal(t, *agent.IPAddress, proto.IpAddress)
	assert.Equal(t, string(agent.Platform), proto.Platform)
	assert.Equal(t, agent.Architecture, proto.Architecture)
	assert.Equal(t, agent.Version, proto.Version)
	assert.Equal(t, string(agent.Status), proto.Status)
	assert.Equal(t, agent.Capabilities, proto.Capabilities)
	assert.NotNil(t, proto.LastHeartbeatAt)
	assert.NotNil(t, proto.RegisteredAt)

	// Verify timestamps are close
	assert.WithinDuration(t, agent.LastHeartbeatAt, proto.LastHeartbeatAt.AsTime(), time.Second)
	assert.WithinDuration(t, agent.RegisteredAt, proto.RegisteredAt.AsTime(), time.Second)
}
