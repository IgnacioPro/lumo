package checkers

import (
	"context"
	"strings"
	"testing"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestNewDiskChecker(t *testing.T) {
	thresholds := diagnostics.DiskThresholds{
		UsageWarn:     85.0,
		UsageCritical: 95.0,
	}

	checker := NewDiskChecker(thresholds)
	if checker == nil {
		t.Fatal("NewDiskChecker returned nil")
	}
	if checker.thresholds.UsageWarn != 85.0 {
		t.Errorf("Expected UsageWarn 85.0, got %v", checker.thresholds.UsageWarn)
	}
}

func TestDiskChecker_Name(t *testing.T) {
	checker := NewDiskChecker(diagnostics.DiskThresholds{})
	if got := checker.Name(); got != "disk_check" {
		t.Errorf("Name() = %v, want disk_check", got)
	}
}

func TestDiskChecker_Category(t *testing.T) {
	checker := NewDiskChecker(diagnostics.DiskThresholds{})
	if got := checker.Category(); got != diagnostics.CategoryDisk {
		t.Errorf("Category() = %v, want %v", got, diagnostics.CategoryDisk)
	}
}

func TestDiskChecker_Description(t *testing.T) {
	checker := NewDiskChecker(diagnostics.DiskThresholds{})
	desc := checker.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
	if !strings.Contains(strings.ToLower(desc), "disk") {
		t.Error("Description should mention disk")
	}
}

func TestDiskChecker_RequiresRoot(t *testing.T) {
	checker := NewDiskChecker(diagnostics.DiskThresholds{})
	if checker.RequiresRoot() {
		t.Error("DiskChecker should not require root")
	}
}

func TestDiskChecker_Run(t *testing.T) {
	thresholds := diagnostics.DiskThresholds{
		UsageWarn:     85.0,
		UsageCritical: 95.0,
		InodeWarn:     85.0,
		InodeCritical: 95.0,
	}

	dfOutput := `Filesystem     1K-blocks      Used Available Use% Mounted on
/dev/sda1      104806400  52403200  47252224  53% /`

	dfInodeOutput := `Filesystem     Inodes  IUsed  IFree IUse% Mounted on
/dev/sda1      6553600 655360 5898240   10% /`

	executor := &mockExecutor{
		responses: map[string]mockResponse{
			"df -k": {
				stdout:   dfOutput,
				exitCode: 0,
			},
			"df -i": {
				stdout:   dfInodeOutput,
				exitCode: 0,
			},
		},
	}

	checker := NewDiskChecker(thresholds)
	result, err := checker.Run(context.Background(), executor)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Name != "disk_check" {
		t.Errorf("Result.Name = %v, want disk_check", result.Name)
	}

	if result.Category != diagnostics.CategoryDisk {
		t.Errorf("Result.Category = %v, want %v", result.Category, diagnostics.CategoryDisk)
	}

	if result.Status != diagnostics.StatusCompleted {
		t.Errorf("Result.Status = %v, want %v", result.Status, diagnostics.StatusCompleted)
	}

	// Check that filesystems data is present
	if _, ok := result.Data["filesystems"]; !ok {
		t.Error("Expected filesystems data to be present")
	}

	// Check message is generated
	if result.Message == "" {
		t.Error("Expected non-empty message")
	}
}
