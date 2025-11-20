package remediation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// AuditEntry represents a single audit log entry.
type AuditEntry struct {
	Timestamp      time.Time      `json:"timestamp"`
	ActionID       string         `json:"action_id"`
	ActionName     string         `json:"action_name"`
	Category       ActionCategory `json:"category"`
	Risk           RiskLevel      `json:"risk"`
	Status         ActionStatus   `json:"status"`
	Message        string         `json:"message"`
	Duration       time.Duration  `json:"duration"`
	User           string         `json:"user,omitempty"`
	Hostname       string         `json:"hostname,omitempty"`
	ChangesApplied []string       `json:"changes_applied,omitempty"`
	Error          string         `json:"error,omitempty"`
}

// Auditor handles audit logging for remediation actions.
type Auditor struct {
	logFile  *os.File
	encoder  *json.Encoder
	logger   *logrus.Logger
	mu       sync.Mutex
	hostname string
	user     string
}

// NewAuditor creates a new auditor that writes to the specified log file.
func NewAuditor(logPath string, logger *logrus.Logger) (*Auditor, error) {
	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	// Open log file for appending
	// #nosec G304 -- logPath is expected to be variable
	file, err := os.OpenFile(filepath.Clean(logPath), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	// Get hostname and user for attribution
	hostname, _ := os.Hostname()
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME") // Windows fallback
	}

	auditor := &Auditor{
		logFile:  file,
		encoder:  json.NewEncoder(file),
		logger:   logger,
		hostname: hostname,
		user:     user,
	}

	logger.WithField("log_path", logPath).Info("Audit logger initialized")

	return auditor, nil
}

// LogAction logs a remediation action result to the audit log.
func (a *Auditor) LogAction(action Action, result *ActionResult) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry := AuditEntry{
		Timestamp:      time.Now(),
		ActionID:       action.ID(),
		ActionName:     action.Name(),
		Category:       action.Category(),
		Risk:           action.Risk(),
		Status:         result.Status,
		Message:        result.Message,
		Duration:       result.Duration,
		User:           a.user,
		Hostname:       a.hostname,
		ChangesApplied: result.ChangesApplied,
		Error:          result.Error,
	}

	if err := a.encoder.Encode(&entry); err != nil {
		a.logger.WithError(err).Error("Failed to write audit log entry")
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	// Ensure data is flushed to disk
	if err := a.logFile.Sync(); err != nil {
		a.logger.WithError(err).Warn("Failed to sync audit log to disk")
	}

	a.logger.WithFields(logrus.Fields{
		"action_id": entry.ActionID,
		"status":    entry.Status,
		"duration":  entry.Duration,
	}).Debug("Audit entry logged")

	return nil
}

// LogCustomEntry logs a custom audit entry (for manual events).
func (a *Auditor) LogCustomEntry(actionID, actionName, message string, category ActionCategory, risk RiskLevel, status ActionStatus) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	entry := AuditEntry{
		Timestamp:  time.Now(),
		ActionID:   actionID,
		ActionName: actionName,
		Category:   category,
		Risk:       risk,
		Status:     status,
		Message:    message,
		User:       a.user,
		Hostname:   a.hostname,
	}

	if err := a.encoder.Encode(&entry); err != nil {
		a.logger.WithError(err).Error("Failed to write custom audit entry")
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	if err := a.logFile.Sync(); err != nil {
		a.logger.WithError(err).Warn("Failed to sync audit log to disk")
	}

	return nil
}

// Close closes the audit log file.
func (a *Auditor) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.logFile != nil {
		if err := a.logFile.Close(); err != nil {
			return fmt.Errorf("failed to close audit log file: %w", err)
		}
		a.logFile = nil
	}

	a.logger.Info("Audit logger closed")
	return nil
}

// ReadAuditLog reads and parses audit log entries from the log file.
func ReadAuditLog(logPath string) ([]AuditEntry, error) {
	// #nosec G304 -- logPath is expected to be variable
	file, err := os.Open(filepath.Clean(logPath))
	if err != nil {
		if os.IsNotExist(err) {
			return []AuditEntry{}, nil
		}
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	var entries []AuditEntry
	decoder := json.NewDecoder(file)

	for decoder.More() {
		var entry AuditEntry
		if err := decoder.Decode(&entry); err != nil {
			return entries, fmt.Errorf("failed to decode audit entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// FilterAuditLog filters audit entries based on criteria.
func FilterAuditLog(entries []AuditEntry, criteria AuditFilter) []AuditEntry {
	filtered := make([]AuditEntry, 0)

	for _, entry := range entries {
		if criteria.Matches(entry) {
			filtered = append(filtered, entry)
		}
	}

	return filtered
}

// AuditFilter defines criteria for filtering audit log entries.
type AuditFilter struct {
	ActionID string         // Filter by action ID
	Category ActionCategory // Filter by category
	Risk     RiskLevel      // Filter by risk level
	Status   ActionStatus   // Filter by status
	User     string         // Filter by user
	Hostname string         // Filter by hostname
	Since    time.Time      // Only entries after this time
	Before   time.Time      // Only entries before this time
}

// Matches returns true if an audit entry matches the filter criteria.
func (f AuditFilter) Matches(entry AuditEntry) bool {
	if f.ActionID != "" && entry.ActionID != f.ActionID {
		return false
	}
	if f.Category != "" && entry.Category != f.Category {
		return false
	}
	if f.Risk != "" && entry.Risk != f.Risk {
		return false
	}
	if f.Status != "" && entry.Status != f.Status {
		return false
	}
	if f.User != "" && entry.User != f.User {
		return false
	}
	if f.Hostname != "" && entry.Hostname != f.Hostname {
		return false
	}
	if !f.Since.IsZero() && entry.Timestamp.Before(f.Since) {
		return false
	}
	if !f.Before.IsZero() && entry.Timestamp.After(f.Before) {
		return false
	}
	return true
}

// AuditSummary provides statistics from audit log entries.
type AuditSummary struct {
	TotalEntries    int                    `json:"total_entries"`
	ByStatus        map[ActionStatus]int   `json:"by_status"`
	ByRisk          map[RiskLevel]int      `json:"by_risk"`
	ByCategory      map[ActionCategory]int `json:"by_category"`
	SuccessRate     float64                `json:"success_rate"`
	AverageDuration time.Duration          `json:"average_duration"`
	TimeRange       struct {
		Earliest time.Time `json:"earliest"`
		Latest   time.Time `json:"latest"`
	} `json:"time_range"`
}

// GenerateAuditSummary generates statistics from audit log entries.
func GenerateAuditSummary(entries []AuditEntry) AuditSummary {
	summary := AuditSummary{
		TotalEntries: len(entries),
		ByStatus:     make(map[ActionStatus]int),
		ByRisk:       make(map[RiskLevel]int),
		ByCategory:   make(map[ActionCategory]int),
	}

	if len(entries) == 0 {
		return summary
	}

	var totalDuration time.Duration
	var successCount int

	summary.TimeRange.Earliest = entries[0].Timestamp
	summary.TimeRange.Latest = entries[0].Timestamp

	for _, entry := range entries {
		// Count by status
		summary.ByStatus[entry.Status]++

		// Count by risk
		summary.ByRisk[entry.Risk]++

		// Count by category
		summary.ByCategory[entry.Category]++

		// Track success
		if entry.Status == StatusSuccess {
			successCount++
		}

		// Accumulate duration
		totalDuration += entry.Duration

		// Track time range
		if entry.Timestamp.Before(summary.TimeRange.Earliest) {
			summary.TimeRange.Earliest = entry.Timestamp
		}
		if entry.Timestamp.After(summary.TimeRange.Latest) {
			summary.TimeRange.Latest = entry.Timestamp
		}
	}

	// Calculate success rate
	if summary.TotalEntries > 0 {
		summary.SuccessRate = float64(successCount) / float64(summary.TotalEntries) * 100
	}

	// Calculate average duration
	if summary.TotalEntries > 0 {
		summary.AverageDuration = totalDuration / time.Duration(summary.TotalEntries)
	}

	return summary
}
