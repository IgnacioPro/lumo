# Changelog

All notable changes to Lumo will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.1] - 2025-11-16

### Security

#### CRITICAL Fixes
- **Fixed SSH Host Key Verification (CWE-295)**: Changed default from `StrictHostKeyChecking: false` to `true` to prevent man-in-the-middle attacks. Added warnings when verification is disabled.
- **Fixed Command Injection via WorkingDir (CWE-78)**: Implemented comprehensive input validation with `sanitizeWorkingDir()` function that blocks shell metacharacters, null bytes, and enforces absolute paths.
- **Fixed Password CLI Exposure (CWE-214)**: Removed `--password` and `-P` flags from `diagnose` and `connect` commands to prevent credential visibility in process listings and shell history. Password authentication now uses secure prompting only.

#### MEDIUM Fixes
- **Enhanced File Permission Validation (CWE-732)**: Changed from bitwise checking to exact permission matching. SSH keys now require exactly 0600 or 0400 permissions.
- **Added AI Endpoint Validation (CWE-918)**: Implemented `ValidateEndpoint()` function to prevent SSRF attacks. Validates HTTPS, blocks localhost/private IPs (except Ollama), and prevents access to cloud metadata services.
- **Improved Shell Quoting**: Enhanced documentation and verified POSIX compliance for shell argument quoting. Added comprehensive examples and edge case handling.

### Changed
- SSH host key verification now enabled by default for security
- SSH key file permissions now strictly enforced (0600 or 0400 only)
- All AI provider endpoints now validated before use
- Password authentication via CLI removed (use secure prompting instead)

### Documentation
- Added comprehensive security fixes report: `REPORTS/security-fixes-2025-11-16.md`
- Updated `CLAUDE.md` with security enhancements section
- Added security audit status to project documentation

### Notes
- This release addresses all CRITICAL findings from the security audit (REPORTS/security-audit-2025-11-15.md)
- Security posture improved from HIGH RISK to LOW-MODERATE RISK
- All existing tests passing with updated security expectations
- Breaking change: `--password` flag removed from CLI commands

## [0.4.0] - 2025-11-15

### Added
- Complete AI integration with 4 providers (Anthropic, OpenAI, Ollama, Gemini)
- Local execution support (no SSH for localhost)
- All 6 core diagnostic checkers (CPU, Memory, Disk, Process, Service, Network)
- Comprehensive test coverage (37.1% overall, 100% in formatters)
- Security-focused memory checker with input validation

### Features
- SSH connection with 4 authentication methods
- Remote and local diagnostic execution
- AI-powered analysis with streaming support
- Cross-platform support (Linux, macOS, BSD)
- Multiple output formats (text, JSON)

### Documentation
- Complete CLAUDE.md guide for AI assistants
- DEVELOPMENT.md with detailed examples
- Security audit report
- Production readiness review

---

**Legend:**
- 🔴 CRITICAL: Security vulnerabilities requiring immediate action
- 🟠 HIGH: Important issues requiring prompt attention
- 🟡 MEDIUM: Issues that should be addressed soon
- 🟢 LOW: Minor issues or improvements
