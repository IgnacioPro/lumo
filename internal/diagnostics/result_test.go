package diagnostics

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCheckResult_Success(t *testing.T) {
	tests := []struct {
		name   string
		result *CheckResult
		want   bool
	}{
		{
			name: "completed with no error",
			result: &CheckResult{
				Status: StatusCompleted,
				Error:  "",
			},
			want: true,
		},
		{
			name: "completed with error",
			result: &CheckResult{
				Status: StatusCompleted,
				Error:  "some error",
			},
			want: false,
		},
		{
			name: "failed status",
			result: &CheckResult{
				Status: StatusFailed,
				Error:  "",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Success(); got != tt.want {
				t.Errorf("Success() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckResult_IsCritical(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     bool
	}{
		{"ok", SeverityOK, false},
		{"info", SeverityInfo, false},
		{"warning", SeverityWarning, false},
		{"critical", SeverityCritical, true},
		{"error", SeverityError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CheckResult{Severity: tt.severity}
			if got := result.IsCritical(); got != tt.want {
				t.Errorf("IsCritical() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckResult_IsWarning(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		want     bool
	}{
		{"ok", SeverityOK, false},
		{"info", SeverityInfo, false},
		{"warning", SeverityWarning, true},
		{"critical", SeverityCritical, true},
		{"error", SeverityError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CheckResult{Severity: tt.severity}
			if got := result.IsWarning(); got != tt.want {
				t.Errorf("IsWarning() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckResult_GetDataValue(t *testing.T) {
	result := &CheckResult{
		Data: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		},
	}

	// Test existing key
	val, ok := result.GetDataValue("key1")
	if !ok {
		t.Error("Expected key1 to exist")
	}
	if val != "value1" {
		t.Errorf("Expected value1, got %v", val)
	}

	// Test non-existing key
	_, ok = result.GetDataValue("nonexistent")
	if ok {
		t.Error("Expected nonexistent key to return false")
	}

	// Test nil Data map
	emptyResult := &CheckResult{}
	_, ok = emptyResult.GetDataValue("any")
	if ok {
		t.Error("Expected nil Data map to return false")
	}
}

func TestCheckResult_GetDataFloat(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]interface{}
		key       string
		wantValue float64
		wantOk    bool
	}{
		{
			name:      "float64 value",
			data:      map[string]interface{}{"key": 3.14},
			key:       "key",
			wantValue: 3.14,
			wantOk:    true,
		},
		{
			name:      "float32 value",
			data:      map[string]interface{}{"key": float32(2.5)},
			key:       "key",
			wantValue: 2.5,
			wantOk:    true,
		},
		{
			name:      "int value",
			data:      map[string]interface{}{"key": 42},
			key:       "key",
			wantValue: 42.0,
			wantOk:    true,
		},
		{
			name:      "int64 value",
			data:      map[string]interface{}{"key": int64(100)},
			key:       "key",
			wantValue: 100.0,
			wantOk:    true,
		},
		{
			name:      "string value",
			data:      map[string]interface{}{"key": "not a number"},
			key:       "key",
			wantValue: 0,
			wantOk:    false,
		},
		{
			name:      "missing key",
			data:      map[string]interface{}{},
			key:       "missing",
			wantValue: 0,
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CheckResult{Data: tt.data}
			gotValue, gotOk := result.GetDataFloat(tt.key)
			if gotOk != tt.wantOk {
				t.Errorf("GetDataFloat() ok = %v, want %v", gotOk, tt.wantOk)
			}
			if gotValue != tt.wantValue {
				t.Errorf("GetDataFloat() value = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestCheckResult_GetDataString(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]interface{}
		key       string
		wantValue string
		wantOk    bool
	}{
		{
			name:      "string value",
			data:      map[string]interface{}{"key": "hello"},
			key:       "key",
			wantValue: "hello",
			wantOk:    true,
		},
		{
			name:      "non-string value",
			data:      map[string]interface{}{"key": 42},
			key:       "key",
			wantValue: "",
			wantOk:    false,
		},
		{
			name:      "missing key",
			data:      map[string]interface{}{},
			key:       "missing",
			wantValue: "",
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CheckResult{Data: tt.data}
			gotValue, gotOk := result.GetDataString(tt.key)
			if gotOk != tt.wantOk {
				t.Errorf("GetDataString() ok = %v, want %v", gotOk, tt.wantOk)
			}
			if gotValue != tt.wantValue {
				t.Errorf("GetDataString() value = %v, want %v", gotValue, tt.wantValue)
			}
		})
	}
}

func TestCheckResult_GetMetric(t *testing.T) {
	result := &CheckResult{
		Metrics: []Metric{
			{Name: "cpu_usage", Value: 80.0},
			{Name: "memory_usage", Value: 60.0},
		},
	}

	// Test existing metric
	metric, ok := result.GetMetric("cpu_usage")
	if !ok {
		t.Error("Expected cpu_usage metric to exist")
	}
	if metric.Value != 80.0 {
		t.Errorf("Expected value 80.0, got %v", metric.Value)
	}

	// Test non-existing metric
	_, ok = result.GetMetric("nonexistent")
	if ok {
		t.Error("Expected nonexistent metric to return false")
	}
}

func TestCheckResult_AddMetric(t *testing.T) {
	result := &CheckResult{}

	metric := Metric{Name: "test", Value: 42.0}
	result.AddMetric(metric)

	if len(result.Metrics) != 1 {
		t.Errorf("Expected 1 metric, got %d", len(result.Metrics))
	}

	if result.Metrics[0].Name != "test" {
		t.Errorf("Expected metric name 'test', got %s", result.Metrics[0].Name)
	}
}

func TestCheckResult_SetData(t *testing.T) {
	result := &CheckResult{}

	// Test setting data on nil map
	result.SetData("key1", "value1")
	if result.Data["key1"] != "value1" {
		t.Errorf("Expected value1, got %v", result.Data["key1"])
	}

	// Test setting multiple values
	result.SetData("key2", 42)
	if result.Data["key2"] != 42 {
		t.Errorf("Expected 42, got %v", result.Data["key2"])
	}
}

func TestReport_GetResultsByCategory(t *testing.T) {
	report := &Report{
		Results: []*CheckResult{
			{Name: "cpu1", Category: CategoryCPU},
			{Name: "mem1", Category: CategoryMemory},
			{Name: "cpu2", Category: CategoryCPU},
		},
	}

	cpuResults := report.GetResultsByCategory(CategoryCPU)
	if len(cpuResults) != 2 {
		t.Errorf("Expected 2 CPU results, got %d", len(cpuResults))
	}

	memResults := report.GetResultsByCategory(CategoryMemory)
	if len(memResults) != 1 {
		t.Errorf("Expected 1 Memory result, got %d", len(memResults))
	}

	diskResults := report.GetResultsByCategory(CategoryDisk)
	if len(diskResults) != 0 {
		t.Errorf("Expected 0 Disk results, got %d", len(diskResults))
	}
}

func TestReport_GetResultsBySeverity(t *testing.T) {
	report := &Report{
		Results: []*CheckResult{
			{Name: "check1", Severity: SeverityOK},
			{Name: "check2", Severity: SeverityWarning},
			{Name: "check3", Severity: SeverityWarning},
			{Name: "check4", Severity: SeverityCritical},
		},
	}

	warnings := report.GetResultsBySeverity(SeverityWarning)
	if len(warnings) != 2 {
		t.Errorf("Expected 2 warnings, got %d", len(warnings))
	}

	critical := report.GetResultsBySeverity(SeverityCritical)
	if len(critical) != 1 {
		t.Errorf("Expected 1 critical, got %d", len(critical))
	}
}

func TestReport_GetCriticalResults(t *testing.T) {
	report := &Report{
		Results: []*CheckResult{
			{Name: "ok", Severity: SeverityOK},
			{Name: "warn", Severity: SeverityWarning},
			{Name: "crit", Severity: SeverityCritical},
			{Name: "err", Severity: SeverityError},
		},
	}

	critical := report.GetCriticalResults()
	if len(critical) != 2 {
		t.Errorf("Expected 2 critical results, got %d", len(critical))
	}

	// Verify we got critical and error
	names := make(map[string]bool)
	for _, r := range critical {
		names[r.Name] = true
	}
	if !names["crit"] || !names["err"] {
		t.Error("Expected 'crit' and 'err' in critical results")
	}
}

func TestReport_HasCriticalIssues(t *testing.T) {
	tests := []struct {
		name    string
		summary ReportSummary
		want    bool
	}{
		{
			name:    "has critical",
			summary: ReportSummary{CriticalCount: 1},
			want:    true,
		},
		{
			name:    "has error",
			summary: ReportSummary{ErrorCount: 1},
			want:    true,
		},
		{
			name:    "no critical or error",
			summary: ReportSummary{WarningCount: 1},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{Summary: tt.summary}
			if got := report.HasCriticalIssues(); got != tt.want {
				t.Errorf("HasCriticalIssues() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReport_HasWarnings(t *testing.T) {
	tests := []struct {
		name    string
		summary ReportSummary
		want    bool
	}{
		{
			name:    "has warnings",
			summary: ReportSummary{WarningCount: 1},
			want:    true,
		},
		{
			name:    "no warnings",
			summary: ReportSummary{OKCount: 1},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{Summary: tt.summary}
			if got := report.HasWarnings(); got != tt.want {
				t.Errorf("HasWarnings() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReport_IsHealthy(t *testing.T) {
	tests := []struct {
		name    string
		summary ReportSummary
		want    bool
	}{
		{
			name:    "all ok",
			summary: ReportSummary{OKCount: 5},
			want:    true,
		},
		{
			name:    "has warnings",
			summary: ReportSummary{OKCount: 4, WarningCount: 1},
			want:    false,
		},
		{
			name:    "has critical",
			summary: ReportSummary{OKCount: 4, CriticalCount: 1},
			want:    false,
		},
		{
			name:    "has errors",
			summary: ReportSummary{OKCount: 4, ErrorCount: 1},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{Summary: tt.summary}
			if got := report.IsHealthy(); got != tt.want {
				t.Errorf("IsHealthy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReport_GetWorstSeverity(t *testing.T) {
	tests := []struct {
		name    string
		summary ReportSummary
		want    Severity
	}{
		{
			name:    "has error",
			summary: ReportSummary{ErrorCount: 1, CriticalCount: 1},
			want:    SeverityError,
		},
		{
			name:    "has critical",
			summary: ReportSummary{CriticalCount: 1, WarningCount: 1},
			want:    SeverityCritical,
		},
		{
			name:    "has warning",
			summary: ReportSummary{WarningCount: 1, InfoCount: 1},
			want:    SeverityWarning,
		},
		{
			name:    "has info",
			summary: ReportSummary{InfoCount: 1, OKCount: 1},
			want:    SeverityInfo,
		},
		{
			name:    "all ok",
			summary: ReportSummary{OKCount: 1},
			want:    SeverityOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{Summary: tt.summary}
			if got := report.GetWorstSeverity(); got != tt.want {
				t.Errorf("GetWorstSeverity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReport_AddMetadata(t *testing.T) {
	report := &Report{}

	// Test adding to nil map
	report.AddMetadata("key1", "value1")
	if report.Metadata["key1"] != "value1" {
		t.Errorf("Expected value1, got %v", report.Metadata["key1"])
	}

	// Test adding multiple values
	report.AddMetadata("key2", 42)
	if report.Metadata["key2"] != 42 {
		t.Errorf("Expected 42, got %v", report.Metadata["key2"])
	}
}

func TestReport_ToJSON(t *testing.T) {
	report := &Report{
		Timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Duration:  100 * time.Millisecond,
		Results: []*CheckResult{
			{
				Name:     "test_check",
				Category: CategoryCPU,
				Severity: SeverityOK,
				Status:   StatusCompleted,
			},
		},
		Summary: ReportSummary{
			TotalChecks: 1,
			OKCount:     1,
		},
	}

	jsonStr, err := report.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify some fields
	if parsed["summary"] == nil {
		t.Error("Expected summary field in JSON")
	}
	if parsed["results"] == nil {
		t.Error("Expected results field in JSON")
	}
}

func TestGenerateSummary(t *testing.T) {
	results := []*CheckResult{
		{Category: CategoryCPU, Severity: SeverityOK},
		{Category: CategoryCPU, Severity: SeverityWarning},
		{Category: CategoryMemory, Severity: SeverityCritical},
		{Category: CategoryDisk, Severity: SeverityError},
		{Category: CategoryDisk, Severity: SeverityInfo},
	}

	summary := generateSummary(results)

	// Test total counts
	if summary.TotalChecks != 5 {
		t.Errorf("Expected 5 total checks, got %d", summary.TotalChecks)
	}
	if summary.OKCount != 1 {
		t.Errorf("Expected 1 OK, got %d", summary.OKCount)
	}
	if summary.WarningCount != 1 {
		t.Errorf("Expected 1 Warning, got %d", summary.WarningCount)
	}
	if summary.CriticalCount != 1 {
		t.Errorf("Expected 1 Critical, got %d", summary.CriticalCount)
	}
	if summary.ErrorCount != 1 {
		t.Errorf("Expected 1 Error, got %d", summary.ErrorCount)
	}
	if summary.InfoCount != 1 {
		t.Errorf("Expected 1 Info, got %d", summary.InfoCount)
	}

	// Test category breakdown
	cpuCat := summary.ByCategory[CategoryCPU]
	if cpuCat.Total != 2 {
		t.Errorf("Expected 2 CPU checks, got %d", cpuCat.Total)
	}
	if cpuCat.OK != 1 {
		t.Errorf("Expected 1 CPU OK, got %d", cpuCat.OK)
	}
	if cpuCat.Warning != 1 {
		t.Errorf("Expected 1 CPU Warning, got %d", cpuCat.Warning)
	}

	memoryCat := summary.ByCategory[CategoryMemory]
	if memoryCat.Total != 1 {
		t.Errorf("Expected 1 Memory check, got %d", memoryCat.Total)
	}
	if memoryCat.Critical != 1 {
		t.Errorf("Expected 1 Memory Critical, got %d", memoryCat.Critical)
	}

	diskCat := summary.ByCategory[CategoryDisk]
	if diskCat.Total != 2 {
		t.Errorf("Expected 2 Disk checks, got %d", diskCat.Total)
	}
	if diskCat.Error != 1 {
		t.Errorf("Expected 1 Disk Error, got %d", diskCat.Error)
	}
}
