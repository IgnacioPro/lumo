package handlers

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// API handler metrics for observability
var (
	// Event processing metrics
	eventsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_api_events_processed_total",
			Help: "Total number of events processed by the API",
		},
		[]string{"status"}, // accepted, rejected
	)

	asyncProcessingErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_api_async_processing_errors_total",
			Help: "Total number of errors during async event processing",
		},
		[]string{"operation"}, // ai_analysis, notification
	)

	// Notification metrics
	notificationsSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_api_notifications_sent_total",
			Help: "Total number of notifications sent",
		},
		[]string{"provider", "status"}, // provider name, success/failure
	)

	// AI analysis metrics
	aiAnalysisTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lumo_api_ai_analysis_total",
			Help: "Total number of AI analysis operations",
		},
		[]string{"status"}, // success, failure
	)

	aiAnalysisDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "lumo_api_ai_analysis_duration_seconds",
			Help:    "Duration of AI analysis operations",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~100s
		},
		[]string{"provider"},
	)
)
