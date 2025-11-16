package formatters

import (
	"fmt"

	"github.com/alpkeskin/gotoon"
	"github.com/ignacio/lumo/internal/diagnostics"
)

// ToonFormatter formats diagnostic results in TOON (Token-Oriented Object Notation) format
// TOON is optimized for LLM consumption, achieving 30-60% token reduction compared to JSON
// while maintaining high comprehension accuracy.
type ToonFormatter struct {
	indent       int
	lengthMarker bool
}

// ToonFormatterOption allows customization of the TOON formatter
type ToonFormatterOption func(*ToonFormatter)

// WithIndent sets the indentation level (default: 2 spaces)
func WithIndent(spaces int) ToonFormatterOption {
	return func(f *ToonFormatter) {
		f.indent = spaces
	}
}

// WithLengthMarker adds '#' prefix to array counts for clarity
func WithLengthMarker() ToonFormatterOption {
	return func(f *ToonFormatter) {
		f.lengthMarker = true
	}
}

// NewToonFormatter creates a new TOON formatter with optional customizations
func NewToonFormatter(opts ...ToonFormatterOption) *ToonFormatter {
	f := &ToonFormatter{
		indent:       2, // Default 2 spaces
		lengthMarker: false,
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

// FormatReport formats a complete diagnostic report in TOON format
func (f *ToonFormatter) FormatReport(report *diagnostics.Report) string {
	// Convert Report to a map structure optimized for TOON encoding
	data := f.reportToMap(report)

	// Build TOON encoding options
	opts := []gotoon.EncodeOption{
		gotoon.WithIndent(f.indent),
	}

	if f.lengthMarker {
		opts = append(opts, gotoon.WithLengthMarker())
	}

	// Encode to TOON format
	encoded, err := gotoon.Encode(data, opts...)
	if err != nil {
		// Fallback: if TOON encoding fails, return error message
		return fmt.Sprintf("Error encoding to TOON: %v", err)
	}

	return encoded
}

// FormatResult formats a single check result in TOON format
func (f *ToonFormatter) FormatResult(result *diagnostics.CheckResult) string {
	data := f.resultToMap(result)

	opts := []gotoon.EncodeOption{
		gotoon.WithIndent(f.indent),
	}

	if f.lengthMarker {
		opts = append(opts, gotoon.WithLengthMarker())
	}

	encoded, err := gotoon.Encode(data, opts...)
	if err != nil {
		return fmt.Sprintf("Error encoding to TOON: %v", err)
	}

	return encoded
}

// reportToMap converts a Report to a map structure optimized for TOON
func (f *ToonFormatter) reportToMap(report *diagnostics.Report) map[string]interface{} {
	data := map[string]interface{}{
		"timestamp": report.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		"duration":  report.Duration.Milliseconds(),
		"summary": map[string]interface{}{
			"total_checks":   report.Summary.TotalChecks,
			"ok_count":       report.Summary.OKCount,
			"info_count":     report.Summary.InfoCount,
			"warning_count":  report.Summary.WarningCount,
			"critical_count": report.Summary.CriticalCount,
			"error_count":    report.Summary.ErrorCount,
		},
	}

	// Convert results to uniform array structure
	if len(report.Results) > 0 {
		results := make([]map[string]interface{}, len(report.Results))
		for i, result := range report.Results {
			results[i] = f.resultToMap(result)
		}
		data["results"] = results
	}

	// Add metadata if present
	if len(report.Metadata) > 0 {
		data["metadata"] = report.Metadata
	}

	return data
}

// resultToMap converts a CheckResult to a map structure
func (f *ToonFormatter) resultToMap(result *diagnostics.CheckResult) map[string]interface{} {
	data := map[string]interface{}{
		"name":     result.Name,
		"category": string(result.Category),
		"status":   string(result.Status),
		"severity": string(result.Severity),
		"message":  result.Message,
		"duration": result.Duration.Milliseconds(),
	}

	// Add timestamp
	if !result.Timestamp.IsZero() {
		data["timestamp"] = result.Timestamp.Format("2006-01-02T15:04:05Z07:00")
	}

	// Add metrics if present (perfect for TOON's tabular format)
	if len(result.Metrics) > 0 {
		metrics := make([]map[string]interface{}, len(result.Metrics))
		for i, metric := range result.Metrics {
			m := map[string]interface{}{
				"name":  metric.Name,
				"value": metric.Value,
				"unit":  metric.Unit,
			}
			if metric.Threshold > 0 {
				m["threshold"] = metric.Threshold
				m["threshold_type"] = string(metric.ThresholdType)
			}
			metrics[i] = m
		}
		data["metrics"] = metrics
	}

	// Add structured data if present
	if len(result.Data) > 0 {
		data["data"] = result.Data
	}

	// Add error if present
	if result.Error != "" {
		data["error"] = result.Error
	}

	return data
}
