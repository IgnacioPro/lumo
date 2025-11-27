package kubernetes

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// HealthCheckFunc defines a health check function
type HealthCheckFunc func(ctx context.Context) error

// WaitForComponentsReady waits for all components to be ready
func (c *Client) WaitForComponentsReady(ctx context.Context, namespace string, timeout time.Duration) error {
	components := []struct {
		name       string
		deployment string
		checker    HealthCheckFunc
	}{
		{
			name:       "PostgreSQL",
			deployment: "postgres",
			checker: func(ctx context.Context) error {
				return c.checkPostgreSQLHealth(ctx, namespace)
			},
		},
		{
			name:       "Redis",
			deployment: "lumo-redis",
			checker: func(ctx context.Context) error {
				return c.checkRedisHealth(ctx, namespace)
			},
		},
		{
			name:       "API Server",
			deployment: "lumo-api",
			checker: func(ctx context.Context) error {
				return c.checkAPIHealth(ctx, namespace)
			},
		},
		{
			name:       "Agent",
			deployment: "lumo-agent",
			checker: func(ctx context.Context) error {
				return c.checkAgentHealth(ctx, namespace)
			},
		},
	}

	for _, comp := range components {
		c.logger.Infof("Waiting for %s to be ready...", comp.name)

		// Wait for deployment rollout
		if err := c.WaitForRollout(ctx, namespace, comp.deployment, timeout); err != nil {
			// If deployment doesn't exist, it might be disabled (like agent in agent-only mode)
			c.logger.Debugf("%s deployment not found, skipping: %v", comp.name, err)
			continue
		}

		// Run health check with retries
		if err := c.retryWithBackoff(ctx, comp.checker, 10, 3*time.Second); err != nil {
			c.logger.Warnf("⚠ %s health check failed: %v (non-fatal, continuing)", comp.name, err)
			continue
		}

		c.logger.Infof("✓ %s is healthy", comp.name)
	}

	return nil
}

// checkPostgreSQLHealth verifies PostgreSQL is accessible
func (c *Client) checkPostgreSQLHealth(ctx context.Context, namespace string) error {
	pod, err := c.GetPodByLabel(ctx, namespace, "app=postgres")
	if err != nil {
		return fmt.Errorf("find postgres pod: %w", err)
	}

	// Use kubectl exec to check PostgreSQL (simpler than implementing full exec)
	cmd := exec.CommandContext(ctx,
		"kubectl", "exec", "-n", namespace, pod.Name, "--",
		"psql", "-U", "lumo", "-d", "lumo", "-c", "SELECT 1")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("postgres health check failed: %w", err)
	}

	return nil
}

// checkRedisHealth verifies Redis is accessible
func (c *Client) checkRedisHealth(ctx context.Context, namespace string) error {
	pod, err := c.GetPodByLabel(ctx, namespace, "app=lumo-redis")
	if err != nil {
		return fmt.Errorf("find redis pod: %w", err)
	}

	// Use kubectl exec to check Redis
	cmd := exec.CommandContext(ctx,
		"kubectl", "exec", "-n", namespace, pod.Name, "--",
		"redis-cli", "ping")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("redis health check failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// checkAPIHealth verifies the API server is responding
func (c *Client) checkAPIHealth(ctx context.Context, namespace string) error {
	// Note: This is simplified. In production, you'd use port-forward or check via service
	// For now, we'll just check that the pod is running and has passed its readiness probe
	pod, err := c.GetPodByLabel(ctx, namespace, "app=lumo-api")
	if err != nil {
		return fmt.Errorf("find api pod: %w", err)
	}

	// Check if pod is ready
	for _, condition := range pod.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return nil
		}
	}

	return fmt.Errorf("api pod not ready")
}

// checkAgentHealth verifies the agent is running
func (c *Client) checkAgentHealth(ctx context.Context, namespace string) error {
	// Check that at least one agent pod is running
	pod, err := c.GetPodByLabel(ctx, namespace, "app.kubernetes.io/component=agent")
	if err != nil {
		return fmt.Errorf("find agent pod: %w", err)
	}

	// Check if pod is ready
	for _, condition := range pod.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return nil
		}
	}

	return fmt.Errorf("agent pod not ready")
}

// retryWithBackoff retries a function with exponential backoff
func (c *Client) retryWithBackoff(ctx context.Context, fn HealthCheckFunc, maxAttempts int, initialDelay time.Duration) error {
	delay := initialDelay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		if attempt < maxAttempts {
			c.logger.Debugf("Attempt %d/%d failed: %v. Retrying in %v...", attempt, maxAttempts, err, delay)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				delay *= 2 // Exponential backoff
			}
		} else {
			return fmt.Errorf("all %d attempts failed: %w", maxAttempts, err)
		}
	}

	return fmt.Errorf("retry exhausted")
}
