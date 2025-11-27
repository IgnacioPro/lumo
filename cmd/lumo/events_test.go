package main

import (
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/database/models"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{
			name:     "hours",
			input:    "24h",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "days",
			input:    "7d",
			expected: 7 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "minutes",
			input:    "30m",
			expected: 30 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "seconds",
			input:    "60s",
			expected: 60 * time.Second,
			wantErr:  false,
		},
		{
			name:     "one day",
			input:    "1d",
			expected: 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "thirty days",
			input:    "30d",
			expected: 30 * 24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "complex duration",
			input:    "1h30m",
			expected: 90 * time.Minute,
			wantErr:  false,
		},
		{
			name:    "invalid format",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "invalid days format",
			input:   "xd",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("parseDuration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsValidSeverity(t *testing.T) {
	tests := []struct {
		name     string
		severity models.EventSeverity
		expected bool
	}{
		{
			name:     "low is valid",
			severity: models.EventSeverityLow,
			expected: true,
		},
		{
			name:     "medium is valid",
			severity: models.EventSeverityMedium,
			expected: true,
		},
		{
			name:     "high is valid",
			severity: models.EventSeverityHigh,
			expected: true,
		},
		{
			name:     "critical is valid",
			severity: models.EventSeverityCritical,
			expected: true,
		},
		{
			name:     "empty is invalid",
			severity: "",
			expected: false,
		},
		{
			name:     "unknown is invalid",
			severity: "unknown",
			expected: false,
		},
		{
			name:     "uppercase LOW is invalid",
			severity: "LOW",
			expected: false,
		},
		{
			name:     "typo is invalid",
			severity: "hgih",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidSeverity(tt.severity)
			if got != tt.expected {
				t.Errorf("isValidSeverity(%q) = %v, want %v", tt.severity, got, tt.expected)
			}
		})
	}
}

func TestFormatEventRow(t *testing.T) {
	namespace := "default"
	analysis := "Test analysis"
	timestamp := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	event := &models.Event{
		EventType:      "oom-killed",
		Severity:       models.EventSeverityCritical,
		ResourceKind:   "Pod",
		ResourceName:   "test-pod-abc123",
		Namespace:      &namespace,
		EventTimestamp: timestamp,
		AIAnalysis:     &analysis,
		Metadata:       map[string]interface{}{"key": "value"},
	}

	tests := []struct {
		name         string
		noAnalysis   bool
		showMetadata bool
		expectedLen  int
	}{
		{
			name:         "default columns",
			noAnalysis:   false,
			showMetadata: false,
			expectedLen:  6, // type, severity, resource, namespace, ai, timestamp
		},
		{
			name:         "no analysis",
			noAnalysis:   true,
			showMetadata: false,
			expectedLen:  5, // type, severity, resource, namespace, timestamp
		},
		{
			name:         "with metadata",
			noAnalysis:   false,
			showMetadata: true,
			expectedLen:  7, // type, severity, resource, namespace, metadata, ai, timestamp
		},
		{
			name:         "with metadata no analysis",
			noAnalysis:   true,
			showMetadata: true,
			expectedLen:  6, // type, severity, resource, namespace, metadata, timestamp
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := formatEventRow(event, tt.noAnalysis, tt.showMetadata)
			if len(row) != tt.expectedLen {
				t.Errorf("formatEventRow() returned %d columns, want %d", len(row), tt.expectedLen)
			}
			// Verify event type is first column
			if row[0] != "oom-killed" {
				t.Errorf("formatEventRow() first column = %q, want %q", row[0], "oom-killed")
			}
			// Verify timestamp is last column
			expectedTimestamp := "2025-01-15 10:30:00"
			if row[len(row)-1] != expectedTimestamp {
				t.Errorf("formatEventRow() last column = %q, want %q", row[len(row)-1], expectedTimestamp)
			}
		})
	}
}

func TestFormatEventRowNilNamespace(t *testing.T) {
	timestamp := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	event := &models.Event{
		EventType:      "node-not-ready",
		Severity:       models.EventSeverityCritical,
		ResourceKind:   "Node",
		ResourceName:   "node-1",
		Namespace:      nil, // Cluster-scoped resource
		EventTimestamp: timestamp,
	}

	row := formatEventRow(event, false, false)
	// Namespace should be "-" for nil
	if row[3] != "-" {
		t.Errorf("formatEventRow() namespace = %q, want %q", row[3], "-")
	}
}

func TestFormatEventRowEmptyNamespace(t *testing.T) {
	timestamp := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	emptyNs := ""
	event := &models.Event{
		EventType:      "node-not-ready",
		Severity:       models.EventSeverityCritical,
		ResourceKind:   "Node",
		ResourceName:   "node-1",
		Namespace:      &emptyNs,
		EventTimestamp: timestamp,
	}

	row := formatEventRow(event, false, false)
	// Empty namespace should also show "-"
	if row[3] != "-" {
		t.Errorf("formatEventRow() namespace = %q, want %q", row[3], "-")
	}
}

func TestFormatEventRowLongMetadata(t *testing.T) {
	timestamp := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	namespace := "default"
	event := &models.Event{
		EventType:      "pod-pending",
		Severity:       models.EventSeverityMedium,
		ResourceKind:   "Pod",
		ResourceName:   "test-pod",
		Namespace:      &namespace,
		EventTimestamp: timestamp,
		Metadata: map[string]interface{}{
			"very_long_key_name": "this is a very long value that should be truncated for display",
		},
	}

	row := formatEventRow(event, true, true)
	// Find metadata column (should be at index 4)
	metadata := row[4]
	if len(metadata) > 43 { // 40 + "..."
		t.Errorf("formatEventRow() metadata not truncated: len=%d", len(metadata))
	}
	if len(metadata) == 40 && metadata[len(metadata)-3:] != "..." {
		t.Errorf("formatEventRow() metadata should end with '...'")
	}
}

func TestFormatEventRowAIAnalysisStatus(t *testing.T) {
	timestamp := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	namespace := "default"

	tests := []struct {
		name       string
		aiAnalysis *string
		expected   string
	}{
		{
			name:       "with analysis",
			aiAnalysis: ptrString("Some analysis"),
			expected:   "Yes",
		},
		{
			name:       "nil analysis",
			aiAnalysis: nil,
			expected:   "No",
		},
		{
			name:       "empty analysis",
			aiAnalysis: ptrString(""),
			expected:   "No",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &models.Event{
				EventType:      "pod-pending",
				Severity:       models.EventSeverityMedium,
				ResourceKind:   "Pod",
				ResourceName:   "test-pod",
				Namespace:      &namespace,
				EventTimestamp: timestamp,
				AIAnalysis:     tt.aiAnalysis,
			}

			row := formatEventRow(event, false, false)
			// AI analysis is at index 4 (after type, severity, resource, namespace)
			if row[4] != tt.expected {
				t.Errorf("formatEventRow() AI status = %q, want %q", row[4], tt.expected)
			}
		})
	}
}

func TestMaxEventLimit(t *testing.T) {
	if MaxEventLimit != 10000 {
		t.Errorf("MaxEventLimit = %d, want 10000", MaxEventLimit)
	}
}

func TestDefaultEventLimit(t *testing.T) {
	if DefaultEventLimit != 20 {
		t.Errorf("DefaultEventLimit = %d, want 20", DefaultEventLimit)
	}
}

// ptrString returns a pointer to the given string
func ptrString(s string) *string {
	return &s
}
