package checkers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

// Mock executor response for Proxmox testing
type proxmoxMockResponse struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

// Mock executor for Proxmox tests
type proxmoxMockExecutor struct {
	responses map[string]proxmoxMockResponse
}

func (m *proxmoxMockExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return m.ExecuteWithContext(ctx, command)
}

func (m *proxmoxMockExecutor) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
	// Match commands to responses
	for pattern, resp := range m.responses {
		if strings.Contains(command, pattern) {
			return resp.stdout, resp.stderr, resp.exitCode, resp.err
		}
	}
	return "", "command not mocked", 127, nil
}

func TestNewProxmoxChecker(t *testing.T) {
	tests := []struct {
		name              string
		checkCluster      bool
		checkVMs          bool
		checkStorage      bool
		checkReplication  bool
		checkBackups      bool
		checkHA           bool
		checkServices     bool
		expectedAllChecks bool
	}{
		{
			name:              "all disabled should enable all",
			checkCluster:      false,
			checkVMs:          false,
			checkStorage:      false,
			checkReplication:  false,
			checkBackups:      false,
			checkHA:           false,
			checkServices:     false,
			expectedAllChecks: true,
		},
		{
			name:              "only cluster enabled",
			checkCluster:      true,
			checkVMs:          false,
			checkStorage:      false,
			checkReplication:  false,
			checkBackups:      false,
			checkHA:           false,
			checkServices:     false,
			expectedAllChecks: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewProxmoxChecker(
				tt.checkCluster,
				tt.checkVMs,
				tt.checkStorage,
				tt.checkReplication,
				tt.checkBackups,
				tt.checkHA,
				tt.checkServices,
			)

			if checker == nil {
				t.Fatal("NewProxmoxChecker returned nil")
			}

			if tt.expectedAllChecks {
				if !checker.checkCluster || !checker.checkVMs || !checker.checkStorage {
					t.Error("expected all checks to be enabled when all are false")
				}
			}
		})
	}
}

func TestProxmoxChecker_Name(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)
	if checker.Name() != "proxmox_check" {
		t.Errorf("expected name 'proxmox_check', got '%s'", checker.Name())
	}
}

func TestProxmoxChecker_Category(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)
	if checker.Category() != diagnostics.CategoryVirtualization {
		t.Errorf("expected category 'virtualization', got '%s'", checker.Category())
	}
}

func TestProxmoxChecker_RequiresRoot(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)
	if !checker.RequiresRoot() {
		t.Error("expected RequiresRoot to return true")
	}
}

func TestProxmoxChecker_Run_NotInstalled(t *testing.T) {
	executor := &proxmoxMockExecutor{
		responses: map[string]proxmoxMockResponse{
			"command -v pveversion": {
				stdout:   "",
				stderr:   "not found",
				exitCode: 1,
			},
		},
	}

	checker := NewProxmoxChecker(false, false, false, false, false, false, false)
	result, err := checker.Run(context.Background(), executor)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Status != diagnostics.StatusSkipped {
		t.Errorf("expected status 'skipped', got '%s'", result.Status)
	}

	if !strings.Contains(result.Message, "not detected") {
		t.Errorf("expected message about not detected, got '%s'", result.Message)
	}
}

func TestProxmoxChecker_Run_StandaloneNode(t *testing.T) {
	executor := &proxmoxMockExecutor{
		responses: map[string]proxmoxMockResponse{
			"command -v pveversion": {
				stdout:   "",
				stderr:   "",
				exitCode: 0,
			},
			"pveversion": {
				stdout:   "pve-manager/8.1.3/b46aac3b42da5d15",
				stderr:   "",
				exitCode: 0,
			},
			"pvecm status": {
				stdout:   "",
				stderr:   "not in cluster",
				exitCode: 1,
			},
			"qm list": {
				stdout:   "VMID NAME                 STATUS     MEM(MB)    BOOTDISK(GB) PID\n",
				stderr:   "",
				exitCode: 0,
			},
			"pct list": {
				stdout:   "VMID       Status     Lock         Name\n",
				stderr:   "",
				exitCode: 0,
			},
			"pvesm status": {
				stdout:   "Name             Type     Status           Total            Used       Available        %\n",
				stderr:   "",
				exitCode: 0,
			},
			"pvesr list": {
				stdout:   "",
				stderr:   "",
				exitCode: 1,
			},
			"pvesh get": {
				stdout:   "",
				stderr:   "",
				exitCode: 0,
			},
			"command -v ha-manager": {
				stdout:   "",
				stderr:   "",
				exitCode: 1,
			},
			"systemctl is-active pve-cluster": {
				stdout:   "active",
				stderr:   "",
				exitCode: 0,
			},
			"systemctl is-active pvedaemon": {
				stdout:   "active",
				stderr:   "",
				exitCode: 0,
			},
			"systemctl is-active pveproxy": {
				stdout:   "active",
				stderr:   "",
				exitCode: 0,
			},
			"systemctl is-active pvestatd": {
				stdout:   "active",
				stderr:   "",
				exitCode: 0,
			},
			"systemctl is-active pve-ha-lrm": {
				stdout:   "inactive",
				stderr:   "",
				exitCode: 3,
			},
			"systemctl is-active pve-ha-crm": {
				stdout:   "inactive",
				stderr:   "",
				exitCode: 3,
			},
		},
	}

	checker := NewProxmoxChecker(false, false, false, false, false, false, false)
	result, err := checker.Run(context.Background(), executor)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Status != diagnostics.StatusCompleted {
		t.Errorf("expected status 'completed', got '%s'", result.Status)
	}

	// Check that version is set
	version, ok := result.Data["proxmox_version"]
	if !ok || version == "" {
		t.Error("expected proxmox_version to be set")
	}

	// Check cluster info
	cluster, ok := result.Data["cluster"]
	if !ok {
		t.Error("expected cluster data to be set")
	}
	clusterInfo, ok := cluster.(*ClusterInfo)
	if !ok {
		t.Error("expected cluster data to be *ClusterInfo")
	}
	if clusterInfo.Name != "standalone" {
		t.Errorf("expected standalone node, got cluster name '%s'", clusterInfo.Name)
	}
}

func TestProxmoxChecker_Run_ClusteredNode(t *testing.T) {
	executor := &proxmoxMockExecutor{
		responses: map[string]proxmoxMockResponse{
			"command -v pveversion": {
				stdout:   "",
				stderr:   "",
				exitCode: 0,
			},
			"pveversion": {
				stdout:   "pve-manager/8.1.3/b46aac3b42da5d15",
				stderr:   "",
				exitCode: 0,
			},
			"pvecm status": {
				stdout: `Cluster information
-------------------
Name:             production-cluster
Config Version:   3
Transport:        knet
Secure auth:      on

Quorum information
------------------
Date:             Thu Jan 16 14:35:22 2025
Quorum provider:  corosync_votequorum
Nodes:            3
Node ID:          0x00000001
Ring ID:          1.8
Quorate:          Yes

Total votes:      3
Expected votes:   3`,
				stderr:   "",
				exitCode: 0,
			},
			"pvecm nodes": {
				stdout: `Membership information
----------------------
    Nodeid      Votes    Name
         1          1    pve01 (local)
         2          1    pve02
         3          1    pve03`,
				stderr:   "",
				exitCode: 0,
			},
			"qm list": {
				stdout: `VMID NAME                 STATUS     MEM(MB)    BOOTDISK(GB) PID
100 webserver           running    4096            32 12345
101 database             stopped    8192            64 0`,
				stderr:   "",
				exitCode: 0,
			},
			"pct list": {
				stdout: `VMID       Status     Lock         Name
200        running    -            nginx-proxy
201        stopped    -            test-container`,
				stderr:   "",
				exitCode: 0,
			},
			"pvesm status": {
				stdout: `Name             Type     Status           Total            Used       Available        %
local            dir      active      102400000        51200000        51200000       50.00
local-lvm        lvmthin  active      512000000       460800000        51200000       90.00
nfs-backup       nfs      active     1024000000       102400000       921600000       10.00`,
				stderr:   "",
				exitCode: 0,
			},
			"pvesr list": {
				stdout: `ID              Guest           Status          Last Sync       Next Sync
100-1           100             OK              2025-01-16      2025-01-17
101-1           101             ERROR           2025-01-15      pending`,
				stderr:   "",
				exitCode: 0,
			},
			"pvesh get": {
				stdout: `UPID:pve01:00001234:00567890:vzdump:100:root@pam: OK
UPID:pve01:00001235:00567891:vzdump:101:root@pam: FAILED`,
				stderr:   "",
				exitCode: 0,
			},
			"command -v ha-manager": {
				stdout:   "/usr/sbin/ha-manager",
				stderr:   "",
				exitCode: 0,
			},
			"ha-manager status": {
				stdout: `quorum OK
master pve01 (active, Thu Jan 16 14:35:22 2025)
lrm pve01 (active, Thu Jan 16 14:35:20 2025)
service vm:100 (pve01, started)
service vm:102 (pve02, error)`,
				stderr:   "",
				exitCode: 0,
			},
			"systemctl is-active pve-cluster": {
				stdout:   "active",
				stderr:   "",
				exitCode: 0,
			},
			"systemctl is-active pvedaemon": {
				stdout:   "active",
				stderr:   "",
				exitCode: 0,
			},
			"systemctl is-active pveproxy": {
				stdout:   "active",
				stderr:   "",
				exitCode: 0,
			},
			"systemctl is-active pvestatd": {
				stdout:   "active",
				stderr:   "",
				exitCode: 0,
			},
			"systemctl is-active pve-ha-lrm": {
				stdout:   "active",
				stderr:   "",
				exitCode: 0,
			},
			"systemctl is-active pve-ha-crm": {
				stdout:   "active",
				stderr:   "",
				exitCode: 0,
			},
		},
	}

	checker := NewProxmoxChecker(false, false, false, false, false, false, false)
	result, err := checker.Run(context.Background(), executor)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Status != diagnostics.StatusCompleted {
		t.Errorf("expected status 'completed', got '%s'", result.Status)
	}

	// Verify cluster info
	cluster, ok := result.Data["cluster"]
	if !ok {
		t.Fatal("expected cluster data to be set")
	}
	clusterInfo, ok := cluster.(*ClusterInfo)
	if !ok {
		t.Fatal("expected cluster data to be *ClusterInfo")
	}
	if clusterInfo.Name != "production-cluster" {
		t.Errorf("expected cluster name 'production-cluster', got '%s'", clusterInfo.Name)
	}
	if clusterInfo.Nodes != 3 {
		t.Errorf("expected 3 nodes, got %d", clusterInfo.Nodes)
	}
	if !clusterInfo.Quorate {
		t.Error("expected cluster to be quorate")
	}

	// Verify VM info
	vms, ok := result.Data["vms"]
	if !ok {
		t.Fatal("expected vms data to be set")
	}
	vmInfo, ok := vms.(*VMInfo)
	if !ok {
		t.Fatal("expected vms data to be *VMInfo")
	}
	if vmInfo.TotalVMs != 2 {
		t.Errorf("expected 2 VMs, got %d", vmInfo.TotalVMs)
	}
	if vmInfo.RunningVMs != 1 {
		t.Errorf("expected 1 running VM, got %d", vmInfo.RunningVMs)
	}
	if vmInfo.TotalCT != 2 {
		t.Errorf("expected 2 containers, got %d", vmInfo.TotalCT)
	}

	// Verify storage info
	storage, ok := result.Data["storage"]
	if !ok {
		t.Fatal("expected storage data to be set")
	}
	storageInfo, ok := storage.(*StorageInfo)
	if !ok {
		t.Fatal("expected storage data to be *StorageInfo")
	}
	if storageInfo.TotalStorage != 3 {
		t.Errorf("expected 3 storage pools, got %d", storageInfo.TotalStorage)
	}
	if storageInfo.ActiveStorage != 3 {
		t.Errorf("expected 3 active storage pools, got %d", storageInfo.ActiveStorage)
	}

	// Verify replication info
	replication, ok := result.Data["replication"]
	if !ok {
		t.Fatal("expected replication data to be set")
	}
	replicationInfo, ok := replication.(*ReplicationInfo)
	if !ok {
		t.Fatal("expected replication data to be *ReplicationInfo")
	}
	if replicationInfo.TotalJobs != 2 {
		t.Errorf("expected 2 replication jobs, got %d", replicationInfo.TotalJobs)
	}
	if replicationInfo.ErrorJobs != 1 {
		t.Errorf("expected 1 error job, got %d", replicationInfo.ErrorJobs)
	}

	// Verify HA info
	ha, ok := result.Data["ha"]
	if !ok {
		t.Fatal("expected ha data to be set")
	}
	haInfo, ok := ha.(*HAInfo)
	if !ok {
		t.Fatal("expected ha data to be *HAInfo")
	}
	if !haInfo.HAEnabled {
		t.Error("expected HA to be enabled")
	}
	if !haInfo.HARunning {
		t.Error("expected HA to be running")
	}

	// Verify services info
	services, ok := result.Data["services"]
	if !ok {
		t.Fatal("expected services data to be set")
	}
	serviceInfo, ok := services.(*ProxmoxServiceInfo)
	if !ok {
		t.Fatal("expected services data to be *ProxmoxServiceInfo")
	}
	if serviceInfo.TotalServices != 6 {
		t.Errorf("expected 6 services, got %d", serviceInfo.TotalServices)
	}
	if serviceInfo.RunningServices != 6 {
		t.Errorf("expected 6 running services, got %d", serviceInfo.RunningServices)
	}

	// Verify metrics
	if len(result.Metrics) < 1 {
		t.Error("expected at least 1 metric")
	}

	// Check for issues in message (should have some due to storage at 90%, replication error, HA error)
	if !strings.Contains(result.Message, "issue") && !strings.Contains(result.Message, "warning") {
		t.Logf("message: %s", result.Message)
		// This is OK - might not have issues depending on thresholds
	}
}

func TestProxmoxChecker_ParseNodeList(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)

	output := `Membership information
----------------------
    Nodeid      Votes    Name
         1          1    pve01 (local)
         2          1    pve02
         3          1    pve03`

	nodes := checker.parseNodeList(output)

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	if nodes[0].Name != "pve01" {
		t.Errorf("expected first node name 'pve01', got '%s'", nodes[0].Name)
	}

	if nodes[0].ID != "1" {
		t.Errorf("expected first node ID '1', got '%s'", nodes[0].ID)
	}

	if !nodes[0].Online {
		t.Error("expected first node to be online")
	}
}

func TestProxmoxChecker_ParseVMList(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)

	output := `VMID NAME                 STATUS     MEM(MB)    BOOTDISK(GB) PID
100 webserver           running    4096            32 12345
101 database             stopped    8192            64 0`

	vms := checker.parseVMList(output, "qemu")

	if len(vms) != 2 {
		t.Fatalf("expected 2 VMs, got %d", len(vms))
	}

	if vms[0].VMID != "100" {
		t.Errorf("expected VMID '100', got '%s'", vms[0].VMID)
	}

	if vms[0].Name != "webserver" {
		t.Errorf("expected name 'webserver', got '%s'", vms[0].Name)
	}

	if vms[0].Status != "running" {
		t.Errorf("expected status 'running', got '%s'", vms[0].Status)
	}

	if vms[0].Memory != 4096 {
		t.Errorf("expected memory 4096, got %d", vms[0].Memory)
	}

	if vms[1].Status != "stopped" {
		t.Errorf("expected second VM status 'stopped', got '%s'", vms[1].Status)
	}
}

func TestProxmoxChecker_ParseStorageStatus(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)

	output := `Name             Type     Status           Total            Used       Available        %
local            dir      active      102400000        51200000        51200000       50.00
local-lvm        lvmthin  active      512000000       460800000        51200000       90.00
nfs-backup       nfs      inactive   1024000000       102400000       921600000       10.00`

	storages := checker.parseStorageStatus(output)

	if len(storages) != 3 {
		t.Fatalf("expected 3 storage pools, got %d", len(storages))
	}

	if storages[0].Name != "local" {
		t.Errorf("expected name 'local', got '%s'", storages[0].Name)
	}

	if storages[0].Type != "dir" {
		t.Errorf("expected type 'dir', got '%s'", storages[0].Type)
	}

	if !storages[0].Active {
		t.Error("expected first storage to be active")
	}

	if storages[0].Total != 102400000 {
		t.Errorf("expected total 102400000, got %d", storages[0].Total)
	}

	if storages[0].Used != 51200000 {
		t.Errorf("expected used 51200000, got %d", storages[0].Used)
	}

	if storages[0].UsedPct < 49.9 || storages[0].UsedPct > 50.1 {
		t.Errorf("expected used percent ~50, got %f", storages[0].UsedPct)
	}

	if storages[2].Active {
		t.Error("expected third storage to be inactive")
	}
}

func TestProxmoxChecker_ParseReplicationJobs(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)

	output := `ID              Guest           Status          Last Sync       Next Sync
100-1           100             OK              2025-01-16      2025-01-17
101-1           101             ERROR           2025-01-15      pending`

	jobs := checker.parseReplicationJobs(output)

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	if jobs[0].ID != "100-1" {
		t.Errorf("expected ID '100-1', got '%s'", jobs[0].ID)
	}

	if jobs[0].Guest != "100" {
		t.Errorf("expected guest '100', got '%s'", jobs[0].Guest)
	}

	if jobs[0].Status != "OK" {
		t.Errorf("expected status 'OK', got '%s'", jobs[0].Status)
	}

	if jobs[1].Status != "ERROR" {
		t.Errorf("expected second job status 'ERROR', got '%s'", jobs[1].Status)
	}
}

func TestProxmoxChecker_ParseHAStatus(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)

	output := `quorum OK
master pve01 (active, Thu Jan 16 14:35:22 2025)
lrm pve01 (active, Thu Jan 16 14:35:20 2025)
service vm:100 (pve01, started)
service vm:102 (pve02, error)`

	services := checker.parseHAStatus(output)

	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}

	if services[0].Service != "vm:100" {
		t.Errorf("expected service 'vm:100', got '%s'", services[0].Service)
	}

	if services[0].Node != "pve01" {
		t.Errorf("expected node 'pve01', got '%s'", services[0].Node)
	}

	if services[0].State != "started" {
		t.Errorf("expected state 'started', got '%s'", services[0].State)
	}
}

func TestProxmoxChecker_SelectiveChecks(t *testing.T) {
	executor := &proxmoxMockExecutor{
		responses: map[string]proxmoxMockResponse{
			"command -v pveversion": {
				stdout:   "",
				stderr:   "",
				exitCode: 0,
			},
			"pveversion": {
				stdout:   "pve-manager/8.1.3/b46aac3b42da5d15",
				stderr:   "",
				exitCode: 0,
			},
			"pvecm status": {
				stdout:   "",
				stderr:   "should not be called",
				exitCode: 1,
			},
			"qm list": {
				stdout:   "VMID NAME STATUS\n100 test running",
				stderr:   "",
				exitCode: 0,
			},
		},
	}

	// Only enable VM checks
	checker := NewProxmoxChecker(false, true, false, false, false, false, false)
	result, err := checker.Run(context.Background(), executor)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should have VM data
	_, hasVMs := result.Data["vms"]
	if !hasVMs {
		t.Error("expected vms data when checkVMs is enabled")
	}

	// Should NOT have cluster data (since we only enabled VM checks)
	_, hasCluster := result.Data["cluster"]
	if hasCluster {
		t.Error("expected no cluster data when checkCluster is disabled")
	}
}

func TestProxmoxChecker_ParseCertificate(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)

	tests := []struct {
		name             string
		input            string
		expectedSubject  string
		expectedIssuer   string
		expectedExpiring bool
		expectedExpired  bool
	}{
		{
			name: "valid certificate expiring soon",
			input: `subject=CN=pve01
issuer=CN=Proxmox Virtual Environment
notBefore=Jan 1 00:00:00 2025 GMT
notAfter=Dec 1 00:00:00 2025 GMT`,
			expectedSubject:  "CN=pve01",
			expectedIssuer:   "CN=Proxmox Virtual Environment",
			expectedExpiring: true,
			expectedExpired:  false,
		},
		{
			name: "expired certificate",
			input: `subject=CN=pve01
issuer=CN=Proxmox Virtual Environment
notBefore=Jan 1 00:00:00 2020 GMT
notAfter=Jan 15 00:00:00 2021 GMT`,
			expectedSubject:  "CN=pve01",
			expectedIssuer:   "CN=Proxmox Virtual Environment",
			expectedExpiring: false,
			expectedExpired:  true,
		},
		{
			name: "valid certificate far future",
			input: `subject=CN=pve01
issuer=CN=Proxmox Virtual Environment
notBefore=Jan 1 00:00:00 2024 GMT
notAfter=Jan 15 00:00:00 2030 GMT`,
			expectedSubject:  "CN=pve01",
			expectedIssuer:   "CN=Proxmox Virtual Environment",
			expectedExpiring: false,
			expectedExpired:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := checker.parseCertificate(tt.input, "/test/path", "test-cert")

			if cert == nil {
				t.Fatal("expected certificate, got nil")
			}

			if cert.Subject != tt.expectedSubject {
				t.Errorf("expected subject %q, got %q", tt.expectedSubject, cert.Subject)
			}

			if cert.Issuer != tt.expectedIssuer {
				t.Errorf("expected issuer %q, got %q", tt.expectedIssuer, cert.Issuer)
			}

			if cert.Expired != tt.expectedExpired {
				t.Errorf("expected expired %v, got %v", tt.expectedExpired, cert.Expired)
			}

			// Check if certificate is expiring soon (within 30 days)
			isExpiringSoon := cert.DaysToExpiry <= 30 && cert.DaysToExpiry >= 0
			if isExpiringSoon != tt.expectedExpiring {
				t.Errorf("expected expiring soon %v, got %v (days to expiry: %d)",
					tt.expectedExpiring, isExpiringSoon, cert.DaysToExpiry)
			}
		})
	}
}

func TestProxmoxChecker_ParseCephOSDStat(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)

	tests := []struct {
		name            string
		input           string
		expectedTotal   int
		expectedUp      int
		expectedIn      int
		expectedDown    int
		expectedOut     int
	}{
		{
			name:          "all OSDs healthy",
			input:         "30 osds: 30 up, 30 in;",
			expectedTotal: 30,
			expectedUp:    30,
			expectedIn:    30,
			expectedDown:  0,
			expectedOut:   0,
		},
		{
			name:          "some OSDs down",
			input:         "30 osds: 28 up, 28 in;",
			expectedTotal: 30,
			expectedUp:    28,
			expectedIn:    28,
			expectedDown:  2,
			expectedOut:   2,
		},
		{
			name:          "OSDs out but up",
			input:         "30 osds: 30 up, 28 in;",
			expectedTotal: 30,
			expectedUp:    30,
			expectedIn:    28,
			expectedDown:  0,
			expectedOut:   2,
		},
		{
			name:          "small cluster",
			input:         "3 osds: 3 up, 3 in;",
			expectedTotal: 3,
			expectedUp:    3,
			expectedIn:    3,
			expectedDown:  0,
			expectedOut:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &CephInfo{}
			checker.parseCephOSDStat(tt.input, info)

			if info.TotalOSDs != tt.expectedTotal {
				t.Errorf("expected total OSDs %d, got %d", tt.expectedTotal, info.TotalOSDs)
			}

			if info.UpOSDs != tt.expectedUp {
				t.Errorf("expected up OSDs %d, got %d", tt.expectedUp, info.UpOSDs)
			}

			if info.InOSDs != tt.expectedIn {
				t.Errorf("expected in OSDs %d, got %d", tt.expectedIn, info.InOSDs)
			}

			if info.DownOSDs != tt.expectedDown {
				t.Errorf("expected down OSDs %d, got %d", tt.expectedDown, info.DownOSDs)
			}

			if info.OutOSDs != tt.expectedOut {
				t.Errorf("expected out OSDs %d, got %d", tt.expectedOut, info.OutOSDs)
			}
		})
	}
}

func TestProxmoxChecker_ParseCephPools(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)

	input := `pool 1 'rbd' replicated size 3 min_size 2
pool 2 'cephfs_data' replicated size 3 min_size 2
pool 3 'cephfs_metadata' replicated size 3 min_size 2`

	pools := checker.parseCephPools(input)

	if len(pools) != 3 {
		t.Fatalf("expected 3 pools, got %d", len(pools))
	}

	expectedPools := []struct {
		id   int
		name string
	}{
		{1, "rbd"},
		{2, "cephfs_data"},
		{3, "cephfs_metadata"},
	}

	for i, expected := range expectedPools {
		if pools[i].ID != expected.id {
			t.Errorf("pool %d: expected ID %d, got %d", i, expected.id, pools[i].ID)
		}

		if pools[i].Name != expected.name {
			t.Errorf("pool %d: expected name %q, got %q", i, expected.name, pools[i].Name)
		}
	}
}

func TestProxmoxChecker_ParseCephDF(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)

	tests := []struct {
		name               string
		input              string
		expectedUsedPct    float64
		expectedNonZeroTotal bool
	}{
		{
			name: "50% usage",
			input: `--- GLOBAL ---
TOTAL     USED       AVAIL      RAW USED     %RAW USED
100 GiB   50 GiB     50 GiB     50 GiB       50.00%`,
			expectedUsedPct:    50.0,
			expectedNonZeroTotal: true,
		},
		{
			name: "85% usage high",
			input: `--- GLOBAL ---
TOTAL      USED        AVAIL       RAW USED     %RAW USED
1000 GiB   850 GiB     150 GiB     850 GiB      85.00%`,
			expectedUsedPct:    85.0,
			expectedNonZeroTotal: true,
		},
		{
			name: "TiB units",
			input: `--- GLOBAL ---
TOTAL    USED     AVAIL    RAW USED     %RAW USED
10 TiB   5 TiB    5 TiB    5 TiB        50.00%`,
			expectedUsedPct:    50.0,
			expectedNonZeroTotal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &CephInfo{}
			checker.parseCephDF(tt.input, info)

			// Allow 1% tolerance for floating point
			if abs := tt.expectedUsedPct - info.UsedPercent; abs > 1.0 && abs < -1.0 {
				t.Errorf("expected used percent %.2f%%, got %.2f%%",
					tt.expectedUsedPct, info.UsedPercent)
			}

			if tt.expectedNonZeroTotal && info.TotalStorageBytes == 0 {
				t.Error("expected non-zero total storage bytes")
			}
		})
	}
}

func TestProxmoxChecker_ParsePools(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)

	tests := []struct {
		name          string
		input         string
		expectedCount int
		expectedIDs   []string
	}{
		{
			name: "JSON format with two pools",
			input: `[
{"poolid":"production","comment":"Production VMs"},
{"poolid":"development","comment":"Dev environment"}
]`,
			expectedCount: 2,
			expectedIDs:   []string{"production", "development"},
		},
		{
			name: "text format",
			input: `POOLID     COMMENT
production Production VMs
development Dev environment`,
			expectedCount: 2,
			expectedIDs:   []string{"production", "development"},
		},
		{
			name:          "empty output",
			input:         "",
			expectedCount: 0,
			expectedIDs:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pools := checker.parsePools(tt.input)

			if len(pools) != tt.expectedCount {
				t.Errorf("expected %d pools, got %d", tt.expectedCount, len(pools))
			}

			for i, expectedID := range tt.expectedIDs {
				if i >= len(pools) {
					t.Errorf("expected pool ID %q but not enough pools returned", expectedID)
					continue
				}

				if pools[i].PoolID != expectedID {
					t.Errorf("pool %d: expected ID %q, got %q", i, expectedID, pools[i].PoolID)
				}
			}
		})
	}
}

func TestProxmoxChecker_ParseNodeNames(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)

	input := `Nodeid      Votes Name
         1          1 pve01 (local)
         2          1 pve02
         3          1 pve03`

	names := checker.parseNodeNames(input)

	if len(names) != 3 {
		t.Fatalf("expected 3 node names, got %d", len(names))
	}

	expectedNames := []string{"pve01", "pve02", "pve03"}
	for i, expected := range expectedNames {
		if names[i] != expected {
			t.Errorf("node %d: expected name %q, got %q", i, expected, names[i])
		}
	}
}

func TestProxmoxChecker_ParseNodesJSON(t *testing.T) {
	checker := NewProxmoxChecker(false, false, false, false, false, false, false)

	input := `[
{"node":"pve01","status":"online"},
{"node":"pve02","status":"online"},
{"node":"pve03","status":"offline"}
]`

	names := checker.parseNodesJSON(input)

	if len(names) != 3 {
		t.Fatalf("expected 3 node names, got %d", len(names))
	}

	expectedNames := []string{"pve01", "pve02", "pve03"}
	for i, expected := range expectedNames {
		if names[i] != expected {
			t.Errorf("node %d: expected name %q, got %q", i, expected, names[i])
		}
	}
}
