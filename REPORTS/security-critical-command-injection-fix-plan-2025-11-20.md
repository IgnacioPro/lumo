# 🚨 CRITICAL Security Fix Plan - Command Injection Vulnerabilities

**Date:** 2025-11-20
**Severity:** CRITICAL (CVSS 9.0-9.8)
**Status:** IN PROGRESS
**Auditor:** Claude (Anthropic AI Assistant)

---

## Executive Summary

A comprehensive security audit of Lumo has identified **5 CRITICAL command injection vulnerabilities** and **8 HIGH/MEDIUM severity security issues** that could allow unauthorized remote code execution, data exfiltration, and complete system compromise.

**IMMEDIATE ACTION REQUIRED** - Do not deploy to production until all CRITICAL vulnerabilities are patched.

---

## Critical Findings

### 🔴 CRITICAL-1: Service Restart Action - Command Injection
**File:** `internal/remediation/actions_service.go`
**Lines:** 60, 80, 147
**CVSS Score:** 9.8 (Critical)

**Vulnerability:**
```go
// VULNERABLE CODE
cmd := fmt.Sprintf("systemctl status %s", a.serviceName)
cmd := fmt.Sprintf("systemctl restart %s", a.serviceName)
cmd := fmt.Sprintf("grep -E '^Restart=' /etc/systemd/system/%s.service", a.serviceName)
```

**Attack Vector:**
```bash
# Attacker provides service name: "nginx; rm -rf / #"
# Resulting command: systemctl status nginx; rm -rf / #
```

**Impact:**
- Remote code execution as root (if lumo runs privileged)
- Complete system compromise
- Data destruction

---

### 🔴 CRITICAL-2: Service Start Action - Command Injection
**File:** `internal/remediation/actions_service.go`
**Lines:** 199, 215, 243
**CVSS Score:** 9.8 (Critical)

**Vulnerability:**
```go
// VULNERABLE CODE in Validate()
cmd := fmt.Sprintf("systemctl is-enabled %s 2>&1 | grep -q 'enabled'", a.serviceName)

// VULNERABLE CODE in Execute()
cmd := fmt.Sprintf("systemctl start %s", a.serviceName)

// VULNERABLE CODE in Rollback()
cmd := fmt.Sprintf("systemctl stop %s", a.serviceName)
```

**Attack Vector:** Same as CRITICAL-1

---

### 🔴 CRITICAL-3: Service Stop Action - Command Injection
**File:** `internal/remediation/actions_service.go`
**Lines:** 307, 335
**CVSS Score:** 9.8 (Critical)

**Vulnerability:**
```go
// VULNERABLE CODE
cmd := fmt.Sprintf("systemctl stop %s", a.serviceName)
cmd := fmt.Sprintf("systemctl start %s", a.serviceName) // in Rollback
```

**Attack Vector:** Same as CRITICAL-1

---

### 🔴 CRITICAL-4: Kill Process Action - Command Injection
**File:** `internal/remediation/actions_process.go`
**Lines:** 367, 385
**CVSS Score:** 9.0 (Critical)

**Vulnerability:**
```go
// VULNERABLE CODE in Validate()
cmd := fmt.Sprintf("pgrep -f '%s'", a.processPattern)

// VULNERABLE CODE in Execute()
cmd := fmt.Sprintf("pkill -%d -f '%s'", a.signal, a.processPattern)
```

**Attack Vector:**
```bash
# Attacker provides pattern: "nginx' && cat /etc/shadow || pgrep -f '"
# Resulting command: pgrep -f 'nginx' && cat /etc/shadow || pgrep -f ''
```

**Impact:**
- Arbitrary command execution
- Password hash exfiltration
- Privilege escalation

---

### 🔴 CRITICAL-5: Service Checker - Path Traversal + Command Injection
**File:** `internal/diagnostics/checkers/service.go`
**Line:** 241
**CVSS Score:** 9.0 (Critical)

**Vulnerability:**
```go
// VULNERABLE CODE - directory listing names used in commands
cmd := fmt.Sprintf("systemctl status %s", filename)
```

**Attack Vector:**
```bash
# Attacker creates file: /etc/init.d/nginx;rm -rf /
# When service checker runs, it executes: systemctl status nginx;rm -rf /
```

---

## High/Medium Severity Findings

### 🟠 HIGH-1: SSH Target SSRF
**File:** `internal/api/handlers/diagnostics.go`, `remediation.go`
**Lines:** 172, 213
**CVSS Score:** 8.1 (High)

**Issue:** No validation of SSH targets - can be used to:
- Port scan internal networks (169.254.0.0/16, 10.0.0.0/8)
- Access cloud metadata services (169.254.169.254)
- Pivot to internal systems

**Fix:** Block reserved IP ranges, implement target allowlist

---

### 🟠 HIGH-2: API Auto-Approve Bypass
**File:** `internal/api/handlers/remediation.go`
**Lines:** 259-260
**CVSS Score:** 7.5 (High)

**Issue:**
```go
// User can bypass approval controls!
if req.AutoApprove {
    approved = true
}
```

**Impact:** Users can execute destructive actions without human-in-the-loop approval

**Fix:** Remove `auto_approve` from API, enforce server-side approval workflows

---

### 🟠 HIGH-3: Missing Scope-Based Authorization
**File:** `internal/api/middleware/auth.go`
**Lines:** 83-87
**CVSS Score:** 7.3 (High)

**Issue:**
```go
// Only checks if key exists, not what it's allowed to do
if apiKey == "" {
    Unauthorized(w, "Missing API key")
    return
}
```

**Impact:** API keys with "read-only" scope can execute remediation actions

**Fix:** Implement scope checking (read, write, admin, remediation)

---

### 🟡 MEDIUM-1: String Injection in Process Checker
**File:** `internal/diagnostics/checkers/process.go`
**Lines:** 182, 195
**CVSS Score:** 5.3 (Medium)

**Issue:**
```go
cmd := fmt.Sprintf("ps aux | head -n %d", limit)
```

**Impact:** Could cause command failure or DoS if limit is malicious

**Fix:** Validate `limit` is numeric and within bounds (1-10000)

---

### 🟡 MEDIUM-2: No Input Validation on Checks Parameter
**File:** `internal/api/handlers/diagnostics.go`
**Lines:** 187-188
**CVSS Score:** 5.0 (Medium)

**Issue:** User can request arbitrary checker names

**Fix:** Validate against allowlist of known checkers

---

### 🟡 MEDIUM-3: No Rate Limiting
**File:** `internal/api/handlers/remediation.go`
**Impact:** DoS via remediation spam

**Fix:** Add per-IP and per-key rate limiting

---

### 🟡 MEDIUM-4: Password Exposure Risk
**File:** `internal/api/handlers/diagnostics.go`
**Status:** Currently prevented, but needs documentation

**Issue:** SSH password field exists in request struct

**Fix:** Add comment documenting that passwords are never stored

---

### 🟡 MEDIUM-5: Weak Target Validation
**File:** `internal/api/handlers/diagnostics.go`
**Lines:** 173-176

**Issue:** Only checks if target is empty, not if it's valid

**Fix:** Comprehensive validation (hostname format, IP range restrictions)

---

## Positive Security Controls ✅

1. **`shellQuote()` function exists** (`internal/ssh/executor.go:158-166`) - Just needs to be used everywhere
2. **Working directory sanitization** (`sanitizeWorkingDir()`) - Prevents path traversal
3. **Context-based timeouts** - All SSH commands have timeouts
4. **Passwords NOT stored** - API handlers don't persist passwords in database
5. **Consistent error wrapping** - Good for audit logging

---

## Fix Implementation Plan

### Phase 1: CRITICAL Patches (Immediate - 2 hours)

**1.1 Fix Service Actions Command Injection**
- File: `internal/remediation/actions_service.go`
- Action: Add `shellQuote()` to all service name parameters
- Lines: 60, 80, 147, 199, 215, 243, 307, 335

**Changes:**
```go
// BEFORE
cmd := fmt.Sprintf("systemctl restart %s", a.serviceName)

// AFTER
cmd := fmt.Sprintf("systemctl restart %s", shellQuote(a.serviceName))

// OR validate against allowlist
if !isValidServiceName(a.serviceName) {
    return fmt.Errorf("invalid service name: must match ^[a-zA-Z0-9_-]+$")
}
```

**1.2 Fix Process Actions Command Injection**
- File: `internal/remediation/actions_process.go`
- Action: Add `shellQuote()` to process pattern parameters
- Lines: 367, 385

**1.3 Fix Service Checker Command Injection**
- File: `internal/diagnostics/checkers/service.go`
- Action: Validate filenames before using in commands
- Line: 241

**1.4 Create Shared Input Validation**
- File: `internal/remediation/validation.go` (NEW)
- Functions:
  - `isValidServiceName(name string) bool` - Regex: `^[a-zA-Z0-9_.-]+$`
  - `isValidProcessPattern(pattern string) bool` - Escape or reject shell metacharacters
  - `shellQuote(s string) string` - Move from ssh package to shared location

---

### Phase 2: HIGH Severity Fixes (Same Day - 3 hours)

**2.1 Fix SSH Target SSRF**
- File: `internal/ssh/validation.go` (NEW)
- Function: `ValidateSSHTarget(target string) error`
- Block:
  - Private IP ranges: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
  - Link-local: 169.254.0.0/16
  - Loopback: 127.0.0.0/8
  - Metadata services: 169.254.169.254
- Allow: User-configured allowlist

**2.2 Remove Auto-Approve from API**
- File: `internal/api/handlers/remediation.go`
- Remove: `AutoApprove` field from request struct
- Add: Server-side approval workflow configuration
- Lines: 259-260

**2.3 Implement Scope-Based Authorization**
- File: `internal/api/middleware/auth.go`
- Add: Scope validation
- Required scopes:
  - `diagnostics:read` - View diagnostic results
  - `diagnostics:write` - Run diagnostics
  - `remediation:read` - View remediation plans
  - `remediation:write` - Execute remediation (requires approval)
  - `admin` - Full access

**Changes:**
```go
func RequireScope(scope string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            apiKey := r.Context().Value(apiKeyContextKey).(*models.APIKey)
            if !apiKey.HasScope(scope) {
                Forbidden(w, "Insufficient permissions")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

---

### Phase 3: MEDIUM Severity Fixes (1 day)

**3.1 Input Validation**
- File: `internal/diagnostics/checkers/process.go`
- Validate: `limit` parameter (1-10000, numeric only)

**3.2 Checker Allowlist**
- File: `internal/api/handlers/diagnostics.go`
- Add: Validation against known checkers

**3.3 Rate Limiting**
- File: `internal/api/middleware/ratelimit.go` (NEW)
- Library: `golang.org/x/time/rate`
- Limits:
  - 10 diagnostics/minute per API key
  - 5 remediations/minute per API key
  - 100 requests/minute per IP

**3.4 Documentation**
- Add security comments to password handling code

---

### Phase 4: Security Testing (1 day)

**4.1 Command Injection Tests**
- File: `internal/remediation/actions_security_test.go` (NEW)
- Test cases:
  - Service names with shell metacharacters: `; rm -rf /`, `$(whoami)`, `` `cat /etc/passwd` ``
  - Process patterns with quote escapes
  - Verify `shellQuote()` prevents injection

**4.2 SSRF Tests**
- File: `internal/ssh/validation_test.go` (NEW)
- Test blocked IP ranges
- Test metadata service blocking

**4.3 Authorization Tests**
- File: `internal/api/middleware/auth_test.go`
- Test scope enforcement
- Test permission denied scenarios

**4.4 Fuzzing**
- File: `internal/remediation/fuzz_test.go` (NEW)
- Fuzz all action parameters
- Fuzz SSH targets

---

## Testing Strategy

### Manual Testing

**Test 1: Service Restart Injection**
```bash
# Before fix - DANGEROUS! DO NOT RUN IN PROD
curl -X POST http://localhost:8080/api/v1/remediation \
  -H "X-API-Key: test-key" \
  -d '{
    "target": "localhost",
    "actions": [{
      "type": "restart_service",
      "parameters": {"service_name": "nginx; cat /etc/passwd #"}
    }]
  }'

# After fix - Should return validation error
# Expected: "invalid service name: must match ^[a-zA-Z0-9_.-]+$"
```

**Test 2: SSRF to Metadata Service**
```bash
# Before fix - Should succeed (BAD!)
curl -X POST http://localhost:8080/api/v1/diagnostics \
  -H "X-API-Key: test-key" \
  -d '{"target": "169.254.169.254", "checks": ["cpu"]}'

# After fix - Should return error
# Expected: "invalid target: blocked IP range"
```

**Test 3: Auto-Approve Bypass**
```bash
# Before fix - Executes without approval (BAD!)
curl -X POST http://localhost:8080/api/v1/remediation \
  -H "X-API-Key: test-key" \
  -d '{
    "target": "localhost",
    "auto_approve": true,
    "actions": [{"type": "restart_service", "parameters": {"service_name": "nginx"}}]
  }'

# After fix - auto_approve field ignored or rejected
# Expected: Requires approval workflow
```

### Automated Testing

```bash
# Run all security tests
go test -v ./internal/remediation/... -run Security
go test -v ./internal/ssh/... -run Validation
go test -v ./internal/api/middleware/... -run Auth

# Run fuzz tests (1 minute each)
go test -fuzz=FuzzServiceName -fuzztime=1m ./internal/remediation/
go test -fuzz=FuzzSSHTarget -fuzztime=1m ./internal/ssh/
```

---

## Rollout Plan

### Immediate Actions (Next 2 Hours)
1. ✅ Create this security fix plan document
2. ⏳ Implement CRITICAL patches (Phase 1)
3. ⏳ Write security tests for CRITICAL vulnerabilities
4. ⏳ Run `make ci` to ensure no regressions

### Same Day Actions (Next 8 Hours)
5. ⏳ Implement HIGH severity fixes (Phase 2)
6. ⏳ Update API documentation with security notes
7. ⏳ Test all fixes in development environment

### Next Day Actions
8. ⏳ Implement MEDIUM severity fixes (Phase 3)
9. ⏳ Comprehensive security testing (Phase 4)
10. ⏳ Create SECURITY.md file with vulnerability disclosure policy
11. ⏳ Update CLAUDE.md with security best practices

### Before Production Deployment
12. ⏳ External security audit (recommended)
13. ⏳ Penetration testing
14. ⏳ Update deployment documentation with security hardening steps

---

## Code Changes Summary

### Files to Modify (8 files)
1. `internal/remediation/actions_service.go` - Add shellQuote to 8 locations
2. `internal/remediation/actions_process.go` - Add shellQuote to 2 locations
3. `internal/diagnostics/checkers/service.go` - Add filename validation
4. `internal/api/handlers/remediation.go` - Remove auto_approve, add scope check
5. `internal/api/handlers/diagnostics.go` - Add target validation, scope check
6. `internal/api/middleware/auth.go` - Add scope validation
7. `internal/diagnostics/checkers/process.go` - Add limit validation
8. `internal/config/config.go` - Add target allowlist config

### Files to Create (7 files)
1. `internal/remediation/validation.go` - Shared validation functions
2. `internal/ssh/validation.go` - SSH target validation
3. `internal/api/middleware/ratelimit.go` - Rate limiting middleware
4. `internal/remediation/actions_security_test.go` - Security tests
5. `internal/ssh/validation_test.go` - Target validation tests
6. `internal/remediation/fuzz_test.go` - Fuzzing tests
7. `SECURITY.md` - Security policy and vulnerability disclosure

### Estimated Lines of Code
- Modifications: ~50 lines
- New code: ~800 lines (validation, tests, middleware)
- Total effort: 2-3 days with testing

---

## Success Criteria

### Must Have (Before ANY Production Use)
- [ ] All 5 CRITICAL vulnerabilities patched
- [ ] Security tests pass for command injection
- [ ] `make ci` passes with no regressions
- [ ] Manual testing confirms injections blocked

### Should Have (Before Public Release)
- [ ] All HIGH severity issues fixed
- [ ] Scope-based authorization implemented
- [ ] Rate limiting deployed
- [ ] SSRF protections active

### Nice to Have
- [ ] External security audit completed
- [ ] Penetration testing performed
- [ ] Bug bounty program established
- [ ] Security documentation complete

---

## Risk Assessment

### Current Risk (Before Patches)
- **Likelihood:** HIGH - Vulnerabilities are easy to exploit
- **Impact:** CRITICAL - Complete system compromise possible
- **Overall Risk:** CRITICAL - Do not deploy to production

### Residual Risk (After Patches)
- **Likelihood:** LOW - Input validation + shellQuote prevents known attacks
- **Impact:** LOW - Limited attack surface remains
- **Overall Risk:** LOW - Acceptable for production with monitoring

---

## Monitoring & Detection

### Post-Deployment Monitoring
1. **Alert on unusual SSH targets** - Flag connections to private IPs
2. **Log all remediation actions** - Audit trail for compliance
3. **Monitor for validation failures** - Potential attack attempts
4. **Rate limit violations** - Detect abuse attempts

### Incident Response Plan
1. If command injection detected:
   - Immediately disable affected API endpoints
   - Review audit logs for compromise indicators
   - Rotate all API keys
   - Assess scope of breach

---

## References

- **OWASP Top 10 2021:** A03:2021 - Injection
- **CWE-77:** Command Injection
- **CWE-918:** Server-Side Request Forgery (SSRF)
- **CVSS Calculator:** https://www.first.org/cvss/calculator/3.1

---

## Approval & Sign-Off

**Security Review:** ⏳ Pending implementation
**Code Review:** ⏳ Pending implementation
**Testing Verification:** ⏳ Pending implementation
**Production Deployment:** ⏳ BLOCKED until CRITICAL patches applied

---

**Next Steps:** Proceed with Phase 1 implementation immediately.

