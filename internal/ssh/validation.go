package ssh

import (
	"fmt"
	"net"
	"strings"
)

// ValidateSSHTarget validates an SSH target hostname or IP address to prevent SSRF attacks
// Returns an error if the target points to a private/reserved IP range or is otherwise suspicious
func ValidateSSHTarget(target string) error {
	if target == "" {
		return fmt.Errorf("SSH target cannot be empty")
	}

	// Extract hostname/IP (strip port if present)
	host := target
	if strings.Contains(target, ":") {
		var err error
		host, _, err = net.SplitHostPort(target)
		if err != nil {
			// If SplitHostPort fails, it might be an IPv6 address without port
			// Try parsing as is
			host = target
		}
	}

	// Try to resolve the hostname to IP addresses
	ips, err := net.LookupIP(host)
	if err != nil {
		// If DNS lookup fails, check if it's a direct IP address
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("invalid SSH target: cannot resolve hostname or parse IP: %s", host)
		}
		ips = []net.IP{ip}
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

	// If allowlist is specified, target must be in it
	if len(allowlist) > 0 {
		// Extract hostname/IP (strip port if present)
		host := target
		if strings.Contains(target, ":") {
			var err error
			host, _, err = net.SplitHostPort(target)
			if err != nil {
				host = target
			}
		}

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
