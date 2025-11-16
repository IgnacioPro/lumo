package checkers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// ProxmoxChecker performs Proxmox Virtual Environment (PVE) diagnostics
type ProxmoxChecker struct {
	checkCluster      bool
	checkVMs          bool
	checkStorage      bool
	checkReplication  bool
	checkBackups      bool
	checkHA           bool
	checkServices     bool
}

// NewProxmoxChecker creates a new Proxmox checker
// If all bool params are false, all checks are enabled by default
func NewProxmoxChecker(checkCluster, checkVMs, checkStorage, checkReplication, checkBackups, checkHA, checkServices bool) *ProxmoxChecker {
	// If no specific checks enabled, enable all
	allDisabled := !checkCluster && !checkVMs && !checkStorage && !checkReplication && !checkBackups && !checkHA && !checkServices

	return &ProxmoxChecker{
		checkCluster:     allDisabled || checkCluster,
		checkVMs:         allDisabled || checkVMs,
		checkStorage:     allDisabled || checkStorage,
		checkReplication: allDisabled || checkReplication,
		checkBackups:     allDisabled || checkBackups,
		checkHA:          allDisabled || checkHA,
		checkServices:    allDisabled || checkServices,
	}
}

// Name returns the checker name
func (p *ProxmoxChecker) Name() string {
	return "proxmox_check"
}

// Category returns the checker category
func (p *ProxmoxChecker) Category() diagnostics.CheckCategory {
	return diagnostics.CategoryVirtualization
}

// Description returns the checker description
func (p *ProxmoxChecker) Description() string {
	return "Checks Proxmox VE cluster, VMs, containers, storage, replication, backups, and HA status"
}

// RequiresRoot returns true as many Proxmox commands require root
func (p *ProxmoxChecker) RequiresRoot() bool {
	return true
}

// Run executes the Proxmox check
func (p *ProxmoxChecker) Run(ctx context.Context, executor diagnostics.CommandExecutor) (*diagnostics.CheckResult, error) {
	result := &diagnostics.CheckResult{
		Name:      p.Name(),
		Category:  p.Category(),
		Status:    diagnostics.StatusCompleted,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
		Metrics:   []diagnostics.Metric{},
	}

	startTime := time.Now()

	// First, check if Proxmox is installed
	isProxmox, version, err := p.detectProxmox(ctx, executor)
	if err != nil {
		return nil, fmt.Errorf("failed to detect Proxmox: %w", err)
	}

	if !isProxmox {
		result.Status = diagnostics.StatusSkipped
		result.Message = "Proxmox VE not detected on this system"
		result.Duration = time.Since(startTime)
		return result, nil
	}

	result.SetData("proxmox_version", version)
	result.SetData("is_proxmox", true)

	var issues []string
	var warnings []string

	// Check cluster status
	if p.checkCluster {
		clusterInfo, clusterIssues, err := p.checkClusterStatus(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("cluster check failed: %v", err))
		} else {
			result.SetData("cluster", clusterInfo)
			issues = append(issues, clusterIssues...)
		}
	}

	// Check VMs and containers
	if p.checkVMs {
		vmInfo, vmIssues, err := p.checkVMsAndContainers(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("VM/container check failed: %v", err))
		} else {
			result.SetData("vms", vmInfo)
			issues = append(issues, vmIssues...)
		}
	}

	// Check storage
	if p.checkStorage {
		storageInfo, storageIssues, err := p.checkStorageStatus(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("storage check failed: %v", err))
		} else {
			result.SetData("storage", storageInfo)
			issues = append(issues, storageIssues...)
		}
	}

	// Check replication
	if p.checkReplication {
		replicationInfo, replicationIssues, err := p.checkReplicationStatus(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("replication check failed: %v", err))
		} else {
			result.SetData("replication", replicationInfo)
			issues = append(issues, replicationIssues...)
		}
	}

	// Check backups
	if p.checkBackups {
		backupInfo, backupIssues, err := p.checkBackupStatus(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("backup check failed: %v", err))
		} else {
			result.SetData("backups", backupInfo)
			issues = append(issues, backupIssues...)
		}
	}

	// Check HA
	if p.checkHA {
		haInfo, haIssues, err := p.checkHAStatus(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("HA check failed: %v", err))
		} else {
			result.SetData("ha", haInfo)
			issues = append(issues, haIssues...)
		}
	}

	// Check Proxmox services
	if p.checkServices {
		serviceInfo, serviceIssues, err := p.checkProxmoxServices(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("service check failed: %v", err))
		} else {
			result.SetData("services", serviceInfo)
			issues = append(issues, serviceIssues...)
		}
	}

	// Add metrics
	result.AddMetric(diagnostics.Metric{
		Name:          "total_issues",
		Value:         float64(len(issues)),
		Unit:          "count",
		Threshold:     1.0,
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.AddMetric(diagnostics.Metric{
		Name:          "total_warnings",
		Value:         float64(len(warnings)),
		Unit:          "count",
		Threshold:     3.0,
		ThresholdType: diagnostics.ThresholdTypeMax,
	})

	result.Duration = time.Since(startTime)

	// Generate message
	result.Message = p.formatMessage(version, len(issues), len(warnings), issues, warnings)

	return result, nil
}

// ProxmoxVersion holds version information
type ProxmoxVersion struct {
	Version string `json:"version"`
	Release string `json:"release"`
}

// ClusterInfo holds cluster information
type ClusterInfo struct {
	Name       string       `json:"name"`
	Nodes      int          `json:"nodes"`
	Quorate    bool         `json:"quorate"`
	QuorumVotes int         `json:"quorum_votes"`
	NodeList   []ClusterNode `json:"node_list"`
}

// ClusterNode holds node information
type ClusterNode struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Online bool   `json:"online"`
	Local  bool   `json:"local"`
}

// VMInfo holds VM/container information
type VMInfo struct {
	TotalVMs       int      `json:"total_vms"`
	RunningVMs     int      `json:"running_vms"`
	StoppedVMs     int      `json:"stopped_vms"`
	TotalCT        int      `json:"total_containers"`
	RunningCT      int      `json:"running_containers"`
	StoppedCT      int      `json:"stopped_containers"`
	VMs            []VMEntry `json:"vms"`
	Containers     []VMEntry `json:"containers"`
}

// VMEntry holds individual VM/container information
type VMEntry struct {
	VMID   string  `json:"vmid"`
	Name   string  `json:"name"`
	Status string  `json:"status"`
	Memory int64   `json:"memory_mb"`
	CPU    float64 `json:"cpu_percent"`
}

// StorageInfo holds storage information
type StorageInfo struct {
	TotalStorage    int              `json:"total_storage"`
	ActiveStorage   int              `json:"active_storage"`
	InactiveStorage int              `json:"inactive_storage"`
	Storages        []StorageEntry   `json:"storages"`
}

// StorageEntry holds individual storage information
type StorageEntry struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Active    bool    `json:"active"`
	Used      int64   `json:"used_bytes"`
	Total     int64   `json:"total_bytes"`
	UsedPct   float64 `json:"used_percent"`
	Available int64   `json:"available_bytes"`
}

// ReplicationInfo holds replication status
type ReplicationInfo struct {
	TotalJobs  int                `json:"total_jobs"`
	OKJobs     int                `json:"ok_jobs"`
	ErrorJobs  int                `json:"error_jobs"`
	Jobs       []ReplicationJob   `json:"jobs"`
}

// ReplicationJob holds individual replication job information
type ReplicationJob struct {
	ID         string `json:"id"`
	Guest      string `json:"guest"`
	Status     string `json:"status"`
	LastSync   string `json:"last_sync"`
	NextSync   string `json:"next_sync"`
}

// BackupInfo holds backup information
type BackupInfo struct {
	RecentBackups    int           `json:"recent_backups"`
	FailedBackups    int           `json:"failed_backups"`
	SuccessfulBackups int          `json:"successful_backups"`
	Backups          []BackupEntry `json:"backups"`
}

// BackupEntry holds individual backup information
type BackupEntry struct {
	VMID      string `json:"vmid"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
	Size      int64  `json:"size_bytes"`
}

// HAInfo holds HA status
type HAInfo struct {
	HAEnabled    bool      `json:"ha_enabled"`
	HARunning    bool      `json:"ha_running"`
	ManagedVMs   int       `json:"managed_vms"`
	HAServices   []HAService `json:"services"`
}

// HAService holds HA service information
type HAService struct {
	Service string `json:"service"`
	Node    string `json:"node"`
	State   string `json:"state"`
}

// ProxmoxServiceInfo holds Proxmox service information
type ProxmoxServiceInfo struct {
	TotalServices  int            `json:"total_services"`
	RunningServices int           `json:"running_services"`
	FailedServices int            `json:"failed_services"`
	Services       []ProxmoxService `json:"services"`
}

// ProxmoxService holds individual service information
type ProxmoxService struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Active bool   `json:"active"`
}

// detectProxmox checks if Proxmox is installed and returns version
func (p *ProxmoxChecker) detectProxmox(ctx context.Context, executor diagnostics.CommandExecutor) (bool, string, error) {
	// Check for pveversion command
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "command -v pveversion >/dev/null 2>&1")
	if exitCode != 0 {
		return false, "", nil
	}

	// Get Proxmox version
	stdout, _, exitCode, err := executor.ExecuteWithContext(ctx, "pveversion")
	if err != nil || exitCode != 0 {
		return true, "unknown", nil
	}

	version := strings.TrimSpace(stdout)
	return true, version, nil
}

// checkClusterStatus checks Proxmox cluster status
func (p *ProxmoxChecker) checkClusterStatus(ctx context.Context, executor diagnostics.CommandExecutor) (*ClusterInfo, []string, error) {
	var issues []string
	info := &ClusterInfo{}

	// Check if node is in a cluster
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "pvecm status 2>/dev/null")
	if exitCode != 0 {
		// Not in a cluster - this is OK for standalone nodes
		info.Name = "standalone"
		info.Nodes = 1
		info.Quorate = true
		return info, issues, nil
	}

	// Parse cluster status
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Name:") {
			info.Name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
		} else if strings.HasPrefix(line, "Quorum information") {
			// Next lines will have quorum details
			continue
		} else if strings.HasPrefix(line, "Nodes:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				nodes, _ := strconv.Atoi(parts[1])
				info.Nodes = nodes
			}
		} else if strings.Contains(line, "Quorate:") {
			info.Quorate = strings.Contains(line, "Yes")
			if !info.Quorate {
				issues = append(issues, "cluster is not quorate")
			}
		} else if strings.Contains(line, "Total votes:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				votes, _ := strconv.Atoi(parts[2])
				info.QuorumVotes = votes
			}
		}
	}

	// Get node list
	stdout, _, exitCode, _ = executor.ExecuteWithContext(ctx, "pvecm nodes 2>/dev/null")
	if exitCode == 0 {
		info.NodeList = p.parseNodeList(stdout)

		// Check for offline nodes
		for _, node := range info.NodeList {
			if !node.Online {
				issues = append(issues, fmt.Sprintf("node %s is offline", node.Name))
			}
		}
	}

	return info, issues, nil
}

// parseNodeList parses pvecm nodes output
// Format: Nodeid  Votes  Name
// Example:     1      1    pve01 (local)
func (p *ProxmoxChecker) parseNodeList(output string) []ClusterNode {
	var nodes []ClusterNode
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Nodeid") || strings.HasPrefix(line, "Membership") || strings.Contains(line, "---") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		nodeName := fields[2]
		isLocal := false

		// Check if (local) is in the name or as a separate field
		if len(fields) > 3 && fields[3] == "(local)" {
			isLocal = true
		} else if strings.Contains(nodeName, "(local)") {
			isLocal = true
			nodeName = strings.TrimSpace(strings.Replace(nodeName, "(local)", "", 1))
		}

		node := ClusterNode{
			ID:     fields[0],
			Name:   nodeName,
			Online: true, // Nodes in pvecm nodes output are online cluster members
			Local:  isLocal,
		}
		nodes = append(nodes, node)
	}

	return nodes
}

// checkVMsAndContainers checks status of VMs and containers
func (p *ProxmoxChecker) checkVMsAndContainers(ctx context.Context, executor diagnostics.CommandExecutor) (*VMInfo, []string, error) {
	var issues []string
	info := &VMInfo{}

	// Check QEMU VMs
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "qm list 2>/dev/null")
	if exitCode == 0 {
		vms := p.parseVMList(stdout, "qemu")
		info.VMs = vms
		info.TotalVMs = len(vms)

		for _, vm := range vms {
			if vm.Status == "running" {
				info.RunningVMs++
			} else {
				info.StoppedVMs++
			}
		}
	}

	// Check LXC containers
	stdout, _, exitCode, _ = executor.ExecuteWithContext(ctx, "pct list 2>/dev/null")
	if exitCode == 0 {
		containers := p.parseVMList(stdout, "lxc")
		info.Containers = containers
		info.TotalCT = len(containers)

		for _, ct := range containers {
			if ct.Status == "running" {
				info.RunningCT++
			} else {
				info.StoppedCT++
			}
		}
	}

	// No issues - VM states are informational
	return info, issues, nil
}

// parseVMList parses qm list or pct list output
func (p *ProxmoxChecker) parseVMList(output string, vmType string) []VMEntry {
	var vms []VMEntry
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		// Skip header
		if i == 0 {
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		vm := VMEntry{
			VMID:   fields[0],
			Name:   fields[1],
			Status: fields[2],
		}

		// Parse memory if available (in MB or with suffix)
		if len(fields) >= 4 {
			memStr := fields[3]
			memStr = strings.TrimSuffix(memStr, "MB")
			memStr = strings.TrimSuffix(memStr, "M")
			if mem, err := strconv.ParseInt(memStr, 10, 64); err == nil {
				vm.Memory = mem
			}
		}

		// Parse CPU if available
		if len(fields) >= 5 {
			cpuStr := strings.TrimSuffix(fields[4], "%")
			if cpu, err := strconv.ParseFloat(cpuStr, 64); err == nil {
				vm.CPU = cpu
			}
		}

		vms = append(vms, vm)
	}

	return vms
}

// checkStorageStatus checks Proxmox storage status
func (p *ProxmoxChecker) checkStorageStatus(ctx context.Context, executor diagnostics.CommandExecutor) (*StorageInfo, []string, error) {
	var issues []string
	info := &StorageInfo{}

	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "pvesm status 2>/dev/null")
	if exitCode != 0 {
		return info, issues, fmt.Errorf("failed to get storage status")
	}

	storages := p.parseStorageStatus(stdout)
	info.Storages = storages
	info.TotalStorage = len(storages)

	for _, storage := range storages {
		if storage.Active {
			info.ActiveStorage++

			// Check for high usage
			if storage.UsedPct > 90 {
				issues = append(issues, fmt.Sprintf("storage %s is %0.1f%% full", storage.Name, storage.UsedPct))
			}
		} else {
			info.InactiveStorage++
			issues = append(issues, fmt.Sprintf("storage %s is inactive", storage.Name))
		}
	}

	return info, issues, nil
}

// parseStorageStatus parses pvesm status output
func (p *ProxmoxChecker) parseStorageStatus(output string) []StorageEntry {
	var storages []StorageEntry
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		// Skip header
		if i == 0 {
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		storage := StorageEntry{
			Name:   fields[0],
			Type:   fields[1],
			Active: fields[2] == "1" || strings.ToLower(fields[2]) == "active",
		}

		// Parse total size
		if total, err := strconv.ParseInt(fields[3], 10, 64); err == nil {
			storage.Total = total
		}

		// Parse used size
		if used, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
			storage.Used = used
		}

		// Parse available size
		if avail, err := strconv.ParseInt(fields[5], 10, 64); err == nil {
			storage.Available = avail
		}

		// Calculate percentage
		if storage.Total > 0 {
			storage.UsedPct = float64(storage.Used) / float64(storage.Total) * 100
		}

		storages = append(storages, storage)
	}

	return storages
}

// checkReplicationStatus checks Proxmox replication status
func (p *ProxmoxChecker) checkReplicationStatus(ctx context.Context, executor diagnostics.CommandExecutor) (*ReplicationInfo, []string, error) {
	var issues []string
	info := &ReplicationInfo{}

	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "pvesr list 2>/dev/null")
	if exitCode != 0 {
		// Replication might not be configured - not an error
		return info, issues, nil
	}

	jobs := p.parseReplicationJobs(stdout)
	info.Jobs = jobs
	info.TotalJobs = len(jobs)

	for _, job := range jobs {
		if job.Status == "OK" || job.Status == "ok" {
			info.OKJobs++
		} else {
			info.ErrorJobs++
			issues = append(issues, fmt.Sprintf("replication job %s has status: %s", job.ID, job.Status))
		}
	}

	return info, issues, nil
}

// parseReplicationJobs parses pvesr list output
func (p *ProxmoxChecker) parseReplicationJobs(output string) []ReplicationJob {
	var jobs []ReplicationJob
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		// Skip header
		if i == 0 {
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		job := ReplicationJob{
			ID:     fields[0],
			Guest:  fields[1],
			Status: fields[2],
		}

		if len(fields) >= 4 {
			job.LastSync = fields[3]
		}

		if len(fields) >= 5 {
			job.NextSync = fields[4]
		}

		jobs = append(jobs, job)
	}

	return jobs
}

// checkBackupStatus checks recent backup status
func (p *ProxmoxChecker) checkBackupStatus(ctx context.Context, executor diagnostics.CommandExecutor) (*BackupInfo, []string, error) {
	var issues []string
	info := &BackupInfo{}

	// Check recent backup tasks from task log
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx,
		"pvesh get /cluster/tasks --limit 50 --typefilter vzdump 2>/dev/null | grep -v '^┌\\|^│\\|^└\\|^Task' || true")

	if exitCode == 0 && stdout != "" {
		backups := p.parseBackupTasks(stdout)
		info.Backups = backups
		info.RecentBackups = len(backups)

		for _, backup := range backups {
			if backup.Status == "OK" {
				info.SuccessfulBackups++
			} else {
				info.FailedBackups++
			}
		}

		if info.FailedBackups > 0 {
			issues = append(issues, fmt.Sprintf("%d recent backup(s) failed", info.FailedBackups))
		}
	}

	return info, issues, nil
}

// parseBackupTasks parses backup task output
func (p *ProxmoxChecker) parseBackupTasks(output string) []BackupEntry {
	var backups []BackupEntry
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		backup := BackupEntry{
			VMID:      "unknown",
			Timestamp: fields[0],
			Status:    fields[len(fields)-1],
		}

		// Try to extract VMID from task description
		for _, field := range fields {
			if _, err := strconv.Atoi(field); err == nil && len(field) <= 5 {
				backup.VMID = field
				break
			}
		}

		backups = append(backups, backup)
	}

	return backups
}

// checkHAStatus checks HA (High Availability) status
func (p *ProxmoxChecker) checkHAStatus(ctx context.Context, executor diagnostics.CommandExecutor) (*HAInfo, []string, error) {
	var issues []string
	info := &HAInfo{}

	// Check if HA manager is available
	_, _, exitCode, _ := executor.ExecuteWithContext(ctx, "command -v ha-manager >/dev/null 2>&1")
	if exitCode != 0 {
		info.HAEnabled = false
		return info, issues, nil
	}

	info.HAEnabled = true

	// Check HA status
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "ha-manager status 2>/dev/null")
	if exitCode == 0 {
		info.HARunning = true
		services := p.parseHAStatus(stdout)
		info.HAServices = services
		info.ManagedVMs = len(services)

		for _, svc := range services {
			if svc.State != "started" && svc.State != "running" {
				issues = append(issues, fmt.Sprintf("HA service %s is in state: %s", svc.Service, svc.State))
			}
		}
	} else {
		info.HARunning = false
		issues = append(issues, "HA manager is enabled but not running")
	}

	return info, issues, nil
}

// parseHAStatus parses ha-manager status output
// Only parses lines starting with "service"
func (p *ProxmoxChecker) parseHAStatus(output string) []HAService {
	var services []HAService
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "service") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		// Format: service vm:100 (pve01, started)
		// fields[0] = "service"
		// fields[1] = "vm:100"
		// fields[2] = "(pve01,"
		// fields[3] = "started)"

		serviceName := fields[1]
		node := ""
		state := ""

		if len(fields) >= 4 {
			// Extract node and state from parentheses
			node = strings.TrimPrefix(fields[2], "(")
			node = strings.TrimSuffix(node, ",")
			state = strings.TrimSuffix(fields[3], ")")
		}

		service := HAService{
			Service: serviceName,
			Node:    node,
			State:   state,
		}

		services = append(services, service)
	}

	return services
}

// checkProxmoxServices checks critical Proxmox services
func (p *ProxmoxChecker) checkProxmoxServices(ctx context.Context, executor diagnostics.CommandExecutor) (*ProxmoxServiceInfo, []string, error) {
	var issues []string
	info := &ProxmoxServiceInfo{}

	// Critical Proxmox services to monitor
	criticalServices := []string{
		"pve-cluster",
		"pvedaemon",
		"pveproxy",
		"pvestatd",
		"pve-ha-lrm",
		"pve-ha-crm",
	}

	var services []ProxmoxService

	for _, svcName := range criticalServices {
		stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx,
			fmt.Sprintf("systemctl is-active %s 2>/dev/null", svcName))

		status := strings.TrimSpace(stdout)
		active := exitCode == 0 && status == "active"

		service := ProxmoxService{
			Name:   svcName,
			Status: status,
			Active: active,
		}

		services = append(services, service)
		info.TotalServices++

		if active {
			info.RunningServices++
		} else {
			info.FailedServices++
			// HA services might not be running if HA is not configured
			if !strings.Contains(svcName, "ha-") {
				issues = append(issues, fmt.Sprintf("critical service %s is not running", svcName))
			}
		}
	}

	info.Services = services

	return info, issues, nil
}

// formatMessage creates a human-readable summary message
func (p *ProxmoxChecker) formatMessage(version string, issueCount, warningCount int, issues, warnings []string) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("Proxmox VE %s", version))

	if issueCount == 0 && warningCount == 0 {
		parts = append(parts, "all checks passed")
	} else {
		if issueCount > 0 {
			if issueCount <= 3 {
				parts = append(parts, fmt.Sprintf("%d issue(s): %s", issueCount, strings.Join(issues, "; ")))
			} else {
				parts = append(parts, fmt.Sprintf("%d issue(s): %s and %d more", issueCount, strings.Join(issues[:3], "; "), issueCount-3))
			}
		}

		if warningCount > 0 {
			if warningCount <= 2 {
				parts = append(parts, fmt.Sprintf("%d warning(s): %s", warningCount, strings.Join(warnings, "; ")))
			} else {
				parts = append(parts, fmt.Sprintf("%d warning(s)", warningCount))
			}
		}
	}

	return strings.Join(parts, ", ")
}
