package ingestion

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/intelligence/vectorstore"
)

// DocumentBuilder converts diagnostic reports into RAG documents
type DocumentBuilder struct{}

func NewDocumentBuilder() *DocumentBuilder {
	return &DocumentBuilder{}
}

// BuildFromReport creates a document from a diagnostic report
func (b *DocumentBuilder) BuildFromReport(report *diagnostics.Report, hostname string) (*vectorstore.Document, error) {
	// Build searchable content
	var content strings.Builder

	// Add summary
	content.WriteString(fmt.Sprintf("Diagnostic Report for %s\n", hostname))
	content.WriteString(fmt.Sprintf("Timestamp: %s\n", report.Timestamp.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("Overall Severity: %s\n", report.GetWorstSeverity()))
	content.WriteString(fmt.Sprintf("Total Checks: %d (Critical: %d, Warnings: %d)\n\n",
		report.Summary.TotalChecks,
		report.Summary.CriticalCount,
		report.Summary.WarningCount))

	// Add failed/warning checks
	for _, result := range report.Results {
		if result.Severity == diagnostics.SeverityCritical ||
			result.Severity == diagnostics.SeverityError ||
			result.Severity == diagnostics.SeverityWarning {
			content.WriteString(fmt.Sprintf("[%s] %s: %s\n",
				result.Severity,
				result.Name,
				result.Message))

			// Add relevant log entries
			if len(result.LogEntries) > 0 {
				content.WriteString("  Relevant logs:\n")
				for _, logEntry := range result.LogEntries[:min(3, len(result.LogEntries))] {
					content.WriteString(fmt.Sprintf("    - %s: %s\n",
						logEntry.Timestamp.Format("15:04:05"),
						truncate(logEntry.Message, 100)))
				}
			}
			content.WriteString("\n")
		}
	}

	// Generate document ID (hash of content)
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(content.String())))[:16]

	// Build metadata
	metadata := map[string]interface{}{
		vectorstore.MetadataHostname:  hostname,
		vectorstore.MetadataTimestamp: report.Timestamp.Format(time.RFC3339),
		vectorstore.MetadataSeverity:  report.GetWorstSeverity().String(),
		"total_checks":                report.Summary.TotalChecks,
		"critical_count":              report.Summary.CriticalCount,
		"warning_count":               report.Summary.WarningCount,
	}

	return &vectorstore.Document{
		ID:        id,
		Content:   content.String(),
		Metadata:  metadata,
		Timestamp: report.Timestamp,
	}, nil
}

// BuildFromRemediation creates a document from a remediation action
func (b *DocumentBuilder) BuildFromRemediation(actionType string, outcome string, details string) (*vectorstore.Document, error) {
	var content strings.Builder

	content.WriteString(fmt.Sprintf("Remediation Action: %s\n", actionType))
	content.WriteString(fmt.Sprintf("Outcome: %s\n", outcome))
	content.WriteString(fmt.Sprintf("Details:\n%s\n", details))

	id := fmt.Sprintf("%x", sha256.Sum256([]byte(content.String())))[:16]

	metadata := map[string]interface{}{
		"action_type":                 actionType,
		vectorstore.MetadataOutcome:   outcome,
		vectorstore.MetadataTimestamp: time.Now().Format(time.RFC3339),
	}

	return &vectorstore.Document{
		ID:        id,
		Content:   content.String(),
		Metadata:  metadata,
		Timestamp: time.Now(),
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// BuildFromReportJSON creates a document from a JSON-encoded diagnostic report
func (b *DocumentBuilder) BuildFromReportJSON(reportJSON string, hostname string) (*vectorstore.Document, error) {
	var report diagnostics.Report
	if err := json.Unmarshal([]byte(reportJSON), &report); err != nil {
		return nil, fmt.Errorf("failed to parse report JSON: %w", err)
	}
	return b.BuildFromReport(&report, hostname)
}
