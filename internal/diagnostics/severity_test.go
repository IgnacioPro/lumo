package diagnostics

import (
	"testing"
)

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		severity Severity
		want     string
	}{
		{SeverityOK, "ok"},
		{SeverityInfo, "info"},
		{SeverityWarning, "warning"},
		{SeverityCritical, "critical"},
		{SeverityError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.severity.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeverity_IsHigherThan(t *testing.T) {
	tests := []struct {
		name  string
		s1    Severity
		s2    Severity
		want  bool
	}{
		{"error > critical", SeverityError, SeverityCritical, true},
		{"critical > warning", SeverityCritical, SeverityWarning, true},
		{"warning > info", SeverityWarning, SeverityInfo, true},
		{"info > ok", SeverityInfo, SeverityOK, true},
		{"ok < info", SeverityOK, SeverityInfo, false},
		{"warning < critical", SeverityWarning, SeverityCritical, false},
		{"equal severities", SeverityWarning, SeverityWarning, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s1.IsHigherThan(tt.s2); got != tt.want {
				t.Errorf("IsHigherThan() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeverityRank(t *testing.T) {
	tests := []struct {
		severity Severity
		want     int
	}{
		{SeverityError, 4},
		{SeverityCritical, 3},
		{SeverityWarning, 2},
		{SeverityInfo, 1},
		{SeverityOK, 0},
		{Severity("unknown"), -1},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			if got := severityRank(tt.severity); got != tt.want {
				t.Errorf("severityRank() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultThresholds(t *testing.T) {
	thresholds := DefaultThresholds()

	// Test CPU thresholds
	if thresholds.CPU.UsageWarn != 80.0 {
		t.Errorf("CPU.UsageWarn = %v, want 80.0", thresholds.CPU.UsageWarn)
	}
	if thresholds.CPU.UsageCritical != 95.0 {
		t.Errorf("CPU.UsageCritical = %v, want 95.0", thresholds.CPU.UsageCritical)
	}

	// Test Memory thresholds
	if thresholds.Memory.UsageWarn != 85.0 {
		t.Errorf("Memory.UsageWarn = %v, want 85.0", thresholds.Memory.UsageWarn)
	}
	if thresholds.Memory.SwapWarn != 25.0 {
		t.Errorf("Memory.SwapWarn = %v, want 25.0", thresholds.Memory.SwapWarn)
	}

	// Test Disk thresholds
	if thresholds.Disk.UsageWarn != 85.0 {
		t.Errorf("Disk.UsageWarn = %v, want 85.0", thresholds.Disk.UsageWarn)
	}

	// Test Process thresholds
	if thresholds.Process.TotalWarn != 500 {
		t.Errorf("Process.TotalWarn = %v, want 500", thresholds.Process.TotalWarn)
	}
	if thresholds.Process.ZombieWarn != 5 {
		t.Errorf("Process.ZombieWarn = %v, want 5", thresholds.Process.ZombieWarn)
	}

	// Test Network thresholds
	if thresholds.Network.ConnectionsWarn != 1000 {
		t.Errorf("Network.ConnectionsWarn = %v, want 1000", thresholds.Network.ConnectionsWarn)
	}
}

func TestEvaluateThresholds(t *testing.T) {
	thresholds := DefaultThresholds()

	tests := []struct {
		name   string
		result *CheckResult
		want   Severity
	}{
		{
			name:   "nil result",
			result: nil,
			want:   SeverityOK,
		},
		{
			name: "no metrics",
			result: &CheckResult{
				Category: CategoryCPU,
				Metrics:  []Metric{},
			},
			want: SeverityOK,
		},
		{
			name: "CPU usage OK",
			result: &CheckResult{
				Category: CategoryCPU,
				Metrics: []Metric{
					{Name: "cpu_usage", Value: 50.0},
				},
			},
			want: SeverityOK,
		},
		{
			name: "CPU usage warning",
			result: &CheckResult{
				Category: CategoryCPU,
				Metrics: []Metric{
					{Name: "cpu_usage", Value: 85.0},
				},
			},
			want: SeverityWarning,
		},
		{
			name: "CPU usage critical",
			result: &CheckResult{
				Category: CategoryCPU,
				Metrics: []Metric{
					{Name: "cpu_usage", Value: 98.0},
				},
			},
			want: SeverityCritical,
		},
		{
			name: "Memory usage warning",
			result: &CheckResult{
				Category: CategoryMemory,
				Metrics: []Metric{
					{Name: "memory_used_percent", Value: 90.0},
				},
			},
			want: SeverityWarning,
		},
		{
			name: "Multiple metrics - highest severity wins",
			result: &CheckResult{
				Category: CategoryCPU,
				Metrics: []Metric{
					{Name: "cpu_usage", Value: 50.0},        // OK
					{Name: "load_1min", Value: 1.8},         // Warning
				},
			},
			want: SeverityWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateThresholds(tt.result, thresholds)
			if got != tt.want {
				t.Errorf("EvaluateThresholds() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateCPUMetric(t *testing.T) {
	thresholds := CPUThresholds{
		UsageWarn:     80.0,
		UsageCritical: 95.0,
		LoadWarn:      1.5,
		LoadCritical:  2.0,
	}

	tests := []struct {
		name   string
		metric Metric
		want   Severity
	}{
		{
			name:   "cpu usage OK",
			metric: Metric{Name: "cpu_usage", Value: 50.0},
			want:   SeverityOK,
		},
		{
			name:   "cpu usage warning",
			metric: Metric{Name: "cpu_usage", Value: 85.0},
			want:   SeverityWarning,
		},
		{
			name:   "cpu usage critical",
			metric: Metric{Name: "cpu_usage", Value: 98.0},
			want:   SeverityCritical,
		},
		{
			name:   "cpu percent warning",
			metric: Metric{Name: "cpu_percent", Value: 82.0},
			want:   SeverityWarning,
		},
		{
			name:   "load warning",
			metric: Metric{Name: "load_1min", Value: 1.7},
			want:   SeverityWarning,
		},
		{
			name:   "load critical",
			metric: Metric{Name: "load_1min", Value: 2.5},
			want:   SeverityCritical,
		},
		{
			name:   "load per cpu warning",
			metric: Metric{Name: "load_per_cpu_1min", Value: 1.8},
			want:   SeverityWarning,
		},
		{
			name:   "unknown metric",
			metric: Metric{Name: "unknown", Value: 100.0},
			want:   SeverityOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateCPUMetric(tt.metric, thresholds)
			if got != tt.want {
				t.Errorf("evaluateCPUMetric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateMemoryMetric(t *testing.T) {
	thresholds := MemoryThresholds{
		UsageWarn:     85.0,
		UsageCritical: 95.0,
		SwapWarn:      25.0,
		SwapCritical:  75.0,
	}

	tests := []struct {
		name   string
		metric Metric
		want   Severity
	}{
		{
			name:   "memory OK",
			metric: Metric{Name: "memory_used", Value: 70.0},
			want:   SeverityOK,
		},
		{
			name:   "memory warning",
			metric: Metric{Name: "memory_used", Value: 90.0},
			want:   SeverityWarning,
		},
		{
			name:   "memory critical",
			metric: Metric{Name: "memory_used", Value: 98.0},
			want:   SeverityCritical,
		},
		{
			name:   "memory percent warning",
			metric: Metric{Name: "memory_used_percent", Value: 87.0},
			want:   SeverityWarning,
		},
		{
			name:   "swap OK",
			metric: Metric{Name: "swap_used", Value: 20.0},
			want:   SeverityOK,
		},
		{
			name:   "swap warning",
			metric: Metric{Name: "swap_used", Value: 50.0},
			want:   SeverityWarning,
		},
		{
			name:   "swap critical",
			metric: Metric{Name: "swap_used", Value: 80.0},
			want:   SeverityCritical,
		},
		{
			name:   "swap percent warning",
			metric: Metric{Name: "swap_used_percent", Value: 30.0},
			want:   SeverityWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateMemoryMetric(tt.metric, thresholds)
			if got != tt.want {
				t.Errorf("evaluateMemoryMetric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateDiskMetric(t *testing.T) {
	thresholds := DiskThresholds{
		UsageWarn:     85.0,
		UsageCritical: 95.0,
		InodeWarn:     85.0,
		InodeCritical: 95.0,
	}

	tests := []struct {
		name   string
		metric Metric
		want   Severity
	}{
		{
			name:   "disk usage OK",
			metric: Metric{Name: "disk_used", Value: 70.0},
			want:   SeverityOK,
		},
		{
			name:   "disk usage warning",
			metric: Metric{Name: "disk_used", Value: 90.0},
			want:   SeverityWarning,
		},
		{
			name:   "disk usage critical",
			metric: Metric{Name: "disk_used", Value: 98.0},
			want:   SeverityCritical,
		},
		{
			name:   "disk percent warning",
			metric: Metric{Name: "disk_used_percent", Value: 87.0},
			want:   SeverityWarning,
		},
		{
			name:   "inode OK",
			metric: Metric{Name: "inode_used_percent", Value: 70.0},
			want:   SeverityOK,
		},
		{
			name:   "inode warning",
			metric: Metric{Name: "inode_used_percent", Value: 90.0},
			want:   SeverityWarning,
		},
		{
			name:   "inode critical",
			metric: Metric{Name: "inode_used_percent", Value: 98.0},
			want:   SeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateDiskMetric(tt.metric, thresholds)
			if got != tt.want {
				t.Errorf("evaluateDiskMetric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateProcessMetric(t *testing.T) {
	thresholds := ProcessThresholds{
		TotalWarn:      500,
		TotalCritical:  1000,
		ZombieWarn:     5,
		ZombieCritical: 20,
	}

	tests := []struct {
		name   string
		metric Metric
		want   Severity
	}{
		{
			name:   "total processes OK",
			metric: Metric{Name: "total_processes", Value: 300.0},
			want:   SeverityOK,
		},
		{
			name:   "total processes warning",
			metric: Metric{Name: "total_processes", Value: 700.0},
			want:   SeverityWarning,
		},
		{
			name:   "total processes critical",
			metric: Metric{Name: "total_processes", Value: 1500.0},
			want:   SeverityCritical,
		},
		{
			name:   "zombie processes OK",
			metric: Metric{Name: "zombie_processes", Value: 2.0},
			want:   SeverityOK,
		},
		{
			name:   "zombie processes warning",
			metric: Metric{Name: "zombie_processes", Value: 10.0},
			want:   SeverityWarning,
		},
		{
			name:   "zombie processes critical",
			metric: Metric{Name: "zombie_processes", Value: 25.0},
			want:   SeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateProcessMetric(tt.metric, thresholds)
			if got != tt.want {
				t.Errorf("evaluateProcessMetric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateNetworkMetric(t *testing.T) {
	thresholds := NetworkThresholds{
		ConnectionsWarn:     1000,
		ConnectionsCritical: 5000,
	}

	tests := []struct {
		name   string
		metric Metric
		want   Severity
	}{
		{
			name:   "connections OK",
			metric: Metric{Name: "tcp_established", Value: 500.0},
			want:   SeverityOK,
		},
		{
			name:   "connections warning",
			metric: Metric{Name: "tcp_established", Value: 2000.0},
			want:   SeverityWarning,
		},
		{
			name:   "connections critical",
			metric: Metric{Name: "tcp_established", Value: 6000.0},
			want:   SeverityCritical,
		},
		{
			name:   "unknown metric",
			metric: Metric{Name: "unknown", Value: 10000.0},
			want:   SeverityOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateNetworkMetric(tt.metric, thresholds)
			if got != tt.want {
				t.Errorf("evaluateNetworkMetric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateLogMetric(t *testing.T) {
	thresholds := LogThresholds{
		ErrorCountWarn:     50,
		ErrorCountCritical: 200,
	}

	tests := []struct {
		name   string
		metric Metric
		want   Severity
	}{
		{
			name:   "error count OK",
			metric: Metric{Name: "error_count", Value: 30.0},
			want:   SeverityOK,
		},
		{
			name:   "error count warning",
			metric: Metric{Name: "error_count", Value: 100.0},
			want:   SeverityWarning,
		},
		{
			name:   "error count critical",
			metric: Metric{Name: "error_count", Value: 300.0},
			want:   SeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateLogMetric(tt.metric, thresholds)
			if got != tt.want {
				t.Errorf("evaluateLogMetric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateSecurityMetric(t *testing.T) {
	thresholds := SecurityThresholds{
		FailedLoginsWarn:     50,
		FailedLoginsCritical: 200,
	}

	tests := []struct {
		name   string
		metric Metric
		want   Severity
	}{
		{
			name:   "failed logins OK",
			metric: Metric{Name: "failed_logins", Value: 30.0},
			want:   SeverityOK,
		},
		{
			name:   "failed logins warning",
			metric: Metric{Name: "failed_logins", Value: 100.0},
			want:   SeverityWarning,
		},
		{
			name:   "failed logins critical",
			metric: Metric{Name: "failed_logins", Value: 300.0},
			want:   SeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateSecurityMetric(tt.metric, thresholds)
			if got != tt.want {
				t.Errorf("evaluateSecurityMetric() = %v, want %v", got, tt.want)
			}
		})
	}
}
