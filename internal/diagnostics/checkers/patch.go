package checkers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// PatchChecker checks for available system updates and security patches
type PatchChecker struct{}

// NewPatchChecker creates a new patch status checker
func NewPatchChecker() *PatchChecker {
	return &PatchChecker{}
}

// Name returns the checker name
func (p *PatchChecker) Name() string {
	return "patch_status"
}

// Category returns the checker category
func (p *PatchChecker) Category() diagnostics.CheckCategory {
	return diagnostics.CategorySecurity
}

// Description returns the checker description
func (p *PatchChecker) Description() string {
	return "Checks for available system updates and security patches"
}

// RequiresRoot returns false as patch checks don't need root
func (p *PatchChecker) RequiresRoot() bool {
	return false
}

// Run executes the patch status check
func (p *PatchChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
	result := &diagnostics.CheckResult{
		Name:      p.Name(),
		Category:  p.Category(),
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
		Metrics:   []diagnostics.Metric{},
	}

	startTime := time.Now()

	// Detect OS and package manager
	osType, pkgManager, err := p.detectOS(ctx, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to detect OS: %w", err)
	}

	result.SetData("os_type", osType)
	result.SetData("package_manager", pkgManager)

	// Get available updates
	updates, securityUpdates, err := p.getAvailableUpdates(ctx, executor, pkgManager)
	if err != nil {
		// Don't fail the check, just report the issue
		result.SetData("error", fmt.Sprintf("Failed to check updates: %v", err))
		result.Message = fmt.Sprintf("Unable to check for updates: %v", err)
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.SetData("total_updates", len(updates))
	result.SetData("security_updates", len(securityUpdates))
	result.SetData("updates_list", updates)
	if len(securityUpdates) > 0 {
		result.SetData("security_updates_list", securityUpdates)
	}

	// Add metrics for threshold evaluation
	result.AddMetric(diagnostics.Metric{
		Name:          "security_updates_available",
		Value:         float64(len(securityUpdates)),
		Unit:          "count",
		Threshold:     1, // Any security update is a concern
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.AddMetric(diagnostics.Metric{
		Name:          "total_updates_available",
		Value:         float64(len(updates)),
		Unit:          "count",
		Threshold:     10, // Warning if more than 10 updates pending
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.Duration = time.Since(startTime)
	result.Message = p.formatMessage(len(updates), len(securityUpdates))

	return result, nil
}

// detectOS detects the operating system and package manager
func (p *PatchChecker) detectOS(ctx context.Context, executor diagnostics.CommandExecutor) (string, string, error) {
	// Try /etc/os-release first (most Linux distros)
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "cat /etc/os-release 2>/dev/null")
	if exitCode == 0 && stdout != "" {
		osType, pkgManager := p.parseOSRelease(stdout)
		if osType != "" && pkgManager != "" {
			return osType, pkgManager, nil
		}
	}

	// Fallback: try uname and detect package manager by binary presence
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "uname -s")
	if err != nil || exitCode != 0 {
		return "", "", fmt.Errorf("unable to detect OS")
	}

	osType := strings.TrimSpace(stdout)

	// Detect package manager
	pkgManager, err := p.detectPackageManager(ctx, executor)
	if err != nil {
		return osType, "", err
	}

	return osType, pkgManager, nil
}

// parseOSRelease parses /etc/os-release content
func (p *PatchChecker) parseOSRelease(content string) (string, string) {
	lines := strings.Split(content, "\n")
	osID := ""

	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			osID = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			break
		}
	}

	// Map OS ID to package manager
	pkgManagerMap := map[string]string{
		"ubuntu":    "apt",
		"debian":    "apt",
		"linuxmint": "apt",
		"pop":       "apt",
		"rhel":      "yum",
		"centos":    "yum",
		"fedora":    "dnf",
		"rocky":     "dnf",
		"alma":      "dnf",
		"alpine":    "apk",
		"arch":      "pacman",
		"manjaro":   "pacman",
	}

	pkgManager := pkgManagerMap[osID]
	return osID, pkgManager
}

// detectPackageManager detects the package manager by checking for binaries
func (p *PatchChecker) detectPackageManager(ctx context.Context, executor diagnostics.CommandExecutor) (string, error) {
	managers := []string{"apt", "dnf", "yum", "apk", "pacman", "brew"}

	for _, mgr := range managers {
		_, _, exitCode, _ := executor.ExecuteWithContext(ctx, fmt.Sprintf("which %s 2>/dev/null", mgr))
		if exitCode == 0 {
			return mgr, nil
		}
	}

	return "", fmt.Errorf("no supported package manager found")
}

// getAvailableUpdates retrieves available updates for the detected package manager
func (p *PatchChecker) getAvailableUpdates(ctx context.Context, executor diagnostics.CommandExecutor, pkgManager string) ([]string, []string, error) {
	switch pkgManager {
	case "apt":
		return p.getAptUpdates(ctx, executor)
	case "yum", "dnf":
		return p.getYumDnfUpdates(ctx, executor, pkgManager)
	case "apk":
		return p.getApkUpdates(ctx, executor)
	case "pacman":
		return p.getPacmanUpdates(ctx, executor)
	case "brew":
		return p.getBrewUpdates(ctx, executor)
	default:
		return nil, nil, fmt.Errorf("unsupported package manager: %s", pkgManager)
	}
}

// getAptUpdates gets updates for Debian/Ubuntu systems
func (p *PatchChecker) getAptUpdates(ctx context.Context, executor diagnostics.CommandExecutor) ([]string, []string, error) {
	// Update cache silently (best effort)
	executor.ExecuteWithContext(ctx, "apt-get update -qq 2>/dev/null")

	// Get upgradable packages
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "apt list --upgradable 2>/dev/null")
	if err != nil || exitCode != 0 {
		return nil, nil, fmt.Errorf("apt list failed: %w", err)
	}

	return p.parseAptOutput(stdout)
}

// parseAptOutput parses apt list --upgradable output
func (p *PatchChecker) parseAptOutput(output string) ([]string, []string, error) {
	lines := strings.Split(output, "\n")
	updates := []string{}
	securityUpdates := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Listing") {
			continue
		}

		// Extract package name (first field before /)
		fields := strings.Fields(line)
		if len(fields) > 0 {
			pkgName := strings.Split(fields[0], "/")[0]
			updates = append(updates, pkgName)

			// Check if it's a security update
			if strings.Contains(strings.ToLower(line), "security") {
				securityUpdates = append(securityUpdates, pkgName)
			}
		}
	}

	return updates, securityUpdates, nil
}

// getYumDnfUpdates gets updates for RHEL/CentOS/Fedora systems
func (p *PatchChecker) getYumDnfUpdates(ctx context.Context, executor diagnostics.CommandExecutor, pkgManager string) ([]string, []string, error) {
	cmd := fmt.Sprintf("%s check-update -q 2>/dev/null", pkgManager)
	stdout, _, _, _ := executor.ExecuteWithContext(ctx, cmd)

	return p.parseYumDnfOutput(stdout)
}

// parseYumDnfOutput parses yum/dnf check-update output
func (p *PatchChecker) parseYumDnfOutput(output string) ([]string, []string, error) {
	lines := strings.Split(output, "\n")
	updates := []string{}
	securityUpdates := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Last metadata") || strings.HasPrefix(line, "Security:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			pkgName := strings.Split(fields[0], ".")[0]
			updates = append(updates, pkgName)

			// Security updates typically have "security" in repository name
			if len(fields) >= 3 && strings.Contains(strings.ToLower(fields[2]), "security") {
				securityUpdates = append(securityUpdates, pkgName)
			}
		}
	}

	return updates, securityUpdates, nil
}

// getApkUpdates gets updates for Alpine Linux
func (p *PatchChecker) getApkUpdates(ctx context.Context, executor diagnostics.CommandExecutor) ([]string, []string, error) {
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "apk list -u 2>/dev/null")
	if err != nil || exitCode != 0 {
		return nil, nil, fmt.Errorf("apk list failed: %w", err)
	}

	return p.parseApkOutput(stdout)
}

// parseApkOutput parses apk list -u output
func (p *PatchChecker) parseApkOutput(output string) ([]string, []string, error) {
	lines := strings.Split(output, "\n")
	updates := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Package names are before the space and version
		fields := strings.Fields(line)
		if len(fields) > 0 {
			pkgName := strings.Split(fields[0], "-")[0]
			updates = append(updates, pkgName)
		}
	}

	// Alpine doesn't distinguish security updates easily
	return updates, []string{}, nil
}

// getPacmanUpdates gets updates for Arch Linux
func (p *PatchChecker) getPacmanUpdates(ctx context.Context, executor diagnostics.CommandExecutor) ([]string, []string, error) {
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "pacman -Qu 2>/dev/null")
	if err != nil || exitCode != 0 {
		return nil, nil, fmt.Errorf("pacman query failed: %w", err)
	}

	return p.parsePacmanOutput(stdout)
}

// parsePacmanOutput parses pacman -Qu output
func (p *PatchChecker) parsePacmanOutput(output string) ([]string, []string, error) {
	lines := strings.Split(output, "\n")
	updates := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) > 0 {
			updates = append(updates, fields[0])
		}
	}

	// Pacman doesn't distinguish security updates easily
	return updates, []string{}, nil
}

// getBrewUpdates gets updates for macOS Homebrew
func (p *PatchChecker) getBrewUpdates(ctx context.Context, executor diagnostics.CommandExecutor) ([]string, []string, error) {
	// Update Homebrew database (best effort, don't fail if it times out)
	executor.ExecuteWithContext(ctx, "brew update 2>/dev/null")

	// Get outdated packages
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "brew outdated 2>/dev/null")
	if err != nil || exitCode != 0 {
		return nil, nil, fmt.Errorf("brew outdated failed: %w", err)
	}

	return p.parseBrewOutput(stdout)
}

// parseBrewOutput parses brew outdated output
func (p *PatchChecker) parseBrewOutput(output string) ([]string, []string, error) {
	lines := strings.Split(output, "\n")
	updates := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Brew outdated format: "package (version) < new_version"
		// or just "package"
		fields := strings.Fields(line)
		if len(fields) > 0 {
			updates = append(updates, fields[0])
		}
	}

	// Homebrew doesn't distinguish security updates in standard output
	// but all updates should be applied for security
	return updates, []string{}, nil
}

// formatMessage creates a human-readable message from patch status
func (p *PatchChecker) formatMessage(totalUpdates, securityUpdates int) string {
	if totalUpdates == 0 {
		return "System is up-to-date, no pending updates"
	}

	if securityUpdates > 0 {
		return fmt.Sprintf("⚠️  %d security updates available (total: %d updates pending)", securityUpdates, totalUpdates)
	}

	return fmt.Sprintf("%d updates available (no security updates)", totalUpdates)
}
