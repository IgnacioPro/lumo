package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ignacio/lumo/internal/ai"
	"github.com/ignacio/lumo/internal/config"
	"github.com/ignacio/lumo/internal/diagnostics"
	"github.com/ignacio/lumo/internal/diagnostics/checkers"
	"github.com/ignacio/lumo/internal/diagnostics/formatters"
	"github.com/ignacio/lumo/internal/ssh"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose [[user@]host]",
	Short: "Run diagnostic commands on remote or local systems",
	Long: `Run diagnostic commands to check system health including:
  - CPU usage and load average
  - Memory and swap usage
  - Disk space and inode usage
  - Process counts, zombies, and resource consumers
  - Service status (systemd, init, launchd)
  - Network interfaces, connectivity, and DNS
  - Kubernetes cluster health (if enabled in config)

For remote hosts, connects via SSH. For localhost, runs commands directly
without SSH overhead. Kubernetes diagnostics use native client (no kubectl).

If no host is specified, defaults to localhost.

Examples:
  lumo diagnose                              # Run locally (defaults to localhost)
  lumo diagnose localhost                    # Run locally without SSH
  lumo diagnose user@example.com             # Remote server via SSH
  lumo diagnose root@192.168.1.10 --port 2222
  lumo diagnose admin@server --checks cpu,memory,disk
  lumo diagnose --analyze                    # Local with AI analysis
  lumo diagnose --checks kubernetes          # Kubernetes cluster diagnostics only`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runDiagnostics(cmd, args); err != nil {
			log.Errorf("Diagnostics failed: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(diagnoseCmd)

	// SSH connection flags
	diagnoseCmd.Flags().IntP("port", "p", 22, "SSH port")
	diagnoseCmd.Flags().StringP("identity", "i", "", "SSH private key file")
	// NOTE: Password flag removed for security - password prompt will be used if needed
	diagnoseCmd.Flags().DurationP("timeout", "t", 30*time.Second, "Connection timeout")

	// Diagnostic flags
	diagnoseCmd.Flags().StringSliceP("checks", "c", []string{}, "Specific checks to run (cpu, memory, disk, process, service, network, kubernetes)")
	diagnoseCmd.Flags().BoolP("all", "a", true, "Run all available diagnostic checks")
	diagnoseCmd.Flags().StringP("format", "f", "text", "Output format (text, json, toon)")
	diagnoseCmd.Flags().BoolP("no-color", "n", false, "Disable colored output")

	// AI analysis flags
	diagnoseCmd.Flags().BoolP("analyze", "A", false, "Enable AI-powered analysis (requires AI provider configuration)")
	diagnoseCmd.Flags().StringSlice("focus", []string{}, "Focus AI analysis on specific areas (cpu, memory, disk, etc.)")
}

func runDiagnostics(cmd *cobra.Command, args []string) error {
	// Default to localhost if no host argument provided
	hostArg := "localhost"
	if len(args) > 0 {
		hostArg = args[0]
	}

	// Parse host argument (supports user@host format)
	var username, hostname string
	if strings.Contains(hostArg, "@") {
		parts := strings.SplitN(hostArg, "@", 2)
		username = parts[0]
		hostname = parts[1]
	} else {
		hostname = hostArg
	}

	// Get flags
	port, _ := cmd.Flags().GetInt("port")
	identityFile, _ := cmd.Flags().GetString("identity")
	checksFilter, _ := cmd.Flags().GetStringSlice("checks")
	format, _ := cmd.Flags().GetString("format")
	noColor, _ := cmd.Flags().GetBool("no-color")
	enableAI, _ := cmd.Flags().GetBool("analyze")
	focusAreas, _ := cmd.Flags().GetStringSlice("focus")

	// Default to current user if not specified
	if username == "" {
		currentUser, err := user.Current()
		if err != nil {
			return fmt.Errorf("failed to get current user: %w", err)
		}
		username = currentUser.Username
	}

	// Check if this is a localhost execution (no SSH needed)
	isLocal := isLocalhost(hostname)

	if isLocal {
		log.Info("Running diagnostics locally (no SSH connection needed)")
	} else {
		log.Infof("Starting diagnostics for %s@%s", username, hostname)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Warnf("Failed to load config, using defaults: %v", err)
		cfg = config.DefaultConfig()
	}

	// Create command executor (local or SSH)
	var executor diagnostics.CommandExecutor
	if isLocal {
		// Use local executor - no SSH needed
		executor = diagnostics.NewLocalExecutor()
		log.Debug("Using local command executor")
	} else {
		// Create SSH client configuration
		sshClientConfig := ssh.NewClientConfig(cfg.SSH)

		// Override with command-line flags
		if identityFile != "" {
			if err := sshClientConfig.SetKeyPath(identityFile); err != nil {
				return fmt.Errorf("invalid key path: %w", err)
			}
		}

		// Password authentication will use secure prompting via SSH auth methods
		// No password flag for security - prevents exposure in process lists and history

		// Create SSH client
		log.Debug("Creating SSH client")
		sshClient, err := ssh.NewClient(sshClientConfig, log)
		if err != nil {
			return fmt.Errorf("failed to create SSH client: %w", err)
		}

		// Connect
		log.Infof("Connecting to %s@%s:%d", username, hostname, port)
		if err := sshClient.Connect(hostname, port, username); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer sshClient.Disconnect()

		log.Info("Connected successfully")

		// Create SSH executor
		executor = diagnostics.NewSSHExecutor(sshClient)
	}

	// Create diagnostic runner
	diagConfig := diagnostics.DefaultConfig()
	// If specific checks are requested, use only those (overrides --all)
	if len(checksFilter) > 0 {
		diagConfig.EnabledChecks = checksFilter
	}

	thresholds := diagnostics.DefaultThresholds()
	runner := diagnostics.NewRunner(diagConfig, thresholds, executor, log)

	// Register checkers (Phase 5 Complete: 6 core + 4 security + 2 specialized checkers)
	log.Debug("Registering diagnostic checkers")

	// Build checker list dynamically
	checkersToRegister := []diagnostics.Checker{
		// Core system checkers
		checkers.NewCPUChecker(thresholds.CPU),
		checkers.NewMemoryChecker(thresholds.Memory),
		checkers.NewDiskChecker(thresholds.Disk),
		checkers.NewProcessChecker(thresholds.Process),
		checkers.NewServiceChecker([]string{}), // Empty list = check all services
		checkers.NewNetworkChecker(thresholds.Network, cfg.Diagnostics.Network.Targets),
		// Security checkers (Phase 5)
		checkers.NewPatchChecker(),
		checkers.NewPortsChecker(cfg.Diagnostics.Security.PortCheck.WhitelistedPorts),
		checkers.NewSSHSecurityChecker(),
		checkers.NewAuthFailuresChecker(
			cfg.Diagnostics.Security.AuthFailureCheck.LookbackHours,
			cfg.Diagnostics.Security.AuthFailureCheck.FailureThreshold,
		),
		// Proxmox virtualization checker (always registered, auto-skips if not installed)
		checkers.NewProxmoxChecker(false, false, false, false, false, false, false), // All checks enabled
	}

	// Add Kubernetes checker if enabled in config
	if cfg.Diagnostics.Kubernetes.Enabled {
		log.Debug("Kubernetes diagnostics enabled")
		checkersToRegister = append(checkersToRegister, checkers.NewKubernetesChecker(cfg.Diagnostics.Kubernetes, log))
	}

	runner.RegisterCheckers(checkersToRegister...)

	// Run diagnostics
	log.Info("Running diagnostic checks...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	report, err := runner.RunAll(ctx)
	if err != nil {
		return fmt.Errorf("diagnostic run failed: %w", err)
	}

	// Run AI analysis if requested
	var analysis *ai.AnalysisResponse
	if enableAI {
		log.Info("Running AI-powered analysis...")

		analysis, err = runAIAnalysis(cfg, report, hostname, checksFilter, focusAreas)
		if err != nil {
			log.Warnf("AI analysis failed: %v", err)
			// Continue without AI analysis rather than failing completely
		} else {
			log.Infof("AI analysis complete (provider: %s, model: %s, duration: %v)",
				analysis.Provider, analysis.Model, analysis.Duration)
		}
	}

	// Format and display results
	switch format {
	case "json":
		jsonOutput, err := report.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to format JSON: %w", err)
		}
		fmt.Println(jsonOutput)

		// Display AI analysis separately for JSON format
		if analysis != nil {
			fmt.Println("\n--- AI ANALYSIS ---")
			analysisJSON, err := formatAIAnalysisJSON(analysis)
			if err != nil {
				log.Warnf("Failed to format AI analysis: %v", err)
			} else {
				fmt.Println(analysisJSON)
			}
		}

	case "toon":
		// TOON format - optimized for LLM consumption (30-60% token reduction vs JSON)
		toonFormatter := formatters.NewToonFormatter()
		output := toonFormatter.FormatReport(report)
		fmt.Println(output)

		// Display AI analysis in TOON format too
		if analysis != nil {
			fmt.Println("\n--- AI ANALYSIS ---")
			// For AI analysis, use JSON since it's not a uniform data structure
			analysisJSON, err := formatAIAnalysisJSON(analysis)
			if err != nil {
				log.Warnf("Failed to format AI analysis: %v", err)
			} else {
				fmt.Println(analysisJSON)
			}
		}

	default:
		// Text format (default)
		formatter := formatters.NewTextFormatter(!noColor, verbose)
		output := formatter.FormatReport(report)
		fmt.Println(output)

		// Display AI analysis
		if analysis != nil {
			fmt.Println("\n" + formatAIAnalysisText(analysis, !noColor))
		}
	}

	// Exit with error code if critical issues found
	if report.HasCriticalIssues() {
		log.Warn("Critical issues detected")
		return nil // Don't return error, just log warning
	}

	log.Info("Diagnostics completed successfully")
	return nil
}

// runAIAnalysis performs AI-powered analysis of diagnostic results.
func runAIAnalysis(cfg *config.Config, report *diagnostics.Report, hostname string, selectedChecks []string, focusAreas []string) (*ai.AnalysisResponse, error) {
	// Parse provider type
	providerType, err := ai.ParseProviderType(cfg.AI.Provider)
	if err != nil {
		return nil, fmt.Errorf("invalid AI provider: %w", err)
	}

	// Build provider config
	providerConfig := &ai.ProviderConfig{
		Name:        string(providerType),
		APIKey:      cfg.AI.GetAPIKeyForProvider(cfg.AI.Provider),
		Model:       cfg.AI.GetModelForProvider(cfg.AI.Provider),
		Endpoint:    cfg.AI.Endpoint,
		Timeout:     cfg.AI.Timeout,
		MaxRetries:  cfg.AI.MaxRetries,
		Temperature: cfg.AI.Temperature,
		MaxTokens:   cfg.AI.MaxTokens,
	}

	// Create provider
	provider, err := ai.NewProvider(providerType, providerConfig, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI provider: %w", err)
	}

	// Check provider health
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := provider.Health(ctx); err != nil {
		return nil, fmt.Errorf("AI provider health check failed: %w", err)
	}

	// Build analysis request
	req := &ai.AnalysisRequest{
		Report: report,
		SystemInfo: ai.SystemInfo{
			Hostname: hostname,
		},
		SelectedChecks: selectedChecks,
		Focus:          focusAreas,
	}

	// Run analysis
	analysisCtx, analysisCancel := context.WithTimeout(context.Background(), cfg.AI.Timeout)
	defer analysisCancel()

	return provider.Analyze(analysisCtx, req)
}

// formatAIAnalysisText formats AI analysis for human-readable text output.
func formatAIAnalysisText(analysis *ai.AnalysisResponse, color bool) string {
	var sb strings.Builder

	// Header
	sb.WriteString("╔════════════════════════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                           AI-POWERED ANALYSIS                                  ║\n")
	sb.WriteString("╚════════════════════════════════════════════════════════════════════════════════╝\n\n")

	// Summary
	sb.WriteString(fmt.Sprintf("📊 Overall Health: %s\n", formatHealthStatus(analysis.OverallHealth, color)))
	sb.WriteString(fmt.Sprintf("🎯 Confidence: %.0f%%\n", analysis.Confidence*100))
	sb.WriteString(fmt.Sprintf("🤖 Provider: %s (%s)\n", analysis.Provider, analysis.Model))
	sb.WriteString(fmt.Sprintf("⏱️  Duration: %v\n", analysis.Duration))
	if analysis.TokensUsed != nil {
		sb.WriteString(fmt.Sprintf("💬 Tokens: %d\n", analysis.TokensUsed.TotalTokens))
	}
	sb.WriteString("\n")

	// Summary text
	sb.WriteString("📝 SUMMARY\n")
	sb.WriteString("─────────────────────────────────────────────────────────────────────────────────\n")
	sb.WriteString(analysis.Summary)
	sb.WriteString("\n\n")

	// Findings
	if len(analysis.Findings) > 0 {
		sb.WriteString(fmt.Sprintf("🔍 FINDINGS (%d)\n", len(analysis.Findings)))
		sb.WriteString("─────────────────────────────────────────────────────────────────────────────────\n")
		for i, finding := range analysis.Findings {
			sb.WriteString(fmt.Sprintf("\n%d. [%s] %s\n", i+1, formatSeverity(finding.Severity, color), finding.Title))
			sb.WriteString(fmt.Sprintf("   Category: %s\n", finding.Category))
			sb.WriteString(fmt.Sprintf("   %s\n", finding.Description))
		}
		sb.WriteString("\n")
	}

	// Recommendations
	if len(analysis.Recommendations) > 0 {
		sb.WriteString(fmt.Sprintf("💡 RECOMMENDATIONS (%d)\n", len(analysis.Recommendations)))
		sb.WriteString("─────────────────────────────────────────────────────────────────────────────────\n")
		for i, rec := range analysis.Recommendations {
			sb.WriteString(fmt.Sprintf("\n%d. [%s] %s\n", i+1, formatPriority(rec.Priority, color), rec.Title))
			sb.WriteString(fmt.Sprintf("   Risk: %s\n", formatRisk(rec.Risk, color)))
			sb.WriteString(fmt.Sprintf("   %s\n", rec.Description))

			if len(rec.Commands) > 0 {
				sb.WriteString("   Commands:\n")
				for _, cmd := range rec.Commands {
					sb.WriteString(fmt.Sprintf("     $ %s\n", cmd))
				}
			}

			if rec.EstimatedImpact != "" {
				sb.WriteString(fmt.Sprintf("   Impact: %s\n", rec.EstimatedImpact))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("─────────────────────────────────────────────────────────────────────────────────\n")

	return sb.String()
}

// formatAIAnalysisJSON formats AI analysis as JSON.
func formatAIAnalysisJSON(analysis *ai.AnalysisResponse) (string, error) {
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Helper formatting functions

func formatHealthStatus(status ai.HealthStatus, color bool) string {
	if !color {
		return string(status)
	}

	switch status {
	case ai.HealthHealthy:
		return "\033[32m✓ HEALTHY\033[0m"
	case ai.HealthDegraded:
		return "\033[33m⚠ DEGRADED\033[0m"
	case ai.HealthCritical:
		return "\033[31m✗ CRITICAL\033[0m"
	default:
		return "\033[90m? UNKNOWN\033[0m"
	}
}

func formatSeverity(severity diagnostics.Severity, color bool) string {
	if !color {
		return string(severity)
	}

	switch severity {
	case diagnostics.SeverityInfo:
		return "\033[36mINFO\033[0m"
	case diagnostics.SeverityWarning:
		return "\033[33mWARN\033[0m"
	case diagnostics.SeverityError:
		return "\033[31mERROR\033[0m"
	case diagnostics.SeverityCritical:
		return "\033[1;31mCRITICAL\033[0m"
	default:
		return string(severity)
	}
}

func formatPriority(priority ai.Priority, color bool) string {
	if !color {
		return string(priority)
	}

	switch priority {
	case ai.PriorityCritical:
		return "\033[1;31mCRITICAL\033[0m"
	case ai.PriorityHigh:
		return "\033[31mHIGH\033[0m"
	case ai.PriorityMedium:
		return "\033[33mMEDIUM\033[0m"
	case ai.PriorityLow:
		return "\033[32mLOW\033[0m"
	default:
		return string(priority)
	}
}

func formatRisk(risk ai.RiskLevel, color bool) string {
	if !color {
		return string(risk)
	}

	switch risk {
	case ai.RiskSafe:
		return "\033[32mSAFE\033[0m"
	case ai.RiskLow:
		return "\033[36mLOW\033[0m"
	case ai.RiskModerate:
		return "\033[33mMODERATE\033[0m"
	case ai.RiskHigh:
		return "\033[31mHIGH\033[0m"
	case ai.RiskCritical:
		return "\033[1;31mCRITICAL\033[0m"
	default:
		return string(risk)
	}
}

// isLocalhost checks if the given hostname refers to the local machine.
func isLocalhost(hostname string) bool {
	// Normalize hostname to lowercase for comparison
	hostname = strings.ToLower(strings.TrimSpace(hostname))

	// Common localhost patterns
	localhostPatterns := []string{
		"localhost",
		"127.0.0.1",
		"::1",     // IPv6 localhost
		"0.0.0.0", // All interfaces (treated as local)
		"",        // Empty hostname (treated as local)
		"localhost.localdomain",
	}

	for _, pattern := range localhostPatterns {
		if hostname == pattern {
			return true
		}
	}

	return false
}
