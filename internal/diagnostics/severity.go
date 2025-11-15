package diagnostics

// Severity represents the severity level of a diagnostic check result
type Severity string

const (
	// SeverityOK indicates all metrics are within normal range
	SeverityOK Severity = "ok"

	// SeverityInfo is informational, no action needed
	SeverityInfo Severity = "info"

	// SeverityWarning indicates metrics are approaching thresholds
	SeverityWarning Severity = "warning"

	// SeverityCritical indicates metrics exceed thresholds, action required
	SeverityCritical Severity = "critical"

	// SeverityError indicates the check failed to execute
	SeverityError Severity = "error"
)

// String returns the string representation of the severity
func (s Severity) String() string {
	return string(s)
}

// IsHigherThan returns true if this severity is more severe than the other
func (s Severity) IsHigherThan(other Severity) bool {
	return severityRank(s) > severityRank(other)
}

// severityRank returns a numeric rank for severity comparison
func severityRank(s Severity) int {
	switch s {
	case SeverityError:
		return 4
	case SeverityCritical:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	case SeverityOK:
		return 0
	default:
		return -1
	}
}

// ThresholdConfig contains threshold values for all check types
type ThresholdConfig struct {
	CPU      CPUThresholds      `mapstructure:"cpu"`
	Memory   MemoryThresholds   `mapstructure:"memory"`
	Disk     DiskThresholds     `mapstructure:"disk"`
	Process  ProcessThresholds  `mapstructure:"process"`
	Network  NetworkThresholds  `mapstructure:"network"`
	Logs     LogThresholds      `mapstructure:"logs"`
	Security SecurityThresholds `mapstructure:"security"`
}

// CPUThresholds contains CPU-specific thresholds
type CPUThresholds struct {
	UsageWarn     float64 `mapstructure:"usage_warn"`
	UsageCritical float64 `mapstructure:"usage_critical"`
	LoadWarn      float64 `mapstructure:"load_warn"`
	LoadCritical  float64 `mapstructure:"load_critical"`
}

// MemoryThresholds contains memory-specific thresholds
type MemoryThresholds struct {
	UsageWarn      float64 `mapstructure:"usage_warn"`
	UsageCritical  float64 `mapstructure:"usage_critical"`
	SwapWarn       float64 `mapstructure:"swap_warn"`
	SwapCritical   float64 `mapstructure:"swap_critical"`
	SwapIOWarn     float64 `mapstructure:"swap_io_warn"`
	SwapIOCritical float64 `mapstructure:"swap_io_critical"`
}

// DiskThresholds contains disk-specific thresholds
type DiskThresholds struct {
	UsageWarn       float64 `mapstructure:"usage_warn"`
	UsageCritical   float64 `mapstructure:"usage_critical"`
	InodeWarn       float64 `mapstructure:"inode_warn"`
	InodeCritical   float64 `mapstructure:"inode_critical"`
	IOUtilWarn      float64 `mapstructure:"io_util_warn"`
	IOUtilCritical  float64 `mapstructure:"io_util_critical"`
	IOAwaitWarn     float64 `mapstructure:"io_await_warn"`
	IOAwaitCritical float64 `mapstructure:"io_await_critical"`
}

// ProcessThresholds contains process-specific thresholds
type ProcessThresholds struct {
	TotalWarn      int `mapstructure:"total_warn"`
	TotalCritical  int `mapstructure:"total_critical"`
	ZombieWarn     int `mapstructure:"zombie_warn"`
	ZombieCritical int `mapstructure:"zombie_critical"`
}

// NetworkThresholds contains network-specific thresholds
type NetworkThresholds struct {
	ConnectionsWarn       int     `mapstructure:"connections_warn"`
	ConnectionsCritical   int     `mapstructure:"connections_critical"`
	ErrorsPerHourWarn     int     `mapstructure:"errors_per_hour_warn"`
	ErrorsPerHourCritical int     `mapstructure:"errors_per_hour_critical"`
	PacketLossWarn        float64 `mapstructure:"packet_loss_warn"`
	PacketLossCritical    float64 `mapstructure:"packet_loss_critical"`
	DNSLatencyWarn        float64 `mapstructure:"dns_latency_warn"`
	DNSLatencyCritical    float64 `mapstructure:"dns_latency_critical"`
}

// LogThresholds contains log analysis thresholds
type LogThresholds struct {
	ErrorCountWarn        int `mapstructure:"error_count_warn"`
	ErrorCountCritical    int `mapstructure:"error_count_critical"`
	CriticalCountWarn     int `mapstructure:"critical_count_warn"`
	CriticalCountCritical int `mapstructure:"critical_count_critical"`
	AuthFailuresWarn      int `mapstructure:"auth_failures_warn"`
	AuthFailuresCritical  int `mapstructure:"auth_failures_critical"`
}

// SecurityThresholds contains security check thresholds
type SecurityThresholds struct {
	FailedLoginsWarn      int `mapstructure:"failed_logins_warn"`
	FailedLoginsCritical  int `mapstructure:"failed_logins_critical"`
	WritableFilesWarn     int `mapstructure:"writable_files_warn"`
	WritableFilesCritical int `mapstructure:"writable_files_critical"`
	OpenPortsWarn         int `mapstructure:"open_ports_warn"`
	OpenPortsCritical     int `mapstructure:"open_ports_critical"`
}

// DefaultThresholds returns default threshold configuration
func DefaultThresholds() *ThresholdConfig {
	return &ThresholdConfig{
		CPU: CPUThresholds{
			UsageWarn:     80.0,
			UsageCritical: 95.0,
			LoadWarn:      1.5,
			LoadCritical:  2.0,
		},
		Memory: MemoryThresholds{
			UsageWarn:      85.0,
			UsageCritical:  95.0,
			SwapWarn:       25.0,
			SwapCritical:   75.0,
			SwapIOWarn:     100,
			SwapIOCritical: 500,
		},
		Disk: DiskThresholds{
			UsageWarn:       85.0,
			UsageCritical:   95.0,
			InodeWarn:       85.0,
			InodeCritical:   95.0,
			IOUtilWarn:      80.0,
			IOUtilCritical:  95.0,
			IOAwaitWarn:     50.0,
			IOAwaitCritical: 100.0,
		},
		Process: ProcessThresholds{
			TotalWarn:      500,
			TotalCritical:  1000,
			ZombieWarn:     5,
			ZombieCritical: 20,
		},
		Network: NetworkThresholds{
			ConnectionsWarn:       1000,
			ConnectionsCritical:   5000,
			ErrorsPerHourWarn:     100,
			ErrorsPerHourCritical: 1000,
			PacketLossWarn:        1.0,
			PacketLossCritical:    5.0,
			DNSLatencyWarn:        100.0,
			DNSLatencyCritical:    500.0,
		},
		Logs: LogThresholds{
			ErrorCountWarn:        50,
			ErrorCountCritical:    200,
			CriticalCountWarn:     5,
			CriticalCountCritical: 20,
			AuthFailuresWarn:      20,
			AuthFailuresCritical:  100,
		},
		Security: SecurityThresholds{
			FailedLoginsWarn:      50,
			FailedLoginsCritical:  200,
			WritableFilesWarn:     0,
			WritableFilesCritical: 5,
			OpenPortsWarn:         10,
			OpenPortsCritical:     20,
		},
	}
}

// EvaluateThresholds determines severity based on metrics and thresholds
func EvaluateThresholds(result *CheckResult, thresholds *ThresholdConfig) Severity {
	if result == nil || len(result.Metrics) == 0 {
		return SeverityOK
	}

	highestSeverity := SeverityOK

	// Evaluate each metric
	for _, metric := range result.Metrics {
		severity := evaluateMetric(metric, result.Category, thresholds)
		if severity.IsHigherThan(highestSeverity) {
			highestSeverity = severity
		}
	}

	return highestSeverity
}

// evaluateMetric evaluates a single metric against thresholds
func evaluateMetric(metric Metric, category CheckCategory, thresholds *ThresholdConfig) Severity {
	switch category {
	case CategoryCPU:
		return evaluateCPUMetric(metric, thresholds.CPU)
	case CategoryMemory:
		return evaluateMemoryMetric(metric, thresholds.Memory)
	case CategoryDisk:
		return evaluateDiskMetric(metric, thresholds.Disk)
	case CategoryProcess:
		return evaluateProcessMetric(metric, thresholds.Process)
	case CategoryNetwork:
		return evaluateNetworkMetric(metric, thresholds.Network)
	case CategoryLog:
		return evaluateLogMetric(metric, thresholds.Logs)
	case CategorySecurity:
		return evaluateSecurityMetric(metric, thresholds.Security)
	default:
		return SeverityOK
	}
}

// Helper functions for evaluating specific metric types

func evaluateCPUMetric(metric Metric, thresholds CPUThresholds) Severity {
	switch metric.Name {
	case "cpu_usage", "cpu_percent":
		if metric.Value >= thresholds.UsageCritical {
			return SeverityCritical
		}
		if metric.Value >= thresholds.UsageWarn {
			return SeverityWarning
		}
	case "load_1min", "load_per_cpu_1min":
		if metric.Value >= thresholds.LoadCritical {
			return SeverityCritical
		}
		if metric.Value >= thresholds.LoadWarn {
			return SeverityWarning
		}
	}
	return SeverityOK
}

func evaluateMemoryMetric(metric Metric, thresholds MemoryThresholds) Severity {
	switch metric.Name {
	case "memory_used", "memory_used_percent":
		if metric.Value >= thresholds.UsageCritical {
			return SeverityCritical
		}
		if metric.Value >= thresholds.UsageWarn {
			return SeverityWarning
		}
	case "swap_used", "swap_used_percent":
		if metric.Value >= thresholds.SwapCritical {
			return SeverityCritical
		}
		if metric.Value >= thresholds.SwapWarn {
			return SeverityWarning
		}
	}
	return SeverityOK
}

func evaluateDiskMetric(metric Metric, thresholds DiskThresholds) Severity {
	switch metric.Name {
	case "disk_used", "disk_used_percent":
		if metric.Value >= thresholds.UsageCritical {
			return SeverityCritical
		}
		if metric.Value >= thresholds.UsageWarn {
			return SeverityWarning
		}
	case "inode_used_percent":
		if metric.Value >= thresholds.InodeCritical {
			return SeverityCritical
		}
		if metric.Value >= thresholds.InodeWarn {
			return SeverityWarning
		}
	}
	return SeverityOK
}

func evaluateProcessMetric(metric Metric, thresholds ProcessThresholds) Severity {
	switch metric.Name {
	case "total_processes":
		if metric.Value >= float64(thresholds.TotalCritical) {
			return SeverityCritical
		}
		if metric.Value >= float64(thresholds.TotalWarn) {
			return SeverityWarning
		}
	case "zombie_processes":
		if metric.Value >= float64(thresholds.ZombieCritical) {
			return SeverityCritical
		}
		if metric.Value >= float64(thresholds.ZombieWarn) {
			return SeverityWarning
		}
	}
	return SeverityOK
}

func evaluateNetworkMetric(metric Metric, thresholds NetworkThresholds) Severity {
	switch metric.Name {
	case "tcp_established":
		if metric.Value >= float64(thresholds.ConnectionsCritical) {
			return SeverityCritical
		}
		if metric.Value >= float64(thresholds.ConnectionsWarn) {
			return SeverityWarning
		}
	}
	return SeverityOK
}

func evaluateLogMetric(metric Metric, thresholds LogThresholds) Severity {
	switch metric.Name {
	case "error_count":
		if metric.Value >= float64(thresholds.ErrorCountCritical) {
			return SeverityCritical
		}
		if metric.Value >= float64(thresholds.ErrorCountWarn) {
			return SeverityWarning
		}
	}
	return SeverityOK
}

func evaluateSecurityMetric(metric Metric, thresholds SecurityThresholds) Severity {
	switch metric.Name {
	case "failed_logins":
		if metric.Value >= float64(thresholds.FailedLoginsCritical) {
			return SeverityCritical
		}
		if metric.Value >= float64(thresholds.FailedLoginsWarn) {
			return SeverityWarning
		}
	}
	return SeverityOK
}
