package remediation

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/sirupsen/logrus"
)

// Suggester generates remediation suggestions from diagnostic results.
type Suggester struct {
	registry *Registry
	logger   *logrus.Logger
}

// NewSuggester creates a new remediation suggester.
func NewSuggester(registry *Registry, logger *logrus.Logger) *Suggester {
	return &Suggester{
		registry: registry,
		logger:   logger,
	}
}

// SuggestFromReport analyzes a diagnostic report and suggests remediation actions.
func (s *Suggester) SuggestFromReport(report *diagnostics.Report) (*RemediationPlan, error) {
	plan := NewRemediationPlan(s.logger)

	s.logger.WithField("results_count", len(report.Results)).Debug("Analyzing diagnostic report for remediation suggestions")

	for _, result := range report.Results {
		suggestions := s.suggestForCheckResult(result)
		for _, action := range suggestions {
			plan.AddAction(action)
		}
	}

	s.logger.WithField("suggested_actions", len(plan.Actions)).Info("Generated remediation plan")

	return plan, nil
}

// suggestForCheckResult suggests actions for a single check result.
func (s *Suggester) suggestForCheckResult(result *diagnostics.CheckResult) []Action {
	var suggestions []Action

	// Only suggest remediations for warning and critical severity
	if result.Severity != diagnostics.SeverityWarning && result.Severity != diagnostics.SeverityCritical {
		return suggestions
	}

	s.logger.WithFields(logrus.Fields{
		"check_name": result.Name,
		"severity":   result.Severity,
	}).Debug("Analyzing check result for remediation suggestions")

	// Route to specific suggestion functions based on check name
	switch result.Name {
	case "service_check":
		suggestions = append(suggestions, s.suggestServiceActions(result)...)
	case "disk_check":
		suggestions = append(suggestions, s.suggestDiskActions(result)...)
	case "process_check":
		suggestions = append(suggestions, s.suggestProcessActions(result)...)
	case "memory_check":
		suggestions = append(suggestions, s.suggestMemoryActions(result)...)
	default:
		s.logger.WithField("check_name", result.Name).Debug("No specific remediation suggestions for this check")
	}

	return suggestions
}

// suggestServiceActions suggests actions for service check failures.
func (s *Suggester) suggestServiceActions(result *diagnostics.CheckResult) []Action {
	var suggestions []Action

	// Parse the check message for failed services
	// Expected format: "Failed services: service1, service2"
	if strings.Contains(result.Message, "Failed services:") {
		parts := strings.Split(result.Message, ":")
		if len(parts) > 1 {
			serviceNames := strings.Split(parts[1], ",")
			for _, serviceName := range serviceNames {
				serviceName = strings.TrimSpace(serviceName)
				if serviceName == "" {
					continue
				}

				// Suggest restarting the failed service using registry
				action, err := s.registry.Create("service.restart", map[string]interface{}{
					"service_name": serviceName,
				})
				if err != nil {
					s.logger.WithError(err).Warn("Failed to create restart service action")
					continue
				}

				suggestions = append(suggestions, action)
				s.logger.WithField("service", serviceName).Debug("Suggested restart for failed service")
			}
		}
	}

	return suggestions
}

// suggestDiskActions suggests actions for disk space issues.
func (s *Suggester) suggestDiskActions(result *diagnostics.CheckResult) []Action {
	var suggestions []Action

	// Check if it's a disk space issue
	if !strings.Contains(result.Message, "usage") && !strings.Contains(result.Message, "disk") {
		return suggestions
	}

	// Extract usage percentage if available
	var usagePercent float64
	if strings.Contains(result.Message, "%") {
		// Try to parse percentage from message
		parts := strings.Fields(result.Message)
		for _, part := range parts {
			if strings.HasSuffix(part, "%") {
				percentStr := strings.TrimSuffix(part, "%")
				if val, err := strconv.ParseFloat(percentStr, 64); err == nil {
					usagePercent = val
					break
				}
			}
		}
	}

	// Suggest cleanup actions based on severity
	if usagePercent > 80 || result.Severity == diagnostics.SeverityCritical {
		// High disk usage - suggest aggressive cleanup

		// Clean old log files
		action, _ := s.registry.Create("disk.clean_logs", map[string]interface{}{
			"older_than_days": 30,
		})
		if action != nil {
			suggestions = append(suggestions, action)
			s.logger.Debug("Suggested cleaning old log files")
		}

		// Clean temp files
		action, _ = s.registry.Create("disk.clean_temp", map[string]interface{}{
			"older_than_days": 7,
		})
		if action != nil {
			suggestions = append(suggestions, action)
			s.logger.Debug("Suggested cleaning temp files")
		}

		// Clean APT cache (if available)
		action, _ = s.registry.Create("disk.clean_apt_cache", nil)
		if action != nil {
			suggestions = append(suggestions, action)
			s.logger.Debug("Suggested cleaning APT cache")
		}

		// Clean user cache
		action, _ = s.registry.Create("disk.clean_cache", nil)
		if action != nil {
			suggestions = append(suggestions, action)
			s.logger.Debug("Suggested cleaning user cache")
		}
	} else if usagePercent > 70 || result.Severity == diagnostics.SeverityWarning {
		// Moderate disk usage - suggest conservative cleanup

		// Clean old log files (more conservative)
		action, _ := s.registry.Create("disk.clean_logs", map[string]interface{}{
			"older_than_days": 60,
		})
		if action != nil {
			suggestions = append(suggestions, action)
			s.logger.Debug("Suggested cleaning very old log files")
		}

		// Clean temp files
		action, _ = s.registry.Create("disk.clean_temp", map[string]interface{}{
			"older_than_days": 14,
		})
		if action != nil {
			suggestions = append(suggestions, action)
			s.logger.Debug("Suggested cleaning old temp files")
		}
	}

	return suggestions
}

// suggestProcessActions suggests actions for process-related issues.
func (s *Suggester) suggestProcessActions(result *diagnostics.CheckResult) []Action {
	var suggestions []Action

	// Check for zombie processes
	if strings.Contains(strings.ToLower(result.Message), "zombie") {
		// Parse zombie process count
		var zombieCount int
		if strings.Contains(result.Message, "zombies") {
			parts := strings.Fields(result.Message)
			for i, part := range parts {
				if part == "zombies" && i > 0 {
					if count, err := strconv.Atoi(parts[i-1]); err == nil {
						zombieCount = count
						break
					}
				}
			}
		}

		if zombieCount > 10 {
			s.logger.WithField("zombie_count", zombieCount).Warn("Many zombie processes detected, but automatic remediation is risky")
			// Note: We don't automatically suggest killing zombies as it requires killing the parent
			// This would need manual intervention or a more sophisticated approach
		}
	}

	// Check for high process count
	if strings.Contains(strings.ToLower(result.Message), "high process count") {
		s.logger.Debug("High process count detected, but automatic remediation requires manual analysis")
		// Note: High process count typically requires manual investigation
		// to determine which processes can be safely killed
	}

	return suggestions
}

// suggestMemoryActions suggests actions for memory-related issues.
func (s *Suggester) suggestMemoryActions(result *diagnostics.CheckResult) []Action {
	var suggestions []Action

	// Check if it's a memory pressure issue
	if !strings.Contains(strings.ToLower(result.Message), "memory") {
		return suggestions
	}

	// For memory issues, we can suggest cleaning caches to free memory
	if result.Severity == diagnostics.SeverityCritical {
		action, _ := s.registry.Create("disk.clean_cache", nil)
		if action != nil {
			suggestions = append(suggestions, action)
			s.logger.Debug("Suggested cleaning cache to free memory")
		}
	}

	// Note: Killing high-memory processes is risky and requires manual intervention
	// We could add this in the future with more sophisticated analysis

	return suggestions
}

// SuggestForSeverity suggests generic actions based on overall severity.
func (s *Suggester) SuggestForSeverity(report *diagnostics.Report, minSeverity diagnostics.Severity) []Action {
	var suggestions []Action

	// Count issues by severity
	severityCounts := make(map[diagnostics.Severity]int)
	for _, result := range report.Results {
		severityCounts[result.Severity]++
	}

	s.logger.WithFields(logrus.Fields{
		"critical": severityCounts[diagnostics.SeverityCritical],
		"warning":  severityCounts[diagnostics.SeverityWarning],
		"ok":       severityCounts[diagnostics.SeverityOK],
	}).Debug("Severity distribution in report")

	// If there are many critical issues, suggest aggressive cleanup
	if severityCounts[diagnostics.SeverityCritical] >= 3 {
		s.logger.Info("Multiple critical issues detected, suggesting comprehensive cleanup")

		// Suggest multiple cleanup actions
		action, _ := s.registry.Create("disk.clean_logs", map[string]interface{}{
			"older_than_days": 30,
		})
		if action != nil {
			suggestions = append(suggestions, action)
		}

		action, _ = s.registry.Create("disk.clean_temp", map[string]interface{}{
			"older_than_days": 7,
		})
		if action != nil {
			suggestions = append(suggestions, action)
		}
	}

	return suggestions
}

// ExplainSuggestion generates a human-readable explanation for why an action was suggested.
func ExplainSuggestion(action Action, checkResult *diagnostics.CheckResult) string {
	explanation := fmt.Sprintf(
		"Action '%s' was suggested because:\n"+
			"  Check: %s\n"+
			"  Severity: %s\n"+
			"  Issue: %s\n"+
			"  Expected Impact: %s",
		action.Name(),
		checkResult.Name,
		checkResult.Severity,
		checkResult.Message,
		action.EstimateImpact(),
	)
	return explanation
}
