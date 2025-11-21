package formatters

import (
	"fmt"
	"strings"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// TextFormatter formats diagnostic results as human-readable text
type TextFormatter struct {
	colorEnabled bool
	verbose      bool
}

// NewTextFormatter creates a new text formatter
func NewTextFormatter(colorEnabled, verbose bool) *TextFormatter {
	return &TextFormatter{
		colorEnabled: colorEnabled,
		verbose:      verbose,
	}
}

// FormatReport formats a complete diagnostic report
func (f *TextFormatter) FormatReport(report *diagnostics.Report) string {
	var sb strings.Builder

	// Header
	sb.WriteString(f.formatHeader(report))
	sb.WriteString("\n")

	// Results grouped by category
	categories := f.groupResultsByCategory(report.Results)

	for _, results := range categories {
		for _, result := range results {
			sb.WriteString(f.formatResult(result))
		}
	}

	// Summary - Removed to match design
	// sb.WriteString(f.formatSummary(report))

	return sb.String()
}

// FormatResult formats a single check result
func (f *TextFormatter) FormatResult(result *diagnostics.CheckResult) string {
	return f.formatResult(result)
}

// formatHeader formats the report header
func (f *TextFormatter) formatHeader(report *diagnostics.Report) string {
	// Header is handled by the main command now
	return ""
}

// formatCategoryHeader formats a category section header
func (f *TextFormatter) formatCategoryHeader(category diagnostics.CheckCategory) string {
	categoryName := strings.ToUpper(string(category))
	return fmt.Sprintf("\n%s CHECKS\n%s", categoryName, strings.Repeat("-", len(categoryName)+7))
}

// formatResult formats a single check result
func (f *TextFormatter) formatResult(result *diagnostics.CheckResult) string {
	var sb strings.Builder

	// Status icon and name
	icon := f.getSeverityIcon(result.Severity)

	// Format: [✓] Check Name (STATUS)
	// Status text logic: OK (Green), WARNING (Yellow), ERROR (Red)
	statusText := "OK"
	statusColor := colorGreen

	if result.Severity == diagnostics.SeverityWarning {
		statusText = "WARNING"
		statusColor = colorYellow
	} else if result.Severity == diagnostics.SeverityError || result.Severity == diagnostics.SeverityCritical {
		statusText = "ERROR"
		statusColor = colorRed
	}

	sb.WriteString(fmt.Sprintf("%s %s (%s)\n", icon, f.colorize(result.Name, statusColor), f.colorize(statusText, statusColor)))

	// Message (indented)
	if result.Message != "" {
		lines := strings.Split(result.Message, "\n")
		for _, line := range lines {
			sb.WriteString(fmt.Sprintf("  %s\n", line))
		}
	}

	// Verbose mode: show metrics
	// Always show metrics if present, as per design
	if len(result.Metrics) > 0 {
		for _, metric := range result.Metrics {
			sb.WriteString(f.formatMetric(metric))
			sb.WriteString("\n")
		}
	}

	// Error message if present
	if result.Error != "" {
		sb.WriteString(f.colorize(fmt.Sprintf("  Error: %s\n", result.Error), colorRed))
	}

	sb.WriteString("\n")
	return sb.String()
}

// formatMetric formats a single metric
func (f *TextFormatter) formatMetric(metric diagnostics.Metric) string {
	// Format:   Metric Name: Value Unit
	// Example:   Load Average: 1.23, 1.45, 1.67 (8 cores)

	// Use %v to let Go format the float nicely (e.g. 15.3 instead of 15.30)
	valueStr := fmt.Sprintf("%v", metric.Value)
	if metric.Unit != "" {
		// If unit starts with %, append it directly, otherwise space
		if strings.HasPrefix(metric.Unit, "%") {
			valueStr += metric.Unit
		} else {
			valueStr += " " + metric.Unit
		}
	}

	// Special handling for load average or multi-value metrics if they were passed as string in unit or value
	// But based on struct, Value is float64.
	// The image shows "Load Average: 1.23, 1.45, 1.67 (8 cores)" which implies the metric value might be formatted differently
	// or the metric struct usage in the repro script is simplified.
	// For now, we stick to the simple formatting but clean indentation.

	return fmt.Sprintf("  %s: %s", metric.Name, valueStr)
}

// formatSummary formats the report summary
func (f *TextFormatter) formatSummary(report *diagnostics.Report) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("=" + strings.Repeat("=", 78) + "\n")
	sb.WriteString("  SUMMARY\n")
	sb.WriteString("=" + strings.Repeat("=", 78) + "\n")

	// Overall status
	overallStatus := f.getOverallStatus(report)
	statusColor := f.getSeverityColor(report.GetWorstSeverity())
	sb.WriteString(f.colorize(fmt.Sprintf("  Overall Status: %s\n", overallStatus), statusColor))
	sb.WriteString("\n")

	// Counts by severity
	sb.WriteString(fmt.Sprintf("  Total Checks:   %d\n", report.Summary.TotalChecks))
	sb.WriteString(f.colorize(fmt.Sprintf("  ✓ OK:           %d\n", report.Summary.OKCount), colorGreen))

	if report.Summary.InfoCount > 0 {
		sb.WriteString(f.colorize(fmt.Sprintf("  ℹ Info:         %d\n", report.Summary.InfoCount), colorBlue))
	}
	if report.Summary.WarningCount > 0 {
		sb.WriteString(f.colorize(fmt.Sprintf("  ⚠ Warnings:     %d\n", report.Summary.WarningCount), colorYellow))
	}
	if report.Summary.CriticalCount > 0 {
		sb.WriteString(f.colorize(fmt.Sprintf("  ✗ Critical:     %d\n", report.Summary.CriticalCount), colorRed))
	}
	if report.Summary.ErrorCount > 0 {
		sb.WriteString(f.colorize(fmt.Sprintf("  ✗ Errors:       %d\n", report.Summary.ErrorCount), colorRed))
	}

	sb.WriteString("=" + strings.Repeat("=", 78))

	return sb.String()
}

// groupResultsByCategory groups results by category
func (f *TextFormatter) groupResultsByCategory(results []*diagnostics.CheckResult) map[diagnostics.CheckCategory][]*diagnostics.CheckResult {
	grouped := make(map[diagnostics.CheckCategory][]*diagnostics.CheckResult)

	for _, result := range results {
		grouped[result.Category] = append(grouped[result.Category], result)
	}

	return grouped
}

// getOverallStatus returns a human-readable overall status
func (f *TextFormatter) getOverallStatus(report *diagnostics.Report) string {
	if report.Summary.ErrorCount > 0 {
		return "FAILED"
	}
	if report.Summary.CriticalCount > 0 {
		return "CRITICAL"
	}
	if report.Summary.WarningCount > 0 {
		return "WARNING"
	}
	if report.Summary.InfoCount > 0 {
		return "INFORMATIONAL"
	}
	return "HEALTHY"
}

// getSeverityIcon returns an icon for the severity level
func (f *TextFormatter) getSeverityIcon(severity diagnostics.Severity) string {
	switch severity {
	case diagnostics.SeverityOK:
		return f.colorize("[✓]", colorGreen)
	case diagnostics.SeverityInfo:
		return f.colorize("[ℹ]", colorBlue)
	case diagnostics.SeverityWarning:
		return f.colorize("[!]", colorYellow)
	case diagnostics.SeverityCritical:
		return f.colorize("[✗]", colorRed)
	case diagnostics.SeverityError:
		return f.colorize("[✗]", colorRed)
	default:
		return "[?]"
	}
}

// getSeverityColor returns the color for a severity level
func (f *TextFormatter) getSeverityColor(severity diagnostics.Severity) string {
	switch severity {
	case diagnostics.SeverityOK:
		return colorGreen
	case diagnostics.SeverityInfo:
		return colorBlue
	case diagnostics.SeverityWarning:
		return colorYellow
	case diagnostics.SeverityCritical, diagnostics.SeverityError:
		return colorRed
	default:
		return colorReset
	}
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
)

// colorize applies color to text if color is enabled
func (f *TextFormatter) colorize(text, color string) string {
	if !f.colorEnabled {
		return text
	}
	return color + text + colorReset
}
