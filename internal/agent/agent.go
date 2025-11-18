package agent

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/diagnostics/checkers"
	"github.com/ignacio/lumo/internal/messaging"
	"github.com/sirupsen/logrus"
)

const Version = "1.0.0"

// Agent represents the Lumo agent daemon
type Agent struct {
	ID        uuid.UUID
	Hostname  string
	Mode      string
	StartTime time.Time

	cfg         *config.Config
	logger      *logrus.Logger
	reporter    *Reporter
	cache       *Cache
	scheduler   *Scheduler
	healthCheck *HealthCheck
	metrics     *Metrics
	publisher   messaging.Publisher // Phase 11: Messaging integration

	stopCh chan struct{}
}

// New creates a new agent instance
func New(cfg *config.Config, logger *logrus.Logger) (*Agent, error) {
	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	// Create reporter
	reporter := NewReporter(&cfg.Agent, logger)

	// Create cache
	cache, err := NewCache(
		cfg.Agent.CachePath,
		cfg.Agent.CacheMaxSize,
		cfg.Agent.CacheTTL,
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	// Create scheduler
	scheduler := NewScheduler(logger)

	// Create messaging publisher (Phase 11)
	msgCfg := messaging.FromAgentConfig(&cfg.Agent.Messaging)
	publisher, err := messaging.NewPublisher(msgCfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create messaging publisher: %w", err)
	}

	// Create agent
	agent := &Agent{
		ID:        uuid.New(),
		Hostname:  hostname,
		Mode:      cfg.Agent.Mode,
		StartTime: time.Now(),
		cfg:       cfg,
		logger:    logger,
		reporter:  reporter,
		cache:     cache,
		scheduler: scheduler,
		publisher: publisher,
		stopCh:    make(chan struct{}),
	}

	// Create health check server
	agent.healthCheck = NewHealthCheck(cfg.Agent.HealthCheckPort, agent, logger)

	// Create metrics server
	agent.metrics = NewMetrics(cfg.Agent.MetricsPort, logger)

	return agent, nil
}

// Start starts the agent
func (a *Agent) Start(ctx context.Context) error {
	a.logger.Info("Starting Lumo agent")

	// Set agent info metrics
	a.metrics.SetAgentInfo(Version, a.Hostname, runtime.GOOS, a.Mode)

	// Start health check server
	if err := a.healthCheck.Start(); err != nil {
		return fmt.Errorf("failed to start health check server: %w", err)
	}

	// Start metrics server
	if err := a.metrics.Start(); err != nil {
		return fmt.Errorf("failed to start metrics server: %w", err)
	}

	// Register with API server
	if err := a.register(); err != nil {
		a.logger.WithError(err).Warn("Failed to register with API server")
		if !a.cfg.Agent.OfflineMode {
			return fmt.Errorf("failed to register agent: %w", err)
		}
		a.logger.Info("Continuing in offline mode")
	}

	// Start heartbeat routine
	go a.heartbeatLoop(ctx)

	// Set up scheduled diagnostics based on mode
	if err := a.setupMode(ctx); err != nil {
		return fmt.Errorf("failed to setup agent mode: %w", err)
	}

	a.logger.WithFields(logrus.Fields{
		"mode":              a.Mode,
		"hostname":          a.Hostname,
		"health_check_port": a.cfg.Agent.HealthCheckPort,
		"metrics_port":      a.cfg.Agent.MetricsPort,
	}).Info("Agent started successfully")

	return nil
}

// Stop stops the agent
func (a *Agent) Stop() error {
	a.logger.Info("Stopping Lumo agent")

	close(a.stopCh)

	// Stop scheduler
	a.scheduler.Stop()

	// Close messaging publisher (Phase 11)
	if a.publisher != nil {
		if err := a.publisher.Close(); err != nil {
			a.logger.WithError(err).Warn("Failed to close messaging publisher")
		}
	}

	// Stop health check server
	if err := a.healthCheck.Stop(); err != nil {
		a.logger.WithError(err).Warn("Failed to stop health check server")
	}

	// Stop metrics server
	if err := a.metrics.Stop(); err != nil {
		a.logger.WithError(err).Warn("Failed to stop metrics server")
	}

	a.logger.Info("Agent stopped successfully")
	return nil
}

// register registers the agent with the API server
func (a *Agent) register() error {
	// Determine platform
	platform := runtime.GOOS
	if a.cfg.Agent.Kubernetes.Enabled {
		platform = "kubernetes"
	}

	// Build capabilities list from enabled checks
	capabilities := a.cfg.Agent.EnabledChecks

	// Build labels
	labels := make(map[string]interface{})
	labels["version"] = Version
	labels["mode"] = a.Mode

	// Build registration request
	req := RegisterAgentRequest{
		Name:         fmt.Sprintf("lumo-agent-%s", a.Hostname),
		Hostname:     a.Hostname,
		Platform:     platform,
		Architecture: runtime.GOARCH,
		Version:      Version,
		Capabilities: capabilities,
		Labels:       labels,
	}

	// Add Kubernetes metadata if enabled
	if a.cfg.Agent.Kubernetes.Enabled {
		req.KubernetesMetadata = &KubernetesMetadata{
			Cluster:   a.cfg.Agent.Kubernetes.Cluster,
			Namespace: a.cfg.Agent.Kubernetes.Namespace,
			NodeName:  a.cfg.Agent.Kubernetes.NodeName,
			PodName:   a.cfg.Agent.Kubernetes.PodName,
		}
	}

	// Register with API
	resp, err := a.reporter.RegisterAgent(req)
	if err != nil {
		return err
	}

	// Parse agent ID
	agentID, err := uuid.Parse(resp.AgentID)
	if err != nil {
		return fmt.Errorf("invalid agent ID: %w", err)
	}

	a.ID = agentID
	a.logger.WithField("agent_id", a.ID).Info("Agent registered successfully")

	return nil
}

// heartbeatLoop sends periodic heartbeats to the API server
func (a *Agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(a.cfg.Agent.HeartbeatSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			if err := a.reporter.SendHeartbeat(); err != nil {
				a.logger.WithError(err).Warn("Failed to send heartbeat")
				a.metrics.RecordHeartbeat(false)
				a.metrics.UpdateAPIAvailability(false)
			} else {
				a.metrics.RecordHeartbeat(true)
				a.metrics.UpdateAPIAvailability(true)
			}
		}
	}
}

// setupMode sets up the agent based on the configured mode
func (a *Agent) setupMode(ctx context.Context) error {
	switch a.Mode {
	case "scheduled":
		return a.setupScheduledMode(ctx)
	case "on-demand":
		return a.setupOnDemandMode(ctx)
	case "continuous":
		return a.setupContinuousMode(ctx)
	case "hybrid":
		return a.setupHybridMode(ctx)
	default:
		return fmt.Errorf("invalid agent mode: %s", a.Mode)
	}
}

// setupScheduledMode sets up scheduled diagnostic runs
func (a *Agent) setupScheduledMode(ctx context.Context) error {
	a.logger.WithField("schedule", a.cfg.Agent.Schedule).Info("Setting up scheduled mode")

	task := func(ctx context.Context) error {
		return a.runDiagnostics(ctx)
	}

	if err := a.scheduler.AddTask("diagnostics", a.cfg.Agent.Schedule, task); err != nil {
		return fmt.Errorf("failed to add scheduled task: %w", err)
	}

	a.scheduler.Start()
	return nil
}

// setupOnDemandMode sets up on-demand mode (waits for API requests)
func (a *Agent) setupOnDemandMode(ctx context.Context) error {
	a.logger.Info("Setting up on-demand mode")
	// In on-demand mode, diagnostics are triggered via API calls
	// This would be implemented with a webhook or message queue listener
	// For now, we just log that we're in on-demand mode
	return nil
}

// setupContinuousMode sets up continuous diagnostic monitoring
func (a *Agent) setupContinuousMode(ctx context.Context) error {
	a.logger.Info("Setting up continuous mode")

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-a.stopCh:
				return
			default:
				if err := a.runDiagnostics(ctx); err != nil {
					a.logger.WithError(err).Error("Diagnostic run failed")
				}
				// Small delay between runs
				time.Sleep(30 * time.Second)
			}
		}
	}()

	return nil
}

// setupHybridMode sets up hybrid mode (scheduled + on-demand)
func (a *Agent) setupHybridMode(ctx context.Context) error {
	a.logger.WithField("schedule", a.cfg.Agent.Schedule).Info("Setting up hybrid mode")

	// Set up scheduled diagnostics
	task := func(ctx context.Context) error {
		return a.runDiagnostics(ctx)
	}

	if err := a.scheduler.AddTask("diagnostics", a.cfg.Agent.Schedule, task); err != nil {
		return fmt.Errorf("failed to add scheduled task: %w", err)
	}

	a.scheduler.Start()

	// On-demand functionality would be added here
	return nil
}

// runDiagnostics executes a diagnostic run
func (a *Agent) runDiagnostics(ctx context.Context) error {
	start := time.Now()
	a.logger.Info("Running diagnostics")

	// Create local executor for localhost
	executor := diagnostics.NewLocalExecutor()

	// Create diagnostic config
	diagConfig := diagnostics.DefaultConfig()
	if len(a.cfg.Agent.EnabledChecks) > 0 {
		diagConfig.EnabledChecks = a.cfg.Agent.EnabledChecks
	}

	// Create diagnostic runner
	thresholds := diagnostics.DefaultThresholds()
	runner := diagnostics.NewRunner(diagConfig, thresholds, executor, a.logger)

	// Register all checkers
	a.registerCheckers(runner)

	// Execute diagnostics
	report, err := runner.RunAll(ctx)
	if err != nil {
		a.healthCheck.UpdateStatus(HealthStatusUnhealthy, []string{err.Error()})
		a.metrics.RecordDiagnostic("error", time.Since(start), "all")
		return fmt.Errorf("diagnostic run failed: %w", err)
	}

	// Update health status
	a.healthCheck.UpdateStatus(HealthStatusHealthy, nil)

	// Record metrics
	for _, result := range report.Results {
		if result.Error != "" {
			a.metrics.RecordDiagnosticError(result.Name)
		}
	}
	a.metrics.RecordDiagnostic("success", time.Since(start), "all")

	// Publish diagnostic results to messaging system (Phase 11)
	if a.cfg.Agent.Messaging.Enabled && a.publisher != nil {
		msg := messaging.NewMessage(messaging.TopicDiagnostics, map[string]interface{}{
			"report":   report,
			"checks":   a.cfg.Agent.EnabledChecks,
			"format":   a.cfg.Agent.ReportFormat,
			"duration": time.Since(start).Seconds(),
		})
		msg.AgentID = a.ID.String()
		msg.Hostname = a.Hostname

		if err := a.publisher.Publish(ctx, msg); err != nil {
			a.logger.WithError(err).Warn("Failed to publish diagnostic results to messaging system")
		} else {
			a.logger.Debug("Published diagnostic results to messaging system")
		}
	}

	// Try to submit results to API
	if a.reporter.IsAvailable() {
		diagnosticResult := DiagnosticResult{
			Target: "localhost",
			Checks: a.cfg.Agent.EnabledChecks,
			Format: a.cfg.Agent.ReportFormat,
		}

		if err := a.reporter.SubmitDiagnosticResult(diagnosticResult); err != nil {
			a.logger.WithError(err).Warn("Failed to submit diagnostic results")
			// Cache results for later submission
			if err := a.cache.Set(fmt.Sprintf("diagnostic-%d", time.Now().Unix()), report); err != nil {
				a.logger.WithError(err).Warn("Failed to cache diagnostic results")
			}
		}
	} else if a.cfg.Agent.OfflineMode {
		// Cache results when offline
		a.logger.Debug("API unavailable, caching results")
		if err := a.cache.Set(fmt.Sprintf("diagnostic-%d", time.Now().Unix()), report); err != nil {
			a.logger.WithError(err).Warn("Failed to cache diagnostic results")
		}
		a.metrics.UpdateAPIAvailability(false)
	}

	// Update cache size metric
	if size, err := a.cache.Size(); err == nil {
		a.metrics.UpdateCacheSize(size)
	}

	a.logger.WithField("duration", time.Since(start)).Info("Diagnostic run completed")
	return nil
}

// registerCheckers registers all diagnostic checkers with the runner
func (a *Agent) registerCheckers(runner *diagnostics.Runner) {
	thresholds := diagnostics.DefaultThresholds()

	// Core checkers
	runner.RegisterCheckers(
		checkers.NewCPUChecker(thresholds.CPU),
		checkers.NewMemoryChecker(thresholds.Memory),
		checkers.NewDiskChecker(thresholds.Disk),
		checkers.NewProcessChecker(thresholds.Process),
		checkers.NewServiceChecker([]string{}), // Empty list = check all services
		checkers.NewNetworkChecker(thresholds.Network, a.cfg.Diagnostics.Network.Targets),
	)

	// Security checkers
	runner.RegisterCheckers(
		checkers.NewPortsChecker(a.cfg.Diagnostics.Security.PortCheck.WhitelistedPorts),
		checkers.NewSSHSecurityChecker(),
		checkers.NewAuthFailuresChecker(
			a.cfg.Diagnostics.Security.AuthFailureCheck.LookbackHours,
			a.cfg.Diagnostics.Security.AuthFailureCheck.FailureThreshold,
		),
	)

	// Kubernetes checker (if enabled)
	if a.cfg.Diagnostics.Kubernetes.Enabled {
		runner.RegisterChecker(checkers.NewKubernetesChecker(a.cfg.Diagnostics.Kubernetes, a.logger))
	}
}
