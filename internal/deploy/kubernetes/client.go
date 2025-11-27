package kubernetes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client wraps the Kubernetes client to provide clean operations
type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
	logger    *logrus.Logger
}

// NewClient creates a new Kubernetes client
func NewClient(kubeconfig, context string, logger *logrus.Logger) (*Client, error) {
	var config *rest.Config
	var err error

	// If kubeconfig is provided, use it. Otherwise, use default locations
	if kubeconfig == "" {
		// Try in-cluster config first
		config, err = rest.InClusterConfig()
		if err != nil {
			// Fall back to kubeconfig file
			kubeconfig = os.Getenv("KUBECONFIG")
			if kubeconfig == "" {
				if home := homedir.HomeDir(); home != "" {
					kubeconfig = filepath.Join(home, ".kube", "config")
				}
			}
		}
	}

	// If we don't have config yet (not in-cluster), load from kubeconfig file
	if config == nil {
		configLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if kubeconfig != "" {
			configLoadingRules.ExplicitPath = kubeconfig
		}

		configOverrides := &clientcmd.ConfigOverrides{}
		if context != "" {
			configOverrides.CurrentContext = context
		}

		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			configLoadingRules,
			configOverrides,
		)

		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("create kubernetes config: %w", err)
		}
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes clientset: %w", err)
	}

	logger.Info("✓ Kubernetes client initialized")

	return &Client{
		clientset: clientset,
		config:    config,
		logger:    logger,
	}, nil
}

// GetPodByLabel retrieves a pod by label selector
func (c *Client) GetPodByLabel(ctx context.Context, namespace, labelSelector string) (*corev1.Pod, error) {
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found with label %s", labelSelector)
	}

	// Return the first pod
	return &pods.Items[0], nil
}

// ExecInPod executes a command in a pod
func (c *Client) ExecInPod(ctx context.Context, namespace, podName, containerName string, cmd []string) (string, error) {
	// Note: This is a simplified version. Full implementation would use remotecommand.Executor
	// For now, we'll use kubectl exec via shell which is already tested in the script
	return "", fmt.Errorf("exec in pod not yet implemented - use kubectl directly")
}

// WaitForRollout waits for a deployment to complete its rollout
func (c *Client) WaitForRollout(ctx context.Context, namespace, deploymentName string, timeout time.Duration) error {
	c.logger.Infof("Waiting for deployment %s to be ready", deploymentName)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(deadline)):
			return fmt.Errorf("timeout waiting for deployment %s", deploymentName)
		case <-ticker.C:
			deployment, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
			if err != nil {
				c.logger.Debugf("Failed to get deployment %s: %v", deploymentName, err)
				continue
			}

			// Check if deployment is ready
			if deployment.Status.ReadyReplicas == deployment.Status.Replicas &&
				deployment.Status.Replicas == *deployment.Spec.Replicas &&
				deployment.Status.UpdatedReplicas == deployment.Status.Replicas {
				c.logger.Infof("✓ Deployment %s is ready (%d/%d replicas)",
					deploymentName,
					deployment.Status.ReadyReplicas,
					deployment.Status.Replicas)
				return nil
			}

			c.logger.Debugf("Deployment %s not ready yet: %d/%d replicas ready",
				deploymentName,
				deployment.Status.ReadyReplicas,
				deployment.Status.Replicas)
		}
	}
}

// GetDeployment retrieves a deployment
func (c *Client) GetDeployment(ctx context.Context, namespace, name string) (interface{}, error) {
	return c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetService retrieves a service
func (c *Client) GetService(ctx context.Context, namespace, name string) (*corev1.Service, error) {
	return c.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
}
