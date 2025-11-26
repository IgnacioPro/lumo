package ingestion

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestDocumentBuilder_BuildFromReport(t *testing.T) {
	builder := NewDocumentBuilder()

	// Create test report with critical findings
	report := &diagnostics.Report{
		Timestamp: time.Now(),
		Results: []*diagnostics.CheckResult{
			{
				Name:     "CPU Check",
				Category: diagnostics.CategoryCPU,
				Status:   diagnostics.StatusCompleted,
				Severity: diagnostics.SeverityCritical,
				Message:  "CPU usage at 95%",
			},
			{
				Name:     "Memory Check",
				Category: diagnostics.CategoryMemory,
				Status:   diagnostics.StatusCompleted,
				Severity: diagnostics.SeverityWarning,
				Message:  "Memory usage at 82%",
			},
			{
				Name:     "Disk Check",
				Category: diagnostics.CategoryDisk,
				Status:   diagnostics.StatusCompleted,
				Severity: diagnostics.SeverityOK,
				Message:  "Disk usage normal",
			},
		},
		Summary: diagnostics.ReportSummary{
			TotalChecks:   3,
			CriticalCount: 1,
			WarningCount:  1,
			OKCount:       1,
		},
	}

	// Build document
	doc, err := builder.BuildFromReport(report, "test-host")
	require.NoError(t, err)
	require.NotNil(t, doc)

	// Verify document structure
	assert.NotEmpty(t, doc.ID)
	assert.NotEmpty(t, doc.Content)
	assert.Equal(t, "test-host", doc.Metadata["hostname"])
	assert.Equal(t, "critical", doc.Metadata["severity"])
	assert.Equal(t, 3, doc.Metadata["total_checks"])
	assert.Equal(t, 1, doc.Metadata["critical_count"])
	assert.Equal(t, 1, doc.Metadata["warning_count"])

	// Verify content includes critical/warning checks
	assert.Contains(t, doc.Content, "CPU usage at 95%")
	assert.Contains(t, doc.Content, "Memory usage at 82%")
	// Should NOT include OK checks
	assert.NotContains(t, doc.Content, "Disk usage normal")
}

func TestDocumentBuilder_BuildFromRemediation(t *testing.T) {
	builder := NewDocumentBuilder()

	doc, err := builder.BuildFromRemediation(
		"restart_service",
		"success",
		"Service restarted successfully after high memory usage",
	)
	require.NoError(t, err)
	require.NotNil(t, doc)

	assert.NotEmpty(t, doc.ID)
	assert.NotEmpty(t, doc.Content)
	assert.Contains(t, doc.Content, "restart_service")
	assert.Contains(t, doc.Content, "success")
	assert.Contains(t, doc.Content, "Service restarted successfully")
	assert.Equal(t, "restart_service", doc.Metadata["action_type"])
	assert.Equal(t, "success", doc.Metadata["outcome"])
}
