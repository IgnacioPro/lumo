package checkers

import (
	"context"
	"strings"
	"testing"

	"github.com/ignacio/lumo/internal/diagnostics"
)

func TestAuthFailuresChecker_Getters(t *testing.T) {
	checker := NewAuthFailuresChecker(1, 20)

	t.Run("Name", func(t *testing.T) {
		if got := checker.Name(); got != "auth_failures" {
			t.Errorf("Name() = %q, want 'auth_failures'", got)
		}
	})

	t.Run("Category", func(t *testing.T) {
		if got := checker.Category(); got != diagnostics.CategorySecurity {
			t.Errorf("Category() = %v, want %v", got, diagnostics.CategorySecurity)
		}
	})

	t.Run("Description", func(t *testing.T) {
		desc := checker.Description()
		if !strings.Contains(desc, "failed authentication") {
			t.Errorf("Description() should mention 'failed authentication', got: %q", desc)
		}
	})

	t.Run("RequiresRoot", func(t *testing.T) {
		if checker.RequiresRoot() {
			t.Error("RequiresRoot() should return false")
		}
	})
}

func TestNewAuthFailuresChecker(t *testing.T) {
	tests := []struct {
		name              string
		lookbackHours     int
		failureThreshold  int
		wantLookback      int
		wantThreshold     int
	}{
		{
			name:              "custom values",
			lookbackHours:     3,
			failureThreshold:  50,
			wantLookback:      3,
			wantThreshold:     50,
		},
		{
			name:              "zero lookback uses default",
			lookbackHours:     0,
			failureThreshold:  50,
			wantLookback:      1, // Default
			wantThreshold:     50,
		},
		{
			name:              "negative lookback uses default",
			lookbackHours:     -1,
			failureThreshold:  50,
			wantLookback:      1,
			wantThreshold:     50,
		},
		{
			name:              "zero threshold uses default",
			lookbackHours:     3,
			failureThreshold:  0,
			wantLookback:      3,
			wantThreshold:     20, // Default
		},
		{
			name:              "negative threshold uses default",
			lookbackHours:     3,
			failureThreshold:  -1,
			wantLookback:      3,
			wantThreshold:     20,
		},
		{
			name:              "both zero use defaults",
			lookbackHours:     0,
			failureThreshold:  0,
			wantLookback:      1,
			wantThreshold:     20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewAuthFailuresChecker(tt.lookbackHours, tt.failureThreshold)
			if checker.lookbackHours != tt.wantLookback {
				t.Errorf("lookbackHours = %d, want %d", checker.lookbackHours, tt.wantLookback)
			}
			if checker.failureThreshold != tt.wantThreshold {
				t.Errorf("failureThreshold = %d, want %d", checker.failureThreshold, tt.wantThreshold)
			}
		})
	}
}

func TestAuthFailuresChecker_DetectAuthLog(t *testing.T) {
	checker := NewAuthFailuresChecker(1, 20)
	ctx := context.Background()

	t.Run("debian auth.log found", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"test -r /var/log/auth.log": {stdout: "", exitCode: 0},
			},
		}

		logPath, err := checker.detectAuthLog(ctx, executor)
		if err != nil {
			t.Errorf("detectAuthLog() unexpected error: %v", err)
		}
		if logPath != "/var/log/auth.log" {
			t.Errorf("detectAuthLog() = %q, want '/var/log/auth.log'", logPath)
		}
	})

	t.Run("rhel secure log found", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"test -r /var/log/auth.log": {stdout: "", exitCode: 1},
				"test -r /var/log/secure":   {stdout: "", exitCode: 0},
			},
		}

		logPath, err := checker.detectAuthLog(ctx, executor)
		if err != nil {
			t.Errorf("detectAuthLog() unexpected error: %v", err)
		}
		if logPath != "/var/log/secure" {
			t.Errorf("detectAuthLog() = %q, want '/var/log/secure'", logPath)
		}
	})

	t.Run("macos system.log found", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"test -r /var/log/auth.log":   {stdout: "", exitCode: 1},
				"test -r /var/log/secure":     {stdout: "", exitCode: 1},
				"test -r /var/log/system.log": {stdout: "", exitCode: 0},
			},
		}

		logPath, err := checker.detectAuthLog(ctx, executor)
		if err != nil {
			t.Errorf("detectAuthLog() unexpected error: %v", err)
		}
		if logPath != "/var/log/system.log" {
			t.Errorf("detectAuthLog() = %q, want '/var/log/system.log'", logPath)
		}
	})

	t.Run("no readable log found", func(t *testing.T) {
		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"test -r /var/log/auth.log":   {stdout: "", exitCode: 1},
				"test -r /var/log/secure":     {stdout: "", exitCode: 1},
				"test -r /var/log/system.log": {stdout: "", exitCode: 1},
			},
		}

		logPath, err := checker.detectAuthLog(ctx, executor)
		if err == nil {
			t.Error("detectAuthLog() expected error when no log found")
		}
		if logPath != "" {
			t.Errorf("detectAuthLog() = %q, want empty string on error", logPath)
		}
		if !strings.Contains(err.Error(), "no readable auth log") {
			t.Errorf("detectAuthLog() error = %q, should mention 'no readable auth log'", err.Error())
		}
	})
}

func TestAuthFailuresChecker_IsFailedPasswordAttempt(t *testing.T) {
	checker := NewAuthFailuresChecker(1, 20)

	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "Failed password",
			line: "Nov 17 10:30:45 server sshd[1234]: Failed password for user from 192.168.1.100 port 12345 ssh2",
			want: true,
		},
		{
			name: "authentication failure",
			line: "Nov 17 10:30:45 server sshd[1234]: authentication failure; logname= uid=0 euid=0 tty=ssh ruser= rhost=192.168.1.100",
			want: true,
		},
		{
			name: "successful login",
			line: "Nov 17 10:30:45 server sshd[1234]: Accepted password for user from 192.168.1.100 port 12345 ssh2",
			want: false,
		},
		{
			name: "empty line",
			line: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checker.isFailedPasswordAttempt(tt.line); got != tt.want {
				t.Errorf("isFailedPasswordAttempt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthFailuresChecker_IsInvalidUserAttempt(t *testing.T) {
	checker := NewAuthFailuresChecker(1, 20)

	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "Invalid user",
			line: "Nov 17 10:30:45 server sshd[1234]: Invalid user admin from 192.168.1.100 port 12345",
			want: true,
		},
		{
			name: "invalid user lowercase",
			line: "Nov 17 10:30:45 server sshd[1234]: input_userauth_request: invalid user admin [preauth]",
			want: true,
		},
		{
			name: "valid user",
			line: "Nov 17 10:30:45 server sshd[1234]: Accepted publickey for validuser from 192.168.1.100",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checker.isInvalidUserAttempt(tt.line); got != tt.want {
				t.Errorf("isInvalidUserAttempt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthFailuresChecker_IsConnectionClosedAuth(t *testing.T) {
	checker := NewAuthFailuresChecker(1, 20)

	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "Connection closed by authenticating user",
			line: "Nov 17 10:30:45 server sshd[1234]: Connection closed by authenticating user root 192.168.1.100 port 12345 [preauth]",
			want: true,
		},
		{
			name: "Connection closed normal",
			line: "Nov 17 10:30:45 server sshd[1234]: Connection closed by 192.168.1.100 port 12345",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checker.isConnectionClosedAuth(tt.line); got != tt.want {
				t.Errorf("isConnectionClosedAuth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthFailuresChecker_IsDisconnectedAuth(t *testing.T) {
	checker := NewAuthFailuresChecker(1, 20)

	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "Disconnected from authenticating user",
			line: "Nov 17 10:30:45 server sshd[1234]: Disconnected from authenticating user root 192.168.1.100 port 12345 [preauth]",
			want: true,
		},
		{
			name: "Disconnected normal",
			line: "Nov 17 10:30:45 server sshd[1234]: Disconnected from user bob 192.168.1.100 port 12345",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checker.isDisconnectedAuth(tt.line); got != tt.want {
				t.Errorf("isDisconnectedAuth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthFailuresChecker_ExtractFailureDetails(t *testing.T) {
	checker := NewAuthFailuresChecker(1, 20)

	tests := []struct {
		name          string
		line          string
		reason        string
		wantTimestamp string
		wantUser      string
		wantSourceIP  string
		wantNil       bool
	}{
		{
			name:          "failed password with user and IP",
			line:          "Nov 17 10:30:45 server sshd[1234]: Failed password for admin from 192.168.1.100 port 12345 ssh2",
			reason:        "Failed password",
			wantTimestamp: "Nov 17 10:30:45",
			wantUser:      "admin",
			wantSourceIP:  "192.168.1.100",
		},
		{
			name:          "invalid user",
			line:          "Nov 17 10:30:45 server sshd[1234]: Invalid user hacker from 10.0.0.5 port 54321",
			reason:        "Invalid user",
			wantTimestamp: "Nov 17 10:30:45",
			wantUser:      "hacker",
			wantSourceIP:  "10.0.0.5",
		},
		{
			name:          "user with spaces after 'for'",
			line:          "Nov 17 10:30:45 server sshd[1234]: Failed password for    spaceuser    from 192.168.1.100 port 12345",
			reason:        "Failed password",
			wantTimestamp: "Nov 17 10:30:45",
			wantUser:      "spaceuser",
			wantSourceIP:  "192.168.1.100",
		},
		{
			name:          "no user or IP in line",
			line:          "Nov 17 10:30:45 server sshd[1234]: Connection reset by peer",
			reason:        "Connection reset",
			wantTimestamp: "Nov 17 10:30:45",
			wantUser:      "unknown",
			wantSourceIP:  "unknown",
		},
		{
			name:    "line too short",
			line:    "Nov 17",
			reason:  "Failed password",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failure := checker.extractFailureDetails(tt.line, tt.reason)

			if tt.wantNil {
				if failure != nil {
					t.Errorf("extractFailureDetails() = %+v, want nil", failure)
				}
				return
			}

			if failure == nil {
				t.Fatal("extractFailureDetails() = nil, want non-nil")
			}

			if failure.Timestamp != tt.wantTimestamp {
				t.Errorf("Timestamp = %q, want %q", failure.Timestamp, tt.wantTimestamp)
			}
			if failure.User != tt.wantUser {
				t.Errorf("User = %q, want %q", failure.User, tt.wantUser)
			}
			if failure.SourceIP != tt.wantSourceIP {
				t.Errorf("SourceIP = %q, want %q", failure.SourceIP, tt.wantSourceIP)
			}
			if failure.Reason != tt.reason {
				t.Errorf("Reason = %q, want %q", failure.Reason, tt.reason)
			}
		})
	}
}

func TestAuthFailuresChecker_ParseLogContent(t *testing.T) {
	checker := NewAuthFailuresChecker(1, 20)

	t.Run("multiple failure types", func(t *testing.T) {
		content := `Nov 17 10:30:45 server sshd[1234]: Failed password for admin from 192.168.1.100 port 12345 ssh2
Nov 17 10:30:46 server sshd[1235]: Invalid user hacker from 10.0.0.5 port 54321
Nov 17 10:30:47 server sshd[1236]: Accepted publickey for validuser from 192.168.1.1 port 22
Nov 17 10:30:48 server sshd[1237]: Connection closed by authenticating user root 192.168.1.100 port 12345 [preauth]
Nov 17 10:30:49 server sshd[1238]: Disconnected from authenticating user bob 10.0.0.5 port 54321 [preauth]`

		failures := checker.parseLogContent(content)

		// Should find 4 failures (failed password, invalid user, connection closed, disconnected)
		if len(failures) != 4 {
			t.Errorf("parseLogContent() found %d failures, want 4", len(failures))
		}

		// Verify first failure
		if failures[0].User != "admin" {
			t.Errorf("First failure User = %q, want 'admin'", failures[0].User)
		}
		if failures[0].Reason != "Failed password" {
			t.Errorf("First failure Reason = %q, want 'Failed password'", failures[0].Reason)
		}

		// Verify second failure (invalid user)
		if failures[1].User != "hacker" {
			t.Errorf("Second failure User = %q, want 'hacker'", failures[1].User)
		}
		if failures[1].Reason != "Invalid user" {
			t.Errorf("Second failure Reason = %q, want 'Invalid user'", failures[1].Reason)
		}
	})

	t.Run("empty content", func(t *testing.T) {
		failures := checker.parseLogContent("")
		if len(failures) != 0 {
			t.Errorf("parseLogContent('') = %d failures, want 0", len(failures))
		}
	})

	t.Run("no failures", func(t *testing.T) {
		content := `Nov 17 10:30:45 server sshd[1234]: Accepted publickey for user1 from 192.168.1.1 port 22
Nov 17 10:30:46 server sshd[1235]: Session opened for user user1 by (uid=1000)`

		failures := checker.parseLogContent(content)
		if len(failures) != 0 {
			t.Errorf("parseLogContent() = %d failures, want 0", len(failures))
		}
	})
}

func TestAuthFailuresChecker_FilterRecentFailures(t *testing.T) {
	checker := NewAuthFailuresChecker(1, 20)

	failures := []FailedAuth{
		{Timestamp: "Nov 17 10:30:45", User: "admin", SourceIP: "192.168.1.100"},
		{Timestamp: "Nov 17 10:30:46", User: "root", SourceIP: "10.0.0.5"},
	}

	// Current implementation returns all failures
	recent := checker.filterRecentFailures(failures)
	if len(recent) != len(failures) {
		t.Errorf("filterRecentFailures() returned %d failures, want %d", len(recent), len(failures))
	}
}

func TestAuthFailuresChecker_IdentifyAttackSources(t *testing.T) {
	checker := NewAuthFailuresChecker(1, 20)

	t.Run("multiple failures from same IP", func(t *testing.T) {
		failures := []FailedAuth{
			{Timestamp: "Nov 17 10:30:45", User: "admin", SourceIP: "192.168.1.100"},
			{Timestamp: "Nov 17 10:30:46", User: "root", SourceIP: "192.168.1.100"},
			{Timestamp: "Nov 17 10:30:47", User: "admin", SourceIP: "192.168.1.100"},
			{Timestamp: "Nov 17 10:30:48", User: "user1", SourceIP: "10.0.0.5"},
		}

		sources := checker.identifyAttackSources(failures)

		if len(sources) != 2 {
			t.Fatalf("identifyAttackSources() = %d sources, want 2", len(sources))
		}

		// Sources should be sorted by count (descending)
		if sources[0].FailureCount != 3 {
			t.Errorf("Top source FailureCount = %d, want 3", sources[0].FailureCount)
		}
		if sources[0].IP != "192.168.1.100" {
			t.Errorf("Top source IP = %q, want '192.168.1.100'", sources[0].IP)
		}

		// Should have 2 unique users
		if len(sources[0].Users) != 2 {
			t.Errorf("Top source Users = %d, want 2", len(sources[0].Users))
		}

		// Second source
		if sources[1].FailureCount != 1 {
			t.Errorf("Second source FailureCount = %d, want 1", sources[1].FailureCount)
		}
	})

	t.Run("skip unknown IPs", func(t *testing.T) {
		failures := []FailedAuth{
			{Timestamp: "Nov 17 10:30:45", User: "admin", SourceIP: "unknown"},
			{Timestamp: "Nov 17 10:30:46", User: "root", SourceIP: "192.168.1.100"},
		}

		sources := checker.identifyAttackSources(failures)

		// Should only have 1 source (unknown skipped)
		if len(sources) != 1 {
			t.Errorf("identifyAttackSources() = %d sources, want 1 (unknown should be skipped)", len(sources))
		}

		if sources[0].IP == "unknown" {
			t.Error("identifyAttackSources() should skip 'unknown' IPs")
		}
	})

	t.Run("empty failures", func(t *testing.T) {
		sources := checker.identifyAttackSources([]FailedAuth{})
		if len(sources) != 0 {
			t.Errorf("identifyAttackSources([]) = %d sources, want 0", len(sources))
		}
	})
}

func TestAuthFailuresChecker_ExtractInvalidUsers(t *testing.T) {
	checker := NewAuthFailuresChecker(1, 20)

	t.Run("extract invalid users", func(t *testing.T) {
		failures := []FailedAuth{
			{Timestamp: "Nov 17 10:30:45", User: "admin", SourceIP: "192.168.1.100", Reason: "Invalid user"},
			{Timestamp: "Nov 17 10:30:46", User: "root", SourceIP: "192.168.1.100", Reason: "Failed password"},
			{Timestamp: "Nov 17 10:30:47", User: "hacker", SourceIP: "10.0.0.5", Reason: "Invalid user"},
			{Timestamp: "Nov 17 10:30:48", User: "admin", SourceIP: "10.0.0.5", Reason: "Invalid user"}, // Duplicate
		}

		users := checker.extractInvalidUsers(failures)

		if len(users) != 2 {
			t.Errorf("extractInvalidUsers() = %d users, want 2 (admin, hacker)", len(users))
		}

		// Check both users are present (order doesn't matter)
		foundAdmin := false
		foundHacker := false
		for _, u := range users {
			if u == "admin" {
				foundAdmin = true
			}
			if u == "hacker" {
				foundHacker = true
			}
		}

		if !foundAdmin {
			t.Error("extractInvalidUsers() should include 'admin'")
		}
		if !foundHacker {
			t.Error("extractInvalidUsers() should include 'hacker'")
		}
	})

	t.Run("no invalid users", func(t *testing.T) {
		failures := []FailedAuth{
			{Timestamp: "Nov 17 10:30:45", User: "admin", SourceIP: "192.168.1.100", Reason: "Failed password"},
		}

		users := checker.extractInvalidUsers(failures)
		if len(users) != 0 {
			t.Errorf("extractInvalidUsers() = %d users, want 0", len(users))
		}
	})
}

func TestAuthFailuresChecker_FormatMessage(t *testing.T) {
	checker := NewAuthFailuresChecker(2, 20) // 2 hours lookback

	tests := []struct {
		name            string
		totalFailures   int
		attackSources   int
		criticalSources int
		wantContains    []string
	}{
		{
			name:          "no failures",
			totalFailures: 0,
			wantContains:  []string{"No failed authentication"},
		},
		{
			name:            "with critical sources",
			totalFailures:   500,
			attackSources:   10,
			criticalSources: 3,
			wantContains:    []string{"500 failed auth attempts", "2 hour(s)", "3 IP(s) with >100 attempts"},
		},
		{
			name:          "with attack sources but no critical",
			totalFailures: 50,
			attackSources: 5,
			wantContains:  []string{"50 failed auth attempts", "5 unique IP(s)"},
		},
		{
			name:          "failures but no grouped sources",
			totalFailures: 10,
			attackSources: 0,
			wantContains:  []string{"10 failed auth attempts"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := checker.formatMessage(tt.totalFailures, tt.attackSources, tt.criticalSources)

			for _, want := range tt.wantContains {
				if !strings.Contains(msg, want) {
					t.Errorf("formatMessage() = %q, should contain %q", msg, want)
				}
			}
		})
	}
}

func TestAuthFailuresChecker_HelperFunctions(t *testing.T) {
	t.Run("min", func(t *testing.T) {
		tests := []struct {
			a, b, want int
		}{
			{5, 10, 5},
			{10, 5, 5},
			{7, 7, 7},
			{0, 5, 0},
			{-1, 5, -1},
		}

		for _, tt := range tests {
			if got := min(tt.a, tt.b); got != tt.want {
				t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		}
	})

	t.Run("contains", func(t *testing.T) {
		slice := []string{"apple", "banana", "cherry"}

		if !contains(slice, "banana") {
			t.Error("contains() should find 'banana'")
		}
		if contains(slice, "grape") {
			t.Error("contains() should not find 'grape'")
		}
		if contains([]string{}, "anything") {
			t.Error("contains() should return false for empty slice")
		}
	})
}

func TestAuthFailuresChecker_Run(t *testing.T) {
	t.Run("successful check with failures", func(t *testing.T) {
		checker := NewAuthFailuresChecker(1, 20)

		logContent := `Nov 17 10:30:45 server sshd[1234]: Failed password for admin from 192.168.1.100 port 12345 ssh2
Nov 17 10:30:46 server sshd[1235]: Invalid user hacker from 10.0.0.5 port 54321
Nov 17 10:30:47 server sshd[1236]: Failed password for admin from 192.168.1.100 port 12346 ssh2`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"test -r /var/log/auth.log":        {stdout: "", exitCode: 0},
				"tail -n 1000 /var/log/auth.log":   {stdout: logContent, exitCode: 0},
			},
		}

		ctx := context.Background()
		result, err := checker.Run(ctx, executor)

		if err != nil {
			t.Errorf("Run() unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("Run() result is nil")
		}

		// Check data values
		totalFailures, _ := result.GetDataValue("total_failures")
		if totalFailures != 3 {
			t.Errorf("total_failures = %v, want 3", totalFailures)
		}

		attackSources, _ := result.GetDataValue("unique_attack_sources")
		if attackSources != 2 {
			t.Errorf("unique_attack_sources = %v, want 2", attackSources)
		}

		invalidUsers, _ := result.GetDataValue("invalid_users_attempted")
		if invalidUsers != 1 {
			t.Errorf("invalid_users_attempted = %v, want 1 (hacker)", invalidUsers)
		}

		// Check message
		if !strings.Contains(result.Message, "3 failed auth attempts") {
			t.Errorf("Message should contain '3 failed auth attempts', got: %q", result.Message)
		}
	})

	t.Run("log detection fails", func(t *testing.T) {
		checker := NewAuthFailuresChecker(1, 20)

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"test -r /var/log/auth.log":   {stdout: "", exitCode: 1},
				"test -r /var/log/secure":     {stdout: "", exitCode: 1},
				"test -r /var/log/system.log": {stdout: "", exitCode: 1},
			},
		}

		ctx := context.Background()
		result, err := checker.Run(ctx, executor)

		// Should not return error, but result should contain error data
		if err != nil {
			t.Errorf("Run() should not return error, got: %v", err)
		}

		if result == nil {
			t.Fatal("Run() result is nil")
		}

		// Should have error in data
		errorData, _ := result.GetDataValue("error")
		if errorData == nil || errorData == "" {
			t.Error("Result should contain 'error' when log detection fails")
		}
	})

	t.Run("log parsing fails", func(t *testing.T) {
		checker := NewAuthFailuresChecker(1, 20)

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"test -r /var/log/auth.log":      {stdout: "", exitCode: 0},
				"tail -n 1000 /var/log/auth.log": {stdout: "", exitCode: 1}, // tail fails
			},
		}

		ctx := context.Background()
		result, err := checker.Run(ctx, executor)

		if err != nil {
			t.Errorf("Run() should not return error, got: %v", err)
		}

		// Should have error in data
		errorData, _ := result.GetDataValue("error")
		if errorData == nil || errorData == "" {
			t.Error("Result should contain 'error' when log parsing fails")
		}
	})

	t.Run("no failures found", func(t *testing.T) {
		checker := NewAuthFailuresChecker(1, 20)

		logContent := `Nov 17 10:30:45 server sshd[1234]: Accepted publickey for user1 from 192.168.1.1 port 22
Nov 17 10:30:46 server sshd[1235]: Session opened for user user1 by (uid=1000)`

		executor := &mockExecutor{
			responses: map[string]mockResponse{
				"test -r /var/log/auth.log":      {stdout: "", exitCode: 0},
				"tail -n 1000 /var/log/auth.log": {stdout: logContent, exitCode: 0},
			},
		}

		ctx := context.Background()
		result, err := checker.Run(ctx, executor)

		if err != nil {
			t.Errorf("Run() unexpected error: %v", err)
		}

		totalFailures, _ := result.GetDataValue("total_failures")
		if totalFailures != 0 {
			t.Errorf("total_failures = %v, want 0", totalFailures)
		}

		if !strings.Contains(result.Message, "No failed authentication") {
			t.Errorf("Message should mention no failures, got: %q", result.Message)
		}
	})
}
