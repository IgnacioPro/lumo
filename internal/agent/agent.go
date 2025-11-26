package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/diagnostics/checkers"
	"github.com/ignacio/lumo/internal/version"
	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/agent/eventdriven"
	"github.com/ignacio/lumo/internal/agent/eventdriven/watchers"
)

// Agent represents the Lumo agent daemon
type Agent struct {
	ID        uuid.UUID
	Hostname  string
	Mode      string
	StartTime time.Time

	cfg            *config.Config
	logger         *logrus.Logger
	reporter       *Reporter
	cache          *Cache
	scheduler      *Scheduler
	healthCheck    *HealthCheck
	metrics        *Metrics
	eventDrivenMgr *eventdriven.Manager // Kubernetes event-driven manager
	redisClient    *redis.Client        // Redis client for event-driven mode

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
	a.metrics.SetAgentInfo(version.Version, a.Hostname, runtime.GOOS, a.Mode)

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

	// Stop event-driven manager if running
	if a.eventDrivenMgr != nil {
		a.logger.Info("Stopping event-driven manager")
		if err := a.eventDrivenMgr.Stop(); err != nil {
			a.logger.WithError(err).Warn("Failed to stop event-driven manager")
		}
	}

	// Close Redis client if it was created for event-driven mode
	if a.redisClient != nil {
		a.logger.Info("Closing Redis client")
		if err := a.redisClient.Close(); err != nil {
			a.logger.WithError(err).Warn("Failed to close Redis client")
		}
	}

	// Stop scheduler
	a.scheduler.Stop()

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
	labels["version"] = version.Version
	labels["mode"] = a.Mode

	// Build registration request
	req := RegisterAgentRequest{
		Name:         fmt.Sprintf("lumo-agent-%s", a.Hostname),
		Hostname:     a.Hostname,
		Platform:     platform,
		Architecture: runtime.GOARCH,
		Version:      version.Version,
		Capabilities: capabilities,
		Labels:       labels,
	}

	// Add Kubernetes metadata if enabled
	if a.cfg.Agent.Kubernetes.Enabled {
		// Only add metadata if at least one field is non-empty
		if a.cfg.Agent.Kubernetes.Cluster != "" || a.cfg.Agent.Kubernetes.Namespace != "" ||
			a.cfg.Agent.Kubernetes.NodeName != "" || a.cfg.Agent.Kubernetes.PodName != "" {
			req.KubernetesMetadata = &KubernetesMetadata{
				Cluster:   a.cfg.Agent.Kubernetes.Cluster,
				Namespace: a.cfg.Agent.Kubernetes.Namespace,
				NodeName:  a.cfg.Agent.Kubernetes.NodeName,
				PodName:   a.cfg.Agent.Kubernetes.PodName,
			}
		}
	}

	// Register with API with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := a.reporter.RegisterAgent(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
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
			if err := a.reporter.SendHeartbeat(ctx); err != nil {
				a.logger.WithError(err).Warn("Failed to send heartbeat")
				a.metrics.RecordHeartbeat(false)
				a.metrics.UpdateAPIAvailability(false)
			} else {
				a.metrics.RecordHeartbeat(true)
				a.metrics.UpdateAPIAvailability(true)
				// Keep health check alive
				a.healthCheck.KeepAlive()
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
	case "event-driven":
		return a.setupEventDrivenMode(ctx)
	default:
		return fmt.Errorf("invalid agent mode: %s", a.Mode)
	}
}

// setupScheduledMode sets up scheduled diagnostic runs
func (a *Agent) setupScheduledMode(_ context.Context) error {
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
func (a *Agent) setupOnDemandMode(_ context.Context) error {
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
func (a *Agent) setupHybridMode(_ context.Context) error {
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

// setupEventDrivenMode sets up Kubernetes event-driven monitoring
func (a *Agent) setupEventDrivenMode(ctx context.Context) error {
	a.logger.Info("Setting up event-driven mode for Kubernetes monitoring")

	// Validate prerequisites
	if !a.cfg.Agent.Kubernetes.Enabled {
		return fmt.Errorf("event-driven mode requires Kubernetes agent to be enabled")
	}

	if !a.cfg.Agent.EventDriven.Enabled {
		return fmt.Errorf("event-driven configuration is not enabled")
	}

	// Step 1: Create Kubernetes client
	a.logger.Info("Initializing Kubernetes client")
	k8sClient, err := a.createKubernetesClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Step 2: Create Redis client for debouncer state tracking
	a.logger.Info("Initializing Redis client for event state tracking")
	redisClient, err := a.createRedisClient()
	if err != nil {
		return fmt.Errorf("failed to create Redis client: %w", err)
	}
	a.redisClient = redisClient // Store for cleanup in Stop()

	// Note: AI analysis and notifications now handled by API server
	a.logger.Info("Events will be submitted to API server for centralized AI analysis and notifications")

	// Step 3: Create debouncer
	a.logger.WithFields(logrus.Fields{
		"debounce_window":     a.cfg.Agent.EventDriven.DebounceWindow,
		"max_debounce_window": a.cfg.Agent.EventDriven.MaxDebounceWindow,
	}).Info("Creating event debouncer")
	debouncer, err := eventdriven.NewDebouncer(ctx, &eventdriven.DebouncerConfig{
		DebounceWindow:    a.cfg.Agent.EventDriven.DebounceWindow,
		MaxDebounceWindow: a.cfg.Agent.EventDriven.MaxDebounceWindow,
		RedisClient:       redisClient,
	}, a.logger)
	if err != nil {
		return fmt.Errorf("failed to create debouncer: %w", err)
	}

	// Step 4: Create event grouper
	grouper := eventdriven.NewEventGrouper(a.logger)

	// Step 5: Create event processor (now submits to API instead of analyzing locally)
	processor, err := eventdriven.NewAPIEventProcessor(&eventdriven.APIProcessorConfig{
		Context:     ctx,
		Config:      a.cfg,
		AgentID:     a.ID,
		APIEndpoint: a.cfg.Agent.APIEndpoint,
		APIToken:    a.cfg.Agent.Token,
		RedisClient: redisClient,
	}, a.logger)
	if err != nil {
		return fmt.Errorf("failed to create event processor: %w", err)
	}

	// Step 6: Create event filter
	minSeverity := eventdriven.SeverityLow
	switch a.cfg.Agent.EventDriven.MinSeverity {
	case "medium":
		minSeverity = eventdriven.SeverityMedium
	case "high":
		minSeverity = eventdriven.SeverityHigh
	case "critical":
		minSeverity = eventdriven.SeverityCritical
	}

	filter := &eventdriven.EventFilter{
		EventTypes:  []eventdriven.EventType{}, // Empty = all events
		Namespaces:  a.cfg.Agent.EventDriven.WatchNamespaces,
		MinSeverity: minSeverity,
	}

	// Step 9: Create event handler
	handler := eventdriven.NewBaseHandler(
		debouncer,
		filter,
		processor,
		grouper,
		a.cfg.Agent.EventDriven.GroupRelatedEvents,
		a.logger,
	)

	// Step 10: Create event-driven manager
	a.logger.Info("Creating event-driven manager")
	managerConfig := &eventdriven.Config{
		ResyncPeriod:       a.cfg.Agent.EventDriven.ResyncPeriod,
		Namespaces:         a.cfg.Agent.EventDriven.WatchNamespaces,
		DebounceWindow:     a.cfg.Agent.EventDriven.DebounceWindow,
		MaxEventsPerMinute: a.cfg.Agent.EventDriven.MaxEventsPerMinute,
		GroupRelatedEvents: a.cfg.Agent.EventDriven.GroupRelatedEvents,
	}

	manager, err := eventdriven.NewManager(k8sClient, managerConfig, a.logger)
	if err != nil {
		return fmt.Errorf("failed to create event-driven manager: %w", err)
	}

	a.eventDrivenMgr = manager

	// Step 11: Register watchers based on configuration
	watcherCount := 0

	if a.cfg.Agent.EventDriven.WatchPodEvents {
		a.logger.Info("Registering Pod watcher")
		podWatcher := watchers.NewPodWatcher(a.logger)
		if err := manager.AddWatcher(podWatcher, handler); err != nil {
			return fmt.Errorf("failed to register pod watcher: %w", err)
		}
		watcherCount++
	}

	if a.cfg.Agent.EventDriven.WatchWorkloads {
		a.logger.Info("Registering workload watchers (Deployment, StatefulSet, DaemonSet, Job)")

		deploymentWatcher := watchers.NewDeploymentWatcher(a.logger)
		if err := manager.AddWatcher(deploymentWatcher, handler); err != nil {
			return fmt.Errorf("failed to register deployment watcher: %w", err)
		}

		statefulSetWatcher := watchers.NewStatefulSetWatcher(a.logger)
		if err := manager.AddWatcher(statefulSetWatcher, handler); err != nil {
			return fmt.Errorf("failed to register statefulset watcher: %w", err)
		}

		daemonSetWatcher := watchers.NewDaemonSetWatcher(a.logger)
		if err := manager.AddWatcher(daemonSetWatcher, handler); err != nil {
			return fmt.Errorf("failed to register daemonset watcher: %w", err)
		}

		jobWatcher := watchers.NewJobWatcher(a.logger)
		if err := manager.AddWatcher(jobWatcher, handler); err != nil {
			return fmt.Errorf("failed to register job watcher: %w", err)
		}

		watcherCount += 4
	}

	if a.cfg.Agent.EventDriven.WatchVolumes {
		a.logger.Info("Registering volume watchers (PVC, Events)")

		pvcWatcher := watchers.NewPVCWatcher(a.logger)
		if err := manager.AddWatcher(pvcWatcher, handler); err != nil {
			return fmt.Errorf("failed to register pvc watcher: %w", err)
		}

		watcherCount++
	}

	if a.cfg.Agent.EventDriven.WatchNodes {
		a.logger.Info("Registering Node watcher")
		nodeWatcher := watchers.NewNodeWatcher(a.logger)
		if err := manager.AddWatcher(nodeWatcher, handler); err != nil {
			return fmt.Errorf("failed to register node watcher: %w", err)
		}
		watcherCount++
	}

	if a.cfg.Agent.EventDriven.WatchEvents {
		a.logger.Info("Registering Kubernetes Event watcher")
		eventWatcher := watchers.NewEventWatcher(a.logger)
		if err := manager.AddWatcher(eventWatcher, handler); err != nil {
			return fmt.Errorf("failed to register event watcher: %w", err)
		}
		watcherCount++
	}

	if watcherCount == 0 {
		return fmt.Errorf("no watchers enabled - at least one watcher type must be enabled")
	}

	// Step 12: Start the event-driven manager
	a.logger.WithFields(logrus.Fields{
		"watchers":        watcherCount,
		"debounce_window": a.cfg.Agent.EventDriven.DebounceWindow,
		"resync_period":   a.cfg.Agent.EventDriven.ResyncPeriod,
		"group_events":    a.cfg.Agent.EventDriven.GroupRelatedEvents,
		"min_severity":    a.cfg.Agent.EventDriven.MinSeverity,
		"api_endpoint":    a.cfg.Agent.APIEndpoint,
	}).Info("Starting event-driven manager - events will be submitted to API for AI analysis and notifications")

	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to start event-driven manager: %w", err)
	}

	a.logger.Info("Event-driven mode started successfully - watching for Kubernetes events")
	return nil
}

// createKubernetesClient creates a Kubernetes client (in-cluster or kubeconfig)
func (a *Agent) createKubernetesClient() (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error

	// Try in-cluster configuration first
	config, err = rest.InClusterConfig()
	if err != nil {
		a.logger.Debug("Not running in-cluster, trying kubeconfig")

		// Fall back to kubeconfig
		kubeconfigPath := a.cfg.Diagnostics.Kubernetes.KubeconfigPath
		if kubeconfigPath == "" {
			// Use default location
			if home := os.Getenv("HOME"); home != "" {
				kubeconfigPath = filepath.Join(home, ".kube", "config")
			}
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	return clientset, nil
}

// createRedisClient creates a Redis client for event state tracking
func (a *Agent) createRedisClient() (*redis.Client, error) {
	if !a.cfg.Cache.Enabled {
		return nil, fmt.Errorf("redis cache must be enabled for event-driven mode")
	}

	opts, err := redis.ParseURL(a.cfg.Cache.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", err)
	}

	if a.cfg.Cache.Password != "" {
		opts.Password = a.cfg.Cache.Password
	}

	opts.MaxRetries = a.cfg.Cache.MaxRetries
	opts.PoolSize = a.cfg.Cache.PoolSize

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	a.logger.WithField("url", a.cfg.Cache.RedisURL).Info("Connected to Redis")
	return client, nil
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

	// Try to submit results to API
	if a.reporter.IsAvailable(ctx) {
		diagnosticResult := DiagnosticResult{
			Target: "localhost",
			Checks: a.cfg.Agent.EnabledChecks,
			Format: a.cfg.Agent.ReportFormat,
		}

		if err := a.reporter.SubmitDiagnosticResult(ctx, diagnosticResult); err != nil {
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
		checkers.NewPatchChecker(),
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
