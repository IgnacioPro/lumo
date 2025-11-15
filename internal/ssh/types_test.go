package ssh

import (
	"testing"
	"time"
)

func TestConnectionStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status ConnectionStatus
		want   string
	}{
		{"disconnected", StatusDisconnected, "disconnected"},
		{"connecting", StatusConnecting, "connecting"},
		{"connected", StatusConnected, "connected"},
		{"reconnecting", StatusReconnecting, "reconnecting"},
		{"failed", StatusFailed, "failed"},
		{"unknown", ConnectionStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("ConnectionStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuthMethod_String(t *testing.T) {
	tests := []struct {
		name   string
		method AuthMethod
		want   string
	}{
		{"agent", AuthMethodAgent, "ssh-agent"},
		{"key", AuthMethodKey, "private-key"},
		{"password", AuthMethodPassword, "password"},
		{"interactive", AuthMethodInteractive, "keyboard-interactive"},
		{"unknown", AuthMethod(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.method.String(); got != tt.want {
				t.Errorf("AuthMethod.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandResult_Success(t *testing.T) {
	tests := []struct {
		name   string
		result *CommandResult
		want   bool
	}{
		{
			name: "success - exit 0, no error",
			result: &CommandResult{
				ExitCode: 0,
				Error:    nil,
			},
			want: true,
		},
		{
			name: "failure - exit 1",
			result: &CommandResult{
				ExitCode: 1,
				Error:    nil,
			},
			want: false,
		},
		{
			name: "failure - exit 0 but has error",
			result: &CommandResult{
				ExitCode: 0,
				Error:    &SSHError{Op: "test", Message: "error"},
			},
			want: false,
		},
		{
			name: "failure - exit 1 and error",
			result: &CommandResult{
				ExitCode: 1,
				Error:    &SSHError{Op: "test", Message: "error"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Success(); got != tt.want {
				t.Errorf("CommandResult.Success() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandResult_Output(t *testing.T) {
	tests := []struct {
		name   string
		result *CommandResult
		want   string
	}{
		{
			name: "stdout only",
			result: &CommandResult{
				Stdout: "output line 1\noutput line 2",
				Stderr: "",
			},
			want: "output line 1\noutput line 2",
		},
		{
			name: "stderr only",
			result: &CommandResult{
				Stdout: "",
				Stderr: "error line 1",
			},
			want: "\nerror line 1",
		},
		{
			name: "both stdout and stderr",
			result: &CommandResult{
				Stdout: "output",
				Stderr: "error",
			},
			want: "output\nerror",
		},
		{
			name: "empty output",
			result: &CommandResult{
				Stdout: "",
				Stderr: "",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Output(); got != tt.want {
				t.Errorf("CommandResult.Output() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConnectionInfo(t *testing.T) {
	now := time.Now()
	info := &ConnectionInfo{
		Host:              "example.com",
		Port:              22,
		User:              "admin",
		Status:            StatusConnected,
		AuthMethodUsed:    AuthMethodKey,
		ConnectedAt:       now,
		LastHealthCheck:   now.Add(time.Minute),
		ReconnectAttempts: 0,
	}

	if info.Host != "example.com" {
		t.Errorf("Host = %s, want example.com", info.Host)
	}
	if info.Port != 22 {
		t.Errorf("Port = %d, want 22", info.Port)
	}
	if info.Status != StatusConnected {
		t.Errorf("Status = %v, want %v", info.Status, StatusConnected)
	}
}

func TestCommandOptions(t *testing.T) {
	opts := &CommandOptions{
		Timeout:    30 * time.Second,
		Env:        map[string]string{"FOO": "bar"},
		WorkingDir: "/tmp",
		UsePTY:     true,
	}

	if opts.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", opts.Timeout)
	}
	if opts.Env["FOO"] != "bar" {
		t.Errorf("Env[FOO] = %s, want bar", opts.Env["FOO"])
	}
	if !opts.UsePTY {
		t.Error("UsePTY = false, want true")
	}
}

func TestDefaultConstants(t *testing.T) {
	tests := []struct {
		name  string
		got   time.Duration
		want  time.Duration
	}{
		{"DefaultCommandTimeout", DefaultCommandTimeout, 5 * time.Minute},
		{"DefaultConnectionTimeout", DefaultConnectionTimeout, 30 * time.Second},
		{"DefaultKeepAliveInterval", DefaultKeepAliveInterval, 30 * time.Second},
		{"DefaultRetryInterval", DefaultRetryInterval, 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}

	if DefaultMaxRetries != 3 {
		t.Errorf("DefaultMaxRetries = %d, want 3", DefaultMaxRetries)
	}
	if MaxOutputBufferSize != 10*1024*1024 {
		t.Errorf("MaxOutputBufferSize = %d, want 10485760", MaxOutputBufferSize)
	}
}
