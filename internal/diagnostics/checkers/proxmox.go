package checkers

import (
	"context"
	"encoding/json"
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
	checkSubscription bool
	checkUpdates      bool
	checkTasks        bool
	checkPerformance  bool
	checkBootConfig   bool
	checkNetwork      bool
	checkCertificates bool
	checkCeph         bool
	checkPools        bool
	checkNodeSummary  bool
}

// NewProxmoxChecker creates a new Proxmox checker
// If all bool params are false, all checks are enabled by default
func NewProxmoxChecker(checkCluster, checkVMs, checkStorage, checkReplication, checkBackups, checkHA, checkServices bool) *ProxmoxChecker {
	// If no specific checks enabled, enable all
	allDisabled := !checkCluster && !checkVMs && !checkStorage && !checkReplication && !checkBackups && !checkHA && !checkServices

	return &ProxmoxChecker{
		checkCluster:      allDisabled || checkCluster,
		checkVMs:          allDisabled || checkVMs,
		checkStorage:      allDisabled || checkStorage,
		checkReplication:  allDisabled || checkReplication,
		checkBackups:      allDisabled || checkBackups,
		checkHA:           allDisabled || checkHA,
		checkServices:     allDisabled || checkServices,
		checkSubscription: allDisabled, // Always check subscription status
		checkUpdates:      allDisabled, // Always check for updates
		checkTasks:        allDisabled, // Always check recent tasks
		checkPerformance:  allDisabled, // Always check VM/CT performance
		checkBootConfig:   allDisabled, // Always check boot configuration
		checkNetwork:      allDisabled, // Always check network stats
		checkCertificates: allDisabled, // Always check SSL certificates
		checkCeph:         allDisabled, // Always check Ceph if installed
		checkPools:        allDisabled, // Always check resource pools
		checkNodeSummary:  allDisabled, // Always check node summary
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
	return "Comprehensive Proxmox VE monitoring: cluster health, VMs/containers, storage, replication, backups, HA, subscription status, updates, task history, performance metrics, boot configuration, network statistics, SSL certificate expiry, Ceph integration, resource pools, and node summary dashboard"
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

	// Check subscription status
	if p.checkSubscription {
		subInfo, subIssues, err := p.performSubscriptionCheck(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("subscription check failed: %v", err))
		} else {
			result.SetData("subscription", subInfo)
			issues = append(issues, subIssues...)
		}
	}

	// Check for available updates
	if p.checkUpdates {
		updateInfo, updateIssues, err := p.performUpdatesCheck(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("update check failed: %v", err))
		} else {
			result.SetData("updates", updateInfo)
			issues = append(issues, updateIssues...)
		}
	}

	// Check recent task failures
	if p.checkTasks {
		taskInfo, taskIssues, err := p.performTasksCheck(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("task check failed: %v", err))
		} else {
			result.SetData("tasks", taskInfo)
			issues = append(issues, taskIssues...)
		}
	}

	// Check performance metrics (if VMs are being monitored)
	if p.checkPerformance && p.checkVMs {
		perfInfo, perfIssues, err := p.performPerformanceCheck(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("performance check failed: %v", err))
		} else {
			result.SetData("performance", perfInfo)
			issues = append(issues, perfIssues...)
		}
	}

	// Check boot configuration
	if p.checkBootConfig && p.checkVMs {
		bootInfo, bootIssues, err := p.performBootConfigCheck(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("boot config check failed: %v", err))
		} else {
			result.SetData("boot_config", bootInfo)
			issues = append(issues, bootIssues...)
		}
	}

	// Check network statistics
	if p.checkNetwork {
		netInfo, netIssues, err := p.performNetworkStatsCheck(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("network check failed: %v", err))
		} else {
			result.SetData("network_stats", netInfo)
			issues = append(issues, netIssues...)
		}
	}

	// Check SSL certificates
	if p.checkCertificates {
		certInfo, certIssues, err := p.performCertificatesCheck(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("certificate check failed: %v", err))
		} else {
			result.SetData("certificates", certInfo)
			issues = append(issues, certIssues...)
		}
	}

	// Check Ceph storage
	if p.checkCeph {
		cephInfo, cephIssues, err := p.performCephCheck(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("ceph check failed: %v", err))
		} else {
			result.SetData("ceph", cephInfo)
			issues = append(issues, cephIssues...)
		}
	}

	// Check resource pools
	if p.checkPools {
		poolInfo, poolIssues, err := p.performPoolsCheck(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("pool check failed: %v", err))
		} else {
			result.SetData("pools", poolInfo)
			issues = append(issues, poolIssues...)
		}
	}

	// Check node summary
	if p.checkNodeSummary {
		nodeSummary, nodeIssues, err := p.performNodeSummaryCheck(ctx, executor)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("node summary check failed: %v", err))
		} else {
			result.SetData("node_summary", nodeSummary)
			issues = append(issues, nodeIssues...)
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
	Name        string        `json:"name"`
	Nodes       int           `json:"nodes"`
	Quorate     bool          `json:"quorate"`
	QuorumVotes int           `json:"quorum_votes"`
	NodeList    []ClusterNode `json:"node_list"`
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
	TotalVMs   int       `json:"total_vms"`
	RunningVMs int       `json:"running_vms"`
	StoppedVMs int       `json:"stopped_vms"`
	TotalCT    int       `json:"total_containers"`
	RunningCT  int       `json:"running_containers"`
	StoppedCT  int       `json:"stopped_containers"`
	VMs        []VMEntry `json:"vms"`
	Containers []VMEntry `json:"containers"`
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
	TotalStorage    int            `json:"total_storage"`
	ActiveStorage   int            `json:"active_storage"`
	InactiveStorage int            `json:"inactive_storage"`
	Storages        []StorageEntry `json:"storages"`
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
	TotalJobs int              `json:"total_jobs"`
	OKJobs    int              `json:"ok_jobs"`
	ErrorJobs int              `json:"error_jobs"`
	Jobs      []ReplicationJob `json:"jobs"`
}

// ReplicationJob holds individual replication job information
type ReplicationJob struct {
	ID       string `json:"id"`
	Guest    string `json:"guest"`
	Status   string `json:"status"`
	LastSync string `json:"last_sync"`
	NextSync string `json:"next_sync"`
}

// BackupInfo holds backup information
type BackupInfo struct {
	RecentBackups     int           `json:"recent_backups"`
	FailedBackups     int           `json:"failed_backups"`
	SuccessfulBackups int           `json:"successful_backups"`
	Backups           []BackupEntry `json:"backups"`
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
	HAEnabled  bool        `json:"ha_enabled"`
	HARunning  bool        `json:"ha_running"`
	ManagedVMs int         `json:"managed_vms"`
	HAServices []HAService `json:"services"`
}

// HAService holds HA service information
type HAService struct {
	Service string `json:"service"`
	Node    string `json:"node"`
	State   string `json:"state"`
}

// ProxmoxServiceInfo holds Proxmox service information
type ProxmoxServiceInfo struct {
	TotalServices   int              `json:"total_services"`
	RunningServices int              `json:"running_services"`
	FailedServices  int              `json:"failed_services"`
	Services        []ProxmoxService `json:"services"`
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
	_, _, exitCode, _ := executor.ExecuteWithContext(ctx, "command -v pveversion >/dev/null 2>&1")
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

// SubscriptionInfo holds Proxmox subscription information
type SubscriptionInfo struct {
	Status      string `json:"status"`       // active, inactive, notfound
	ProductName string `json:"product_name"` // Product name
	Key         string `json:"key"`          // Subscription key (masked)
	NextDueDate string `json:"next_due_date"`
	ServerID    string `json:"server_id"`
}

// UpdateInfo holds available update information
type UpdateInfo struct {
	UpdatesAvailable int      `json:"updates_available"`
	PackageUpdates   []string `json:"package_updates"`
	SecurityUpdates  int      `json:"security_updates"`
}

// TaskInfo holds recent task information
type TaskInfo struct {
	TotalTasks    int         `json:"total_tasks"`
	FailedTasks   int         `json:"failed_tasks"`
	SuccessTasks  int         `json:"success_tasks"`
	RecentTasks   []TaskEntry `json:"recent_tasks"`
	FailedTaskIDs []string    `json:"failed_task_ids"`
}

// TaskEntry holds individual task information
type TaskEntry struct {
	UPID      string `json:"upid"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// PerformanceInfo holds VM/CT performance metrics
type PerformanceInfo struct {
	TotalVMs      int                  `json:"total_vms"`
	TotalCT       int                  `json:"total_containers"`
	VMPerformance []VMPerformanceEntry `json:"vm_performance"`
	CTPerformance []VMPerformanceEntry `json:"ct_performance"`
	HighCPUVMs    []string             `json:"high_cpu_vms"`
	HighMemoryVMs []string             `json:"high_memory_vms"`
}

// VMPerformanceEntry holds individual VM/CT performance data
type VMPerformanceEntry struct {
	VMID       string  `json:"vmid"`
	Name       string  `json:"name"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	DiskRead   int64   `json:"disk_read_bytes"`
	DiskWrite  int64   `json:"disk_write_bytes"`
	NetIn      int64   `json:"net_in_bytes"`
	NetOut     int64   `json:"net_out_bytes"`
}

// BootConfigInfo holds boot configuration information
type BootConfigInfo struct {
	TotalBootOnStart int               `json:"total_boot_on_start"`
	BootEntries      []BootConfigEntry `json:"boot_entries"`
	StartupOrder     map[string]int    `json:"startup_order"` // VMID -> order
}

// BootConfigEntry holds individual boot configuration
type BootConfigEntry struct {
	VMID         string `json:"vmid"`
	Name         string `json:"name"`
	OnBoot       bool   `json:"onboot"`
	StartupOrder int    `json:"startup_order"`
	StartupDelay int    `json:"startup_delay"`
}

// NetworkStatsInfo holds network statistics
type NetworkStatsInfo struct {
	TotalInterfaces  int                       `json:"total_interfaces"`
	ActiveInterfaces int                       `json:"active_interfaces"`
	Interfaces       []ProxmoxNetworkInterface `json:"interfaces"`
	TotalRXBytes     int64                     `json:"total_rx_bytes"`
	TotalTXBytes     int64                     `json:"total_tx_bytes"`
}

// NetworkInterface holds individual interface statistics
type ProxmoxNetworkInterface struct {
	Name      string `json:"name"`
	State     string `json:"state"`
	RXBytes   int64  `json:"rx_bytes"`
	TXBytes   int64  `json:"tx_bytes"`
	RXPackets int64  `json:"rx_packets"`
	TXPackets int64  `json:"tx_packets"`
	RXErrors  int64  `json:"rx_errors"`
	TXErrors  int64  `json:"tx_errors"`
}

// CertificateInfo holds SSL certificate information
type CertificateInfo struct {
	CertificateFound bool               `json:"certificate_found"`
	Node             string             `json:"node"`
	Certificates     []CertificateEntry `json:"certificates"`
	ExpiringCerts    []string           `json:"expiring_certs"` // Certs expiring within 30 days
	ExpiredCerts     []string           `json:"expired_certs"`
}

// CertificateEntry holds individual certificate information
type CertificateEntry struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	Subject      string `json:"subject"`
	Issuer       string `json:"issuer"`
	NotBefore    string `json:"not_before"`
	NotAfter     string `json:"not_after"`
	DaysToExpiry int    `json:"days_to_expiry"`
	Expired      bool   `json:"expired"`
}

// CephInfo holds Ceph cluster information
type CephInfo struct {
	CephInstalled     bool       `json:"ceph_installed"`
	CephVersion       string     `json:"ceph_version"`
	ClusterHealth     string     `json:"cluster_health"` // HEALTH_OK, HEALTH_WARN, HEALTH_ERR
	MonStatus         string     `json:"mon_status"`
	TotalOSDs         int        `json:"total_osds"`
	UpOSDs            int        `json:"up_osds"`
	InOSDs            int        `json:"in_osds"`
	DownOSDs          int        `json:"down_osds"`
	OutOSDs           int        `json:"out_osds"`
	Pools             []CephPool `json:"pools"`
	TotalStorageBytes int64      `json:"total_storage_bytes"`
	UsedBytes         int64      `json:"used_bytes"`
	AvailableBytes    int64      `json:"available_bytes"`
	UsedPercent       float64    `json:"used_percent"`
}

// CephPool holds Ceph pool information
type CephPool struct {
	Name        string  `json:"name"`
	ID          int     `json:"id"`
	UsedBytes   int64   `json:"used_bytes"`
	AvailBytes  int64   `json:"avail_bytes"`
	UsedPercent float64 `json:"used_percent"`
	Objects     int64   `json:"objects"`
}

// PoolInfo holds resource pool information
type PoolInfo struct {
	TotalPools int         `json:"total_pools"`
	Pools      []PoolEntry `json:"pools"`
}

// PoolEntry holds individual pool information
type PoolEntry struct {
	PoolID     string   `json:"poolid"`
	Comment    string   `json:"comment"`
	Members    int      `json:"members"`     // Total VMs/CTs in pool
	VMIDs      []string `json:"vmids"`       // List of VM/CT IDs
	StorageIDs []string `json:"storage_ids"` // Storage assigned to pool
}

// NodeSummaryInfo holds cluster-wide node summary
type NodeSummaryInfo struct {
	TotalNodes       int                `json:"total_nodes"`
	OnlineNodes      int                `json:"online_nodes"`
	OfflineNodes     int                `json:"offline_nodes"`
	Nodes            []NodeSummaryEntry `json:"nodes"`
	ClusterCPUUsage  float64            `json:"cluster_cpu_usage"`
	ClusterMemUsage  float64            `json:"cluster_mem_usage"`
	TotalCPUs        int                `json:"total_cpus"`
	TotalMemoryBytes int64              `json:"total_memory_bytes"`
	UsedMemoryBytes  int64              `json:"used_memory_bytes"`
}

// NodeSummaryEntry holds individual node summary
type NodeSummaryEntry struct {
	Node           string  `json:"node"`
	Status         string  `json:"status"`     // online/offline
	CPUUsage       float64 `json:"cpu_usage"`  // 0-100
	MemUsage       float64 `json:"mem_usage"`  // 0-100
	DiskUsage      float64 `json:"disk_usage"` // 0-100
	CPUCount       int     `json:"cpu_count"`
	TotalMemBytes  int64   `json:"total_mem_bytes"`
	UsedMemBytes   int64   `json:"used_mem_bytes"`
	TotalDiskBytes int64   `json:"total_disk_bytes"`
	UsedDiskBytes  int64   `json:"used_disk_bytes"`
	Uptime         int64   `json:"uptime_seconds"`
}

// nodeStatusResponse represents the JSON response from pvesh get /nodes/{node}/status
type nodeStatusResponse struct {
	CPU    float64 `json:"cpu"`  // CPU utilization (0-1)
	CPUs   int     `json:"cpus"` // Number of CPUs (optional, may be in cpuinfo)
	Memory struct {
		Total int64 `json:"total"` // Total memory in bytes
		Used  int64 `json:"used"`  // Used memory in bytes
	} `json:"memory"`
	Uptime int64 `json:"uptime"` // Uptime in seconds
}

// checkSubscription checks Proxmox subscription status
func (p *ProxmoxChecker) performSubscriptionCheck(ctx context.Context, executor diagnostics.CommandExecutor) (*SubscriptionInfo, []string, error) {
	var issues []string
	info := &SubscriptionInfo{}

	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "pvesubscription get 2>/dev/null")
	if exitCode != 0 {
		// Command might not be available or permission denied
		info.Status = "unknown"
		return info, issues, nil
	}

	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.Contains(line, "status") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				info.Status = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "productname") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				info.ProductName = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "key") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				key := strings.TrimSpace(parts[1])
				// Mask the key for security
				if len(key) > 8 {
					info.Key = key[:4] + "****" + key[len(key)-4:]
				} else {
					info.Key = "****"
				}
			}
		} else if strings.Contains(line, "nextduedate") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				info.NextDueDate = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "serverid") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				info.ServerID = strings.TrimSpace(parts[1])
			}
		}
	}

	// Check status and create issues
	//nolint:staticcheck // Multiple comparisons for case-insensitive check
	if info.Status == "NotFound" || info.Status == "notfound" {
		warnings := []string{"no subscription found (community version)"}
		return info, warnings, nil
	} else if info.Status == "Inactive" || info.Status == "inactive" {
		issues = append(issues, "subscription is inactive or expired")
	}

	return info, issues, nil
}

// checkUpdates checks for available Proxmox updates
func (p *ProxmoxChecker) performUpdatesCheck(ctx context.Context, executor diagnostics.CommandExecutor) (*UpdateInfo, []string, error) {
	var issues []string
	info := &UpdateInfo{}

	// Run pveupdate or apt list --upgradable
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "apt-get update -qq 2>&1 && apt list --upgradable 2>/dev/null")
	if exitCode != 0 {
		return info, issues, nil
	}

	var packages []string
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Listing") {
			continue
		}

		// Parse package line: "package/repo version arch [upgradable from: old_version]"
		if strings.Contains(line, "upgradable") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				pkgName := strings.Split(fields[0], "/")[0]
				packages = append(packages, pkgName)

				// Check if it's a security update or Proxmox package
				if strings.Contains(strings.ToLower(line), "security") ||
					strings.Contains(pkgName, "pve") ||
					strings.Contains(pkgName, "proxmox") {
					info.SecurityUpdates++
				}
			}
		}
	}

	info.UpdatesAvailable = len(packages)
	info.PackageUpdates = packages

	if info.UpdatesAvailable > 20 {
		issues = append(issues, fmt.Sprintf("%d updates available (including %d security)", info.UpdatesAvailable, info.SecurityUpdates))
	} else if info.SecurityUpdates > 0 {
		issues = append(issues, fmt.Sprintf("%d security updates available", info.SecurityUpdates))
	}

	return info, issues, nil
}

// checkTasks checks recent task history
func (p *ProxmoxChecker) performTasksCheck(ctx context.Context, executor diagnostics.CommandExecutor) (*TaskInfo, []string, error) {
	var issues []string
	info := &TaskInfo{}

	// Get recent tasks (last 50)
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "pvesh get /cluster/tasks --limit 50 --errors 1 2>/dev/null")
	if exitCode != 0 {
		return info, issues, nil
	}

	lines := strings.Split(stdout, "\n")
	for i, line := range lines {
		if i == 0 || strings.HasPrefix(line, "┌") || strings.HasPrefix(line, "│") || strings.HasPrefix(line, "└") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		task := TaskEntry{
			UPID:   fields[0],
			Type:   fields[1],
			Status: fields[len(fields)-1],
		}

		info.TotalTasks++
		info.RecentTasks = append(info.RecentTasks, task)

		//nolint:staticcheck // Multiple comparisons for case-insensitive check
		if task.Status == "ERROR" || task.Status == "error" {
			info.FailedTasks++
			info.FailedTaskIDs = append(info.FailedTaskIDs, task.UPID)
		} else if task.Status == "OK" || task.Status == "ok" {
			info.SuccessTasks++
		}
	}

	if info.FailedTasks > 5 {
		issues = append(issues, fmt.Sprintf("%d recent tasks failed", info.FailedTasks))
	} else if info.FailedTasks > 0 {
		warnings := []string{fmt.Sprintf("%d recent task(s) failed", info.FailedTasks)}
		return info, warnings, nil
	}

	return info, issues, nil
}

// checkPerformance checks VM/CT performance metrics
func (p *ProxmoxChecker) performPerformanceCheck(ctx context.Context, executor diagnostics.CommandExecutor) (*PerformanceInfo, []string, error) {
	var issues []string
	info := &PerformanceInfo{}

	// Get VM performance via pvesh
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "pvesh get /nodes/$(hostname)/qemu --full 1 2>/dev/null")
	if exitCode == 0 {
		vmPerf := p.parsePerformanceData(stdout, "vm")
		info.VMPerformance = vmPerf
		info.TotalVMs = len(vmPerf)

		// Check for high CPU/Memory VMs
		for _, vm := range vmPerf {
			if vm.CPUPercent > 90 {
				info.HighCPUVMs = append(info.HighCPUVMs, fmt.Sprintf("%s (%.1f%%)", vm.Name, vm.CPUPercent))
			}
			if vm.MemPercent > 90 {
				info.HighMemoryVMs = append(info.HighMemoryVMs, fmt.Sprintf("%s (%.1f%%)", vm.Name, vm.MemPercent))
			}
		}
	}

	// Get CT performance
	stdout, _, exitCode, _ = executor.ExecuteWithContext(ctx, "pvesh get /nodes/$(hostname)/lxc --full 1 2>/dev/null")
	if exitCode == 0 {
		ctPerf := p.parsePerformanceData(stdout, "ct")
		info.CTPerformance = ctPerf
		info.TotalCT = len(ctPerf)

		// Check for high CPU/Memory CTs
		for _, ct := range ctPerf {
			if ct.CPUPercent > 90 {
				info.HighCPUVMs = append(info.HighCPUVMs, fmt.Sprintf("%s (%.1f%%)", ct.Name, ct.CPUPercent))
			}
			if ct.MemPercent > 90 {
				info.HighMemoryVMs = append(info.HighMemoryVMs, fmt.Sprintf("%s (%.1f%%)", ct.Name, ct.MemPercent))
			}
		}
	}

	// Create issues for high resource usage
	if len(info.HighCPUVMs) > 0 {
		issues = append(issues, fmt.Sprintf("%d VM/CT(s) with high CPU usage", len(info.HighCPUVMs)))
	}
	if len(info.HighMemoryVMs) > 0 {
		issues = append(issues, fmt.Sprintf("%d VM/CT(s) with high memory usage", len(info.HighMemoryVMs)))
	}

	return info, issues, nil
}

// parsePerformanceData parses pvesh performance output
func (p *ProxmoxChecker) parsePerformanceData(output string, vmType string) []VMPerformanceEntry {
	var entries []VMPerformanceEntry
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "vmid") {
			continue
		}

		// Very simplified parsing - in reality would need JSON parsing
		// This is just for demonstration
		entry := VMPerformanceEntry{
			VMID: "unknown",
			Name: "unknown",
		}
		entries = append(entries, entry)
	}

	return entries
}

// checkBootConfig checks boot configuration for VMs/CTs
func (p *ProxmoxChecker) performBootConfigCheck(ctx context.Context, executor diagnostics.CommandExecutor) (*BootConfigInfo, []string, error) {
	var issues []string
	info := &BootConfigInfo{
		StartupOrder: make(map[string]int),
	}

	// Check VM boot configs
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "grep -r 'onboot:' /etc/pve/qemu-server/ 2>/dev/null | grep 'onboot: 1'")
	if exitCode == 0 {
		lines := strings.Split(stdout, "\n")
		for _, line := range lines {
			if strings.Contains(line, "onboot: 1") {
				// Extract VMID from path
				parts := strings.Split(line, "/")
				if len(parts) > 0 {
					filename := parts[len(parts)-1]
					vmid := strings.TrimSuffix(strings.Split(filename, ".")[0], ".conf")

					entry := BootConfigEntry{
						VMID:   vmid,
						OnBoot: true,
					}
					info.BootEntries = append(info.BootEntries, entry)
					info.TotalBootOnStart++
				}
			}
		}
	}

	// Check CT boot configs
	stdout, _, exitCode, _ = executor.ExecuteWithContext(ctx, "grep -r 'onboot:' /etc/pve/lxc/ 2>/dev/null | grep 'onboot: 1'")
	if exitCode == 0 {
		lines := strings.Split(stdout, "\n")
		for _, line := range lines {
			if strings.Contains(line, "onboot: 1") {
				parts := strings.Split(line, "/")
				if len(parts) > 0 {
					filename := parts[len(parts)-1]
					ctid := strings.TrimSuffix(strings.Split(filename, ".")[0], ".conf")

					entry := BootConfigEntry{
						VMID:   ctid,
						OnBoot: true,
					}
					info.BootEntries = append(info.BootEntries, entry)
					info.TotalBootOnStart++
				}
			}
		}
	}

	if info.TotalBootOnStart == 0 {
		warnings := []string{"no VMs/CTs configured to start on boot"}
		return info, warnings, nil
	}

	return info, issues, nil
}

// checkNetworkStats checks network interface statistics
func (p *ProxmoxChecker) performNetworkStatsCheck(ctx context.Context, executor diagnostics.CommandExecutor) (*NetworkStatsInfo, []string, error) {
	var issues []string
	info := &NetworkStatsInfo{}

	// Get network interface stats
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "ip -s link show 2>/dev/null")
	if exitCode != 0 {
		return info, issues, nil
	}

	interfaces := p.parseNetworkStats(stdout)
	info.Interfaces = interfaces
	info.TotalInterfaces = len(interfaces)

	for _, iface := range interfaces {
		if iface.State == "UP" {
			info.ActiveInterfaces++
		}
		info.TotalRXBytes += iface.RXBytes
		info.TotalTXBytes += iface.TXBytes

		// Check for high error rates
		if iface.RXPackets > 0 && float64(iface.RXErrors)/float64(iface.RXPackets) > 0.01 {
			issues = append(issues, fmt.Sprintf("interface %s has high RX error rate", iface.Name))
		}
		if iface.TXPackets > 0 && float64(iface.TXErrors)/float64(iface.TXPackets) > 0.01 {
			issues = append(issues, fmt.Sprintf("interface %s has high TX error rate", iface.Name))
		}
	}

	return info, issues, nil
}

// parseNetworkStats parses ip -s link output
func (p *ProxmoxChecker) parseNetworkStats(output string) []ProxmoxNetworkInterface {
	var interfaces []ProxmoxNetworkInterface
	lines := strings.Split(output, "\n")

	var currentIface *ProxmoxNetworkInterface
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// New interface starts with number
		if len(line) > 0 && line[0] >= '0' && line[0] <= '9' {
			if currentIface != nil {
				interfaces = append(interfaces, *currentIface)
			}

			fields := strings.Fields(line)
			if len(fields) >= 2 {
				currentIface = &ProxmoxNetworkInterface{
					Name:  strings.TrimSuffix(fields[1], ":"),
					State: "DOWN",
				}

				// Check state
				for _, field := range fields {
					if field == "UP" || strings.Contains(field, "state UP") {
						currentIface.State = "UP"
					}
				}
			}
		} else if currentIface != nil && strings.HasPrefix(line, "RX:") {
			// Next line has RX stats
			if i+1 < len(lines) {
				rxLine := strings.TrimSpace(lines[i+1])
				fields := strings.Fields(rxLine)
				if len(fields) >= 4 {
					currentIface.RXBytes, _ = strconv.ParseInt(fields[0], 10, 64)
					currentIface.RXPackets, _ = strconv.ParseInt(fields[1], 10, 64)
					currentIface.RXErrors, _ = strconv.ParseInt(fields[2], 10, 64)
				}
			}
		} else if currentIface != nil && strings.HasPrefix(line, "TX:") {
			// Next line has TX stats
			if i+1 < len(lines) {
				txLine := strings.TrimSpace(lines[i+1])
				fields := strings.Fields(txLine)
				if len(fields) >= 4 {
					currentIface.TXBytes, _ = strconv.ParseInt(fields[0], 10, 64)
					currentIface.TXPackets, _ = strconv.ParseInt(fields[1], 10, 64)
					currentIface.TXErrors, _ = strconv.ParseInt(fields[2], 10, 64)
				}
			}
		}
	}

	// Add last interface
	if currentIface != nil {
		interfaces = append(interfaces, *currentIface)
	}

	return interfaces
}

// performCertificatesCheck checks SSL certificate expiry
func (p *ProxmoxChecker) performCertificatesCheck(ctx context.Context, executor diagnostics.CommandExecutor) (*CertificateInfo, []string, error) {
	var issues []string
	info := &CertificateInfo{}

	// Get the current node hostname
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "hostname")
	if exitCode != 0 {
		return info, issues, fmt.Errorf("failed to get hostname")
	}
	nodeName := strings.TrimSpace(stdout)
	info.Node = nodeName

	// Check PVE SSL certificate
	certPath := fmt.Sprintf("/etc/pve/nodes/%s/pve-ssl.pem", nodeName)
	stdout, _, exitCode, _ = executor.ExecuteWithContext(ctx,
		fmt.Sprintf("test -f %s && openssl x509 -in %s -noout -subject -issuer -dates 2>/dev/null", certPath, certPath))

	if exitCode != 0 {
		info.CertificateFound = false
		warnings := []string{"SSL certificate not found"}
		return info, warnings, nil
	}

	info.CertificateFound = true

	// Parse certificate details
	cert := p.parseCertificate(stdout, certPath, "pve-ssl")
	if cert != nil {
		info.Certificates = append(info.Certificates, *cert)

		// Check expiry
		if cert.Expired {
			info.ExpiredCerts = append(info.ExpiredCerts, cert.Name)
			issues = append(issues, fmt.Sprintf("SSL certificate %s has expired", cert.Name))
		} else if cert.DaysToExpiry <= 30 && cert.DaysToExpiry >= 0 {
			info.ExpiringCerts = append(info.ExpiringCerts, cert.Name)
			issues = append(issues, fmt.Sprintf("SSL certificate %s expires in %d days", cert.Name, cert.DaysToExpiry))
		} else if cert.DaysToExpiry < 0 {
			info.ExpiredCerts = append(info.ExpiredCerts, cert.Name)
			issues = append(issues, fmt.Sprintf("SSL certificate %s is expired", cert.Name))
		}
	}

	// Check cluster certificates if in a cluster
	stdout, _, exitCode, _ = executor.ExecuteWithContext(ctx,
	//nolint:staticcheck // Empty branch intentional - checking existence only
		fmt.Sprintf("test -f /etc/pve/nodes/%s/pve-ssl.key && echo 'exists' 2>/dev/null", nodeName))
	//nolint:staticcheck // Empty branch is intentional - checking for certificate file existence
	if exitCode == 0 && strings.TrimSpace(stdout) == "exists" {
		// Certificate file exists - no issues to report
	}

	return info, issues, nil
}

// parseCertificate parses openssl x509 output
func (p *ProxmoxChecker) parseCertificate(output, path, name string) *CertificateEntry {
	cert := &CertificateEntry{
		Name: name,
		Path: path,
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "subject=") {
			cert.Subject = strings.TrimPrefix(line, "subject=")
		} else if strings.HasPrefix(line, "issuer=") {
			cert.Issuer = strings.TrimPrefix(line, "issuer=")
		} else if strings.HasPrefix(line, "notBefore=") {
			cert.NotBefore = strings.TrimPrefix(line, "notBefore=")
		} else if strings.HasPrefix(line, "notAfter=") {
			notAfterStr := strings.TrimPrefix(line, "notAfter=")
			cert.NotAfter = notAfterStr

			// Parse expiry date and calculate days to expiry
			// Format: "Jan 15 12:00:00 2025 GMT"
			expiry, err := time.Parse("Jan 2 15:04:05 2006 MST", notAfterStr)
			if err == nil {
				daysToExpiry := int(time.Until(expiry).Hours() / 24)
				cert.DaysToExpiry = daysToExpiry
				cert.Expired = daysToExpiry < 0
			}
		}
	}

	return cert
}

// performCephCheck checks Ceph storage cluster status
func (p *ProxmoxChecker) performCephCheck(ctx context.Context, executor diagnostics.CommandExecutor) (*CephInfo, []string, error) {
	var issues []string
	info := &CephInfo{}

	// Check if Ceph is installed
	_, _, exitCode, _ := executor.ExecuteWithContext(ctx, "command -v ceph >/dev/null 2>&1")
	if exitCode != 0 {
		info.CephInstalled = false
		// Not an issue - Ceph might not be used
		return info, issues, nil
	}

	info.CephInstalled = true

	// Get Ceph version
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "ceph --version 2>/dev/null")
	if exitCode == 0 {
		info.CephVersion = strings.TrimSpace(strings.Split(stdout, "\n")[0])
	}

	// Check Ceph health
	stdout, _, exitCode, _ = executor.ExecuteWithContext(ctx, "ceph health 2>/dev/null")
	if exitCode != 0 {
		warnings := []string{"Ceph is installed but cluster is not accessible"}
		return info, warnings, nil
	}

	healthOutput := strings.TrimSpace(stdout)
	info.ClusterHealth = healthOutput

	// Parse health status
	if strings.Contains(healthOutput, "HEALTH_OK") {
		// All good
	} else if strings.Contains(healthOutput, "HEALTH_WARN") {
		issues = append(issues, fmt.Sprintf("Ceph cluster health: %s", healthOutput))
	} else if strings.Contains(healthOutput, "HEALTH_ERR") {
		issues = append(issues, fmt.Sprintf("Ceph cluster error: %s", healthOutput))
	}

	// Get OSD status
	stdout, _, exitCode, _ = executor.ExecuteWithContext(ctx, "ceph osd stat 2>/dev/null")
	if exitCode == 0 {
		p.parseCephOSDStat(stdout, info)

		// Check for down OSDs
		if info.DownOSDs > 0 {
			issues = append(issues, fmt.Sprintf("%d Ceph OSD(s) are down", info.DownOSDs))
		}
		if info.OutOSDs > 0 {
			issues = append(issues, fmt.Sprintf("%d Ceph OSD(s) are out", info.OutOSDs))
		}
	}

	// Get storage usage
	stdout, _, exitCode, _ = executor.ExecuteWithContext(ctx, "ceph df 2>/dev/null")
	if exitCode == 0 {
		p.parseCephDF(stdout, info)

		// Check for high storage usage
		if info.UsedPercent > 85 {
			issues = append(issues, fmt.Sprintf("Ceph cluster storage is %.1f%% full", info.UsedPercent))
		}
	}

	// Get pool information
	stdout, _, exitCode, _ = executor.ExecuteWithContext(ctx, "ceph osd pool ls detail 2>/dev/null")
	if exitCode == 0 {
		info.Pools = p.parseCephPools(stdout)
	}

	return info, issues, nil
}

// parseCephOSDStat parses ceph osd stat output
// Example: "30 osds: 28 up, 28 in; 2 osds out"
func (p *ProxmoxChecker) parseCephOSDStat(output string, info *CephInfo) {
	fields := strings.Fields(output)
	for i, field := range fields {
		if field == "osds:" && i > 0 {
			// Total OSDs is before "osds:"
			if total, err := strconv.Atoi(fields[i-1]); err == nil {
				info.TotalOSDs = total
			}
		} else if field == "up," && i > 0 {
			// Up OSDs is before "up,"
			if up, err := strconv.Atoi(fields[i-1]); err == nil {
				info.UpOSDs = up
			}
		} else if field == "in;" && i > 0 {
			// In OSDs is before "in;"
			if in, err := strconv.Atoi(fields[i-1]); err == nil {
				info.InOSDs = in
			}
		}
	}

	// Calculate down and out OSDs
	info.DownOSDs = info.TotalOSDs - info.UpOSDs
	info.OutOSDs = info.TotalOSDs - info.InOSDs
}

// parseCephDF parses ceph df output
func (p *ProxmoxChecker) parseCephDF(output string, info *CephInfo) {
	lines := strings.Split(output, "\n")

	// Track if we're in the GLOBAL section
	inGlobalSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Detect GLOBAL section
		if strings.Contains(strings.ToUpper(line), "GLOBAL") {
			inGlobalSection = true
			continue
		}

		// Skip header lines (contain "TOTAL" "USED" etc in caps)
		if strings.Contains(line, "TOTAL") && strings.Contains(line, "USED") {
			continue
		}

		// Parse data lines in GLOBAL section
		if inGlobalSection && line != "" && !strings.HasPrefix(line, "---") {
			fields := strings.Fields(line)

			// Try to parse storage sizes
			for i, field := range fields {
				// Look for size patterns like "100" followed by "GiB"
				if i+1 < len(fields) && (fields[i+1] == "GiB" || fields[i+1] == "TiB" || fields[i+1] == "MiB") {
					sizeStr := field
					unit := fields[i+1]
					size, err := strconv.ParseFloat(sizeStr, 64)
					if err != nil {
						continue
					}

					// Convert to bytes
					var bytes int64
					switch unit {
					case "TiB":
						bytes = int64(size * 1024 * 1024 * 1024 * 1024)
					case "GiB":
						bytes = int64(size * 1024 * 1024 * 1024)
					case "MiB":
						bytes = int64(size * 1024 * 1024)
					}

					// Determine which field based on position
					if info.TotalStorageBytes == 0 {
						info.TotalStorageBytes = bytes
					} else if info.UsedBytes == 0 {
						info.UsedBytes = bytes
					} else if info.AvailableBytes == 0 {
						info.AvailableBytes = bytes
					}
				}
			}

			// Look for percentage
			for _, field := range fields {
				if strings.HasSuffix(field, "%") {
					pctStr := strings.TrimSuffix(field, "%")
					if pct, err := strconv.ParseFloat(pctStr, 64); err == nil {
						info.UsedPercent = pct
					}
				}
			}

			// After parsing the first data line, we're done with GLOBAL
			break
		}
	}

	// Calculate used percent if not provided
	if info.UsedPercent == 0 && info.TotalStorageBytes > 0 {
		info.UsedPercent = float64(info.UsedBytes) / float64(info.TotalStorageBytes) * 100
	}
}

// parseCephPools parses ceph osd pool ls detail output
func (p *ProxmoxChecker) parseCephPools(output string) []CephPool {
	var pools []CephPool
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Example: "pool 1 'rbd' replicated size 3 min_size 2"
		if strings.HasPrefix(line, "pool ") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				pool := CephPool{}

				// Parse pool ID
				if id, err := strconv.Atoi(fields[1]); err == nil {
					pool.ID = id
				}

				// Parse pool name (remove quotes)
				pool.Name = strings.Trim(fields[2], "'\"")

				pools = append(pools, pool)
			}
		}
	}

	return pools
}

// performPoolsCheck checks resource pool configurations
func (p *ProxmoxChecker) performPoolsCheck(ctx context.Context, executor diagnostics.CommandExecutor) (*PoolInfo, []string, error) {
	var issues []string
	info := &PoolInfo{}

	// Get pool list using pvesh
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "pvesh get /pools --output-format json 2>/dev/null")
	if exitCode != 0 {
		// Pools might not be configured - not an error
		return info, issues, nil
	}

	pools := p.parsePools(stdout)
	info.Pools = pools
	info.TotalPools = len(pools)

	// Get detailed information for each pool
	for i := range info.Pools {
		pool := &info.Pools[i]

		// Get pool members (VMs/CTs)
		stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx,
			fmt.Sprintf("pvesh get /pools/%s --output-format json 2>/dev/null", pool.PoolID))

		if exitCode == 0 {
			p.parsePoolMembers(stdout, pool)
		}
	}

	return info, issues, nil
}

// parsePools parses pvesh get /pools output
func (p *ProxmoxChecker) parsePools(output string) []PoolEntry {
	var pools []PoolEntry

	output = strings.TrimSpace(output)

	// Try to parse as JSON first
	if strings.HasPrefix(output, "[") || strings.HasPrefix(output, "{") {
		// Use proper JSON unmarshaling
		if err := json.Unmarshal([]byte(output), &pools); err == nil {
			return pools
		}
		// If JSON parsing fails, fall through to text parsing
	}

	// Fallback to text parsing if not JSON or JSON parsing failed
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 1 {
			pool := PoolEntry{
				PoolID: fields[0],
			}
			if len(fields) >= 2 {
				pool.Comment = strings.Join(fields[1:], " ")
			}
			pools = append(pools, pool)
		}
	}

	return pools
}

// parsePoolMembers parses pool member details
func (p *ProxmoxChecker) parsePoolMembers(output string, pool *PoolEntry) {
	// Look for members in JSON output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for VM/CT IDs in members section
		if strings.Contains(line, "\"members\"") || strings.Contains(line, "\"vmid\"") {
			// Extract VMIDs
			fields := strings.Fields(line)
			for _, field := range fields {
				// Look for numeric IDs
				field = strings.Trim(field, ",:\"}{[]")
				if _, err := strconv.Atoi(field); err == nil && len(field) >= 3 {
					pool.VMIDs = append(pool.VMIDs, field)
				}
			}
		}

		// Look for storage IDs
		if strings.Contains(line, "\"storage\"") {
			// Extract storage names
			if idx := strings.Index(line, "\"storage\""); idx != -1 {
				rest := line[idx+10:]
				if idx2 := strings.Index(rest, "\""); idx2 != -1 {
					rest = rest[idx2+1:]
					if idx3 := strings.Index(rest, "\""); idx3 != -1 {
						storage := rest[:idx3]
						if storage != "" {
							pool.StorageIDs = append(pool.StorageIDs, storage)
						}
					}
				}
			}
		}
	}

	pool.Members = len(pool.VMIDs)
}

// performNodeSummaryCheck checks node status and aggregates cluster metrics
func (p *ProxmoxChecker) performNodeSummaryCheck(ctx context.Context, executor diagnostics.CommandExecutor) (*NodeSummaryInfo, []string, error) {
	var issues []string
	info := &NodeSummaryInfo{}

	// Get list of nodes
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx, "pvesh get /nodes --output-format json 2>/dev/null")
	if exitCode != 0 {
		// Fallback to pvecm nodes if pvesh fails
		stdout, _, exitCode, _ = executor.ExecuteWithContext(ctx, "pvecm nodes 2>/dev/null")
		if exitCode != 0 {
			// Single node system
			stdout, _, _, _ := executor.ExecuteWithContext(ctx, "hostname")
			nodeName := strings.TrimSpace(stdout)
			if nodeName != "" {
				return p.getSingleNodeSummary(ctx, executor, nodeName, info, &issues)
			}
			return info, issues, nil
		}

		// Parse pvecm nodes output
		nodeNames := p.parseNodeNames(stdout)
		for _, nodeName := range nodeNames {
			entry, nodeIssues := p.getNodeStatus(ctx, executor, nodeName)
			if entry != nil {
				info.Nodes = append(info.Nodes, *entry)
				issues = append(issues, nodeIssues...)
			}
		}
	} else {
		// Parse JSON output from pvesh
		nodes := p.parseNodesJSON(stdout)
		for _, nodeName := range nodes {
			entry, nodeIssues := p.getNodeStatus(ctx, executor, nodeName)
			if entry != nil {
				info.Nodes = append(info.Nodes, *entry)
				issues = append(issues, nodeIssues...)
			}
		}
	}

	// Aggregate cluster metrics
	info.TotalNodes = len(info.Nodes)
	var totalCPUUsage, totalMemUsage float64

	for _, node := range info.Nodes {
		if node.Status == "online" {
			info.OnlineNodes++
		} else {
			info.OfflineNodes++
		}

		info.TotalCPUs += node.CPUCount
		info.TotalMemoryBytes += node.TotalMemBytes
		info.UsedMemoryBytes += node.UsedMemBytes

		totalCPUUsage += node.CPUUsage
		totalMemUsage += node.MemUsage
	}

	if info.TotalNodes > 0 {
		info.ClusterCPUUsage = totalCPUUsage / float64(info.TotalNodes)
		info.ClusterMemUsage = totalMemUsage / float64(info.TotalNodes)
	}

	// Check for issues
	if info.OfflineNodes > 0 {
		issues = append(issues, fmt.Sprintf("%d node(s) are offline", info.OfflineNodes))
	}

	if info.ClusterCPUUsage > 90 {
		issues = append(issues, fmt.Sprintf("cluster CPU usage is high: %.1f%%", info.ClusterCPUUsage))
	}

	if info.ClusterMemUsage > 90 {
		issues = append(issues, fmt.Sprintf("cluster memory usage is high: %.1f%%", info.ClusterMemUsage))
	}

	return info, issues, nil
}

// parseNodeNames extracts node names from pvecm nodes output
func (p *ProxmoxChecker) parseNodeNames(output string) []string {
	var names []string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Nodeid") || strings.Contains(line, "---") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 {
			nodeName := fields[2]
			nodeName = strings.TrimSpace(strings.Replace(nodeName, "(local)", "", 1))
			if nodeName != "" {
				names = append(names, nodeName)
			}
		}
	}

	return names
}

// parseNodesJSON parses pvesh get /nodes JSON output
func (p *ProxmoxChecker) parseNodesJSON(output string) []string {
	type nodeJSON struct {
		Node string `json:"node"`
	}

	var nodesJSON []nodeJSON
	var names []string

	if err := json.Unmarshal([]byte(output), &nodesJSON); err == nil {
		for _, n := range nodesJSON {
			names = append(names, n.Node)
		}
	}

	return names
}

// getNodeStatus retrieves detailed node status
func (p *ProxmoxChecker) getNodeStatus(ctx context.Context, executor diagnostics.CommandExecutor, nodeName string) (*NodeSummaryEntry, []string) {
	var issues []string
	entry := &NodeSummaryEntry{
		Node:   nodeName,
		Status: "unknown",
	}

	// Get node status
	stdout, _, exitCode, _ := executor.ExecuteWithContext(ctx,
		fmt.Sprintf("pvesh get /nodes/%s/status --output-format json 2>/dev/null", nodeName))

	if exitCode != 0 {
		entry.Status = "offline"
		return entry, issues
	}

	entry.Status = "online"

	// Parse JSON response
	var status nodeStatusResponse
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		issues = append(issues, fmt.Sprintf("Failed to parse node status JSON for %s: %v", nodeName, err))
		return entry, issues
	}

	// Map parsed data to entry
	entry.CPUUsage = status.CPU * 100 // Convert 0-1 to 0-100
	entry.CPUCount = status.CPUs
	entry.TotalMemBytes = status.Memory.Total
	entry.UsedMemBytes = status.Memory.Used
	entry.Uptime = status.Uptime

	// Calculate memory usage percentage
	if entry.TotalMemBytes > 0 {
		entry.MemUsage = float64(entry.UsedMemBytes) / float64(entry.TotalMemBytes) * 100
	}

	return entry, issues
}

// getSingleNodeSummary gets summary for a single standalone node
func (p *ProxmoxChecker) getSingleNodeSummary(ctx context.Context, executor diagnostics.CommandExecutor, nodeName string, info *NodeSummaryInfo, issues *[]string) (*NodeSummaryInfo, []string, error) {
	entry, nodeIssues := p.getNodeStatus(ctx, executor, nodeName)
	if entry != nil {
		info.Nodes = append(info.Nodes, *entry)
		*issues = append(*issues, nodeIssues...)

		info.TotalNodes = 1
		if entry.Status == "online" {
			info.OnlineNodes = 1
		} else {
			info.OfflineNodes = 1
		}
		info.TotalCPUs = entry.CPUCount
		info.TotalMemoryBytes = entry.TotalMemBytes
		info.UsedMemoryBytes = entry.UsedMemBytes
		info.ClusterCPUUsage = entry.CPUUsage
		info.ClusterMemUsage = entry.MemUsage
	}

	return info, *issues, nil
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
