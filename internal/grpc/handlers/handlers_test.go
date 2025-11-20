package handlers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	lumov1 "github.com/ignacio/lumo/api/proto/v1"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/database/models"
)

// Mock repository interfaces
type mockJobRepository struct {
	jobs map[uuid.UUID]*models.Job
}

func newMockJobRepository() *mockJobRepository {
	return &mockJobRepository{
		jobs: make(map[uuid.UUID]*models.Job),
	}
}

func (m *mockJobRepository) Create(ctx context.Context, job *models.Job) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *mockJobRepository) Get(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	job, ok := m.jobs[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return job, nil
}

func (m *mockJobRepository) List(ctx context.Context, limit, offset int) ([]*models.Job, error) {
	jobs := make([]*models.Job, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (m *mockJobRepository) Count(ctx context.Context) (int, error) {
	return len(m.jobs), nil
}

func (m *mockJobRepository) Update(ctx context.Context, job *models.Job) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *mockJobRepository) Delete(ctx context.Context, id uuid.UUID) error {
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

func (m *mockAgentRepository) Get(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	agent, ok := m.agents[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return agent, nil
}

func (m *mockAgentRepository) List(ctx context.Context, limit, offset int) ([]*models.Agent, error) {
	agents := make([]*models.Agent, 0, len(m.agents))
	for _, agent := range m.agents {
		agents = append(agents, agent)
	}
	return agents, nil
}

func (m *mockAgentRepository) Count(ctx context.Context) (int, error) {
	return len(m.agents), nil
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

func (m *mockAgentRepository) GetStats(ctx context.Context) (*models.AgentStats, error) {
	return &models.AgentStats{
		TotalAgents:   len(m.agents),
		OnlineAgents:  len(m.agents),
		OfflineAgents: 0,
		ErrorAgents:   0,
		ByPlatform:    map[string]int32{"linux": int32(len(m.agents))},
	}, nil
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
				agent, err := repo.Get(ctx, agentID)
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
	_, err = repo.Get(ctx, agent.ID)
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
	handler := NewDiagnosticsHandler(repo)

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
				job, err := repo.Get(ctx, jobID)
				require.NoError(t, err)
				assert.Equal(t, tt.req.Target, job.Target)
			}
		})
	}
}

func TestDiagnosticsHandler_GetDiagnosticsResult(t *testing.T) {
	repo := newMockJobRepository()
	handler := NewDiagnosticsHandler(repo)

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
	handler := NewDiagnosticsHandler(repo)

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
	agent := &models.Agent{
		ID:              uuid.New(),
		Name:            "test-agent",
		Hostname:        "test-host",
		IPAddress:       "192.168.1.100",
		Platform:        "linux",
		Architecture:    "amd64",
		Version:         "1.0.0",
		Status:          "online",
		Capabilities:    []string{"cpu", "memory"},
		Labels:          map[string]interface{}{"env": "test"},
		LastHeartbeatAt: time.Now(),
		RegisteredAt:    time.Now(),
	}

	proto := toProtoAgent(agent)

	assert.Equal(t, agent.ID.String(), proto.Id)
	assert.Equal(t, agent.Name, proto.Name)
	assert.Equal(t, agent.Hostname, proto.Hostname)
	assert.Equal(t, agent.IPAddress, proto.IpAddress)
	assert.Equal(t, agent.Platform, proto.Platform)
	assert.Equal(t, agent.Architecture, proto.Architecture)
	assert.Equal(t, agent.Version, proto.Version)
	assert.Equal(t, agent.Status, proto.Status)
	assert.Equal(t, agent.Capabilities, proto.Capabilities)
	assert.NotNil(t, proto.LastHeartbeatAt)
	assert.NotNil(t, proto.RegisteredAt)

	// Verify timestamps are close
	assert.WithinDuration(t, agent.LastHeartbeatAt, proto.LastHeartbeatAt.AsTime(), time.Second)
	assert.WithinDuration(t, agent.RegisteredAt, proto.RegisteredAt.AsTime(), time.Second)
}
