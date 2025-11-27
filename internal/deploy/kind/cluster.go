package kind

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
)

// ClusterConfig holds the configuration for creating a Kind cluster
type ClusterConfig struct {
	Name              string
	KubernetesVersion string
	NumWorkers        int
	WaitTimeout       time.Duration
}

// Client wraps Kind operations
type Client struct {
	provider *cluster.Provider
	logger   *logrus.Logger
}

// NewClient creates a new Kind client
func NewClient(logger *logrus.Logger) *Client {
	return &Client{
		provider: cluster.NewProvider(
			cluster.ProviderWithLogger(cmd.NewLogger()),
		),
		logger: logger,
	}
}

// CheckPrerequisites verifies that Docker and kind are available
func (c *Client) CheckPrerequisites(ctx context.Context) error {
	c.logger.Info("Checking prerequisites...")

	// Check for Docker
	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		return fmt.Errorf("docker is not running or not installed: %w", err)
	}

	// Check for kind binary
	if _, err := exec.LookPath("kind"); err != nil {
		return fmt.Errorf("kind binary not found in PATH (install from https://kind.sigs.k8s.io/): %w", err)
	}

	c.logger.Info("✓ Prerequisites satisfied (Docker and kind available)")
	return nil
}

// ClusterExists checks if a Kind cluster with the given name exists
func (c *Client) ClusterExists(name string) (bool, error) {
	clusters, err := c.provider.List()
	if err != nil {
		return false, fmt.Errorf("failed to list clusters: %w", err)
	}

	for _, cluster := range clusters {
		if cluster == name {
			return true, nil
		}
	}

	return false, nil
}

// CreateCluster creates a new Kind cluster with the specified configuration
func (c *Client) CreateCluster(ctx context.Context, config ClusterConfig) error {
	c.logger.Infof("Creating Kind cluster '%s'...", config.Name)

	// Check if cluster already exists
	exists, err := c.ClusterExists(config.Name)
	if err != nil {
		return fmt.Errorf("failed to check if cluster exists: %w", err)
	}

	if exists {
		c.logger.Infof("✓ Cluster '%s' already exists", config.Name)
		return nil
	}

	// Generate Kind cluster config
	kindConfig, err := c.generateKindConfig(config)
	if err != nil {
		return fmt.Errorf("failed to generate Kind config: %w", err)
	}

	// Write config to temporary file
	tmpDir := os.TempDir()
	configPath := filepath.Join(tmpDir, fmt.Sprintf("kind-config-%s.yaml", config.Name))
	if err := os.WriteFile(configPath, []byte(kindConfig), 0644); err != nil {
		return fmt.Errorf("failed to write Kind config: %w", err)
	}
	defer func() {
		_ = os.Remove(configPath) // Best effort cleanup
	}()

	c.logger.Debugf("Kind config written to: %s", configPath)

	// Create cluster using kind binary (more reliable than the Go API for complex configs)
	createCmd := exec.CommandContext(ctx,
		"kind", "create", "cluster",
		"--name", config.Name,
		"--config", configPath,
		"--wait", config.WaitTimeout.String(),
	)

	createCmd.Stdout = c.logger.Writer()
	createCmd.Stderr = c.logger.Writer()

	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create Kind cluster: %w", err)
	}

	c.logger.Infof("✓ Cluster '%s' created successfully", config.Name)

	// Verify nodes are ready
	if err := c.waitForNodes(ctx, config); err != nil {
		return fmt.Errorf("failed waiting for nodes: %w", err)
	}

	return nil
}

// generateKindConfig creates the YAML configuration for a Kind cluster
func (c *Client) generateKindConfig(config ClusterConfig) (string, error) {
	k8sVersion := config.KubernetesVersion
	if k8sVersion == "" {
		k8sVersion = "v1.28.0"
	}

	// Ensure version has 'v' prefix
	if !strings.HasPrefix(k8sVersion, "v") {
		k8sVersion = "v" + k8sVersion
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: %s
nodes:
  # Control plane node
  - role: control-plane
    image: kindest/node:%s
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "lumo.io/monitor=true"
`, config.Name, k8sVersion))

	// Add worker nodes
	for i := 0; i < config.NumWorkers; i++ {
		sb.WriteString(fmt.Sprintf(`
  - role: worker
    image: kindest/node:%s
    kubeadmConfigPatches:
      - |
        kind: JoinConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "lumo.io/monitor=true"
`, k8sVersion))
	}

	// Add networking configuration
	sb.WriteString(`
# Networking configuration
networking:
  apiServerAddress: "127.0.0.1"
  apiServerPort: 6443
  podSubnet: "10.244.0.0/16"
  serviceSubnet: "10.96.0.0/12"
`)

	return sb.String(), nil
}

// waitForNodes waits for all nodes in the cluster to be ready
func (c *Client) waitForNodes(ctx context.Context, config ClusterConfig) error {
	c.logger.Info("Waiting for nodes to be ready...")

	// Use kubectl wait command
	waitCmd := exec.CommandContext(ctx,
		"kubectl", "wait",
		"--for=condition=Ready",
		"nodes", "--all",
		"--timeout=5m",
	)

	if output, err := waitCmd.CombinedOutput(); err != nil {
		c.logger.Debugf("kubectl wait output: %s", string(output))
		return fmt.Errorf("nodes not ready: %w", err)
	}

	c.logger.Info("✓ All nodes are ready")
	return nil
}

// BuildAndLoadImages builds Docker images and loads them into the Kind cluster
func (c *Client) BuildAndLoadImages(ctx context.Context, clusterName string) error {
	c.logger.Info("Building Docker images (API + Agent)...")

	// Find project root (go up from internal/deploy/kind)
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to find project root: %w", err)
	}

	// Build Agent image
	c.logger.Info("Building lumo-agent:local...")
	agentBuildCmd := exec.CommandContext(ctx,
		"docker", "build",
		"-f", filepath.Join(projectRoot, "Dockerfile.agent"),
		"-t", "lumo-agent:local",
		projectRoot,
	)
	agentBuildCmd.Stdout = c.logger.Writer()
	agentBuildCmd.Stderr = c.logger.Writer()

	if err := agentBuildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build agent image: %w", err)
	}

	// Build API image
	c.logger.Info("Building lumo:local...")
	apiBuildCmd := exec.CommandContext(ctx,
		"docker", "build",
		"-f", filepath.Join(projectRoot, "Dockerfile"),
		"-t", "lumo:local",
		projectRoot,
	)
	apiBuildCmd.Stdout = c.logger.Writer()
	apiBuildCmd.Stderr = c.logger.Writer()

	if err := apiBuildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build API image: %w", err)
	}

	c.logger.Info("✓ Images built successfully")

	// Load images into Kind cluster
	c.logger.Infof("Loading images into Kind cluster '%s'...", clusterName)

	// Load agent image
	if err := c.loadImageIntoCluster(ctx, clusterName, "lumo-agent:local"); err != nil {
		return fmt.Errorf("failed to load agent image: %w", err)
	}

	// Load API image
	if err := c.loadImageIntoCluster(ctx, clusterName, "lumo:local"); err != nil {
		return fmt.Errorf("failed to load API image: %w", err)
	}

	c.logger.Info("✓ Images loaded into cluster")

	return nil
}

// loadImageIntoCluster loads a Docker image into a Kind cluster
func (c *Client) loadImageIntoCluster(ctx context.Context, clusterName, imageName string) error {
	loadCmd := exec.CommandContext(ctx,
		"kind", "load", "docker-image",
		imageName,
		"--name", clusterName,
	)

	if output, err := loadCmd.CombinedOutput(); err != nil {
		c.logger.Debugf("kind load output: %s", string(output))
		return fmt.Errorf("failed to load image %s: %w", imageName, err)
	}

	c.logger.Debugf("✓ Loaded image: %s", imageName)
	return nil
}

// DeleteCluster deletes a Kind cluster
func (c *Client) DeleteCluster(name string) error {
	c.logger.Infof("Deleting Kind cluster '%s'...", name)

	if err := c.provider.Delete(name, ""); err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	c.logger.Infof("✓ Cluster '%s' deleted", name)
	return nil
}

// GetKubeconfig returns the kubeconfig path for a Kind cluster
func (c *Client) GetKubeconfig(name string) (string, error) {
	// Kind stores kubeconfig in default kubectl location
	// We need to get the kind-specific context
	kubeconfig, err := c.provider.KubeConfig(name, false)
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	// Write to temporary file
	tmpDir := os.TempDir()
	kubeconfigPath := filepath.Join(tmpDir, fmt.Sprintf("kubeconfig-%s.yaml", name))

	if err := os.WriteFile(kubeconfigPath, []byte(kubeconfig), 0600); err != nil {
		return "", fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return kubeconfigPath, nil
}

// findProjectRoot finds the project root directory by looking for go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find go.mod in parent directories")
		}
		dir = parent
	}
}
