package diagnostics

import (
	"encoding/json"
	"time"
)

// CheckResult represents the result of a single diagnostic check
type CheckResult struct {
	// Name is the unique identifier for this check
	Name string `json:"name"`

	// Category is the category this check belongs to
	Category CheckCategory `json:"category"`

	// Status is the execution status of the check
	Status CheckStatus `json:"status"`

	// Severity is the severity level of the findings
	Severity Severity `json:"severity"`

	// Message is a human-readable summary of the check result
	Message string `json:"message"`

	// Data contains structured check-specific data
	Data map[string]interface{} `json:"data,omitempty"`

	// Metrics contains measurable values with thresholds
	Metrics []Metric `json:"metrics,omitempty"`

	// Timestamp is when the check was executed
	Timestamp time.Time `json:"timestamp"`

	// Duration is how long the check took to execute
	Duration time.Duration `json:"duration"`

	// Error contains the error message if the check failed
	Error string `json:"error,omitempty"`

	// LogEntries contains structured log entries for RAG ingestion
	LogEntries []LogEntry `json:"log_entries,omitempty"`
}

// Metric represents a measurable value with optional threshold information
type Metric struct {
	// Name of the metric
	Name string `json:"name"`

	// Value of the metric
	Value float64 `json:"value"`

	// Unit of measurement (percent, bytes, count, ms, etc.)
	Unit string `json:"unit"`

	// Threshold value that triggers warning/critical
	Threshold float64 `json:"threshold,omitempty"`

	// ThresholdType indicates if threshold is a maximum or minimum
	ThresholdType ThresholdType `json:"threshold_type,omitempty"`
}

// ThresholdType indicates how a threshold should be evaluated
type ThresholdType string

const (
	ThresholdTypeMax ThresholdType = "max" // Value should be below threshold
	ThresholdTypeMin ThresholdType = "min" // Value should be above threshold
)

// LogEntry represents a structured log entry for RAG ingestion
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Source    string                 `json:"source"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Report contains the complete diagnostic report
type Report struct {
	// Timestamp when the diagnostics were run
	Timestamp time.Time `json:"timestamp"`

	// Duration of the entire diagnostic run
	Duration time.Duration `json:"duration"`

	// Results contains all check results
	Results []*CheckResult `json:"results"`

	// Summary provides aggregated statistics
	Summary ReportSummary `json:"summary"`

	// Metadata contains additional context
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ReportSummary contains aggregated statistics from all checks
type ReportSummary struct {
	// Total number of checks run
	TotalChecks int `json:"total_checks"`

	// Number of checks with OK severity
	OKCount int `json:"ok_count"`

	// Number of checks with Info severity
	InfoCount int `json:"info_count"`

	// Number of checks with Warning severity
	WarningCount int `json:"warning_count"`

	// Number of checks with Critical severity
	CriticalCount int `json:"critical_count"`

	// Number of checks that had errors
	ErrorCount int `json:"error_count"`

	// Checks by category
	ByCategory map[CheckCategory]CategorySummary `json:"by_category"`
}

// CategorySummary contains statistics for a specific category
type CategorySummary struct {
	Total    int `json:"total"`
	OK       int `json:"ok"`
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
	Error    int `json:"error"`
}

// generateSummary creates a summary from check results
func generateSummary(results []*CheckResult) ReportSummary {
	summary := ReportSummary{
		TotalChecks: len(results),
		ByCategory:  make(map[CheckCategory]CategorySummary),
	}

	for _, result := range results {
		// Count by severity
		switch result.Severity {
		case SeverityOK:
			summary.OKCount++
		case SeverityInfo:
			summary.InfoCount++
		case SeverityWarning:
			summary.WarningCount++
		case SeverityCritical:
			summary.CriticalCount++
		case SeverityError:
			summary.ErrorCount++
		}

		// Count by category
		catSummary := summary.ByCategory[result.Category]
		catSummary.Total++

		switch result.Severity {
		case SeverityOK:
			catSummary.OK++
		case SeverityWarning:
			catSummary.Warning++
		case SeverityCritical:
			catSummary.Critical++
		case SeverityError:
			catSummary.Error++
		}

		summary.ByCategory[result.Category] = catSummary
	}

	return summary
}

// ToJSON converts the report to JSON string
func (r *Report) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetResultsByCategory returns all results for a specific category
func (r *Report) GetResultsByCategory(category CheckCategory) []*CheckResult {
	results := make([]*CheckResult, 0)
	for _, result := range r.Results {
		if result.Category == category {
			results = append(results, result)
		}
	}
	return results
}

// GetResultsBySeverity returns all results with a specific severity
func (r *Report) GetResultsBySeverity(severity Severity) []*CheckResult {
	results := make([]*CheckResult, 0)
	for _, result := range r.Results {
		if result.Severity == severity {
			results = append(results, result)
		}
	}
	return results
}

// GetCriticalResults returns all results with critical or error severity
func (r *Report) GetCriticalResults() []*CheckResult {
	results := make([]*CheckResult, 0)
	for _, result := range r.Results {
		if result.Severity == SeverityCritical || result.Severity == SeverityError {
			results = append(results, result)
		}
	}
	return results
}

// HasCriticalIssues returns true if there are any critical or error results
func (r *Report) HasCriticalIssues() bool {
	return r.Summary.CriticalCount > 0 || r.Summary.ErrorCount > 0
}

// HasWarnings returns true if there are any warning results
func (r *Report) HasWarnings() bool {
	return r.Summary.WarningCount > 0
}

// IsHealthy returns true if all checks passed without warnings or errors
func (r *Report) IsHealthy() bool {
	return !r.HasWarnings() && !r.HasCriticalIssues()
}

// GetWorstSeverity returns the highest severity level found in the report
func (r *Report) GetWorstSeverity() Severity {
	if r.Summary.ErrorCount > 0 {
		return SeverityError
	}
	if r.Summary.CriticalCount > 0 {
		return SeverityCritical
	}
	if r.Summary.WarningCount > 0 {
		return SeverityWarning
	}
	if r.Summary.InfoCount > 0 {
		return SeverityInfo
	}
	return SeverityOK
}

// AddMetadata adds metadata to the report
func (r *Report) AddMetadata(key string, value interface{}) {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
}

// Success returns true if the check completed successfully (status and no error)
func (cr *CheckResult) Success() bool {
	return cr.Status == StatusCompleted && cr.Error == ""
}

// IsCritical returns true if the check has critical or error severity
func (cr *CheckResult) IsCritical() bool {
	return cr.Severity == SeverityCritical || cr.Severity == SeverityError
}

// IsWarning returns true if the check has warning severity or higher
func (cr *CheckResult) IsWarning() bool {
	return cr.Severity == SeverityWarning || cr.IsCritical()
}

// GetDataValue safely gets a value from the Data map
func (cr *CheckResult) GetDataValue(key string) (interface{}, bool) {
	if cr.Data == nil {
		return nil, false
	}
	value, ok := cr.Data[key]
	return value, ok
}

// GetDataFloat safely gets a float64 value from the Data map
func (cr *CheckResult) GetDataFloat(key string) (float64, bool) {
	value, ok := cr.GetDataValue(key)
	if !ok {
		return 0, false
	}

	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

// GetDataString safely gets a string value from the Data map
func (cr *CheckResult) GetDataString(key string) (string, bool) {
	value, ok := cr.GetDataValue(key)
	if !ok {
		return "", false
	}

	str, ok := value.(string)
	return str, ok
}

// GetMetric finds a metric by name
func (cr *CheckResult) GetMetric(name string) (*Metric, bool) {
	for i := range cr.Metrics {
		if cr.Metrics[i].Name == name {
			return &cr.Metrics[i], true
		}
	}
	return nil, false
}

// AddMetric adds a metric to the check result
func (cr *CheckResult) AddMetric(metric Metric) {
	cr.Metrics = append(cr.Metrics, metric)
}

// SetData sets a value in the Data map
func (cr *CheckResult) SetData(key string, value interface{}) {
	if cr.Data == nil {
		cr.Data = make(map[string]interface{})
	}
	cr.Data[key] = value
}

// NewCheckResult creates a new CheckResult with common fields initialized.
// This factory function reduces boilerplate in checker implementations.
func NewCheckResult(checker Checker) *CheckResult {
	return &CheckResult{
		Name:      checker.Name(),
		Category:  checker.Category(),
		Status:    StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
		Metrics:   []Metric{},
	}
}
