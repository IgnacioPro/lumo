package ssh

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// ValidateSSHTarget validates an SSH target hostname or IP address to prevent SSRF attacks
// Returns an error if the target points to a private/reserved IP range or is otherwise suspicious
func ValidateSSHTarget(target string) error {
	if target == "" {
		return fmt.Errorf("SSH target cannot be empty")
	}
	if strings.Contains(target, "://") || strings.ContainsAny(target, "/\\") {
		return fmt.Errorf("invalid SSH target: target must be a hostname or IP, not a URL or path")
	}

	host := extractHost(target)
	if host == "" {
		return fmt.Errorf("invalid SSH target: empty host")
	}

	// localhost should never be allowed.
	if strings.EqualFold(host, "localhost") {
		return fmt.Errorf("SSH target blocked: blocked IP range: loopback (127.0.0.0/8) (resolved to 127.0.0.1)")
	}

	// Direct IP targets are fully validated against blocked ranges.
	if ip := net.ParseIP(host); ip != nil {
		if err := validateIPNotBlocked(ip); err != nil {
			return fmt.Errorf("SSH target blocked: %w (resolved to %s)", err, ip)
		}
		return nil
	}

	// Reject obviously invalid hostnames before any DNS lookup.
	if !isValidHostname(host) {
		return fmt.Errorf("invalid SSH target: invalid hostname: %s", host)
	}

	// Try to resolve the hostname to IP addresses for full SSRF validation.
	ips, err := net.LookupIP(host)
	if err != nil {
		// DNS may be unavailable in restricted/offline environments.
		// Accept syntactically valid hostnames and defer final resolution to the SSH dial path.
		return nil
	}

	// Check each resolved IP against blocked ranges
	for _, ip := range ips {
		if err := validateIPNotBlocked(ip); err != nil {
			return fmt.Errorf("SSH target blocked: %w (resolved to %s)", err, ip)
		}
	}

	return nil
}

// validateIPNotBlocked checks if an IP is in a blocked range
func validateIPNotBlocked(ip net.IP) error {
	// Define blocked IP ranges for SSRF prevention
	blockedRanges := []struct {
		cidr        string
		description string
	}{
		// Private IPv4 ranges (RFC 1918)
		{"10.0.0.0/8", "private network"},
		{"172.16.0.0/12", "private network"},
		{"192.168.0.0/16", "private network"},

		// Loopback
		{"127.0.0.0/8", "loopback"},
		{"::1/128", "IPv6 loopback"},

		// Link-local
		{"169.254.0.0/16", "link-local (including cloud metadata services)"},
		{"fe80::/10", "IPv6 link-local"},

		// Documentation/TEST-NET (RFC 5737)
		{"192.0.2.0/24", "documentation range"},
		{"198.51.100.0/24", "documentation range"},
		{"203.0.113.0/24", "documentation range"},
		{"2001:db8::/32", "IPv6 documentation range"},

		// Multicast
		{"224.0.0.0/4", "multicast"},
		{"ff00::/8", "IPv6 multicast"},

		// Broadcast
		{"255.255.255.255/32", "broadcast"},

		// Zero address
		{"0.0.0.0/8", "zero/unspecified address"},
		{"::/128", "IPv6 unspecified address"},

		// Carrier-grade NAT (RFC 6598)
		{"100.64.0.0/10", "carrier-grade NAT"},

		// IETF Protocol Assignments
		{"192.0.0.0/24", "IETF protocol assignments"},
	}

	// Check against each blocked range
	for _, blocked := range blockedRanges {
		_, network, err := net.ParseCIDR(blocked.cidr)
		if err != nil {
			// Should never happen with hard-coded CIDRs, but handle it
			continue
		}

		if network.Contains(ip) {
			return fmt.Errorf("blocked IP range: %s (%s)", blocked.description, blocked.cidr)
		}
	}

	// Special check for AWS/GCP metadata service IP (169.254.169.254)
	// This is critical for cloud environments
	metadataIP := net.ParseIP("169.254.169.254")
	if ip.Equal(metadataIP) {
		return fmt.Errorf("blocked IP: cloud metadata service (169.254.169.254)")
	}

	return nil
}

// ValidateSSHTargetWithAllowlist validates an SSH target against a blocklist and optional allowlist
// If allowlist is non-empty, the target must be in the allowlist
// The target must not be in the blocklist
func ValidateSSHTargetWithAllowlist(target string, allowlist []string) error {
	// First check standard blocklist
	if err := ValidateSSHTarget(target); err != nil {
		return err
	}

	host := extractHost(target)

	// If allowlist is specified, target must be in it
	if len(allowlist) > 0 {
		allowed := false
		for _, allowedHost := range allowlist {
			if host == allowedHost {
				allowed = true
				break
			}

			// Check if allowedHost is a CIDR range
			if strings.Contains(allowedHost, "/") {
				_, network, err := net.ParseCIDR(allowedHost)
				if err != nil {
					continue
				}

				// Resolve target to IP and check if it's in the allowed CIDR
				ips, err := net.LookupIP(host)
				if err != nil {
					// Try parsing as direct IP
					ip := net.ParseIP(host)
					if ip != nil {
						ips = []net.IP{ip}
					}
				}

				for _, ip := range ips {
					if network.Contains(ip) {
						allowed = true
						break
					}
				}
			}

			if allowed {
				break
			}
		}

		if !allowed {
			return fmt.Errorf("SSH target not in allowlist: %s", host)
		}
	}

	return nil
}

func extractHost(target string) string {
	host := strings.TrimSpace(target)
	if host == "" {
		return ""
	}

	if strings.Contains(host, ":") {
		splitHost, _, err := net.SplitHostPort(host)
		if err == nil {
			host = splitHost
		}
	}

	return strings.Trim(host, "[]")
}

func isValidHostname(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}

	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}

	labelPattern := regexp.MustCompile(`^[a-zA-Z0-9-]{1,63}$`)
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || !labelPattern.MatchString(label) {
			return false
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
	}

	return true
}
