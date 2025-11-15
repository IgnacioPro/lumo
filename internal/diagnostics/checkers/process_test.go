package checkers

import (
	"context"
	"strings"
	"testing"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestNewProcessChecker(t *testing.T) {
	thresholds := diagnostics.ProcessThresholds{
		TotalWarn:     500,
		TotalCritical: 1000,
	}

	checker := NewProcessChecker(thresholds)
	if checker == nil {
		t.Fatal("NewProcessChecker returned nil")
	}
	if checker.thresholds.TotalWarn != 500 {
		t.Errorf("Expected TotalWarn 500, got %v", checker.thresholds.TotalWarn)
	}
}

func TestProcessChecker_Name(t *testing.T) {
	checker := NewProcessChecker(diagnostics.ProcessThresholds{})
	if got := checker.Name(); got != "process_check" {
		t.Errorf("Name() = %v, want process_check", got)
	}
}

func TestProcessChecker_Category(t *testing.T) {
	checker := NewProcessChecker(diagnostics.ProcessThresholds{})
	if got := checker.Category(); got != diagnostics.CategoryProcess {
		t.Errorf("Category() = %v, want %v", got, diagnostics.CategoryProcess)
	}
}

func TestProcessChecker_Description(t *testing.T) {
	checker := NewProcessChecker(diagnostics.ProcessThresholds{})
	desc := checker.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if !strings.Contains(strings.ToLower(desc), "process") {
		t.Error("Description should mention process")
	}
}

func TestProcessChecker_RequiresRoot(t *testing.T) {
	checker := NewProcessChecker(diagnostics.ProcessThresholds{})
	if checker.RequiresRoot() {
		t.Error("ProcessChecker should not require root")
	}
}

func TestProcessChecker_ParseProcessStates(t *testing.T) {
	psOutput := `R
S
S
Z
S
T
R
S
S
Z`

	checker := NewProcessChecker(diagnostics.ProcessThresholds{})
	counts, err := checker.parseProcessStates(psOutput)

	if err != nil {
		t.Fatalf("parseProcessStates() error = %v", err)
	}

	if counts.Total != 10 {
		t.Errorf("Total = %v, want 10", counts.Total)
	}

	if counts.Running != 2 {
		t.Errorf("Running = %v, want 2", counts.Running)
	}

	if counts.Sleeping != 5 {
		t.Errorf("Sleeping = %v, want 5", counts.Sleeping)
	}

	if counts.Zombie != 2 {
		t.Errorf("Zombie = %v, want 2", counts.Zombie)
	}

	if counts.Stopped != 1 {
		t.Errorf("Stopped = %v, want 1", counts.Stopped)
	}
}

func TestProcessChecker_Run(t *testing.T) {
	thresholds := diagnostics.ProcessThresholds{
		TotalWarn:      500,
		TotalCritical:  1000,
		ZombieWarn:     5,
		ZombieCritical: 20,
	}

	psStateOutput := `R
S
S
S`

	psCPUOutput := `  PID USER     %CPU COMMAND
  123 root     25.5 nginx
  456 www      15.2 apache
  789 mysql    10.1 mysqld`

	psMemOutput := `  PID USER     %MEM COMMAND
  123 root      5.5 java
  456 www       3.2 python
  789 mysql     2.1 mysqld`

	executor := &mockExecutor{
		responses: map[string]mockResponse{
			"ps -eo state": {
				stdout:   psStateOutput,
				exitCode: 0,
			},
			"ps -eo pid,user,%cpu,comm": {
				stdout:   psCPUOutput,
				exitCode: 0,
			},
			"ps -eo pid,user,%mem,comm": {
				stdout:   psMemOutput,
				exitCode: 0,
			},
		},
	}

	checker := NewProcessChecker(thresholds)
	result, err := checker.Run(context.Background(), executor)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Name != "process_check" {
		t.Errorf("Result.Name = %v, want process_check", result.Name)
	}

	if result.Category != diagnostics.CategoryProcess {
		t.Errorf("Result.Category = %v, want %v", result.Category, diagnostics.CategoryProcess)
	}

	if result.Status != diagnostics.StatusCompleted {
		t.Errorf("Result.Status = %v, want %v", result.Status, diagnostics.StatusCompleted)
	}

	// Check that process counts are present
	total, ok := result.GetDataFloat("total_processes")
	if !ok {
		t.Error("Expected total_processes data")
	}
	if total != 4.0 {
		t.Errorf("Expected 4 total processes, got %v", total)
	}

	// Check message is generated
	if result.Message == "" {
		t.Error("Expected non-empty message")
	}
}

func TestProcessChecker_ParseProcessList(t *testing.T) {
	psOutput := `  123 root     25.5 nginx
  456 www      15.2 apache
  789 mysql    10.1 mysqld`

	checker := NewProcessChecker(diagnostics.ProcessThresholds{})
	processes, err := checker.parseProcessList(psOutput, "cpu")

	if err != nil {
		t.Fatalf("parseProcessList() error = %v", err)
	}

	if len(processes) != 3 {
		t.Errorf("Expected 3 processes, got %d", len(processes))
	}

	// Check first process
	if processes[0].PID != 123 {
		t.Errorf("PID = %v, want 123", processes[0].PID)
	}
	if processes[0].User != "root" {
		t.Errorf("User = %v, want root", processes[0].User)
	}
	if processes[0].CPU != 25.5 {
		t.Errorf("CPU = %v, want 25.5", processes[0].CPU)
	}
	if processes[0].Command != "nginx" {
		t.Errorf("Command = %v, want nginx", processes[0].Command)
	}
}
