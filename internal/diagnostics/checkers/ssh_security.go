package checkers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// SSHSecurityChecker checks SSH configuration and key permissions
type SSHSecurityChecker struct{}

// NewSSHSecurityChecker creates a new SSH security checker
func NewSSHSecurityChecker() *SSHSecurityChecker {
	return &SSHSecurityChecker{}
}

// Name returns the checker name
func (s *SSHSecurityChecker) Name() string {
	return "ssh_security"
}

// Category returns the checker category
func (s *SSHSecurityChecker) Category() diagnostics.CheckCategory {
	return diagnostics.CategorySecurity
}

// Description returns the checker description
func (s *SSHSecurityChecker) Description() string {
	return "Checks SSH key permissions and sshd configuration security"
}

// RequiresRoot returns false for basic checks
func (s *SSHSecurityChecker) RequiresRoot() bool {
	return false
}

// SSHKeyIssue represents a security issue with SSH keys
type SSHKeyIssue struct {
	FilePath    string `json:"file_path"`
	Permissions string `json:"permissions"`
	Issue       string `json:"issue"`
	Severity    string `json:"severity"` // "critical", "warning", "info"
}

// SSHDConfigIssue represents a security issue in sshd_config
type SSHDConfigIssue struct {
	Setting          string `json:"setting"`
	CurrentValue     string `json:"current_value"`
	RecommendedValue string `json:"recommended_value"`
	Issue            string `json:"issue"`
	Severity         string `json:"severity"`
}

// Run executes the SSH security check
func (s *SSHSecurityChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
	result := diagnostics.NewCheckResult(s)
	startTime := time.Now()

	// Check SSH key permissions
	keyIssues, err := s.checkSSHKeyPermissions(ctx, executor)
	if err != nil {
		// Log warning but don't fail the check
		result.SetData("key_check_error", fmt.Sprintf("Failed to check SSH keys: %v", err))
	} else {
		result.SetData("ssh_key_issues", keyIssues)
		result.SetData("ssh_key_issues_count", len(keyIssues))
	}

	// Check sshd_config
	configIssues, err := s.checkSSHDConfig(ctx, executor)
	if err != nil {
		// Log warning but don't fail the check
		result.SetData("config_check_error", fmt.Sprintf("Failed to check sshd_config: %v", err))
	} else {
		result.SetData("sshd_config_issues", configIssues)
		result.SetData("sshd_config_issues_count", len(configIssues))
	}

	// Check SSH version
	sshVersion, err := s.getSSHVersion(ctx, executor)
	if err == nil {
		result.SetData("ssh_version", sshVersion)
	}

	// Count critical vs warning issues
	criticalCount := s.countBySeverity(keyIssues, configIssues, "critical")
	warningCount := s.countBySeverity(keyIssues, configIssues, "warning")

	// Add metrics
	result.AddMetric(diagnostics.Metric{
		Name:          "ssh_critical_issues",
		Value:         float64(criticalCount),
		Unit:          "count",
		Threshold:     1, // Any critical issue is a concern
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.AddMetric(diagnostics.Metric{
		Name:          "ssh_warning_issues",
		Value:         float64(warningCount),
		Unit:          "count",
		Threshold:     3, // Warning if more than 3 warnings
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.Duration = time.Since(startTime)
	result.Message = s.formatMessage(criticalCount, warningCount, len(keyIssues)+len(configIssues))

	return result, nil
}

// checkSSHKeyPermissions checks permissions on SSH keys in ~/.ssh
func (s *SSHSecurityChecker) checkSSHKeyPermissions(ctx context.Context, executor diagnostics.CommandExecutor) ([]SSHKeyIssue, error) {
	issues := []SSHKeyIssue{}

	// Get home directory
	homeDir, _, exitCode, err := executor.ExecuteWithContext(ctx, "echo $HOME")
	if err != nil || exitCode != 0 {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	homeDir = strings.TrimSpace(homeDir)

	// List SSH directory with permissions
	sshDir := fmt.Sprintf("%s/.ssh", homeDir)
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, fmt.Sprintf("ls -la %s 2>/dev/null", sshDir))
	if err != nil || exitCode != 0 {
		// SSH directory might not exist, not an error
		return []SSHKeyIssue{}, nil
	}

	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		permissions := fields[0]
		fileName := fields[8]
		filePath := fmt.Sprintf("%s/%s", sshDir, fileName)

		// Check .ssh directory permissions first (before skipping)
		if fileName == "." {
			issue := s.checkSSHDirPermissions(permissions, sshDir)
			if issue != nil {
				issues = append(issues, *issue)
			}
			continue
		}

		// Skip .. entry
		if fileName == ".." {
			continue
		}

		// Check private keys (files without .pub extension)
		if !strings.HasSuffix(fileName, ".pub") && !strings.HasPrefix(fileName, "known_hosts") && !strings.HasPrefix(fileName, "config") && !strings.HasPrefix(fileName, "authorized_keys") {
			issue := s.checkPrivateKeyPermissions(permissions, filePath)
			if issue != nil {
				issues = append(issues, *issue)
			}
		}

		// Check public keys
		if strings.HasSuffix(fileName, ".pub") {
			issue := s.checkPublicKeyPermissions(permissions, filePath)
			if issue != nil {
				issues = append(issues, *issue)
			}
		}
	}

	return issues, nil
}

// checkPrivateKeyPermissions checks if private key has secure permissions
func (s *SSHSecurityChecker) checkPrivateKeyPermissions(permissions, filePath string) *SSHKeyIssue {
	// Private keys should be 600 (rw-------) or 400 (r--------)
	octal := s.permissionsToOctal(permissions)

	if octal > 600 {
		return &SSHKeyIssue{
			FilePath:    filePath,
			Permissions: permissions,
			Issue:       fmt.Sprintf("Private key has insecure permissions (%s). Should be 600 or 400.", permissions),
			Severity:    "critical",
		}
	}

	// Group or others have any permission
	if permissions[4] != '-' || permissions[5] != '-' || permissions[6] != '-' || permissions[7] != '-' || permissions[8] != '-' || permissions[9] != '-' {
		return &SSHKeyIssue{
			FilePath:    filePath,
			Permissions: permissions,
			Issue:       "Private key readable/writable by group or others",
			Severity:    "critical",
		}
	}

	return nil
}

// checkPublicKeyPermissions checks if public key has appropriate permissions
func (s *SSHSecurityChecker) checkPublicKeyPermissions(permissions, filePath string) *SSHKeyIssue {
	// Public keys should be 644 (rw-r--r--) or more restrictive
	octal := s.permissionsToOctal(permissions)

	if octal > 644 {
		return &SSHKeyIssue{
			FilePath:    filePath,
			Permissions: permissions,
			Issue:       fmt.Sprintf("Public key has overly permissive settings (%s). Should be 644 or less.", permissions),
			Severity:    "warning",
		}
	}

	return nil
}

// checkSSHDirPermissions checks if .ssh directory has secure permissions
func (s *SSHSecurityChecker) checkSSHDirPermissions(permissions, dirPath string) *SSHKeyIssue {
	// .ssh directory should be 700 (rwx------)
	octal := s.permissionsToOctal(permissions)

	if octal > 700 {
		return &SSHKeyIssue{
			FilePath:    dirPath,
			Permissions: permissions,
			Issue:       fmt.Sprintf(".ssh directory has insecure permissions (%s). Should be 700.", permissions),
			Severity:    "warning",
		}
	}

	return nil
}

// permissionsToOctal converts permission string to octal value
func (s *SSHSecurityChecker) permissionsToOctal(permissions string) int {
	if len(permissions) < 10 {
		return 0
	}

	// Skip first character (file type)
	perms := permissions[1:]

	octal := 0
	// User permissions
	if perms[0] == 'r' {
		octal += 400
	}
	if perms[1] == 'w' {
		octal += 200
	}
	if perms[2] == 'x' || perms[2] == 's' {
		octal += 100
	}
	// Group permissions
	if perms[3] == 'r' {
		octal += 40
	}
	if perms[4] == 'w' {
		octal += 20
	}
	if perms[5] == 'x' || perms[5] == 's' {
		octal += 10
	}
	// Others permissions
	if perms[6] == 'r' {
		octal += 4
	}
	if perms[7] == 'w' {
		octal += 2
	}
	if perms[8] == 'x' || perms[8] == 't' {
		octal += 1
	}

	return octal
}

// checkSSHDConfig checks sshd_config for security issues
func (s *SSHSecurityChecker) checkSSHDConfig(ctx context.Context, executor diagnostics.CommandExecutor) ([]SSHDConfigIssue, error) {
	issues := []SSHDConfigIssue{}

	// Try to read sshd_config
	configPaths := []string{"/etc/ssh/sshd_config", "/etc/sshd_config"}
	var configContent string

	for _, path := range configPaths {
		stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, fmt.Sprintf("cat %s 2>/dev/null", path))
		if exitCode == 0 && stdout != "" {
			configContent = stdout
			break
		}
	}

	if configContent == "" {
		return nil, fmt.Errorf("could not read sshd_config")
	}

	// Parse config and check settings
	config := s.parseSSHDConfig(configContent)

	// Check PermitRootLogin
	if value, exists := config["PermitRootLogin"]; exists {
		if value != "no" && value != "prohibit-password" && value != "without-password" {
			issues = append(issues, SSHDConfigIssue{
				Setting:          "PermitRootLogin",
				CurrentValue:     value,
				RecommendedValue: "no or prohibit-password",
				Issue:            "Root login should be disabled or restricted to key-based auth only",
				Severity:         "critical",
			})
		}
	}

	// Check PasswordAuthentication
	if value, exists := config["PasswordAuthentication"]; exists {
		if value == "yes" {
			issues = append(issues, SSHDConfigIssue{
				Setting:          "PasswordAuthentication",
				CurrentValue:     value,
				RecommendedValue: "no",
				Issue:            "Password authentication should be disabled, use key-based auth",
				Severity:         "warning",
			})
		}
	}

	// Check PermitEmptyPasswords
	if value, exists := config["PermitEmptyPasswords"]; exists {
		if value == "yes" {
			issues = append(issues, SSHDConfigIssue{
				Setting:          "PermitEmptyPasswords",
				CurrentValue:     value,
				RecommendedValue: "no",
				Issue:            "Empty passwords should never be permitted",
				Severity:         "critical",
			})
		}
	}

	// Check X11Forwarding (should be disabled on servers)
	if value, exists := config["X11Forwarding"]; exists {
		if value == "yes" {
			issues = append(issues, SSHDConfigIssue{
				Setting:          "X11Forwarding",
				CurrentValue:     value,
				RecommendedValue: "no",
				Issue:            "X11Forwarding should be disabled on production servers",
				Severity:         "warning",
			})
		}
	}

	// Check PubkeyAuthentication
	if value, exists := config["PubkeyAuthentication"]; exists {
		if value == "no" {
			issues = append(issues, SSHDConfigIssue{
				Setting:          "PubkeyAuthentication",
				CurrentValue:     value,
				RecommendedValue: "yes",
				Issue:            "Public key authentication should be enabled",
				Severity:         "warning",
			})
		}
	}

	return issues, nil
}

// parseSSHDConfig parses sshd_config content into a map
func (s *SSHSecurityChecker) parseSSHDConfig(content string) map[string]string {
	config := make(map[string]string)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on whitespace
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			key := fields[0]
			value := strings.ToLower(fields[1])
			config[key] = value
		}
	}

	return config
}

// getSSHVersion gets the SSH server version
func (s *SSHSecurityChecker) getSSHVersion(ctx context.Context, executor diagnostics.CommandExecutor) (string, error) {
	stdout, stderr, exitCode, _ := executor.ExecuteWithContext(ctx, "ssh -V 2>&1")
	if exitCode == 0 {
		// SSH version is in stderr
		version := strings.TrimSpace(stdout + stderr)
		return version, nil
	}

	return "", fmt.Errorf("failed to get SSH version")
}

// countBySeverity counts issues by severity level
func (s *SSHSecurityChecker) countBySeverity(keyIssues []SSHKeyIssue, configIssues []SSHDConfigIssue, severity string) int {
	count := 0

	for _, issue := range keyIssues {
		if issue.Severity == severity {
			count++
		}
	}

	for _, issue := range configIssues {
		if issue.Severity == severity {
			count++
		}
	}

	return count
}

// formatMessage creates a human-readable message
func (s *SSHSecurityChecker) formatMessage(criticalCount, warningCount, totalIssues int) string {
	if totalIssues == 0 {
		return "SSH configuration is secure"
	}

	msg := fmt.Sprintf("%d SSH security issues found", totalIssues)

	details := []string{}
	if criticalCount > 0 {
		details = append(details, fmt.Sprintf("%d critical", criticalCount))
	}
	if warningCount > 0 {
		details = append(details, fmt.Sprintf("%d warnings", warningCount))
	}

	if len(details) > 0 {
		msg += fmt.Sprintf(" (%s)", strings.Join(details, ", "))
	}

	return msg
}
