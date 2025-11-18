package agent

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// Metrics provides Prometheus metrics for the agent
type Metrics struct {
	port   int
	server *http.Server
	logger *logrus.Logger

	// Metrics
	diagnosticsTotal      *prometheus.CounterVec
	diagnosticsErrors     *prometheus.CounterVec
	diagnosticsDuration   *prometheus.HistogramVec
	heartbeatsTotal       prometheus.Counter
	heartbeatErrors       prometheus.Counter
	cacheHits             prometheus.Counter
	cacheMisses           prometheus.Counter
	cacheSize             prometheus.Gauge
	apiAvailable          prometheus.Gauge
	lastDiagnosticTime    prometheus.Gauge
	agentInfo             *prometheus.GaugeVec
	scheduledTasksRunning prometheus.Gauge
}

// NewMetrics creates a new metrics server
func NewMetrics(port int, logger *logrus.Logger) *Metrics {
	m := &Metrics{
		port:   port,
		logger: logger,

		// Initialize metrics
		diagnosticsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "lumo_agent_diagnostics_total",
				Help: "Total number of diagnostic runs",
			},
			[]string{"status"},
		),
		diagnosticsErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "lumo_agent_diagnostics_errors_total",
				Help: "Total number of diagnostic errors",
			},
			[]string{"checker"},
		),
		diagnosticsDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "lumo_agent_diagnostics_duration_seconds",
				Help:    "Duration of diagnostic runs in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"checker"},
		),
		heartbeatsTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "lumo_agent_heartbeats_total",
				Help: "Total number of heartbeats sent",
			},
		),
		heartbeatErrors: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "lumo_agent_heartbeat_errors_total",
				Help: "Total number of heartbeat errors",
			},
		),
		cacheHits: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "lumo_agent_cache_hits_total",
				Help: "Total number of cache hits",
			},
		),
		cacheMisses: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "lumo_agent_cache_misses_total",
				Help: "Total number of cache misses",
			},
		),
		cacheSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "lumo_agent_cache_size_bytes",
				Help: "Current cache size in bytes",
			},
		),
		apiAvailable: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "lumo_agent_api_available",
				Help: "API server availability (1 = available, 0 = unavailable)",
			},
		),
		lastDiagnosticTime: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "lumo_agent_last_diagnostic_timestamp",
				Help: "Timestamp of last diagnostic run",
			},
		),
		agentInfo: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "lumo_agent_info",
				Help: "Agent information",
			},
			[]string{"version", "hostname", "platform", "mode"},
		),
		scheduledTasksRunning: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "lumo_agent_scheduled_tasks_running",
				Help: "Number of currently running scheduled tasks",
			},
		),
	}

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	m.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return m
}

// Start starts the metrics server
func (m *Metrics) Start() error {
	m.logger.WithField("port", m.port).Info("Starting metrics server")

	go func() {
		if err := m.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.logger.WithError(err).Error("Metrics server failed")
		}
	}()

	return nil
}

// Stop stops the metrics server
func (m *Metrics) Stop() error {
	m.logger.Info("Stopping metrics server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := m.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown metrics server: %w", err)
	}

	return nil
}

// RecordDiagnostic records a diagnostic run
func (m *Metrics) RecordDiagnostic(status string, duration time.Duration, checker string) {
	m.diagnosticsTotal.WithLabelValues(status).Inc()
	m.diagnosticsDuration.WithLabelValues(checker).Observe(duration.Seconds())
	m.lastDiagnosticTime.SetToCurrentTime()
}

// RecordDiagnosticError records a diagnostic error
func (m *Metrics) RecordDiagnosticError(checker string) {
	m.diagnosticsErrors.WithLabelValues(checker).Inc()
}

// RecordHeartbeat records a heartbeat
func (m *Metrics) RecordHeartbeat(success bool) {
	m.heartbeatsTotal.Inc()
	if !success {
		m.heartbeatErrors.Inc()
	}
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit() {
	m.cacheHits.Inc()
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss() {
	m.cacheMisses.Inc()
}

// UpdateCacheSize updates the current cache size
func (m *Metrics) UpdateCacheSize(size int64) {
	m.cacheSize.Set(float64(size))
}

// UpdateAPIAvailability updates the API availability status
func (m *Metrics) UpdateAPIAvailability(available bool) {
	if available {
		m.apiAvailable.Set(1)
	} else {
		m.apiAvailable.Set(0)
	}
}

// SetAgentInfo sets the agent information
func (m *Metrics) SetAgentInfo(version, hostname, platform, mode string) {
	m.agentInfo.WithLabelValues(version, hostname, platform, mode).Set(1)
}

// UpdateScheduledTasksRunning updates the number of running scheduled tasks
func (m *Metrics) UpdateScheduledTasksRunning(count int) {
	m.scheduledTasksRunning.Set(float64(count))
}
