package remediation

import (
	"fmt"
	"regexp"
	"strings"
)

// Service name validation regex - allows alphanumeric, dots, dashes, and underscores
// This matches standard systemd service naming conventions
var serviceNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// Process pattern validation regex - basic alphanumeric and common safe characters
// For more complex patterns, use shellQuote to prevent injection
var processPatternRegex = regexp.MustCompile(`^[a-zA-Z0-9_./\-]+$`)

// isValidServiceName validates a systemd service name
// Returns true if the name contains only safe characters
func isValidServiceName(name string) bool {
	if name == "" {
		return false
	}

	// Check length (systemd has a 256 char limit for unit names)
	if len(name) > 256 {
		return false
	}

	// Check against regex
	return serviceNameRegex.MatchString(name)
}

// isValidProcessPattern validates a process pattern for pgrep/pkill
// Returns true if the pattern contains only safe characters
func isValidProcessPattern(pattern string) bool {
	if pattern == "" {
		return false
	}

	// Check length
	if len(pattern) > 1024 {
		return false
	}

	// Check against regex
	return processPatternRegex.MatchString(pattern)
}

// shellQuote safely quotes a string for use in POSIX shell commands
// This prevents command injection by escaping all special characters.
//
// Algorithm:
// 1. Wrap the entire string in single quotes
// 2. Replace any single quotes with: '\''  (end quote, escaped quote, start quote)
//
// This is the POSIX-standard way to quote shell arguments and handles all edge cases.
// Examples:
//
//	shellQuote("hello")           → 'hello'
//	shellQuote("hello world")     → 'hello world'
//	shellQuote("it's")            → 'it'\''s'
//	shellQuote("a'b'c")           → 'a'\''b'\''c'
//	shellQuote("$HOME")           → '$HOME'  (literal, not expanded)
//	shellQuote("`whoami`")        → '`whoami`'  (literal, not executed)
//	shellQuote("nginx; rm -rf /") → 'nginx; rm -rf /'  (literal, not executed)
func shellQuote(s string) string {
	// Handle empty string
	if s == "" {
		return "''"
	}

	// POSIX shell quoting: wrap in single quotes and escape existing single quotes
	// Replace ' with '\'' which:
	//   1. Ends the current single-quoted string (')
	//   2. Adds an escaped single quote (\')
	//   3. Starts a new single-quoted string (')
	s = strings.ReplaceAll(s, "'", `'\''`)
	return "'" + s + "'"
}

// validateServiceName validates a service name and returns an error if invalid
func validateServiceName(name string) error {
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	if !isValidServiceName(name) {
		return fmt.Errorf("invalid service name '%s': must contain only alphanumeric characters, dots, dashes, and underscores (max 256 chars)", name)
	}

	return nil
}

// validateProcessPattern validates a process pattern and returns an error if invalid
func validateProcessPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("process pattern cannot be empty")
	}

	if !isValidProcessPattern(pattern) {
		return fmt.Errorf("invalid process pattern '%s': must contain only alphanumeric characters and basic path characters (max 1024 chars)", pattern)
	}

	return nil
}
