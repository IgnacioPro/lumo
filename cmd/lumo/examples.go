package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var examplesCmd = &cobra.Command{
	Use:   "examples [example-number]",
	Short: "Show usage examples and tutorials",
	Long: `Display usage examples and tutorials for Lumo.

Without arguments, shows a list of all available examples.
With an example number, shows detailed information about that specific example.`,
	Example: `  # List all examples
  lumo examples

  # Show specific example
  lumo examples 1
  lumo examples 3

  # Show example by name
  lumo examples ssh
  lumo examples kubernetes`,
	RunE: runExamples,
}

func init() {
	rootCmd.AddCommand(examplesCmd)
}

type Example struct {
	Number      int
	Name        string
	Title       string
	Description string
	Commands    []string
	LearnMore   string
}

var allExamples = []Example{
	{
		Number:      1,
		Name:        "local",
		Title:       "Local Diagnostics",
		Description: "Run diagnostics on your local machine with various output formats.",
		Commands: []string{
			"# Run all checks",
			"lumo diagnose localhost",
			"",
			"# Specific checks only",
			"lumo diagnose localhost --checks cpu,memory,disk",
			"",
			"# JSON output for scripting",
			"lumo diagnose localhost --format json",
			"",
			"# TOON format (30-60% smaller for AI)",
			"lumo diagnose localhost --format toon",
		},
		LearnMore: "examples/01-local-diagnostics/README.md",
	},
	{
		Number:      2,
		Name:        "ssh",
		Title:       "SSH Remote Server",
		Description: "Diagnose remote servers via SSH with multiple authentication methods.",
		Commands: []string{
			"# Using SSH agent (recommended)",
			"lumo diagnose user@server.example.com --use-agent",
			"",
			"# Using private key",
			"lumo diagnose user@server.example.com --key ~/.ssh/id_rsa",
			"",
			"# Custom SSH port",
			"lumo diagnose user@server.example.com --port 2222 --use-agent",
			"",
			"# Multiple servers in parallel",
			"cat servers.txt | parallel 'lumo diagnose {} --use-agent'",
		},
		LearnMore: "examples/02-ssh-remote-server/README.md",
	},
	{
		Number:      3,
		Name:        "ai",
		Title:       "AI-Powered Analysis",
		Description: "Use AI to analyze diagnostics and get actionable insights.",
		Commands: []string{
			"# Analyze with default AI provider",
			"lumo diagnose localhost --analyze",
			"",
			"# Analyze remote server",
			"lumo diagnose user@server --use-agent --analyze",
			"",
			"# Use TOON format for 30-60% cost savings",
			"lumo diagnose localhost --format toon --analyze",
			"",
			"# Security-focused analysis",
			"lumo diagnose server --checks patch,ssh-security,ports --analyze",
		},
		LearnMore: "examples/03-ai-analysis/README.md",
	},
	{
		Number:      4,
		Name:        "fix",
		Title:       "Auto-Remediation",
		Description: "Automatically fix issues with human-in-the-loop approval.",
		Commands: []string{
			"# Dry-run to see what would be fixed",
			"lumo fix localhost --dry-run",
			"",
			"# Interactive approval for each fix",
			"lumo fix localhost",
			"",
			"# Auto-approve safe actions only",
			"lumo fix localhost --auto-approve-safe",
			"",
			"# Fix specific issue types",
			"lumo fix localhost --issues disk-space,services",
		},
		LearnMore: "examples/04-auto-remediation/README.md",
	},
	{
		Number:      5,
		Name:        "kubernetes",
		Title:       "Kubernetes Agent Deployment",
		Description: "Deploy Lumo agents to Kubernetes clusters for continuous monitoring.",
		Commands: []string{
			"# Deploy DaemonSet (per-node monitoring)",
			"kubectl apply -f deployments/kubernetes/base/daemonset.yaml",
			"",
			"# Deploy with Helm",
			"helm install lumo-agent deployments/kubernetes/helm/lumo-agent \\",
			"  --set apiEndpoint=http://lumo-api:8080 \\",
			"  --set apiKey=$LUMO_API_KEY",
			"",
			"# Check agent status",
			"kubectl get pods -l app=lumo-agent",
			"kubectl logs -l app=lumo-agent -f",
		},
		LearnMore: "examples/05-agent-deployment-k8s/README.md",
	},
	{
		Number:      6,
		Name:        "vm",
		Title:       "VM/Bare Metal Agent Deployment",
		Description: "Deploy Lumo agents on VMs and bare metal servers using systemd.",
		Commands: []string{
			"# Install agent",
			"curl -sSL https://raw.githubusercontent.com/ignacio/lumo/main/deployments/systemd/install.sh | sudo bash",
			"",
			"# Configure agent",
			"sudo vi /etc/lumo/agent-config.yaml",
			"",
			"# Start agent",
			"sudo systemctl enable --now lumo-agent",
			"",
			"# Check status",
			"sudo systemctl status lumo-agent",
			"journalctl -u lumo-agent -f",
		},
		LearnMore: "examples/06-agent-deployment-vms/README.md",
	},
}

func runExamples(cmd *cobra.Command, args []string) error {
	// If no arguments, show list of all examples
	if len(args) == 0 {
		showExamplesList()
		return nil
	}

	// Try to find example by number or name
	query := args[0]
	var example *Example

	// Try as number first
	for i := range allExamples {
		if fmt.Sprintf("%d", allExamples[i].Number) == query {
			example = &allExamples[i]
			break
		}
	}

	// Try as name if not found
	if example == nil {
		queryLower := strings.ToLower(query)
		for i := range allExamples {
			if strings.Contains(strings.ToLower(allExamples[i].Name), queryLower) ||
				strings.Contains(strings.ToLower(allExamples[i].Title), queryLower) {
				example = &allExamples[i]
				break
			}
		}
	}

	if example == nil {
		fmt.Printf("Example '%s' not found.\n\n", query)
		showExamplesList()
		return nil
	}

	showExampleDetail(*example)
	return nil
}

func showExamplesList() {
	fmt.Println()
	fmt.Println("Lumo Usage Examples")
	fmt.Println("===================")
	fmt.Println()
	fmt.Println("Run 'lumo examples <number>' to see detailed examples.")
	fmt.Println()

	for _, ex := range allExamples {
		fmt.Printf("%d. %s\n", ex.Number, ex.Title)
		fmt.Printf("   %s\n", ex.Description)
		fmt.Println()
	}

	fmt.Println("Quick Reference:")
	fmt.Println("----------------")
	fmt.Println("  lumo examples 1          # Show example 1 (local diagnostics)")
	fmt.Println("  lumo examples ssh        # Show SSH remote server examples")
	fmt.Println("  lumo examples ai         # Show AI analysis examples")
	fmt.Println()
	fmt.Println("Full Documentation:")
	fmt.Println("  https://github.com/ignacio/lumo/tree/main/examples")
	fmt.Println()
}

func showExampleDetail(ex Example) {
	fmt.Println()
	fmt.Printf("Example %d: %s\n", ex.Number, ex.Title)
	fmt.Println(strings.Repeat("=", len(fmt.Sprintf("Example %d: %s", ex.Number, ex.Title))))
	fmt.Println()
	fmt.Println(ex.Description)
	fmt.Println()

	if len(ex.Commands) > 0 {
		fmt.Println("Commands:")
		fmt.Println("---------")
		for _, cmd := range ex.Commands {
			if strings.HasPrefix(cmd, "#") || cmd == "" {
				fmt.Println(cmd)
			} else {
				fmt.Printf("  %s\n", cmd)
			}
		}
		fmt.Println()
	}

	fmt.Println("Learn More:")
	fmt.Printf("  Full documentation: %s\n", ex.LearnMore)
	fmt.Printf("  Online: https://github.com/ignacio/lumo/tree/main/%s\n", ex.LearnMore)
	fmt.Println()

	// Show navigation hints
	if ex.Number < len(allExamples) {
		fmt.Printf("Next: lumo examples %d\n", ex.Number+1)
	}
	fmt.Println("List: lumo examples")
	fmt.Println()
}
