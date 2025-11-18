package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"
)

// buildAuthMethods creates an ordered list of SSH authentication methods based on configuration
func buildAuthMethods(config *ClientConfig, host, user string) ([]ssh.AuthMethod, []AuthMethod, error) {
	var authMethods []ssh.AuthMethod
	var authMethodTypes []AuthMethod

	for _, method := range config.PreferredAuthMethods {
		switch method {
		case AuthMethodAgent:
			if authMethod, err := trySSHAgent(); err == nil && authMethod != nil {
				authMethods = append(authMethods, authMethod)
				authMethodTypes = append(authMethodTypes, AuthMethodAgent)
			}

		case AuthMethodKey:
			keyPath := config.KeyPath
			if keyPath == "" {
				// Try to find a default key
				if defaultKey, err := FindAvailableKey(); err == nil {
					keyPath = defaultKey
				}
			}

			if keyPath != "" {
				authMethod, err := tryKeyFile(keyPath, config.Passphrase, user, host)
				if err == nil && authMethod != nil {
					authMethods = append(authMethods, authMethod)
					authMethodTypes = append(authMethodTypes, AuthMethodKey)
				}
			}

		case AuthMethodPassword:
			if config.Password != "" {
				authMethods = append(authMethods, ssh.Password(config.Password))
				authMethodTypes = append(authMethodTypes, AuthMethodPassword)
			} else {
				// Prompt for password if not provided
				if authMethod := tryPasswordPrompt(user, host); authMethod != nil {
					authMethods = append(authMethods, authMethod)
					authMethodTypes = append(authMethodTypes, AuthMethodPassword)
				}
			}

		case AuthMethodInteractive:
			authMethod := tryInteractive(user, host)
			authMethods = append(authMethods, authMethod)
			authMethodTypes = append(authMethodTypes, AuthMethodInteractive)
		}
	}

	if len(authMethods) == 0 {
		return nil, nil, fmt.Errorf("no authentication methods available")
	}

	return authMethods, authMethodTypes, nil
}

// trySSHAgent attempts to connect to the SSH agent and get authentication method
func trySSHAgent() (ssh.AuthMethod, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}

	agentClient := agent.NewClient(conn)

	// Test if agent has any keys
	signers, err := agentClient.Signers()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to get signers from SSH agent: %w", err)
	}

	if len(signers) == 0 {
		_ = conn.Close()
		return nil, fmt.Errorf("SSH agent has no keys loaded")
	}

	return ssh.PublicKeysCallback(agentClient.Signers), nil
}

// tryKeyFile attempts to read and parse an SSH private key file
func tryKeyFile(keyPath, passphrase, user, host string) (ssh.AuthMethod, error) {
	// Read the private key file
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file %s: %w", keyPath, err)
	}

	// Try to parse the key
	var signer ssh.Signer

	// First try without passphrase
	signer, err = ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		// Check if the error is due to encrypted key
		if strings.Contains(err.Error(), "cannot decode encrypted private keys") ||
			strings.Contains(err.Error(), "encrypted") {

			// If passphrase is provided, try with it
			if passphrase != "" {
				signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(passphrase))
				if err != nil {
					return nil, fmt.Errorf("failed to parse encrypted key with passphrase: %w", err)
				}
			} else {
				// Prompt for passphrase
				passphrase, promptErr := promptForPassphrase(keyPath, user, host)
				if promptErr != nil {
					return nil, fmt.Errorf("key is encrypted but passphrase prompt failed: %w", promptErr)
				}

				signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(passphrase))
				if err != nil {
					return nil, fmt.Errorf("failed to parse encrypted key with passphrase: %w", err)
				}
			}
		} else {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	return ssh.PublicKeys(signer), nil
}

// tryPasswordPrompt prompts the user for a password
func tryPasswordPrompt(user, host string) ssh.AuthMethod {
	password, err := promptForPassword(user, host)
	if err != nil {
		return nil
	}
	return ssh.Password(password)
}

// tryInteractive creates a keyboard-interactive authentication method
func tryInteractive(user, host string) ssh.AuthMethod {
	return ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, len(questions))

		for i, question := range questions {
			var answer string
			var err error

			if echos[i] {
				// Echo is enabled, read normally
				fmt.Printf("%s ", question)
				_, _ = fmt.Scanln(&answer)
			} else {
				// Echo is disabled, read password securely
				fmt.Printf("%s ", question)
				passwordBytes, readErr := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Println() // Add newline after password input
				if readErr != nil {
					err = readErr
				} else {
					answer = string(passwordBytes)
				}
			}

			if err != nil {
				return nil, fmt.Errorf("failed to read answer for question '%s': %w", question, err)
			}

			answers[i] = answer
		}

		return answers, nil
	})
}

// promptForPassword securely prompts the user for a password
func promptForPassword(user, host string) (string, error) {
	// Write to stderr to ensure prompt is displayed immediately (stderr is unbuffered)
	fmt.Fprintf(os.Stderr, "Password for %s@%s: ", user, host)

	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // Add newline after password input

	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	password := string(passwordBytes)
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	return password, nil
}

// promptForPassphrase securely prompts the user for a key passphrase
func promptForPassphrase(keyPath, user, host string) (string, error) {
	// Write to stderr to ensure prompt is displayed immediately (stderr is unbuffered)
	fmt.Fprintf(os.Stderr, "Passphrase for key %s (%s@%s): ", keyPath, user, host)

	passphraseBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // Add newline after passphrase input

	if err != nil {
		return "", fmt.Errorf("failed to read passphrase: %w", err)
	}

	passphrase := string(passphraseBytes)
	if passphrase == "" {
		return "", fmt.Errorf("passphrase cannot be empty")
	}

	return passphrase, nil
}

// getHostKeyCallback returns the appropriate host key callback based on configuration
func getHostKeyCallback(config *ClientConfig) (ssh.HostKeyCallback, error) {
	// If strict host key checking is disabled, use insecure callback
	if !config.StrictHostKeyChecking {
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: SSH host key verification is DISABLED. Connections are vulnerable to MITM attacks!\n")
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Set strict_host_key_checking: true in config for production use!\n")
		return ssh.InsecureIgnoreHostKey(), nil
	}

	// If known_hosts path is not set, use insecure callback
	if config.KnownHostsPath == "" {
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: No known_hosts file configured. Falling back to insecure mode!\n")
		return ssh.InsecureIgnoreHostKey(), nil
	}

	// Expand home directory if needed
	knownHostsPath := config.KnownHostsPath
	if knownHostsPath[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		knownHostsPath = filepath.Join(home, knownHostsPath[1:])
	}

	// Use known_hosts file for host key verification
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load known_hosts file %s: %w", knownHostsPath, err)
	}

	return callback, nil
}

// validateKeyFile checks if a key file exists and has correct permissions
func validateKeyFile(keyPath string) error {
	info, err := os.Stat(keyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("key file not found: %s", keyPath)
		}
		return fmt.Errorf("failed to stat key file: %w", err)
	}

	// Check permissions - must be exactly 600 or 400 (no group/world access)
	perm := info.Mode().Perm()
	if perm != 0600 && perm != 0400 {
		return fmt.Errorf("key file %s has insecure permissions %o (must be exactly 0600 or 0400)", keyPath, perm)
	}

	return nil
}

// detectKeyType detects the type of SSH key from file contents
func detectKeyType(keyBytes []byte) string {
	keyStr := string(keyBytes)

	if strings.Contains(keyStr, "BEGIN RSA PRIVATE KEY") {
		return "RSA"
	} else if strings.Contains(keyStr, "BEGIN OPENSSH PRIVATE KEY") {
		return "OpenSSH"
	} else if strings.Contains(keyStr, "BEGIN EC PRIVATE KEY") {
		return "ECDSA"
	} else if strings.Contains(keyStr, "BEGIN DSA PRIVATE KEY") {
		return "DSA"
	}

	return "Unknown"
}
