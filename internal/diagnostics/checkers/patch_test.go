package checkers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestNewPatchChecker(t *testing.T) {
	checker := NewPatchChecker()

	if checker == nil {
		t.Fatal("NewPatchChecker() returned nil")
	}
}

func TestPatchChecker_Getters(t *testing.T) {
	checker := NewPatchChecker()

	t.Run("Name", func(t *testing.T) {
		if name := checker.Name(); name != "patch_status" {
			t.Errorf("Name() = %q, want 'patch_status'", name)
		}
	})

	t.Run("Category", func(t *testing.T) {
		if cat := checker.Category(); cat != diagnostics.CategorySecurity {
			t.Errorf("Category() = %v, want %v", cat, diagnostics.CategorySecurity)
		}
	})

	t.Run("Description", func(t *testing.T) {
		desc := checker.Description()
		if desc == "" {
			t.Error("Description() should return non-empty string")
		}
		if !strings.Contains(strings.ToLower(desc), "update") || !strings.Contains(strings.ToLower(desc), "patch") {
			t.Errorf("Description() should mention updates or patches, got %q", desc)
		}
	})

	t.Run("RequiresRoot", func(t *testing.T) {
		if checker.RequiresRoot() {
			t.Error("RequiresRoot() = true, want false")
		}
	})
}

func TestPatchChecker_ParseOSRelease(t *testing.T) {
	checker := NewPatchChecker()

	tests := []struct {
		name           string
		content        string
		wantOS         string
		wantPkgManager string
	}{
		{
			name: "Ubuntu",
			content: `NAME="Ubuntu"
VERSION="20.04.6 LTS (Focal Fossa)"
ID=ubuntu
ID_LIKE=debian`,
			wantOS:         "ubuntu",
			wantPkgManager: "apt",
		},
		{
			name: "Debian",
			content: `PRETTY_NAME="Debian GNU/Linux 11 (bullseye)"
NAME="Debian GNU/Linux"
VERSION_ID="11"
ID=debian`,
			wantOS:         "debian",
			wantPkgManager: "apt",
		},
		{
			name: "CentOS",
			content: `NAME="CentOS Linux"
VERSION="7 (Core)"
ID="centos"
ID_LIKE="rhel fedora"`,
			wantOS:         "centos",
			wantPkgManager: "yum",
		},
		{
			name: "Fedora",
			content: `NAME="Fedora Linux"
VERSION="38 (Workstation Edition)"
ID=fedora`,
			wantOS:         "fedora",
			wantPkgManager: "dnf",
		},
		{
			name: "Alpine",
			content: `NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.18.0`,
			wantOS:         "alpine",
			wantPkgManager: "apk",
		},
		{
			name: "Arch",
			content: `NAME="Arch Linux"
PRETTY_NAME="Arch Linux"
ID=arch`,
			wantOS:         "arch",
			wantPkgManager: "pacman",
		},
		{
			name: "Rocky Linux",
			content: `NAME="Rocky Linux"
VERSION="9.2 (Blue Onyx)"
ID="rocky"`,
			wantOS:         "rocky",
			wantPkgManager: "dnf",
		},
		{
			name: "Unknown OS",
			content: `NAME="CustomOS"
ID=custom`,
			wantOS:         "custom",
			wantPkgManager: "",
		},
		{
			name:           "Empty content",
			content:        "",
			wantOS:         "",
			wantPkgManager: "",
		},
		{
			name:           "No ID field",
			content:        `NAME="Some OS"\nVERSION="1.0"`,
			wantOS:         "",
			wantPkgManager: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOS, gotPkg := checker.parseOSRelease(tt.content)

			if gotOS != tt.wantOS {
				t.Errorf("parseOSRelease() OS = %q, want %q", gotOS, tt.wantOS)
			}

			if gotPkg != tt.wantPkgManager {
				t.Errorf("parseOSRelease() PkgManager = %q, want %q", gotPkg, tt.wantPkgManager)
			}
		})
	}
}

func TestPatchChecker_DetectPackageManager(t *testing.T) {
	checker := NewPatchChecker()

	tests := []struct {
		name        string
		whichResult map[string]int // command -> exit code
		want        string
		wantErr     bool
	}{
		{
			name: "apt available",
			whichResult: map[string]int{
				"which apt": 0,
			},
			want:    "apt",
			wantErr: false,
		},
		{
			name: "dnf available",
			whichResult: map[string]int{
				"which apt": 1,
				"which dnf": 0,
			},
			want:    "dnf",
			wantErr: false,
		},
		{
			name: "yum available",
			whichResult: map[string]int{
				"which apt": 1,
				"which dnf": 1,
				"which yum": 0,
			},
			want:    "yum",
			wantErr: false,
		},
		{
			name: "no package manager",
			whichResult: map[string]int{
				"which apt":    1,
				"which dnf":    1,
				"which yum":    1,
				"which apk":    1,
				"which pacman": 1,
				"which brew":   1,
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				responses: make(map[string]mockResponse),
			}

			// Set up which command responses
			for cmd, exitCode := range tt.whichResult {
				executor.responses[cmd] = mockResponse{
					stdout:   "",
					exitCode: exitCode,
				}
			}

			ctx := context.Background()
			got, err := checker.detectPackageManager(ctx, executor)

			if tt.wantErr {
				if err == nil {
					t.Error("detectPackageManager() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("detectPackageManager() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("detectPackageManager() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPatchChecker_ParseAptOutput(t *testing.T) {
	checker := NewPatchChecker()

	tests := []struct {
		name               string
		output             string
		wantUpdates        []string
		wantSecurityCount  int
	}{
		{
			name: "with security updates",
			output: `Listing...
curl/jammy-security 7.81.0-1ubuntu1.15 amd64 [upgradable from: 7.81.0-1ubuntu1.14]
libcurl4/jammy-updates 7.81.0-1ubuntu1.15 amd64 [upgradable from: 7.81.0-1ubuntu1.14]
nginx/jammy-security 1.18.0-0ubuntu1.4 amd64 [upgradable from: 1.18.0-0ubuntu1.3]`,
			wantUpdates:       []string{"curl", "libcurl4", "nginx"},
			wantSecurityCount: 2, // curl and nginx have "security" in their lines
		},
		{
			name: "no updates",
			output: `Listing...`,
			wantUpdates:       []string{},
			wantSecurityCount: 0,
		},
		{
			name: "regular updates only",
			output: `Listing...
vim/jammy-updates 2:8.2.3995-1ubuntu2.12 amd64 [upgradable from: 2:8.2.3995-1ubuntu2.11]
git/jammy 1:2.34.1-1ubuntu1.10 amd64 [upgradable from: 1:2.34.1-1ubuntu1.9]`,
			wantUpdates:       []string{"vim", "git"},
			wantSecurityCount: 0,
		},
		{
			name:              "empty output",
			output:            "",
			wantUpdates:       []string{},
			wantSecurityCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates, securityUpdates, err := checker.parseAptOutput(tt.output)

			if err != nil {
				t.Errorf("parseAptOutput() unexpected error: %v", err)
				return
			}

			if len(updates) != len(tt.wantUpdates) {
				t.Errorf("parseAptOutput() updates count = %d, want %d", len(updates), len(tt.wantUpdates))
			}

			for i, wantPkg := range tt.wantUpdates {
				if i >= len(updates) || updates[i] != wantPkg {
					t.Errorf("parseAptOutput() updates[%d] = %q, want %q", i, updates[i], wantPkg)
				}
			}

			if len(securityUpdates) != tt.wantSecurityCount {
				t.Errorf("parseAptOutput() security count = %d, want %d", len(securityUpdates), tt.wantSecurityCount)
			}
		})
	}
}

func TestPatchChecker_ParseYumDnfOutput(t *testing.T) {
	checker := NewPatchChecker()

	tests := []struct {
		name              string
		output            string
		wantUpdates       []string
		wantSecurityCount int
	}{
		{
			name: "with security updates",
			output: `Last metadata expiration check: 0:00:03 ago
kernel.x86_64                     5.14.0-284.11.1.el9_2             baseos
openssl.x86_64                    1:3.0.7-16.el9_2                  security-updates
vim-enhanced.x86_64               2:8.2.2637-20.el9_1               appstream`,
			wantUpdates:       []string{"kernel", "openssl", "vim-enhanced"},
			wantSecurityCount: 1, // openssl has "security" in repository name
		},
		{
			name:              "no updates",
			output:            "Last metadata expiration check: 0:00:01 ago",
			wantUpdates:       []string{},
			wantSecurityCount: 0,
		},
		{
			name: "regular updates only",
			output: `kernel.x86_64                     5.14.0-284.11.1.el9_2             baseos
vim.x86_64                        2:8.2.2637-20.el9_1               updates`,
			wantUpdates:       []string{"kernel", "vim"},
			wantSecurityCount: 0,
		},
		{
			name:              "empty output",
			output:            "",
			wantUpdates:       []string{},
			wantSecurityCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates, securityUpdates, err := checker.parseYumDnfOutput(tt.output)

			if err != nil {
				t.Errorf("parseYumDnfOutput() unexpected error: %v", err)
				return
			}

			if len(updates) != len(tt.wantUpdates) {
				t.Errorf("parseYumDnfOutput() updates count = %d, want %d", len(updates), len(tt.wantUpdates))
			}

			for i, wantPkg := range tt.wantUpdates {
				if i >= len(updates) || updates[i] != wantPkg {
					t.Errorf("parseYumDnfOutput() updates[%d] = %q, want %q", i, updates[i], wantPkg)
				}
			}

			if len(securityUpdates) != tt.wantSecurityCount {
				t.Errorf("parseYumDnfOutput() security count = %d, want %d", len(securityUpdates), tt.wantSecurityCount)
			}
		})
	}
}

func TestPatchChecker_ParseApkOutput(t *testing.T) {
	checker := NewPatchChecker()

	tests := []struct {
		name        string
		output      string
		wantUpdates []string
	}{
		{
			name: "with updates",
			output: `curl-7.88.1-r1 [upgradable from: curl-7.88.1-r0]
openssl-3.1.0-r4 [upgradable from: openssl-3.1.0-r3]
nginx-1.24.0-r0 [upgradable from: nginx-1.22.1-r0]`,
			wantUpdates: []string{"curl", "openssl", "nginx"},
		},
		{
			name:        "no updates",
			output:      "",
			wantUpdates: []string{},
		},
		{
			name: "single update",
			output: `busybox-1.36.0-r9 [upgradable from: busybox-1.36.0-r8]`,
			wantUpdates: []string{"busybox"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates, securityUpdates, err := checker.parseApkOutput(tt.output)

			if err != nil {
				t.Errorf("parseApkOutput() unexpected error: %v", err)
				return
			}

			if len(updates) != len(tt.wantUpdates) {
				t.Errorf("parseApkOutput() updates count = %d, want %d", len(updates), len(tt.wantUpdates))
			}

			for i, wantPkg := range tt.wantUpdates {
				if i >= len(updates) || updates[i] != wantPkg {
					t.Errorf("parseApkOutput() updates[%d] = %q, want %q", i, updates[i], wantPkg)
				}
			}

			// Alpine doesn't distinguish security updates
			if len(securityUpdates) != 0 {
				t.Errorf("parseApkOutput() should return empty security updates for Alpine, got %d", len(securityUpdates))
			}
		})
	}
}

func TestPatchChecker_ParsePacmanOutput(t *testing.T) {
	checker := NewPatchChecker()

	tests := []struct {
		name        string
		output      string
		wantUpdates []string
	}{
		{
			name: "with updates",
			output: `linux 6.3.5.arch1-1 -> 6.3.6.arch1-1
pacman 6.0.2-7 -> 6.0.2-8
systemd 253.4-1 -> 253.5-1`,
			wantUpdates: []string{"linux", "pacman", "systemd"},
		},
		{
			name:        "no updates",
			output:      "",
			wantUpdates: []string{},
		},
		{
			name: "single update",
			output: `firefox 114.0-1 -> 114.0.1-1`,
			wantUpdates: []string{"firefox"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates, securityUpdates, err := checker.parsePacmanOutput(tt.output)

			if err != nil {
				t.Errorf("parsePacmanOutput() unexpected error: %v", err)
				return
			}

			if len(updates) != len(tt.wantUpdates) {
				t.Errorf("parsePacmanOutput() updates count = %d, want %d", len(updates), len(tt.wantUpdates))
			}

			for i, wantPkg := range tt.wantUpdates {
				if i >= len(updates) || updates[i] != wantPkg {
					t.Errorf("parsePacmanOutput() updates[%d] = %q, want %q", i, updates[i], wantPkg)
				}
			}

			// Pacman doesn't distinguish security updates easily
			if len(securityUpdates) != 0 {
				t.Errorf("parsePacmanOutput() should return empty security updates, got %d", len(securityUpdates))
			}
		})
	}
}

func TestPatchChecker_ParseBrewOutput(t *testing.T) {
	checker := NewPatchChecker()

	tests := []struct {
		name        string
		output      string
		wantUpdates []string
	}{
		{
			name: "with updates",
			output: `git (2.40.1) < 2.41.0
node (20.2.0) < 20.3.0
python@3.11 (3.11.3) < 3.11.4`,
			wantUpdates: []string{"git", "node", "python@3.11"},
		},
		{
			name:        "no updates",
			output:      "",
			wantUpdates: []string{},
		},
		{
			name: "simple format",
			output: `wget
curl
vim`,
			wantUpdates: []string{"wget", "curl", "vim"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates, securityUpdates, err := checker.parseBrewOutput(tt.output)

			if err != nil {
				t.Errorf("parseBrewOutput() unexpected error: %v", err)
				return
			}

			if len(updates) != len(tt.wantUpdates) {
				t.Errorf("parseBrewOutput() updates count = %d, want %d", len(updates), len(tt.wantUpdates))
			}

			for i, wantPkg := range tt.wantUpdates {
				if i >= len(updates) || updates[i] != wantPkg {
					t.Errorf("parseBrewOutput() updates[%d] = %q, want %q", i, updates[i], wantPkg)
				}
			}

			// Homebrew doesn't distinguish security updates in standard output
			if len(securityUpdates) != 0 {
				t.Errorf("parseBrewOutput() should return empty security updates, got %d", len(securityUpdates))
			}
		})
	}
}

func TestPatchChecker_FormatMessage(t *testing.T) {
	checker := NewPatchChecker()

	tests := []struct {
		name            string
		totalUpdates    int
		securityUpdates int
		wantContains    []string
	}{
		{
			name:            "no updates",
			totalUpdates:    0,
			securityUpdates: 0,
			wantContains:    []string{"up-to-date", "no pending"},
		},
		{
			name:            "with security updates",
			totalUpdates:    10,
			securityUpdates: 3,
			wantContains:    []string{"3 security", "10 updates"},
		},
		{
			name:            "updates without security",
			totalUpdates:    5,
			securityUpdates: 0,
			wantContains:    []string{"5 updates", "no security"},
		},
		{
			name:            "all security",
			totalUpdates:    2,
			securityUpdates: 2,
			wantContains:    []string{"2 security", "2 updates"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checker.formatMessage(tt.totalUpdates, tt.securityUpdates)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("formatMessage() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}

func TestPatchChecker_Run(t *testing.T) {
	t.Run("Ubuntu with security updates", func(t *testing.T) {
		checker := NewPatchChecker()

		osReleaseContent := `NAME="Ubuntu"
ID=ubuntu`

		aptOutput := `Listing...
curl/jammy-security 7.81.0-1ubuntu1.15 amd64 [upgradable from: 7.81.0-1ubuntu1.14]
nginx/jammy-security 1.18.0-0ubuntu1.4 amd64 [upgradable from: 1.18.0-0ubuntu1.3]
vim/jammy-updates 2:8.2.3995-1ubuntu2.12 amd64 [upgradable from: 2:8.2.3995-1ubuntu2.11]`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"cat /etc/os-release":    {stdout: osReleaseContent, exitCode: 0},
				"apt-get update":         {stdout: "", exitCode: 0},
				"apt list --upgradable":  {stdout: aptOutput, exitCode: 0},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := checker.Run(ctx, executor)

		if err != nil {
			t.Fatalf("Run() unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("Run() returned nil result")
		}

		// Check basic fields
		if result.Name != "patch_status" {
			t.Errorf("Name = %q, want 'patch_status'", result.Name)
		}

		if result.Status != diagnostics.StatusCompleted {
			t.Errorf("Status = %v, want %v", result.Status, diagnostics.StatusCompleted)
		}

		// Check data
		osType, _ := result.GetDataValue("os_type")
		if osType != "ubuntu" {
			t.Errorf("os_type = %v, want 'ubuntu'", osType)
		}

		pkgManager, _ := result.GetDataValue("package_manager")
		if pkgManager != "apt" {
			t.Errorf("package_manager = %v, want 'apt'", pkgManager)
		}

		totalUpdates, _ := result.GetDataValue("total_updates")
		if totalUpdates != 3 {
			t.Errorf("total_updates = %v, want 3", totalUpdates)
		}

		securityUpdates, _ := result.GetDataValue("security_updates")
		if securityUpdates != 2 {
			t.Errorf("security_updates = %v, want 2 (curl and nginx)", securityUpdates)
		}

		// Check metrics
		if len(result.Metrics) != 2 {
			t.Errorf("Metrics count = %d, want 2", len(result.Metrics))
		}

		// Check message
		if !strings.Contains(result.Message, "security") {
			t.Errorf("Message should mention security updates, got: %q", result.Message)
		}
	})

	t.Run("Fedora with dnf", func(t *testing.T) {
		checker := NewPatchChecker()

		osReleaseContent := `NAME="Fedora Linux"
ID=fedora`

		dnfOutput := `kernel.x86_64                     6.3.5-200.fc38             updates
vim.x86_64                        2:9.0.1592-1.fc38          updates`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"cat /etc/os-release": {stdout: osReleaseContent, exitCode: 0},
				"dnf check-update":    {stdout: dnfOutput, exitCode: 0},
			},
		}

		ctx := context.Background()
		result, err := checker.Run(ctx, executor)

		if err != nil {
			t.Fatalf("Run() unexpected error: %v", err)
		}

		pkgManager, _ := result.GetDataValue("package_manager")
		if pkgManager != "dnf" {
			t.Errorf("package_manager = %v, want 'dnf'", pkgManager)
		}

		totalUpdates, _ := result.GetDataValue("total_updates")
		if totalUpdates != 2 {
			t.Errorf("total_updates = %v, want 2", totalUpdates)
		}
	})

	t.Run("OS detection failure", func(t *testing.T) {
		checker := NewPatchChecker()

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"cat /etc/os-release": {stdout: "", exitCode: 1},
				"uname -s":            {stdout: "", exitCode: 1},
			},
		}

		ctx := context.Background()
		result, err := checker.Run(ctx, executor)

		if err == nil {
			t.Error("Run() expected error for OS detection failure")
		}

		if result != nil {
			t.Errorf("Run() should return nil result on error, got %+v", result)
		}
	})

	t.Run("Update check failure - continues with error in data", func(t *testing.T) {
		checker := NewPatchChecker()

		osReleaseContent := `NAME="Ubuntu"
ID=ubuntu`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"cat /etc/os-release":   {stdout: osReleaseContent, exitCode: 0},
				"apt-get update":        {stdout: "", exitCode: 0},
				"apt list --upgradable": {stdout: "", exitCode: 1, err: fmt.Errorf("command failed")},
			},
		}

		ctx := context.Background()
		result, err := checker.Run(ctx, executor)

		// Should not return error, but result should contain error in data
		if err != nil {
			t.Errorf("Run() should not return error on update check failure, got: %v", err)
		}

		if result == nil {
			t.Fatal("Run() should return result even on update check failure")
		}

		errorData, ok := result.GetDataValue("error")
		if !ok {
			t.Error("Result should contain 'error' in data when update check fails")
		} else if errorData == "" {
			t.Error("Error data should not be empty")
		}
	})
}

func TestPatchChecker_DetectOS(t *testing.T) {
	checker := NewPatchChecker()

	t.Run("using os-release", func(t *testing.T) {
		osReleaseContent := `NAME="Ubuntu"
ID=ubuntu`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"cat /etc/os-release": {stdout: osReleaseContent, exitCode: 0},
			},
		}

		ctx := context.Background()
		osType, pkgManager, err := checker.detectOS(ctx, executor)

		if err != nil {
			t.Fatalf("detectOS() unexpected error: %v", err)
		}

		if osType != "ubuntu" {
			t.Errorf("osType = %q, want 'ubuntu'", osType)
		}

		if pkgManager != "apt" {
			t.Errorf("pkgManager = %q, want 'apt'", pkgManager)
		}
	})

	t.Run("fallback to uname", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"cat /etc/os-release": {stdout: "", exitCode: 1},
				"uname -s":            {stdout: "Darwin\n", exitCode: 0},
				"which brew":          {stdout: "/usr/local/bin/brew", exitCode: 0},
			},
		}

		ctx := context.Background()
		osType, pkgManager, err := checker.detectOS(ctx, executor)

		if err != nil {
			t.Fatalf("detectOS() unexpected error: %v", err)
		}

		if osType != "Darwin" {
			t.Errorf("osType = %q, want 'Darwin'", osType)
		}

		if pkgManager != "brew" {
			t.Errorf("pkgManager = %q, want 'brew'", pkgManager)
		}
	})

	t.Run("uname failure", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"cat /etc/os-release": {stdout: "", exitCode: 1},
				"uname -s":            {stdout: "", exitCode: 1, err: fmt.Errorf("command failed")},
			},
		}

		ctx := context.Background()
		osType, pkgManager, err := checker.detectOS(ctx, executor)

		if err == nil {
			t.Error("detectOS() expected error when uname fails")
		}

		if osType != "" || pkgManager != "" {
			t.Errorf("detectOS() should return empty strings on error, got osType=%q, pkgManager=%q", osType, pkgManager)
		}
	})
}

func TestPatchChecker_GetApkUpdates(t *testing.T) {
	checker := NewPatchChecker()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		apkOutput := `curl-8.0.1-r0 < 8.5.0-r0 [upgradable from: 8.0.1-r0]
openssl-3.1.0-r0 < 3.1.4-r0 [upgradable from: 3.1.0-r0]
nginx-1.24.0-r0 < 1.25.3-r0 [upgradable from: 1.24.0-r0]`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"apk list -u": {stdout: apkOutput, exitCode: 0},
			},
		}

		updates, security, err := checker.getApkUpdates(ctx, executor)

		if err != nil {
			t.Errorf("getApkUpdates() unexpected error: %v", err)
		}

		if len(updates) != 3 {
			t.Errorf("getApkUpdates() found %d updates, want 3", len(updates))
		}

		// Alpine doesn't distinguish security updates
		if len(security) != 0 {
			t.Errorf("getApkUpdates() found %d security updates, want 0", len(security))
		}
	})

	t.Run("command fails", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"apk list -u": {stdout: "", exitCode: 1},
			},
		}

		updates, security, err := checker.getApkUpdates(ctx, executor)

		if err == nil {
			t.Error("getApkUpdates() expected error when command fails")
		}

		if updates != nil || security != nil {
			t.Errorf("getApkUpdates() should return nil on error, got updates=%v, security=%v", updates, security)
		}
	})
}

func TestPatchChecker_GetPacmanUpdates(t *testing.T) {
	checker := NewPatchChecker()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		pacmanOutput := `linux 6.1.0-1 -> 6.5.9-1
systemd 253.0-1 -> 254.5-1
firefox 118.0-1 -> 119.0-1`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"pacman -Qu": {stdout: pacmanOutput, exitCode: 0},
			},
		}

		updates, security, err := checker.getPacmanUpdates(ctx, executor)

		if err != nil {
			t.Errorf("getPacmanUpdates() unexpected error: %v", err)
		}

		if len(updates) != 3 {
			t.Errorf("getPacmanUpdates() found %d updates, want 3", len(updates))
		}

		// Pacman doesn't distinguish security updates
		if len(security) != 0 {
			t.Errorf("getPacmanUpdates() found %d security updates, want 0", len(security))
		}
	})

	t.Run("command fails", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"pacman -Qu": {stdout: "", exitCode: 1},
			},
		}

		updates, security, err := checker.getPacmanUpdates(ctx, executor)

		if err == nil {
			t.Error("getPacmanUpdates() expected error when command fails")
		}

		if updates != nil || security != nil {
			t.Errorf("getPacmanUpdates() should return nil on error, got updates=%v, security=%v", updates, security)
		}
	})
}

func TestPatchChecker_GetBrewUpdates(t *testing.T) {
	checker := NewPatchChecker()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		brewOutput := `python@3.11 (3.11.5) < 3.11.6
node (18.17.0) < 20.9.0
git (2.41.0) < 2.42.0`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"brew update":   {stdout: "", exitCode: 0}, // Best effort, ignored
				"brew outdated": {stdout: brewOutput, exitCode: 0},
			},
		}

		updates, security, err := checker.getBrewUpdates(ctx, executor)

		if err != nil {
			t.Errorf("getBrewUpdates() unexpected error: %v", err)
		}

		if len(updates) != 3 {
			t.Errorf("getBrewUpdates() found %d updates, want 3", len(updates))
		}

		// Homebrew doesn't distinguish security updates
		if len(security) != 0 {
			t.Errorf("getBrewUpdates() found %d security updates, want 0", len(security))
		}
	})

	t.Run("brew update fails but ignored", func(t *testing.T) {
		brewOutput := `wget (1.21.3) < 1.21.4`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"brew update":   {stdout: "", exitCode: 1}, // Fails but should be ignored
				"brew outdated": {stdout: brewOutput, exitCode: 0},
			},
		}

		updates, _, err := checker.getBrewUpdates(ctx, executor)

		// Should still succeed because brew update failure is ignored
		if err != nil {
			t.Errorf("getBrewUpdates() unexpected error: %v", err)
		}

		if len(updates) != 1 {
			t.Errorf("getBrewUpdates() found %d updates, want 1", len(updates))
		}
	})

	t.Run("brew outdated fails", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"brew update":   {stdout: "", exitCode: 0},
				"brew outdated": {stdout: "", exitCode: 1},
			},
		}

		updates, security, err := checker.getBrewUpdates(ctx, executor)

		if err == nil {
			t.Error("getBrewUpdates() expected error when brew outdated fails")
		}

		if updates != nil || security != nil {
			t.Errorf("getBrewUpdates() should return nil on error, got updates=%v, security=%v", updates, security)
		}
	})
}
