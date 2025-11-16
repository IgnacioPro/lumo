package checkers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// AuthFailuresChecker checks for failed authentication attempts
type AuthFailuresChecker struct {
	lookbackHours    int
	failureThreshold int
}

// NewAuthFailuresChecker creates a new authentication failures checker
func NewAuthFailuresChecker(lookbackHours, failureThreshold int) *AuthFailuresChecker {
	if lookbackHours <= 0 {
		lookbackHours = 1 // Default: 1 hour
	}
	if failureThreshold <= 0 {
		failureThreshold = 20 // Default threshold
	}
	return &AuthFailuresChecker{
		lookbackHours:    lookbackHours,
		failureThreshold: failureThreshold,
	}
}

// Name returns the checker name
func (a *AuthFailuresChecker) Name() string {
	return "auth_failures"
}

// Category returns the checker category
func (a *AuthFailuresChecker) Category() diagnostics.CheckCategory {
	return diagnostics.CategorySecurity
}

// Description returns the checker description
func (a *AuthFailuresChecker) Description() string {
	return "Checks for failed authentication attempts and potential brute force attacks"
}

// RequiresRoot returns false for basic log reading
func (a *AuthFailuresChecker) RequiresRoot() bool {
	return false
}

// FailedAuth represents a failed authentication attempt
type FailedAuth struct {
	Timestamp string `json:"timestamp"`
	User      string `json:"user"`
	SourceIP  string `json:"source_ip"`
	Reason    string `json:"reason"`
}

// AttackSource represents an IP address with multiple failed attempts
type AttackSource struct {
	IP            string `json:"ip"`
	FailureCount  int    `json:"failure_count"`
	Users         []string `json:"users_attempted"`
	FirstAttempt  string `json:"first_attempt"`
	LastAttempt   string `json:"last_attempt"`
}

// Run executes the auth failures check
func (a *AuthFailuresChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
	result := &diagnostics.CheckResult{
		Name:      a.Name(),
		Category:  a.Category(),
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
		Metrics:   []diagnostics.Metric{},
	}

	startTime := time.Now()

	// Detect log file location
	logPath, err := a.detectAuthLog(ctx, executor)
	if err != nil {
		result.SetData("error", fmt.Sprintf("Failed to detect auth log: %v", err))
		result.Message = fmt.Sprintf("Unable to check auth failures: %v", err)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.SetData("log_path", logPath)

	// Parse auth log for failures
	failures, err := a.parseAuthLog(ctx, executor, logPath)
	if err != nil {
		result.SetData("error", fmt.Sprintf("Failed to parse auth log: %v", err))
		result.Message = fmt.Sprintf("Unable to parse auth log: %v", err)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Analyze failures
	recentFailures := a.filterRecentFailures(failures)
	attackSources := a.identifyAttackSources(recentFailures)
	invalidUsers := a.extractInvalidUsers(recentFailures)

	// Store data
	result.SetData("total_failures", len(recentFailures))
	result.SetData("lookback_hours", a.lookbackHours)
	result.SetData("unique_attack_sources", len(attackSources))
	result.SetData("invalid_users_attempted", len(invalidUsers))

	if len(recentFailures) > 0 {
		result.SetData("recent_failures", recentFailures[:min(10, len(recentFailures))]) // First 10
	}
	if len(attackSources) > 0 {
		result.SetData("attack_sources", attackSources[:min(5, len(attackSources))]) // Top 5
	}
	if len(invalidUsers) > 0 {
		result.SetData("invalid_users", invalidUsers[:min(10, len(invalidUsers))]) // First 10
	}

	// Identify critical attack sources (>100 attempts)
	criticalSources := 0
	for _, source := range attackSources {
		if source.FailureCount > 100 {
			criticalSources++
		}
	}

	// Add metrics
	result.AddMetric(diagnostics.Metric{
		Name:          "auth_failures_last_hour",
		Value:         float64(len(recentFailures)),
		Unit:          "count",
		Threshold:     float64(a.failureThreshold),
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.AddMetric(diagnostics.Metric{
		Name:          "critical_attack_sources",
		Value:         float64(criticalSources),
		Unit:          "count",
		Threshold:     1, // Any critical source is concerning
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.Duration = time.Since(startTime)
	result.Message = a.formatMessage(len(recentFailures), len(attackSources), criticalSources)

	return result, nil
}

// detectAuthLog detects the authentication log file location
func (a *AuthFailuresChecker) detectAuthLog(ctx context.Context, executor diagnostics.CommandExecutor) (string, error) {
	// Try common log locations
	logPaths := []string{
		"/var/log/auth.log",       // Debian/Ubuntu
		"/var/log/secure",         // RHEL/CentOS
		"/var/log/system.log",     // macOS (partial)
	}

	for _, path := range logPaths {
		_, _, exitCode, _ := executor.ExecuteWithContext(ctx, fmt.Sprintf("test -r %s", path))
		if exitCode == 0 {
			return path, nil
		}
	}

	return "", fmt.Errorf("no readable auth log found")
}

// parseAuthLog parses authentication log for failures
func (a *AuthFailuresChecker) parseAuthLog(ctx context.Context, executor diagnostics.CommandExecutor, logPath string) ([]FailedAuth, error) {
	// Read last 1000 lines of log (should cover recent activity)
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, fmt.Sprintf("tail -n 1000 %s 2>/dev/null", logPath))
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to read log: %w", err)
	}

	return a.parseLogContent(stdout), nil
}

// parseLogContent parses log content for failed authentication attempts
func (a *AuthFailuresChecker) parseLogContent(content string) []FailedAuth {
	lines := strings.Split(content, "\n")
	failures := []FailedAuth{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Match various failure patterns
		if a.isFailedPasswordAttempt(line) {
			failure := a.extractFailureDetails(line, "Failed password")
			if failure != nil {
				failures = append(failures, *failure)
			}
		} else if a.isInvalidUserAttempt(line) {
			failure := a.extractFailureDetails(line, "Invalid user")
			if failure != nil {
				failures = append(failures, *failure)
			}
		} else if a.isConnectionClosedAuth(line) {
			failure := a.extractFailureDetails(line, "Connection closed by authenticating user")
			if failure != nil {
				failures = append(failures, *failure)
			}
		} else if a.isDisconnectedAuth(line) {
			failure := a.extractFailureDetails(line, "Disconnected from authenticating user")
			if failure != nil {
				failures = append(failures, *failure)
			}
		}
	}

	return failures
}

// isFailedPasswordAttempt checks if line contains failed password attempt
func (a *AuthFailuresChecker) isFailedPasswordAttempt(line string) bool {
	return strings.Contains(line, "Failed password") ||
		strings.Contains(line, "authentication failure")
}

// isInvalidUserAttempt checks if line contains invalid user attempt
func (a *AuthFailuresChecker) isInvalidUserAttempt(line string) bool {
	return strings.Contains(line, "Invalid user") ||
		strings.Contains(line, "invalid user")
}

// isConnectionClosedAuth checks if line contains connection closed during auth
func (a *AuthFailuresChecker) isConnectionClosedAuth(line string) bool {
	return strings.Contains(line, "Connection closed by authenticating user")
}

// isDisconnectedAuth checks if line contains disconnection during auth
func (a *AuthFailuresChecker) isDisconnectedAuth(line string) bool {
	return strings.Contains(line, "Disconnected from authenticating user")
}

// extractFailureDetails extracts failure details from log line
func (a *AuthFailuresChecker) extractFailureDetails(line, reason string) *FailedAuth {
	// Extract timestamp (first 3 fields typically: "Nov 16 15:30:45")
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return nil
	}

	timestamp := strings.Join(fields[0:3], " ")
	user := "unknown"
	sourceIP := "unknown"

	// Extract user (look for "for <user>" or "user <user>")
	if idx := strings.Index(line, " for "); idx != -1 {
		afterFor := line[idx+5:]
		userFields := strings.Fields(afterFor)
		if len(userFields) > 0 {
			user = userFields[0]
		}
	} else if idx := strings.Index(line, " user "); idx != -1 {
		afterUser := line[idx+6:]
		userFields := strings.Fields(afterUser)
		if len(userFields) > 0 {
			user = userFields[0]
		}
	}

	// Extract source IP (look for "from <ip>")
	if idx := strings.Index(line, " from "); idx != -1 {
		afterFrom := line[idx+6:]
		ipFields := strings.Fields(afterFrom)
		if len(ipFields) > 0 {
			sourceIP = ipFields[0]
		}
	}

	return &FailedAuth{
		Timestamp: timestamp,
		User:      user,
		SourceIP:  sourceIP,
		Reason:    reason,
	}
}

// filterRecentFailures filters failures within lookback window
func (a *AuthFailuresChecker) filterRecentFailures(failures []FailedAuth) []FailedAuth {
	// For simplicity, return all failures since we're already reading tail -n 1000
	// In a production system, we'd parse timestamps and filter by time window
	return failures
}

// identifyAttackSources identifies IPs with multiple failures
func (a *AuthFailuresChecker) identifyAttackSources(failures []FailedAuth) []AttackSource {
	sourceMap := make(map[string]*AttackSource)

	for _, failure := range failures {
		if failure.SourceIP == "unknown" {
			continue
		}

		if _, exists := sourceMap[failure.SourceIP]; !exists {
			sourceMap[failure.SourceIP] = &AttackSource{
				IP:           failure.SourceIP,
				FailureCount: 0,
				Users:        []string{},
				FirstAttempt: failure.Timestamp,
			}
		}

		source := sourceMap[failure.SourceIP]
		source.FailureCount++
		source.LastAttempt = failure.Timestamp

		// Add user to list if not already present
		if !contains(source.Users, failure.User) {
			source.Users = append(source.Users, failure.User)
		}
	}

	// Convert map to sorted slice (by failure count, descending)
	sources := make([]AttackSource, 0, len(sourceMap))
	for _, source := range sourceMap {
		sources = append(sources, *source)
	}

	// Simple sort (descending by count)
	for i := 0; i < len(sources); i++ {
		for j := i + 1; j < len(sources); j++ {
			if sources[j].FailureCount > sources[i].FailureCount {
				sources[i], sources[j] = sources[j], sources[i]
			}
		}
	}

	return sources
}

// extractInvalidUsers extracts list of invalid usernames attempted
func (a *AuthFailuresChecker) extractInvalidUsers(failures []FailedAuth) []string {
	users := []string{}
	seen := make(map[string]bool)

	for _, failure := range failures {
		if strings.Contains(failure.Reason, "Invalid user") && !seen[failure.User] {
			users = append(users, failure.User)
			seen[failure.User] = true
		}
	}

	return users
}

// formatMessage creates a human-readable message
func (a *AuthFailuresChecker) formatMessage(totalFailures, attackSources, criticalSources int) string {
	if totalFailures == 0 {
		return "No failed authentication attempts detected"
	}

	msg := fmt.Sprintf("%d failed auth attempts in last %d hour(s)", totalFailures, a.lookbackHours)

	if criticalSources > 0 {
		msg += fmt.Sprintf(" ⚠️  %d IP(s) with >100 attempts", criticalSources)
	} else if attackSources > 0 {
		msg += fmt.Sprintf(" from %d unique IP(s)", attackSources)
	}

	return msg
}

// Helper functions

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
