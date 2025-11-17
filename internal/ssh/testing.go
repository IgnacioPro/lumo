package ssh

import (
	"os"
	"testing"
)

// skipInCI skips the test if running in CI environment.
// This is used to skip SSH tests that require actual SSH infrastructure.
//
// CI is detected by the presence of the LUMO_CI environment variable.
func skipInCI(t *testing.T) {
	t.Helper()
	if os.Getenv("LUMO_CI") != "" {
		t.Skip("Skipping SSH test in CI environment (LUMO_CI is set)")
	}
}

// skipIfNoSSH skips the test if SSH connectivity is not available.
// This checks for the presence of SKIP_SSH_TESTS environment variable.
func skipIfNoSSH(t *testing.T) {
	t.Helper()
	if os.Getenv("SKIP_SSH_TESTS") != "" {
		t.Skip("Skipping SSH test (SKIP_SSH_TESTS is set)")
	}
}
