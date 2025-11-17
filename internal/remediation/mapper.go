package remediation

import (
	"fmt"
	"time"
	"github.com/ignacio/lumo/internal/diagnostics"
)

// Mapper converts diagnostic results into remediation actions
type Mapper struct{}

// NewMapper creates a new remediation action mapper
func NewMapper() *Mapper {
	return &Mapper{}
}

// MapFromDiagnostics converts a diagnostic report into a remediation plan
func (m *Mapper) MapFromDiagnostics(report *diagnostics.Report, hostname, username string) *Plan {
	plan := &Plan{
		ID:       generateID(),
		Hostname: hostname,
		Username: username,
		Actions:  []*Action{},
		CreatedAt: time.Now(),
	}

	// Process each check result and generate actions
	for _, result := range report.Results {
		actions := m.mapCheckResult(result)
		plan.Actions = append(plan.Actions, actions...)
	}

	// Calculate summary statistics
	plan.TotalActions = len(plan.Actions)
	for _, action := range plan.Actions {
		switch action.Risk {
		case RiskSafe:
			plan.SafeActions++
		case RiskModerate:
			plan.ModerateActions++
		case RiskCritical:
			plan.CriticalActions++
		}
		plan.EstimatedDuration += action.EstimatedDuration
	}

	return plan
}

// mapCheckResult converts a single diagnostic check result into remediation actions
func (m *Mapper) mapCheckResult(result *diagnostics.CheckResult) []*Action {
	var actions []*Action

	switch result.Name {
	case "service":
		actions = m.mapServiceCheck(result)
	case "process":
		actions = m.mapProcessCheck(result)
	case "disk":
		actions = m.mapDiskCheck(result)
	case "memory":
		actions = m.mapMemoryCheck(result)
	case "cpu":
		actions = m.mapCPUCheck(result)
	case "patch_status":
		actions = m.mapPatchCheck(result)
	case "open_ports":
		actions = m.mapPortsCheck(result)
	case "ssh_security":
		actions = m.mapSSHSecurityCheck(result)
	case "auth_failures":
		actions = m.mapAuthFailuresCheck(result)
	}

	return actions
}

// mapServiceCheck creates actions for failed services
func (m *Mapper) mapServiceCheck(result *diagnostics.CheckResult) []*Action {
	var actions []*Action

	if result.Severity == diagnostics.SeverityOK {
		return actions
	}

	// Extract failed services from the message
	// Format: "Failed services: service1 (reason), service2 (reason)"
	// This is a simplified parser - in production you'd want structured data
	if result.Severity == diagnostics.SeverityError || result.Severity == diagnostics.SeverityWarning {
		// Service restart is always safe and recoverable
		action := &Action{
			ID:                 generateID(),
			Type:               ActionRestartService,
			Risk:               RiskSafe,
			Title:              "Restart Failed Service",
			Description:        fmt.Sprintf("Restart service that is in failed state: %s", result.Message),
			SourceCheck:        result.Name,
			Metric:             result.Message,
			Commands:           []string{"systemctl restart <service-name>"},
			VerifyCommands:     []string{"systemctl is-active <service-name>"},
			RollbackCommands:   []string{},
			EstimatedDuration: 10 * time.Second,
			Metadata: map[string]string{
				"service_status": "failed",
			},
		}
		actions = append(actions, action)
	}

	return actions
}

// mapProcessCheck creates actions for problematic processes
func (m *Mapper) mapProcessCheck(result *diagnostics.CheckResult) []*Action {
	var actions []*Action

	if result.Severity == diagnostics.SeverityOK {
		return actions
	}

	// High zombie process count can be remediated by reaping them
	if result.Severity == diagnostics.SeverityWarning {
		action := &Action{
			ID:                 generateID(),
			Type:               ActionKillProcess,
			Risk:               RiskModerate,
			Title:              "Kill Zombie Processes",
			Description:        fmt.Sprintf("Clean up zombie processes: %s", result.Message),
			SourceCheck:        result.Name,
			Metric:             result.Message,
			Commands:           []string{"ps aux | grep -E '<defunct>' | awk '{print $2}' | xargs kill -9 2>/dev/null || true"},
			VerifyCommands:     []string{"ps aux | grep -c '<defunct>' || true"},
			RollbackCommands:   []string{},
			EstimatedDuration: 5 * time.Second,
		}
		actions = append(actions, action)
	}

	return actions
}

// mapDiskCheck creates actions for disk space issues
func (m *Mapper) mapDiskCheck(result *diagnostics.CheckResult) []*Action {
	var actions []*Action

	if result.Severity == diagnostics.SeverityOK {
		return actions
	}

	// Disk space warnings can be remediated by cleaning cache/logs
	if result.Severity == diagnostics.SeverityWarning || result.Severity == diagnostics.SeverityError {
		// Clean cache (safe operation)
		cacheAction := &Action{
			ID:                 generateID(),
			Type:               ActionCleanCache,
			Risk:               RiskSafe,
			Title:              "Clean Package Manager Cache",
			Description:        fmt.Sprintf("Free up disk space by cleaning package manager cache: %s", result.Message),
			SourceCheck:        result.Name,
			Metric:             result.Message,
			Commands:           []string{"apt-get clean && apt-get autoclean || yum clean all || dnf clean all || pacman -Sc --noconfirm || true"},
			VerifyCommands:     []string{"df -h | head -2"},
			RollbackCommands:   []string{},
			EstimatedDuration: 15 * time.Second,
		}
		actions = append(actions, cacheAction)

		// Clean logs (moderate - might break log analysis if done improperly)
		if result.Severity == diagnostics.SeverityError {
			logsAction := &Action{
				ID:                 generateID(),
				Type:               ActionCleanLogs,
				Risk:               RiskModerate,
				Title:              "Rotate and Archive Old Logs",
				Description:        fmt.Sprintf("Archive logs older than 30 days to free space: %s", result.Message),
				SourceCheck:        result.Name,
				Metric:             result.Message,
				Commands:           []string{"find /var/log -type f -mtime +30 -exec gzip {} \\; || true"},
				VerifyCommands:     []string{"df -h | head -2"},
				RollbackCommands:   []string{},
				EstimatedDuration: 30 * time.Second,
			}
			actions = append(actions, logsAction)
		}
	}

	return actions
}

// mapMemoryCheck creates actions for memory issues
func (m *Mapper) mapMemoryCheck(result *diagnostics.CheckResult) []*Action {
	var actions []*Action

	if result.Severity == diagnostics.SeverityOK {
		return actions
	}

	// High memory pressure warnings
	if result.Severity == diagnostics.SeverityWarning || result.Severity == diagnostics.SeverityError {
		action := &Action{
			ID:                 generateID(),
			Type:               ActionThrottleProcess,
			Risk:               RiskModerate,
			Title:              "Investigate Memory-Heavy Processes",
			Description:        fmt.Sprintf("Monitor and potentially restart memory-heavy processes: %s", result.Message),
			SourceCheck:        result.Name,
			Metric:             result.Message,
			Commands:           []string{"sync && echo 3 > /proc/sys/vm/drop_caches || true"},
			VerifyCommands:     []string{"free -h | head -2"},
			RollbackCommands:   []string{},
			EstimatedDuration: 5 * time.Second,
			Metadata: map[string]string{
				"note": "Clears page cache only, safe operation",
			},
		}
		actions = append(actions, action)
	}

	return actions
}

// mapCPUCheck creates actions for CPU issues (limited remediation possible)
func (m *Mapper) mapCPUCheck(result *diagnostics.CheckResult) []*Action {
	// High CPU usually requires investigation, not automated remediation
	// Could suggest nice/renice for processes, but this is informational
	return []*Action{}
}

// mapPatchCheck creates actions for missing security patches
func (m *Mapper) mapPatchCheck(result *diagnostics.CheckResult) []*Action {
	var actions []*Action

	if result.Severity == diagnostics.SeverityOK {
		return actions
	}

	// Security patches available
	if result.Severity == diagnostics.SeverityWarning || result.Severity == diagnostics.SeverityError {
		action := &Action{
			ID:                 generateID(),
			Type:               ActionApplySecurityPatch,
			Risk:               RiskCritical, // Security patches are critical
			Title:              "Apply Security Patches",
			Description:        fmt.Sprintf("Install available security patches: %s", result.Message),
			SourceCheck:        result.Name,
			Metric:             result.Message,
			Commands:           []string{"apt-get update && apt-get upgrade -y || yum update -y || dnf upgrade -y || pacman -Syu --noconfirm || true"},
			VerifyCommands:     []string{"apt list --upgradable || yum list updates || dnf list upgrades || pacman -Qu || true"},
			RollbackCommands:   []string{}, // Can't easily rollback patches
			EstimatedDuration: 5 * time.Minute,
			Metadata: map[string]string{
				"requires_approval": "always",
				"may_require_reboot": "true",
			},
		}
		actions = append(actions, action)
	}

	return actions
}

// mapPortsCheck creates actions for unexpected open ports
func (m *Mapper) mapPortsCheck(result *diagnostics.CheckResult) []*Action {
	// Unexpected ports should be investigated, not automatically closed
	// This is more of a security alert than automated remediation
	return []*Action{}
}

// mapSSHSecurityCheck creates actions for SSH configuration issues
func (m *Mapper) mapSSHSecurityCheck(result *diagnostics.CheckResult) []*Action {
	var actions []*Action

	if result.Severity == diagnostics.SeverityOK {
		return actions
	}

	// SSH key permission issues can be auto-fixed (safe)
	if result.Severity == diagnostics.SeverityWarning {
		action := &Action{
			ID:                 generateID(),
			Type:               ActionRotateSSHKeys,
			Risk:               RiskModerate,
			Title:              "Fix SSH Key Permissions",
			Description:        fmt.Sprintf("Fix insecure SSH key file permissions: %s", result.Message),
			SourceCheck:        result.Name,
			Metric:             result.Message,
			Commands:           []string{"find ~/.ssh -type f -name 'id_*' -exec chmod 600 {} \\;"},
			VerifyCommands:     []string{"ls -la ~/.ssh/id_* || true"},
			RollbackCommands:   []string{},
			EstimatedDuration: 2 * time.Second,
		}
		actions = append(actions, action)
	}

	return actions
}

// mapAuthFailuresCheck creates actions for authentication failures
func (m *Mapper) mapAuthFailuresCheck(result *diagnostics.CheckResult) []*Action {
	var actions []*Action

	if result.Severity == diagnostics.SeverityOK {
		return actions
	}

	// Auth failures usually require investigation
	// Could suggest blocking IPs, but that's critical and needs approval
	if result.Severity == diagnostics.SeverityError {
		action := &Action{
			ID:                 generateID(),
			Type:               ActionBlockIP,
			Risk:               RiskCritical,
			Title:              "Block Suspicious IPs",
			Description:        fmt.Sprintf("Block IPs with repeated failed authentication attempts: %s", result.Message),
			SourceCheck:        result.Name,
			Metric:             result.Message,
			Commands:           []string{"# Manual firewall rule required - needs specific IP address"},
			VerifyCommands:     []string{"iptables -L -n | grep <ip>"},
			RollbackCommands:   []string{"iptables -D INPUT -s <ip> -j DROP"},
			EstimatedDuration: 5 * time.Second,
			Metadata: map[string]string{
				"requires_manual_ips": "true",
			},
		}
		actions = append(actions, action)
	}

	return actions
}

// generateID creates a unique identifier
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
