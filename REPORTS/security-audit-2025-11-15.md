# Security Audit Report
**Project**: Lumo - Intelligent SRE/DevOps Automation Agent  
**Date**: November 15, 2025  
**Total Findings**: 18 (3 Critical, 6 High, 5 Medium, 4 Low)

---

## Executive Summary

Lumo is a Go-based SRE/DevOps automation tool in Phase 4 of development that connects to remote servers via SSH, performs system diagnostics, and integrates with AI providers. The security audit reveals **significant security concerns** that must be addressed before production deployment, particularly given the tool's ability to execute commands on remote systems and the planned auto-remediation feature (Phase 5).

**Risk Level**: **HIGH**

**Key Findings**:
1. **CRITICAL**: SSH host key verification disabled by default - enables MITM attacks
2. **CRITICAL**: Command injection vulnerabilities in WorkingDir parameter
3. **CRITICAL**: Passwords accepted via command-line flags (visible in process lists)
4. **HIGH**: No TLS certificate validation for AI provider API calls
5. **HIGH**: Sensitive credentials may be logged in debug mode
6. **HIGH**: Missing input validation on network targets enables SSRF attacks

The project demonstrates good practices in some areas (API keys via environment variables, .gitignore configuration), but critical security gaps exist that could lead to credential theft, unauthorized access, and command injection attacks.

---

## 🔴 CRITICAL Severity Findings

### 1. SSH Host Key Verification Disabled by Default (MITM Attack Vector)

**Location**: `internal/ssh/config.go:58`, `internal/config/config.go:93`, `internal/ssh/auth.go:232-239`  
**Category**: Authentication / Man-in-the-Middle  
**Description**: The SSH client defaults to `StrictHostKeyChecking: false` and uses `ssh.InsecureIgnoreHostKey()` when no known_hosts file is configured. This completely disables host key verification, making the application vulnerable to man-in-the-middle (MITM) attacks.

**Evidence**:
```go
// internal/ssh/config.go:58
StrictHostKeyChecking: false, // Default to false for easier development

// internal/ssh/auth.go:232-239
func getHostKeyCallback(config *ClientConfig) (ssh.HostKeyCallback, error) {
    // If strict host key checking is disabled, use insecure callback
    if !config.StrictHostKeyChecking {
        return ssh.InsecureIgnoreHostKey(), nil
    }
    
    // If known_hosts path is not set, use insecure callback
    if config.KnownHostsPath == "" {
        return ssh.InsecureIgnoreHostKey(), nil
    }
    // ...
}
```

**Impact**: 
- Attackers can intercept SSH connections and impersonate legitimate servers
- Credentials (passwords, keys) can be stolen during authentication
- Commands and diagnostic data can be intercepted or modified
- No warning is shown to users about this insecure configuration

**Recommendation**:
1. **Change default to `StrictHostKeyChecking: true`**
2. **Require explicit opt-out** with a warning message
3. **Implement trust-on-first-use (TOFU)** to automatically add new hosts to known_hosts
4. **Log warnings** when insecure mode is used

```go
// Recommended fix
func NewClientConfig(baseConfig config.SSHConfig) *ClientConfig {
    return &ClientConfig{
        // ...
        StrictHostKeyChecking: true, // SECURE DEFAULT
        KnownHostsPath:        getDefaultKnownHostsPath(),
        // ...
    }
}

// Add warning when disabled
func (c *ClientConfig) EnableStrictHostKeyChecking(enable bool) {
    if !enable {
        log.Warn("⚠️  WARNING: SSH host key verification is DISABLED. Connections are vulnerable to MITM attacks!")
    }
    c.StrictHostKeyChecking = enable
}
```

**References**:
- [CWE-295: Improper Certificate Validation](https://cwe.mitre.org/data/definitions/295.html)
- [OWASP: Insufficient Transport Layer Protection](https://owasp.org/www-community/vulnerabilities/Insufficient_Transport_Layer_Protection)

---

### 2. Command Injection via WorkingDir Parameter

**Location**: `internal/ssh/session.go:137-139`, `internal/ssh/session.go:244-246`  
**Category**: Command Injection  
**Description**: The `WorkingDir` parameter is directly concatenated into shell commands without any sanitization or validation, enabling command injection attacks.

**Evidence**:
```go
// internal/ssh/session.go:137-139
if options.WorkingDir != "" {
    command = fmt.Sprintf("cd %s && %s", options.WorkingDir, command)
}
```

**Impact**: 
- Attackers can inject arbitrary commands by providing malicious WorkingDir values
- Example: `WorkingDir: "/tmp; rm -rf / #"` would execute `cd /tmp; rm -rf / # && <original_command>`
- Complete system compromise on remote servers
- Data exfiltration, privilege escalation, or denial of service

**Recommendation**:
1. **Validate and sanitize** the WorkingDir parameter
2. **Use absolute path validation** to ensure it's a valid directory path
3. **Shell-escape** the parameter before concatenation
4. **Consider using SSH's native working directory** if available

```go
// Recommended fix
func sanitizeWorkingDir(dir string) (string, error) {
    // Reject paths with shell metacharacters
    if strings.ContainsAny(dir, ";|&$`<>(){}[]!*?~") {
        return "", fmt.Errorf("invalid characters in working directory path")
    }
    
    // Ensure it's an absolute path
    if !filepath.IsAbs(dir) {
        return "", fmt.Errorf("working directory must be an absolute path")
    }
    
    // Clean the path
    return filepath.Clean(dir), nil
}

// In executeCommand:
if options.WorkingDir != "" {
    cleanDir, err := sanitizeWorkingDir(options.WorkingDir)
    if err != nil {
        return fmt.Errorf("invalid working directory: %w", err)
    }
    // Use shell quoting
    command = fmt.Sprintf("cd %s && %s", shellQuote(cleanDir), command)
}
```

**References**:
- [CWE-78: OS Command Injection](https://cwe.mitre.org/data/definitions/78.html)
- [OWASP: Command Injection](https://owasp.org/www-community/attacks/Command_Injection)

---

### 3. Password Exposure via Command-Line Arguments

**Location**: `cmd/lumo/diagnose.go:60`, `cmd/lumo/connect.go:47`  
**Category**: Credential Exposure  
**Description**: The application accepts passwords via command-line flags (`--password` / `-P`), which are visible in process listings, shell history, and logs.

**Evidence**:
```go
// cmd/lumo/diagnose.go:60
diagnoseCmd.Flags().StringP("password", "P", "", "SSH password (not recommended, use key-based auth)")

// cmd/lumo/diagnose.go:144-146
if password != "" {
    log.Warn("Using password from command line is not secure!")
    sshClientConfig.SetPassword(password)
}
```

**Impact**:
- Passwords visible in `ps aux` output to all users on the system
- Passwords stored in shell history files (`.bash_history`, `.zsh_history`)
- Passwords may appear in system logs, monitoring tools, or audit trails
- Shared systems expose passwords to other users

**Recommendation**:
1. **Remove password flag entirely** or make it read from stdin/file only
2. **Implement secure password prompting** (already exists in `auth.go`)
3. **Add explicit warnings** in documentation
4. **Consider removing password auth** in favor of key-based authentication only

```go
// Recommended approach - remove the flag and always prompt
// Remove this line:
// diagnoseCmd.Flags().StringP("password", "P", "", "...")

// In runDiagnostics, if password auth is needed:
if needsPasswordAuth {
    password, err := promptForPassword(username, hostname)
    if err != nil {
        return fmt.Errorf("password prompt failed: %w", err)
    }
    sshClientConfig.SetPassword(password)
}
```

**Alternative**: If the flag must exist for automation:
```go
diagnoseCmd.Flags().String("password-file", "", "Path to file containing password (file must have 0600 permissions)")

// Then read from file with permission check
func readPasswordFromFile(path string) (string, error) {
    info, err := os.Stat(path)
    if err != nil {
        return "", err
    }
    
    // Require strict permissions
    if info.Mode().Perm() != 0600 {
        return "", fmt.Errorf("password file must have 0600 permissions")
    }
    
    data, err := os.ReadFile(path)
    if err != nil {
        return "", err
    }
    
    return strings.TrimSpace(string(data)), nil
}
```

**References**:
- [CWE-214: Invocation of Process Using Visible Sensitive Information](https://cwe.mitre.org/data/definitions/214.html)
- [OWASP: Sensitive Data Exposure](https://owasp.org/Top10/A02_2021-Cryptographic_Failures/)

---

## 🟠 HIGH Severity Findings

### 4. No TLS Certificate Validation for AI Provider APIs

**Location**: `internal/ai/anthropic.go:63-65`, `internal/ai/openai.go:60-63`, `internal/ai/gemini.go:60-63`, `internal/ai/ollama.go:60-63`  
**Category**: Insufficient Transport Layer Protection  
**Description**: All AI provider HTTP clients are created with default settings, which may not enforce proper TLS certificate validation, especially for custom endpoints.

**Evidence**:
```go
// internal/ai/anthropic.go:63-65
client: &http.Client{
    Timeout: config.Timeout,
},
```

**Impact**:
- Man-in-the-middle attacks on AI API communications
- Diagnostic data (potentially containing sensitive system information) can be intercepted
- API keys can be stolen during transmission
- Malicious responses can be injected

**Recommendation**:
```go
import (
    "crypto/tls"
    "net/http"
)

// Create secure HTTP client
func newSecureHTTPClient(timeout time.Duration, skipTLSVerify bool) *http.Client {
    tlsConfig := &tls.Config{
        MinVersion: tls.VersionTLS12,
        // Only allow skipping verification if explicitly configured
        InsecureSkipVerify: skipTLSVerify,
    }
    
    if skipTLSVerify {
        log.Warn("⚠️  TLS certificate verification is DISABLED for AI provider")
    }
    
    transport := &http.Transport{
        TLSClientConfig: tlsConfig,
        // Add timeouts to prevent resource exhaustion
        IdleConnTimeout:       90 * time.Second,
        TLSHandshakeTimeout:   10 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,
    }
    
    return &http.Client{
        Timeout:   timeout,
        Transport: transport,
    }
}

// In provider constructors:
return &AnthropicProvider{
    config: config,
    client: newSecureHTTPClient(config.Timeout, config.SkipTLSVerify),
    log:    log,
}, nil
```

**References**:
- [CWE-295: Improper Certificate Validation](https://cwe.mitre.org/data/definitions/295.html)
- [OWASP: Insufficient Transport Layer Protection](https://owasp.org/www-community/vulnerabilities/Insufficient_Transport_Layer_Protection)

---

### 5. Potential Credential Logging in Debug Mode

**Location**: `cmd/lumo/diagnose.go:144`, `cmd/lumo/connect.go:105`, `internal/ssh/auth.go` (various)  
**Category**: Information Disclosure  
**Description**: While the code shows warnings about password usage, there's no explicit sanitization of credentials from debug logs. The verbose logging mode could potentially expose sensitive data.

**Evidence**:
```go
// cmd/lumo/diagnose.go:144
if password != "" {
    log.Warn("Using password from command line is not secure!")
    sshClientConfig.SetPassword(password)  // Password stored in memory
}
```

**Impact**:
- Credentials may appear in debug logs
- Log files could be accessed by unauthorized users
- Centralized logging systems may store credentials
- Debugging sessions may expose sensitive data

**Recommendation**:
1. **Implement credential sanitization** in logging
2. **Never log passwords, API keys, or tokens**
3. **Redact sensitive fields** in structured logging

```go
// Add a sanitizer for log fields
type SanitizedConfig struct {
    *ClientConfig
}

func (c SanitizedConfig) MarshalJSON() ([]byte, error) {
    type Alias ClientConfig
    return json.Marshal(&struct {
        Password   string `json:"password"`
        Passphrase string `json:"passphrase"`
        *Alias
    }{
        Password:   "[REDACTED]",
        Passphrase: "[REDACTED]",
        Alias:      (*Alias)(c.ClientConfig),
    })
}

// Use in logging:
log.WithFields(logrus.Fields{
    "config": SanitizedConfig{sshClientConfig},
}).Debug("SSH configuration")
```

**References**:
- [CWE-532: Insertion of Sensitive Information into Log File](https://cwe.mitre.org/data/definitions/532.html)
- [OWASP: Sensitive Data Exposure](https://owasp.org/Top10/A02_2021-Cryptographic_Failures/)

---

### 6. Server-Side Request Forgery (SSRF) via Network Targets

**Location**: `internal/diagnostics/checkers/network.go:436-502`, `internal/config/config.go:240-264`  
**Category**: Server-Side Request Forgery (SSRF)  
**Description**: The network checker accepts arbitrary host/port targets from configuration without validation, allowing potential SSRF attacks when running diagnostics on remote systems.

**Evidence**:
```go
// internal/diagnostics/checkers/network.go:461-477
func (n *NetworkChecker) testTCPTarget(ctx context.Context, executor diagnostics.CommandExecutor, target config.NetworkTarget) TargetResult {
    // ...
    cmd := fmt.Sprintf(
        "nc -w 5 -zv %s %d 2>&1 || bash -c 'cat < /dev/null > /dev/tcp/%s/%d' 2>&1 || perl -MIO::Socket -e 'IO::Socket::INET->new(\"%s:%d\") or exit 1' 2>&1",
        target.Host, target.Port, target.Host, target.Port, target.Host, target.Port,
    )
    // No validation of target.Host or target.Port
}
```

**Impact**:
- Attackers can scan internal networks from compromised systems
- Port scanning of internal services (databases, admin panels, etc.)
- Potential access to cloud metadata services (169.254.169.254)
- Information disclosure about internal network topology

**Recommendation**:
```go
// Add validation for network targets
func validateNetworkTarget(target config.NetworkTarget) error {
    // Validate hostname
    if target.Host == "" {
        return fmt.Errorf("host cannot be empty")
    }
    
    // Block private IP ranges and localhost (unless explicitly allowed)
    ip := net.ParseIP(target.Host)
    if ip != nil {
        if ip.IsLoopback() {
            return fmt.Errorf("localhost targets not allowed")
        }
        if ip.IsPrivate() {
            return fmt.Errorf("private IP ranges not allowed")
        }
        // Block cloud metadata services
        if ip.String() == "169.254.169.254" {
            return fmt.Errorf("cloud metadata service access not allowed")
        }
    }
    
    // Validate port range
    if target.Port < 0 || target.Port > 65535 {
        return fmt.Errorf("invalid port: %d", target.Port)
    }
    
    // Block privileged ports unless explicitly allowed
    if target.Port > 0 && target.Port < 1024 {
        return fmt.Errorf("privileged ports (< 1024) not allowed")
    }
    
    return nil
}

// In config validation:
for i, target := range c.Diagnostics.Network.Targets {
    if err := validateNetworkTarget(target); err != nil {
        return fmt.Errorf("diagnostics.network.targets[%d]: %w", i, err)
    }
}
```

**References**:
- [CWE-918: Server-Side Request Forgery (SSRF)](https://cwe.mitre.org/data/definitions/918.html)
- [OWASP: Server-Side Request Forgery](https://owasp.org/www-community/attacks/Server_Side_Request_Forgery)

---

### 7. Missing Input Validation on SSH Key Paths

**Location**: `internal/ssh/config.go:125-143`  
**Category**: Path Traversal  
**Description**: The `SetKeyPath` function expands `~` for home directory but doesn't validate against path traversal attacks or symlink attacks.

**Evidence**:
```go
// internal/ssh/config.go:125-143
func (c *ClientConfig) SetKeyPath(path string) error {
    // Expand home directory if needed
    if path != "" && path[0] == '~' {
        home, err := os.UserHomeDir()
        if err != nil {
            return fmt.Errorf("failed to get home directory: %w", err)
        }
        path = filepath.Join(home, path[1:])
    }
    
    // Check if file exists
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return fmt.Errorf("SSH key file not found: %s", path)
    }
    
    c.KeyPath = path
    return nil
}
```

**Impact**:
- Path traversal to read arbitrary files (e.g., `~/../../../etc/passwd`)
- Symlink attacks to access unintended files
- Potential information disclosure

**Recommendation**:
```go
func (c *ClientConfig) SetKeyPath(path string) error {
    // Expand home directory if needed
    if path != "" && path[0] == '~' {
        home, err := os.UserHomeDir()
        if err != nil {
            return fmt.Errorf("failed to get home directory: %w", err)
        }
        path = filepath.Join(home, path[1:])
    }
    
    // Clean and validate the path
    path = filepath.Clean(path)
    
    // Ensure the path is absolute
    if !filepath.IsAbs(path) {
        return fmt.Errorf("key path must be absolute")
    }
    
    // Check if file exists and is a regular file (not symlink)
    info, err := os.Lstat(path) // Use Lstat to detect symlinks
    if err != nil {
        return fmt.Errorf("SSH key file not found: %s", path)
    }
    
    // Reject symlinks for security
    if info.Mode()&os.ModeSymlink != 0 {
        return fmt.Errorf("SSH key file cannot be a symlink: %s", path)
    }
    
    // Ensure it's a regular file
    if !info.Mode().IsRegular() {
        return fmt.Errorf("SSH key path must be a regular file: %s", path)
    }
    
    c.KeyPath = path
    return nil
}
```

**References**:
- [CWE-22: Path Traversal](https://cwe.mitre.org/data/definitions/22.html)
- [CWE-59: Improper Link Resolution Before File Access](https://cwe.mitre.org/data/definitions/59.html)

---

### 8. Insufficient Error Message Sanitization

**Location**: `internal/ai/anthropic.go:239-245`, `internal/ai/openai.go:239-245`  
**Category**: Information Disclosure  
**Description**: API error responses are returned directly to users without sanitization, potentially exposing sensitive information like API keys, internal endpoints, or system details.

**Evidence**:
```go
// internal/ai/anthropic.go:239-245
if resp.StatusCode != http.StatusOK {
    body, _ := io.ReadAll(resp.Body)
    return "", nil, &Error{
        Op:        "api_call",
        Provider:  p.Name(),
        Err:       fmt.Errorf("API error %d: %s", resp.StatusCode, string(body)),
        Retryable: resp.StatusCode >= 500,
    }
}
```

**Impact**:
- API keys or tokens may be exposed in error messages
- Internal system details revealed
- Debugging information aids attackers

**Recommendation**:
```go
func sanitizeAPIError(body []byte, statusCode int) string {
    // Parse error response if JSON
    var errResp map[string]interface{}
    if err := json.Unmarshal(body, &errResp); err == nil {
        // Extract only safe fields
        if msg, ok := errResp["error"].(map[string]interface{}); ok {
            if message, ok := msg["message"].(string); ok {
                return message
            }
        }
    }
    
    // Generic error for unparseable responses
    return fmt.Sprintf("API request failed with status %d", statusCode)
}

// Use in error handling:
if resp.StatusCode != http.StatusOK {
    body, _ := io.ReadAll(resp.Body)
    return "", nil, &Error{
        Op:        "api_call",
        Provider:  p.Name(),
        Err:       fmt.Errorf("API error: %s", sanitizeAPIError(body, resp.StatusCode)),
        Retryable: resp.StatusCode >= 500,
    }
}
```

**References**:
- [CWE-209: Generation of Error Message Containing Sensitive Information](https://cwe.mitre.org/data/definitions/209.html)

---

### 9. No Rate Limiting on AI API Calls

**Location**: `internal/ai/` (all providers)  
**Category**: Denial of Service / Cost Exhaustion  
**Description**: There's no rate limiting or cost controls on AI API calls, which could lead to excessive costs or service abuse.

**Evidence**: No rate limiting implementation found in any AI provider.

**Impact**:
- Runaway costs from excessive API usage
- Potential account suspension by AI providers
- Denial of service through resource exhaustion

**Recommendation**:
```go
import "golang.org/x/time/rate"

type RateLimitedProvider struct {
    provider Provider
    limiter  *rate.Limiter
}

func NewRateLimitedProvider(provider Provider, requestsPerMinute int) *RateLimitedProvider {
    return &RateLimitedProvider{
        provider: provider,
        limiter:  rate.NewLimiter(rate.Limit(requestsPerMinute)/60, 1),
    }
}

func (p *RateLimitedProvider) Analyze(ctx context.Context, req *AnalysisRequest) (*AnalysisResponse, error) {
    if err := p.limiter.Wait(ctx); err != nil {
        return nil, fmt.Errorf("rate limit: %w", err)
    }
    return p.provider.Analyze(ctx, req)
}
```

**References**:
- [CWE-770: Allocation of Resources Without Limits or Throttling](https://cwe.mitre.org/data/definitions/770.html)

---

## 🟡 MEDIUM Severity Findings

### 10. Weak File Permission Validation

**Location**: `internal/ssh/auth.go:289-292`, `internal/ssh/config.go:95-98`  
**Category**: Insecure File Permissions  
**Description**: File permission checks use bitwise operations that may not catch all insecure permission scenarios.

**Evidence**:
```go
// internal/ssh/auth.go:289-292
perm := info.Mode().Perm()
if perm&0077 != 0 {
    return fmt.Errorf("key file %s has insecure permissions %o (should be 600 or 400)", keyPath, perm)
}
```

**Impact**:
- SSH keys with overly permissive permissions may be accepted
- Group/world readable keys are a security risk

**Recommendation**:
```go
func validateKeyFilePermissions(path string) error {
    info, err := os.Stat(path)
    if err != nil {
        return err
    }
    
    perm := info.Mode().Perm()
    
    // Only accept 0600 or 0400
    if perm != 0600 && perm != 0400 {
        return fmt.Errorf("key file %s has insecure permissions %o (must be exactly 0600 or 0400)", path, perm)
    }
    
    return nil
}
```

**References**:
- [CWE-732: Incorrect Permission Assignment for Critical Resource](https://cwe.mitre.org/data/definitions/732.html)

---

### 11. Missing Timeout on Local Command Execution

**Location**: `internal/diagnostics/executor.go:65-74`  
**Category**: Denial of Service  
**Description**: While SSH execution has timeouts, local execution could hang indefinitely if a command doesn't respect context cancellation.

**Evidence**:
```go
// internal/diagnostics/executor.go:65-74
func (e *LocalExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
    if timeout == 0 {
        timeout = 30 * time.Second
    }
    
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    return e.ExecuteWithContext(ctx, command)
}
```

**Impact**:
- Hung processes consuming resources
- Diagnostic runs that never complete
- Resource exhaustion

**Recommendation**: The current implementation is actually correct, but add process cleanup:

```go
func (e *LocalExecutor) ExecuteWithContext(ctx context.Context, command string) (stdout, stderr string, exitCode int, err error) {
    cmd := exec.CommandContext(ctx, "sh", "-c", command)
    
    var stdoutBuf, stderrBuf bytes.Buffer
    cmd.Stdout = &stdoutBuf
    cmd.Stderr = &stderrBuf
    
    // Ensure process is killed on context cancellation
    cmd.Cancel = func() error {
        return cmd.Process.Kill()
    }
    
    err = cmd.Run()
    // ... rest of implementation
}
```

**References**:
- [CWE-400: Uncontrolled Resource Consumption](https://cwe.mitre.org/data/definitions/400.html)

---

### 12. No Validation of AI Provider Endpoints

**Location**: `internal/ai/anthropic.go:58-59`, `internal/ai/openai.go:56-57`  
**Category**: SSRF / Configuration Injection  
**Description**: Custom AI provider endpoints are accepted without validation, potentially allowing SSRF or connections to malicious servers.

**Evidence**:
```go
// internal/ai/anthropic.go:58-59
if config.Endpoint == "" {
    config.Endpoint = AnthropicAPIURL
}
```

**Impact**:
- SSRF attacks via custom endpoints
- Data exfiltration to attacker-controlled servers
- Credential theft

**Recommendation**:
```go
func validateEndpoint(endpoint string) error {
    if endpoint == "" {
        return nil // Will use default
    }
    
    u, err := url.Parse(endpoint)
    if err != nil {
        return fmt.Errorf("invalid endpoint URL: %w", err)
    }
    
    // Require HTTPS
    if u.Scheme != "https" {
        return fmt.Errorf("endpoint must use HTTPS")
    }
    
    // Validate hostname (no localhost, private IPs)
    host := u.Hostname()
    if host == "localhost" || host == "127.0.0.1" || host == "::1" {
        return fmt.Errorf("localhost endpoints not allowed")
    }
    
    // Check for private IP ranges
    if ip := net.ParseIP(host); ip != nil && ip.IsPrivate() {
        return fmt.Errorf("private IP endpoints not allowed")
    }
    
    return nil
}
```

**References**:
- [CWE-918: Server-Side Request Forgery](https://cwe.mitre.org/data/definitions/918.html)

---

### 13. Diagnostic Data May Contain Sensitive Information

**Location**: `internal/diagnostics/checkers/` (all checkers)  
**Category**: Information Disclosure  
**Description**: Diagnostic data sent to AI providers may contain sensitive system information (usernames, process names, network topology) without sanitization.

**Evidence**: All diagnostic checkers collect detailed system information that is sent to AI providers.

**Impact**:
- Exposure of internal system details to third parties
- Compliance violations (GDPR, HIPAA, etc.)
- Information useful for attackers

**Recommendation**:
1. **Add data sanitization** before sending to AI
2. **Implement opt-in for sensitive data**
3. **Document what data is sent** to AI providers
4. **Allow users to review** data before transmission

```go
type DataSanitizer struct {
    sanitizeUsernames bool
    sanitizeIPs       bool
    sanitizePaths     bool
}

func (s *DataSanitizer) SanitizeReport(report *diagnostics.Report) *diagnostics.Report {
    sanitized := *report
    
    for i, result := range sanitized.Results {
        if s.sanitizeUsernames {
            // Replace usernames with generic identifiers
            sanitized.Results[i] = s.sanitizeUsernames(result)
        }
        // ... other sanitization
    }
    
    return &sanitized
}
```

**References**:
- [CWE-200: Exposure of Sensitive Information to an Unauthorized Actor](https://cwe.mitre.org/data/definitions/200.html)

---

### 14. Shell Quoting Function May Be Insufficient

**Location**: `internal/ssh/session.go:380-384`  
**Category**: Command Injection  
**Description**: The `shellQuote` function uses a simple single-quote escaping mechanism that may not handle all edge cases.

**Evidence**:
```go
// internal/ssh/session.go:380-384
func shellQuote(s string) string {
    // Simple shell quoting - wrap in single quotes and escape existing single quotes
    s = strings.ReplaceAll(s, "'", "'\"'\"'")
    return "'" + s + "'"
}
```

**Impact**:
- Potential command injection in edge cases
- Incorrect handling of special characters

**Recommendation**: Use a well-tested shell escaping library or implement comprehensive escaping:

```go
import "github.com/kballard/go-shellquote"

// Or implement comprehensive escaping:
func shellQuote(s string) string {
    // For POSIX shells, single quotes are safest
    // Replace ' with '\''
    s = strings.ReplaceAll(s, "'", `'\''`)
    return "'" + s + "'"
}

// Better: use shellquote.Join for multiple arguments
func buildCommand(args ...string) string {
    return shellquote.Join(args...)
}
```

**References**:
- [CWE-78: OS Command Injection](https://cwe.mitre.org/data/definitions/78.html)

---

## 🟢 LOW Severity Findings

### 15. Verbose Error Messages in Production

**Location**: Various error handling throughout codebase  
**Category**: Information Disclosure  
**Description**: Detailed error messages may expose internal implementation details.

**Recommendation**: Implement error levels (user-facing vs. internal) and log detailed errors while showing generic messages to users.

---

### 16. No Security Headers for Future API Server

**Location**: `internal/config/config.go:54-64` (API config)  
**Category**: Missing Security Controls  
**Description**: The planned API server (Phase 7) configuration doesn't include security headers.

**Recommendation**: Add configuration for security headers:
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 1; mode=block`
- `Strict-Transport-Security: max-age=31536000`
- `Content-Security-Policy`

---

### 17. Missing Audit Logging

**Location**: Throughout codebase  
**Category**: Insufficient Logging & Monitoring  
**Description**: No audit trail for security-relevant events (authentication attempts, command execution, configuration changes).

**Recommendation**: Implement structured audit logging:

```go
type AuditLogger struct {
    log *logrus.Logger
}

func (a *AuditLogger) LogAuthAttempt(user, host string, method AuthMethod, success bool) {
    a.log.WithFields(logrus.Fields{
        "event":   "auth_attempt",
        "user":    user,
        "host":    host,
        "method":  method,
        "success": success,
    }).Info("Authentication attempt")
}

func (a *AuditLogger) LogCommandExecution(user, host, command string) {
    a.log.WithFields(logrus.Fields{
        "event":   "command_execution",
        "user":    user,
        "host":    host,
        "command": command,
    }).Info("Command executed")
}
```

**References**:
- [CWE-778: Insufficient Logging](https://cwe.mitre.org/data/definitions/778.html)

---

### 18. Dependency Vulnerabilities (Potential)

**Location**: `go.mod`, `go.sum`  
**Category**: Vulnerable Dependencies  
**Description**: No automated dependency scanning is evident.

**Recommendation**:
1. **Run `go mod tidy` regularly**
2. **Use `govulncheck`** to scan for known vulnerabilities
3. **Implement automated dependency updates** (Dependabot, Renovate)
4. **Pin dependency versions** in production

```bash
# Install govulncheck
go install golang.org/x/vuln/cmd/govulncheck@latest

# Run vulnerability scan
govulncheck ./...
```

**References**:
- [CWE-1035: Using Components with Known Vulnerabilities](https://cwe.mitre.org/data/definitions/1035.html)

---

## ℹ️ Security Best Practices Recommendations

1. **Implement Security Testing**:
   - Add security-focused unit tests
   - Implement fuzzing for input validation
   - Add integration tests for authentication flows

2. **Code Review Process**:
   - Require security review for SSH and authentication code
   - Use static analysis tools (gosec, staticcheck)
   - Implement pre-commit hooks for security checks

3. **Documentation**:
   - Document security assumptions
   - Provide security hardening guide
   - Create incident response procedures

4. **Phase 5 (Auto-Remediation) Security**:
   - **CRITICAL**: Implement command whitelisting
   - Require explicit user approval for destructive operations
   - Add rollback capabilities
   - Implement dry-run mode by default
   - Log all remediation actions
   - Add rate limiting to prevent abuse

---

## Positive Security Observations

✅ **API keys via environment variables** - Good practice, not in config files  
✅ **Comprehensive .gitignore** - Prevents accidental credential commits  
✅ **SSH key permission validation** - Checks for 0600/0400 permissions  
✅ **Context-based timeouts** - Prevents resource exhaustion  
✅ **Structured logging** - Facilitates security monitoring  
✅ **Error wrapping** - Maintains error context for debugging  
✅ **Separate local/SSH executors** - Good separation of concerns  

---

## Dependency Security Summary

**Total Dependencies**: 14 direct, 16 indirect  
**Outdated**: Requires manual check with `go list -u -m all`  
**Potentially Vulnerable**: Run `govulncheck ./...` to verify

**Key Dependencies**:
- `golang.org/x/crypto v0.44.0` - SSH implementation (latest)
- `github.com/spf13/cobra v1.10.1` - CLI framework
- `github.com/spf13/viper v1.21.0` - Configuration
- `github.com/sirupsen/logrus v1.9.3` - Logging

**Recommendation**: All dependencies appear recent, but run automated scanning.

---

## Security Checklist

- [ ] No hardcoded secrets ⚠️ (Passwords in memory from CLI flags)
- [ ] Input validation on all user inputs ❌ (WorkingDir, network targets)
- [ ] SQL injection protection ✅ (N/A - no database)
- [ ] XSS prevention ✅ (N/A - no web UI yet)
- [ ] CSRF protection ⏳ (Planned for Phase 7)
- [ ] Strong cryptography ✅ (Uses golang.org/x/crypto)
- [ ] HTTPS enforced ⚠️ (AI providers, but no cert validation)
- [ ] Security headers configured ⏳ (Planned for Phase 7)
- [ ] Authentication on sensitive endpoints ✅ (SSH-based)
- [ ] Authorization checks implemented ⚠️ (Minimal)
- [ ] Rate limiting in place ❌ (Missing for AI calls)
- [ ] Secure session management ✅ (SSH sessions)
- [ ] Dependencies up to date ✅ (Appears current)
- [ ] Logging without sensitive data ⚠️ (Needs improvement)
- [ ] Error messages don't expose internals ❌ (Too verbose)

---

## Remediation Priority

### Immediate Action Required (Within 24 hours)

1. **Enable SSH host key verification by default** - Change `StrictHostKeyChecking: true`
2. **Remove password command-line flag** - Force secure password prompting
3. **Add input validation for WorkingDir** - Prevent command injection

### Short Term (Within 1 week)

1. **Implement TLS certificate validation** for AI providers
2. **Add SSRF protection** for network targets
3. **Sanitize error messages** from AI APIs
4. **Add credential sanitization** in logging
5. **Implement rate limiting** for AI calls

### Medium Term (Within 1 month)

1. **Add comprehensive audit logging**
2. **Implement data sanitization** before AI transmission
3. **Add security headers** configuration for API server
4. **Improve shell quoting** implementation
5. **Add automated security scanning** (govulncheck, gosec)

### Long Term (Ongoing)

1. **Security documentation** and hardening guide
2. **Penetration testing** before Phase 5 deployment
3. **Compliance review** (GDPR, SOC 2, etc.)
4. **Security training** for contributors
5. **Bug bounty program** consideration

---

## Conclusion

**Overall Security Posture**: **NEEDS IMPROVEMENT**

Lumo demonstrates good architectural decisions and follows some security best practices, but **critical vulnerabilities exist** that must be addressed before production use. The disabled SSH host key verification and command injection vulnerabilities are particularly concerning given the tool's purpose of executing commands on remote systems.

**Next Steps**:
1. **Address all CRITICAL findings immediately**
2. **Implement security testing** in CI/CD pipeline
3. **Conduct security review** before Phase 5 (auto-remediation)
4. **Consider professional penetration testing** before production deployment

The planned auto-remediation feature (Phase 5) significantly increases the security risk profile. **Do not proceed to Phase 5** until all CRITICAL and HIGH severity findings are resolved and comprehensive security controls are in place.

---

**End of Security Audit Report**
