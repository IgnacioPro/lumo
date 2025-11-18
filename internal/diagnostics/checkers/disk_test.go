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

func TestDiskChecker_ShouldMonitorFilesystem(t *testing.T) {
	tests := []struct {
		name       string
		mountPoint string
		want       bool
	}{
		{
			name:       "root filesystem",
			mountPoint: "/",
			want:       true,
		},
		{
			name:       "home directory",
			mountPoint: "/home",
			want:       true,
		},
		{
			name:       "var directory",
			mountPoint: "/var",
			want:       true,
		},
		{
			name:       "tmp directory",
			mountPoint: "/tmp",
			want:       true,
		},
		{
			name:       "skip /dev (not exact)",
			mountPoint: "/dev/shm",
			want:       false,
		},
		{
			name:       "skip /sys",
			mountPoint: "/sys/kernel",
			want:       false,
		},
		{
			name:       "skip /proc",
			mountPoint: "/proc/sys",
			want:       false,
		},
		{
			name:       "skip /run",
			mountPoint: "/run/user",
			want:       false,
		},
		{
			name:       "skip /snap",
			mountPoint: "/snap/core",
			want:       false,
		},
		{
			name:       "skip /boot/efi",
			mountPoint: "/boot/efi",
			want:       false,
		},
		{
			name:       "allow /boot (not /boot/efi)",
			mountPoint: "/boot",
			want:       true,
		},
		{
			name:       "skip loop devices",
			mountPoint: "/var/lib/snapd/loop1",
			want:       false,
		},
		{
			name:       "skip tmpfs (non-root)",
			mountPoint: "/run/tmpfs",
			want:       false,
		},
		{
			name:       "custom mount point",
			mountPoint: "/mnt/data",
			want:       true,
		},
		{
			name:       "opt directory",
			mountPoint: "/opt",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewDiskChecker(diagnostics.DiskThresholds{})
			got := checker.shouldMonitorFilesystem(tt.mountPoint)
			if got != tt.want {
				t.Errorf("shouldMonitorFilesystem(%q) = %v, want %v", tt.mountPoint, got, tt.want)
			}
		})
	}
}

func TestDiskChecker_SanitizeMountPoint(t *testing.T) {
	tests := []struct {
		name       string
		mountPoint string
		want       string
	}{
		{
			name:       "root filesystem",
			mountPoint: "/",
			want:       "root",
		},
		{
			name:       "home directory",
			mountPoint: "/home",
			want:       "home",
		},
		{
			name:       "nested path",
			mountPoint: "/var/lib/docker",
			want:       "var_lib_docker",
		},
		{
			name:       "mnt path",
			mountPoint: "/mnt/data",
			want:       "mnt_data",
		},
		{
			name:       "multiple levels",
			mountPoint: "/mnt/backup/daily",
			want:       "mnt_backup_daily",
		},
		{
			name:       "trailing slash",
			mountPoint: "/opt/",
			want:       "opt",
		},
		{
			name:       "multiple trailing slashes",
			mountPoint: "/usr/local//",
			want:       "usr_local",
		},
		{
			name:       "single level",
			mountPoint: "/tmp",
			want:       "tmp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeMountPoint(tt.mountPoint)
			if got != tt.want {
				t.Errorf("sanitizeMountPoint(%q) = %q, want %q", tt.mountPoint, got, tt.want)
			}
		})
	}
}

func TestDiskChecker_ParseDfOutput(t *testing.T) {
	thresholds := diagnostics.DiskThresholds{
		UsageWarn:     80.0,
		UsageCritical: 90.0,
	}
	checker := NewDiskChecker(thresholds)

	t.Run("parses Linux df -B1 output correctly", func(t *testing.T) {
		output := `Filesystem                1B-blocks         Used    Available Use% Mounted on
/dev/sda1              104857600000  73400320000  26214400000  74% /
tmpfs                   16777216000   1048576000  15728640000   7% /tmp
/dev/sdb1             1099511627776 824633720832 219469660160  79% /data`

		filesystems, err := checker.parseDfOutput(output)

		if err != nil {
			t.Fatalf("parseDfOutput() unexpected error: %v", err)
		}

		if len(filesystems) != 3 {
			t.Fatalf("parseDfOutput() returned %d filesystems, want 3", len(filesystems))
		}

		// Check first filesystem
		if filesystems[0].Filesystem != "/dev/sda1" {
			t.Errorf("filesystems[0].Filesystem = %q, want /dev/sda1", filesystems[0].Filesystem)
		}
		if filesystems[0].MountPoint != "/" {
			t.Errorf("filesystems[0].MountPoint = %q, want /", filesystems[0].MountPoint)
		}
		if filesystems[0].UsedPercent != 74.0 {
			t.Errorf("filesystems[0].UsedPercent = %v, want 74.0", filesystems[0].UsedPercent)
		}

		// Check /data filesystem
		if filesystems[2].MountPoint != "/data" {
			t.Errorf("filesystems[2].MountPoint = %q, want /data", filesystems[2].MountPoint)
		}
		if filesystems[2].UsedPercent != 79.0 {
			t.Errorf("filesystems[2].UsedPercent = %v, want 79.0", filesystems[2].UsedPercent)
		}
	})

	t.Run("parses macOS df -k output and converts to bytes when needed", func(t *testing.T) {
		output := `Filesystem    1024-blocks      Used Available Capacity  Mounted on
/dev/disk1s1          900000    630000    225000    74%  /
tmpfs                  16000      1000     14000     7%  /tmp`

		filesystems, err := checker.parseDfOutput(output)

		if err != nil {
			t.Fatalf("parseDfOutput() unexpected error: %v", err)
		}

		if len(filesystems) != 2 {
			t.Fatalf("parseDfOutput() returned %d filesystems, want 2", len(filesystems))
		}

		// Values < 1000000 should be converted from KB to bytes (*1024)
		if filesystems[0].TotalBytes != 900000*1024 {
			t.Errorf("filesystems[0].TotalBytes = %d, want %d (900000 KB * 1024)", filesystems[0].TotalBytes, 900000*1024)
		}
	})

	t.Run("skips lines with insufficient fields", func(t *testing.T) {
		output := `Filesystem    1024-blocks      Used Available Capacity  Mounted on
/dev/disk1s1      102400000  71680000  25600000    74%  /
incomplete line
/dev/disk2s1       51200000  35840000  12800000    70%  /home`

		filesystems, err := checker.parseDfOutput(output)

		if err != nil {
			t.Fatalf("parseDfOutput() unexpected error: %v", err)
		}

		// Should skip incomplete line and parse 2 valid filesystems
		if len(filesystems) != 2 {
			t.Fatalf("parseDfOutput() returned %d filesystems, want 2 (skipping incomplete line)", len(filesystems))
		}
	})

	t.Run("skips special filesystems but keeps /dev", func(t *testing.T) {
		output := `Filesystem    1024-blocks      Used Available Capacity  Mounted on
/dev/disk1s1      102400000  71680000  25600000    74%  /
devtmpfs           8192000         0   8192000     0%  /dev
tmpfs              16384000   1024000  14336000     7%  /run`

		filesystems, err := checker.parseDfOutput(output)

		if err != nil {
			t.Fatalf("parseDfOutput() unexpected error: %v", err)
		}

		// Should skip /run but keep /dev (special case) and /
		if len(filesystems) != 2 {
			t.Fatalf("parseDfOutput() returned %d filesystems, want 2 (/ and /dev)", len(filesystems))
		}

		// Check that /run was filtered out
		for _, fs := range filesystems {
			if fs.MountPoint == "/run" {
				t.Error("parseDfOutput() should have filtered out /run")
			}
		}
	})

	t.Run("handles invalid numeric values gracefully", func(t *testing.T) {
		output := `Filesystem    1024-blocks      Used Available Capacity  Mounted on
/dev/disk1s1      102400000  71680000  25600000    74%  /
/dev/disk2s1       invalid  35840000  12800000    70%  /data
/dev/disk3s1       51200000  35840000  12800000    75%  /home`

		filesystems, err := checker.parseDfOutput(output)

		if err != nil {
			t.Fatalf("parseDfOutput() unexpected error: %v", err)
		}

		// Should skip line with invalid total and parse other 2
		if len(filesystems) != 2 {
			t.Fatalf("parseDfOutput() returned %d filesystems, want 2 (skipping invalid)", len(filesystems))
		}
	})

	t.Run("returns error on empty output", func(t *testing.T) {
		output := ``

		_, err := checker.parseDfOutput(output)

		if err == nil {
			t.Error("parseDfOutput() expected error on empty output, got nil")
		}
	})

	t.Run("returns error on header-only output", func(t *testing.T) {
		output := `Filesystem    1024-blocks      Used Available Capacity  Mounted on`

		_, err := checker.parseDfOutput(output)

		if err == nil {
			t.Error("parseDfOutput() expected error on header-only output, got nil")
		}
	})
}

func TestDiskChecker_ParseDfInodeOutput(t *testing.T) {
	thresholds := diagnostics.DiskThresholds{
		UsageWarn:     80.0,
		UsageCritical: 90.0,
	}
	checker := NewDiskChecker(thresholds)

	t.Run("parses df -i output correctly", func(t *testing.T) {
		output := `Filesystem      Inodes   IUsed    IFree IUse% Mounted on
/dev/sda1      6553600 3276800  3276800   50% /
/dev/sdb1     26214400 1310720 24903680    5% /data
tmpfs          4194304   10240  4184064    1% /tmp`

		inodeUsage, err := checker.parseDfInodeOutput(output)

		if err != nil {
			t.Fatalf("parseDfInodeOutput() unexpected error: %v", err)
		}

		if len(inodeUsage) != 3 {
			t.Fatalf("parseDfInodeOutput() returned %d entries, want 3", len(inodeUsage))
		}

		// Check root filesystem
		rootUsage, ok := inodeUsage["/"]
		if !ok {
			t.Fatal("parseDfInodeOutput() missing entry for /")
		}

		if rootUsage.Total != 6553600 {
			t.Errorf("inodeUsage[/].Total = %d, want 6553600", rootUsage.Total)
		}

		if rootUsage.Used != 3276800 {
			t.Errorf("inodeUsage[/].Used = %d, want 3276800", rootUsage.Used)
		}

		if rootUsage.UsedPercent != 50.0 {
			t.Errorf("inodeUsage[/].UsedPercent = %v, want 50.0", rootUsage.UsedPercent)
		}
	})

	t.Run("handles invalid numeric values gracefully", func(t *testing.T) {
		output := `Filesystem      Inodes   IUsed    IFree IUse% Mounted on
/dev/sda1      6553600 3276800  3276800   50% /
/dev/sdb1      invalid 1310720 24903680    5% /data
/dev/sdc1     26214400 2621440 23592960   10% /home`

		inodeUsage, err := checker.parseDfInodeOutput(output)

		if err != nil {
			t.Fatalf("parseDfInodeOutput() unexpected error: %v", err)
		}

		// Should skip line with invalid total and parse other 2
		if len(inodeUsage) != 2 {
			t.Fatalf("parseDfInodeOutput() returned %d entries, want 2 (skipping invalid)", len(inodeUsage))
		}
	})

	t.Run("skips lines with insufficient fields", func(t *testing.T) {
		output := `Filesystem      Inodes   IUsed    IFree IUse% Mounted on
/dev/sda1      6553600 3276800  3276800   50% /
incomplete
/dev/sdc1     26214400 2621440 23592960   10% /home`

		inodeUsage, err := checker.parseDfInodeOutput(output)

		if err != nil {
			t.Fatalf("parseDfInodeOutput() unexpected error: %v", err)
		}

		// Should skip incomplete line
		if len(inodeUsage) != 2 {
			t.Fatalf("parseDfInodeOutput() returned %d entries, want 2", len(inodeUsage))
		}
	})

	t.Run("returns error on empty output", func(t *testing.T) {
		output := ``

		_, err := checker.parseDfInodeOutput(output)

		if err == nil {
			t.Error("parseDfInodeOutput() expected error on empty output, got nil")
		}
	})

	t.Run("returns error on header-only output", func(t *testing.T) {
		output := `Filesystem      Inodes   IUsed    IFree IUse% Mounted on`

		_, err := checker.parseDfInodeOutput(output)

		if err == nil {
			t.Error("parseDfInodeOutput() expected error on header-only output, got nil")
		}
	})
}
