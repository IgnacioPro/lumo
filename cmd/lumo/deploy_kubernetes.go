package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ignacio/lumo/internal/deploy/helm"
	"github.com/ignacio/lumo/internal/deploy/kubernetes"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// newDeployKubernetesCmd creates the Kubernetes deployment command
func newDeployKubernetesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubernetes",
		Short: "Deploy Lumo to Kubernetes cluster",
		Long: `Deploy the full Lumo stack to a Kubernetes cluster.

By default, deploys:
  - PostgreSQL database
  - Redis cache
  - Lumo API server
  - Lumo event-driven agent

Use --kind for local testing with automatic Kind cluster creation.

Examples:
  # Production deployment
  lumo deploy kubernetes \
    --namespace production \
    --db-password $SECURE_PASSWORD \
    --api-jwt-secret $JWT_SECRET \
    --agent-token $AGENT_TOKEN \
    --anthropic-key $ANTHROPIC_KEY \
    --agent-replicas 3

  # Local testing with Kind
  lumo deploy kubernetes --kind \
    --db-password testpass \
    --api-jwt-secret testsecret \
    --agent-token testtoken \
    --anthropic-key $ANTHROPIC_KEY

  # Agent-only deployment (infrastructure exists)
  lumo deploy kubernetes --agent-only \
    --db-host postgres.database.svc.cluster.local \
    --redis-host redis.cache.svc.cluster.local \
    --agent-token $TOKEN \
    --api-jwt-secret $JWT_SECRET

  # Dry run to see what would be deployed
  lumo deploy kubernetes --kind --dry-run
`,
		RunE: runDeployKubernetes,
	}

	// Cluster configuration
	cmd.Flags().String("kubeconfig", "", "Path to kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")
	cmd.Flags().String("context", "", "Kubernetes context to use")
	cmd.Flags().StringP("namespace", "n", "lumo-system", "Namespace to deploy into")

	// Component selection
	cmd.Flags().Bool("agent-only", false, "Deploy only the agent (requires existing infrastructure)")
	cmd.Flags().Bool("skip-agent", false, "Skip agent deployment (infrastructure only)")

	// Kind testing
	cmd.Flags().Bool("kind", false, "Create Kind cluster for local testing")
	cmd.Flags().String("kind-cluster-name", "lumo-test", "Kind cluster name")

	// Database configuration
	cmd.Flags().String("db-password", "", "PostgreSQL password (required, or set LUMO_DB_PASSWORD)")
	cmd.Flags().String("db-host", "", "External PostgreSQL host (if using managed database)")
	cmd.Flags().Int("db-port", 5432, "External PostgreSQL port")

	// Redis configuration
	cmd.Flags().String("redis-host", "", "External Redis host (if using managed Redis)")
	cmd.Flags().Int("redis-port", 6379, "External Redis port")

	// API server configuration
	cmd.Flags().String("api-jwt-secret", "", "JWT secret for API server (required, or set LUMO_API_JWT_SECRET)")
	cmd.Flags().String("api-image", "ghcr.io/ignacio/lumo-api:latest", "API server image")

	// Agent configuration
	cmd.Flags().String("agent-token", "", "Agent authentication token (required, or set LUMO_AGENT_TOKEN)")
	cmd.Flags().String("agent-image", "ghcr.io/ignacio/lumo-agent:latest", "Agent image")
	cmd.Flags().Int("agent-replicas", 2, "Number of agent replicas for HA")

	// AI provider configuration
	cmd.Flags().String("anthropic-key", "", "Anthropic API key (or set LUMO_ANTHROPIC_API_KEY)")
	cmd.Flags().String("openai-key", "", "OpenAI API key (or set LUMO_OPENAI_API_KEY)")
	cmd.Flags().String("gemini-key", "", "Gemini API key (or set LUMO_GEMINI_API_KEY)")
	cmd.Flags().String("ai-provider", "anthropic", "AI provider (anthropic|openai|gemini|ollama|openrouter)")

	// Deployment options
	cmd.Flags().Duration("timeout", 10*time.Minute, "Deployment timeout")
	cmd.Flags().Bool("wait", true, "Wait for deployment to be ready")
	cmd.Flags().Bool("dry-run", false, "Show what would be deployed without applying")
	cmd.Flags().BoolP("verbose", "v", false, "Verbose output")

	// Helm options
	cmd.Flags().String("helm-values", "", "Path to custom Helm values file")
	cmd.Flags().StringArray("set", []string{}, "Set Helm values (--set key=value)")

	// Bind environment variables
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		envVar := ""
		switch f.Name {
		case "db-password":
			envVar = "LUMO_DB_PASSWORD"
		case "api-jwt-secret":
			envVar = "LUMO_API_JWT_SECRET"
		case "agent-token":
			envVar = "LUMO_AGENT_TOKEN"
		case "anthropic-key":
			envVar = "LUMO_ANTHROPIC_API_KEY"
		case "openai-key":
			envVar = "LUMO_OPENAI_API_KEY"
		case "gemini-key":
			envVar = "LUMO_GEMINI_API_KEY"
		}

		if envVar != "" {
			if val := os.Getenv(envVar); val != "" && !cmd.Flags().Changed(f.Name) {
				cmd.Flags().Set(f.Name, val)
			}
		}
	})

	return cmd
}

func runDeployKubernetes(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Initialize logger
	logger := logrus.New()
	if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Build configuration from flags
	config, err := buildDeploymentConfig(cmd)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	logger.Info("========================================")
	logger.Info("  Lumo Kubernetes Deployment")
	logger.Info("========================================")
	logger.Infof("Namespace: %s", config.Namespace)
	logger.Infof("AI Provider: %s", config.AIProvider)
	if config.UseKind {
		logger.Infof("Kind Cluster: %s", config.KindCluster)
	}
	logger.Info("========================================")

	// Step 1: Kind setup (if requested)
	if config.UseKind {
		logger.Info("Note: Kind integration not yet implemented")
		logger.Info("Please create Kind cluster manually and use --kubeconfig flag")
		// TODO: Implement kind cluster creation
		// if err := setupKindCluster(ctx, config, logger); err != nil {
		// 	return fmt.Errorf("kind cluster setup: %w", err)
		// }
	}

	// Step 2: Initialize Kubernetes client
	logger.Info("Initializing Kubernetes client...")
	k8sClient, err := kubernetes.NewClient(config.Kubeconfig, config.Context, logger)
	if err != nil {
		return fmt.Errorf("kubernetes client: %w", err)
	}

	// Step 3: Create namespace
	logger.Infof("Ensuring namespace %s exists...", config.Namespace)
	if err := k8sClient.EnsureNamespace(ctx, config.Namespace); err != nil {
		return fmt.Errorf("namespace creation: %w", err)
	}

	// Step 4: Initialize Helm client
	logger.Info("Initializing Helm client...")
	helmClient, err := helm.NewClient(config.Namespace, config.Kubeconfig, logger)
	if err != nil {
		return fmt.Errorf("helm client: %w", err)
	}

	// Step 5: Generate Helm values
	logger.Info("Generating Helm values...")
	values, err := helm.GenerateHelmValues(config)
	if err != nil {
		return fmt.Errorf("generate values: %w", err)
	}

	// Step 6: Dry run check
	if config.DryRun {
		logger.Info("========================================")
		logger.Info("  DRY RUN MODE")
		logger.Info("========================================")
		logger.Info("Would deploy with the following configuration:")
		logger.Infof("Chart: deployments/kubernetes/helm/lumo-stack")
		logger.Infof("Release: lumo")
		logger.Infof("Namespace: %s", config.Namespace)
		logger.Info("Values:")
		for k, v := range values {
			logger.Infof("  %s: %v", k, v)
		}
		logger.Info("========================================")
		logger.Info("Dry run complete. No changes were made.")
		return nil
	}

	// Step 7: Deploy Helm chart
	chartPath := "deployments/kubernetes/helm/lumo-stack"
	releaseName := "lumo"

	logger.Info("========================================")
	logger.Infof("Deploying Lumo stack to namespace %s...", config.Namespace)
	logger.Info("========================================")

	release, err := helmClient.UpgradeOrInstall(chartPath, releaseName, values, config.Timeout)
	if err != nil {
		return fmt.Errorf("helm deployment: %w", err)
	}

	logger.Infof("✓ Helm release %s deployed (version %d)", release.Name, release.Version)

	// Step 8: Wait for components to be ready (if --wait)
	if config.Wait {
		logger.Info("Waiting for components to be ready...")
		if err := k8sClient.WaitForComponentsReady(ctx, config.Namespace, config.Timeout); err != nil {
			logger.Warnf("⚠ Some components may not be ready: %v", err)
			logger.Info("Deployment completed but some health checks failed")
			logger.Info("Check component logs for details:")
			logger.Infof("  kubectl logs -f -n %s -l app=lumo-api", config.Namespace)
			logger.Infof("  kubectl logs -f -n %s -l app.kubernetes.io/component=agent", config.Namespace)
		} else {
			logger.Info("✓ All components are healthy")
		}
	}

	// Step 9: Bootstrap database (API keys, system agent)
	if !config.AgentOnly && config.DBHost == "" {
		logger.Info("Bootstrapping database...")
		if err := k8sClient.BootstrapDatabase(ctx, config.Namespace, config.AgentToken); err != nil {
			logger.Warnf("⚠ Database bootstrap failed: %v", err)
			logger.Info("You may need to manually create API keys and system agent")
		} else {
			logger.Info("✓ Database bootstrapped successfully")
		}
	}

	// Step 10: Print summary
	printDeploymentSummary(config, logger)

	return nil
}

func buildDeploymentConfig(cmd *cobra.Command) (*helm.DeploymentConfig, error) {
	// Helper to get flag values
	getString := func(name string) string {
		v, _ := cmd.Flags().GetString(name)
		return v
	}
	getInt := func(name string) int {
		v, _ := cmd.Flags().GetInt(name)
		return v
	}
	getBool := func(name string) bool {
		v, _ := cmd.Flags().GetBool(name)
		return v
	}
	getDuration := func(name string) time.Duration {
		v, _ := cmd.Flags().GetDuration(name)
		return v
	}
	getStringArray := func(name string) []string {
		v, _ := cmd.Flags().GetStringArray(name)
		return v
	}

	return &helm.DeploymentConfig{
		// Cluster access
		Kubeconfig: getString("kubeconfig"),
		Context:    getString("context"),
		Namespace:  getString("namespace"),

		// Component selection
		AgentOnly: getBool("agent-only"),
		SkipAgent: getBool("skip-agent"),

		// Kind testing
		UseKind:     getBool("kind"),
		KindCluster: getString("kind-cluster-name"),

		// Database config
		DBPassword: getString("db-password"),
		DBHost:     getString("db-host"),
		DBPort:     getInt("db-port"),

		// Redis config
		RedisHost: getString("redis-host"),
		RedisPort: getInt("redis-port"),

		// API config
		APIJWTSecret: getString("api-jwt-secret"),
		APIImage:     getString("api-image"),

		// Agent config
		AgentToken:    getString("agent-token"),
		AgentImage:    getString("agent-image"),
		AgentReplicas: getInt("agent-replicas"),

		// AI provider
		AIProvider:   getString("ai-provider"),
		AnthropicKey: getString("anthropic-key"),
		OpenAIKey:    getString("openai-key"),
		GeminiKey:    getString("gemini-key"),

		// Deployment options
		Timeout: getDuration("timeout"),
		Wait:    getBool("wait"),
		DryRun:  getBool("dry-run"),
		Verbose: getBool("verbose"),

		// Helm
		HelmValues: getString("helm-values"),
		HelmSets:   getStringArray("set"),
	}, nil
}

func printDeploymentSummary(config *helm.DeploymentConfig, logger *logrus.Logger) {
	logger.Info("")
	logger.Info("========================================")
	logger.Info("  Deployment Complete!")
	logger.Info("========================================")
	logger.Info("")
	logger.Info("Deployed Components:")
	if config.DBHost == "" {
		logger.Info("  ✓ PostgreSQL (embedded)")
	} else {
		logger.Infof("  ✓ PostgreSQL (external: %s:%d)", config.DBHost, config.DBPort)
	}
	if config.RedisHost == "" {
		logger.Info("  ✓ Redis (embedded)")
	} else {
		logger.Infof("  ✓ Redis (external: %s:%d)", config.RedisHost, config.RedisPort)
	}
	if !config.SkipAgent {
		logger.Info("  ✓ Lumo API Server")
	}
	if !config.AgentOnly {
		logger.Infof("  ✓ Lumo Agent (%d replicas)", config.AgentReplicas)
	}
	logger.Info("")
	logger.Info("Next Steps:")
	logger.Infof("  1. Check deployment: kubectl get pods -n %s", config.Namespace)
	logger.Infof("  2. View API logs: kubectl logs -f -n %s -l app=lumo-api", config.Namespace)
	logger.Infof("  3. View agent logs: kubectl logs -f -n %s -l app.kubernetes.io/component=agent", config.Namespace)
	logger.Infof("  4. Access API: kubectl port-forward -n %s svc/lumo-api 8080:8080", config.Namespace)
	logger.Info("")
	logger.Info("For more information, visit: https://github.com/ignacio/lumo")
	logger.Info("========================================")
}
