package helm

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/release"
)

// Client wraps the Helm SDK to provide clean deployment operations
type Client struct {
	settings *cli.EnvSettings
	cfg      *action.Configuration
	logger   *logrus.Logger
}

// NewClient creates a new Helm client for the specified namespace
func NewClient(namespace, kubeconfig string, logger *logrus.Logger) (*Client, error) {
	settings := cli.New()
	settings.SetNamespace(namespace)

	if kubeconfig != "" {
		settings.KubeConfig = kubeconfig
	}

	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), namespace, "secret", debugLog(logger)); err != nil {
		return nil, fmt.Errorf("helm config init: %w", err)
	}

	return &Client{
		settings: settings,
		cfg:      cfg,
		logger:   logger,
	}, nil
}

// Install installs a Helm chart
func (h *Client) Install(chartPath, releaseName string, values map[string]interface{}, timeout time.Duration) (*release.Release, error) {
	client := action.NewInstall(h.cfg)
	client.Namespace = h.settings.Namespace()
	client.ReleaseName = releaseName
	client.Wait = true
	client.Timeout = timeout
	client.CreateNamespace = true

	h.logger.Infof("Loading chart from %s", chartPath)

	// Load chart from filesystem
	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	h.logger.Infof("Installing Helm release %s", releaseName)

	// Install
	rel, err := client.Run(chart, values)
	if err != nil {
		return nil, fmt.Errorf("helm install: %w", err)
	}

	h.logger.Infof("✓ Helm release %s installed successfully (version %d)", rel.Name, rel.Version)

	return rel, nil
}

// Upgrade upgrades an existing Helm release
func (h *Client) Upgrade(chartPath, releaseName string, values map[string]interface{}, timeout time.Duration) (*release.Release, error) {
	client := action.NewUpgrade(h.cfg)
	client.Namespace = h.settings.Namespace()
	client.Wait = true
	client.Timeout = timeout

	h.logger.Infof("Loading chart from %s", chartPath)

	// Load chart from filesystem
	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load chart: %w", err)
	}

	h.logger.Infof("Upgrading Helm release %s", releaseName)

	// Upgrade
	rel, err := client.Run(releaseName, chart, values)
	if err != nil {
		return nil, fmt.Errorf("helm upgrade: %w", err)
	}

	h.logger.Infof("✓ Helm release %s upgraded successfully (version %d)", rel.Name, rel.Version)

	return rel, nil
}

// UpgradeOrInstall upgrades a release if it exists, otherwise installs it
func (h *Client) UpgradeOrInstall(chartPath, releaseName string, values map[string]interface{}, timeout time.Duration) (*release.Release, error) {
	// Check if release exists
	histClient := action.NewHistory(h.cfg)
	histClient.Max = 1

	_, err := histClient.Run(releaseName)
	if err == nil {
		// Release exists, upgrade
		h.logger.Infof("Release %s exists, upgrading", releaseName)
		return h.Upgrade(chartPath, releaseName, values, timeout)
	}

	// Release doesn't exist, install
	h.logger.Infof("Release %s does not exist, installing", releaseName)
	return h.Install(chartPath, releaseName, values, timeout)
}

// Uninstall removes a Helm release
func (h *Client) Uninstall(releaseName string) error {
	client := action.NewUninstall(h.cfg)

	h.logger.Infof("Uninstalling Helm release %s", releaseName)

	_, err := client.Run(releaseName)
	if err != nil {
		return fmt.Errorf("helm uninstall: %w", err)
	}

	h.logger.Infof("✓ Helm release %s uninstalled successfully", releaseName)

	return nil
}

// GetRelease retrieves information about a release
func (h *Client) GetRelease(releaseName string) (*release.Release, error) {
	client := action.NewGet(h.cfg)

	rel, err := client.Run(releaseName)
	if err != nil {
		return nil, fmt.Errorf("get release: %w", err)
	}

	return rel, nil
}

// ListReleases lists all releases in the namespace
func (h *Client) ListReleases() ([]*release.Release, error) {
	client := action.NewList(h.cfg)
	client.All = true

	releases, err := client.Run()
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}

	return releases, nil
}

// ReleaseExists checks if a release exists
func (h *Client) ReleaseExists(releaseName string) (bool, error) {
	histClient := action.NewHistory(h.cfg)
	histClient.Max = 1

	_, err := histClient.Run(releaseName)
	if err != nil {
		// If error contains "not found", release doesn't exist
		return false, nil
	}

	return true, nil
}

// Rollback rolls back a release to a previous revision
func (h *Client) Rollback(releaseName string, revision int) error {
	client := action.NewRollback(h.cfg)
	client.Version = revision
	client.Wait = true

	h.logger.Infof("Rolling back release %s to revision %d", releaseName, revision)

	if err := client.Run(releaseName); err != nil {
		return fmt.Errorf("helm rollback: %w", err)
	}

	h.logger.Infof("✓ Release %s rolled back successfully", releaseName)

	return nil
}

// debugLog creates a Helm debug logger that writes to logrus
func debugLog(logger *logrus.Logger) func(format string, v ...interface{}) {
	return func(format string, v ...interface{}) {
		logger.Debugf(format, v...)
	}
}
