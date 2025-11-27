package kubernetes

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os/exec"
	"strings"
)

// BootstrapDatabase creates the necessary database records for Lumo to function
func (c *Client) BootstrapDatabase(ctx context.Context, namespace, agentToken string) error {
	c.logger.Info("Bootstrapping database...")

	// Get PostgreSQL pod
	pod, err := c.GetPodByLabel(ctx, namespace, "app=postgres")
	if err != nil {
		return fmt.Errorf("find postgres pod: %w", err)
	}

	// 1. Create API key
	keyHash := hashToken(agentToken)

	c.logger.Info("Creating API key...")

	sql := fmt.Sprintf(`
		INSERT INTO api_keys (
			id, key_hash, name, scopes, created_at, revoked
		) VALUES (
			gen_random_uuid(),
			'%s',
			'lumo-agent-key',
			ARRAY['agents:read', 'agents:write', 'events:write', 'jobs:read', 'diagnostics:write'],
			NOW(),
			false
		) ON CONFLICT (key_hash) DO NOTHING;
	`, keyHash)

	if err := c.executePSQL(ctx, namespace, pod.Name, sql); err != nil {
		return fmt.Errorf("create API key: %w", err)
	}

	c.logger.Info("✓ API key created")

	// 2. Create system agent
	c.logger.Info("Creating system agent...")

	sql = `
		INSERT INTO agents (
			id, name, hostname, ip_address, platform, architecture,
			version, status, capabilities, labels,
			registered_at, last_heartbeat_at
		) VALUES (
			'00000000-0000-0000-0000-000000000000',
			'system-api-key',
			'api-server',
			'0.0.0.0',
			'kubernetes',
			'any',
			'n/a',
			'online',
			ARRAY[]::text[],
			'{"type": "system", "auth": "api-key"}'::jsonb,
			NOW(),
			NOW()
		) ON CONFLICT (id) DO UPDATE SET
			last_heartbeat_at = NOW(),
			status = 'online';
	`

	if err := c.executePSQL(ctx, namespace, pod.Name, sql); err != nil {
		return fmt.Errorf("create system agent: %w", err)
	}

	c.logger.Info("✓ System agent created")

	// 3. Verify
	c.logger.Info("Verifying bootstrap...")

	verifySQL := fmt.Sprintf("SELECT COUNT(*) FROM api_keys WHERE key_hash = '%s';", keyHash)
	output, err := c.executePSQLWithOutput(ctx, namespace, pod.Name, verifySQL)
	if err != nil {
		return fmt.Errorf("verify API key: %w", err)
	}

	if !strings.Contains(output, "1") {
		return fmt.Errorf("API key verification failed: %s", output)
	}

	c.logger.Info("✓ Bootstrap completed successfully")

	return nil
}

// executePSQL executes a SQL command in the PostgreSQL pod
func (c *Client) executePSQL(ctx context.Context, namespace, podName, sql string) error {
	cmd := exec.CommandContext(ctx,
		"kubectl", "exec", "-i", "-n", namespace, podName, "--",
		"psql", "-U", "lumo", "-d", "lumo", "-v", "ON_ERROR_STOP=1", "-c", sql)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("psql command failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// executePSQLWithOutput executes a SQL command and returns the output
func (c *Client) executePSQLWithOutput(ctx context.Context, namespace, podName, sql string) (string, error) {
	cmd := exec.CommandContext(ctx,
		"kubectl", "exec", "-i", "-n", namespace, podName, "--",
		"psql", "-U", "lumo", "-d", "lumo", "-t", "-c", sql)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("psql command failed: %w (output: %s)", err, string(output))
	}

	return string(output), nil
}

// hashToken creates a SHA-256 hash of the token
func hashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return fmt.Sprintf("%x", h.Sum(nil))
}
