# Actionable Code Quality Fixes

**Priority:** Address before next release
**Estimated Total Time:** 4-6 hours

---

## 1. FIX: Standardize SSH Logging (MEDIUM PRIORITY)

### File: `internal/ssh/session.go`

#### Current Code (Lines 56, 82, 86, 94):
```go
// ❌ WRONG - Line 56
s.logger.Debugf("Executing command: %s", command)

// ❌ WRONG - Line 82
s.logger.Warnf("Command failed: %s (exit code: %d)", command, result.ExitCode)

// ❌ WRONG - Line 86
s.logger.Debugf("Command succeeded: %s (duration: %v)", command, result.Duration)

// ❌ WRONG - Line 94
s.logger.Warnf("Command timed out: %s (after %v)", command, result.Duration)
```

#### Fixed Code:
```go
// ✅ CORRECT - Line 56
s.logger.WithFields(logrus.Fields{
    "command": command,
    "timeout": options.Timeout,
}).Debug("Executing command")

// ✅ CORRECT - Line 82
s.logger.WithFields(logrus.Fields{
    "command":   command,
    "exit_code": result.ExitCode,
    "stderr":    result.Stderr,
}).Warn("Command failed")

// ✅ CORRECT - Line 86
s.logger.WithFields(logrus.Fields{
    "command":  command,
    "duration": result.Duration,
}).Debug("Command succeeded")

// ✅ CORRECT - Line 94
s.logger.WithFields(logrus.Fields{
    "command":  command,
    "duration": result.Duration,
    "timeout":  options.Timeout,
}).Warn("Command timed out")
```

---

### File: `internal/ssh/client.go`

#### Current Code (Line 74):
```go
// ❌ WRONG
c.logger.Infof("Connecting to %s@%s:%d...", user, host, port)
```

#### Fixed Code:
```go
// ✅ CORRECT
c.logger.WithFields(logrus.Fields{
    "user": user,
    "host": host,
    "port": port,
}).Info("Connecting to SSH host")
```

---

## 2. FIX: API Async Context Handling (MEDIUM PRIORITY)

### File: `internal/api/handlers/diagnostics.go`

#### Current Code (Lines 131-141):
```go
// ❌ WRONG - detached context
go h.executeDiagnostics(context.Background(), job, req)

// Return immediate response
resp := DiagnosticResponse{
    JobID:     job.ID.String(),
    Status:    string(job.Status),
    Target:    job.Target,
    CreatedAt: job.CreatedAt,
}
```

#### Fixed Code:
```go
// ✅ CORRECT - explicit timeout for async operation
asyncCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
go func() {
    defer cancel()
    h.executeDiagnostics(asyncCtx, job, req)
}()

// Return immediate response
resp := DiagnosticResponse{
    JobID:     job.ID.String(),
    Status:    string(job.Status),
    Target:    job.Target,
    CreatedAt: job.CreatedAt,
}
```

---

## 3. FIX: Slice Bounds Check in Tests (LOW PRIORITY)

### File: `internal/remediation/actions_disk_test.go`

#### Current Code (Lines 737-738):
```go
// ❌ WRONG - could panic if ChangesApplied is empty
if !strings.Contains(result.ChangesApplied[0], "250M") {
    t.Errorf("Execute() ChangesApplied[0] = %q, want to mention initial size 250M", result.ChangesApplied[0])
}
```

#### Fixed Code:
```go
// ✅ CORRECT - bounds check before access
if len(result.ChangesApplied) == 0 {
    t.Errorf("Execute() ChangesApplied is empty, want at least one change")
} else if !strings.Contains(result.ChangesApplied[0], "250M") {
    t.Errorf("Execute() ChangesApplied[0] = %q, want to mention initial size 250M", result.ChangesApplied[0])
}
```

#### Alternative (Cleaner):
```go
// ✅ CORRECT - assert first element exists with helper
if len(result.ChangesApplied) < 1 {
    t.Fatal("Execute() ChangesApplied must have at least one entry")
}
if !strings.Contains(result.ChangesApplied[0], "250M") {
    t.Errorf("Execute() ChangesApplied[0] = %q, want to mention initial size 250M", result.ChangesApplied[0])
}
```

---

## 4. FIX: Document Intentional Non-Wrapped Errors (LOW PRIORITY)

### File: `internal/remediation/remediation.go`

#### Current Code (Lines 320, 322):
```go
// ❌ WRONG - Non-wrapped errors without explanation
return fmt.Errorf("action %s is not reversible", b.id)
return fmt.Errorf("Rollback not implemented for %s", b.id)
```

#### Fixed Code:
```go
// ✅ CORRECT - Documented terminal errors
// These are intentional terminal error messages that don't wrap underlying errors
// as they represent final states (not reversible, not implemented)
return fmt.Errorf("action %s is not reversible", b.id)
return fmt.Errorf("rollback not implemented for %s", b.id)
```

---

## 5. OPTIONAL FIX: Move API Constants to Config (NICE TO HAVE)

### File: `internal/ai/anthropic.go`

#### Current Code (Lines 13-22):
```go
const (
    // AnthropicAPIURL is the default Anthropic API endpoint
    AnthropicAPIURL = "https://api.anthropic.com/v1/messages"

    // AnthropicAPIVersion is the required API version header
    AnthropicAPIVersion = "2023-06-01"

    // DefaultAnthropicModel is the default Claude model
    DefaultAnthropicModel = "claude-sonnet-4-5-20250929"
)
```

#### Improvement (Config-driven):
```go
// In config.yaml or config structure:
ai:
  providers:
    anthropic:
      endpoint: "https://api.anthropic.com/v1/messages"
      api_version: "2023-06-01"
      default_model: "claude-sonnet-4-5-20250929"

// In code:
const (
    // Fallback values if config not provided
    defaultAnthropicURL = "https://api.anthropic.com/v1/messages"
)

func NewAnthropicProvider(config *ProviderConfig, log *logrus.Logger) (*AnthropicProvider, error) {
    // Use config value if provided, otherwise fallback
    endpoint := config.Endpoint
    if endpoint == "" {
        endpoint = defaultAnthropicURL
    }
    ...
}
```

---

## 6. ADDITIONAL IMPROVEMENT: Add LocalExecutor Context Handling

### File: `internal/diagnostics/executor.go`

#### Current Code (Lines 65-74):
```go
// Execute runs a command locally and returns the result
func (e *LocalExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
    if timeout == 0 {
        timeout = 30 * time.Second
    }

    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    return e.ExecuteWithContext(ctx, command)
}
```

#### Improved Version:
```go
// Execute runs a command locally with proper context handling
func (e *LocalExecutor) Execute(command string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
    ctx, cancel := context.WithTimeout(context.Background(), e.getTimeout(timeout))
    defer cancel()

    return e.ExecuteWithContext(ctx, command)
}

// Helper method to get timeout with sensible default
func (e *LocalExecutor) getTimeout(timeout time.Duration) time.Duration {
    if timeout > 0 {
        return timeout
    }
    return 30 * time.Second
}
```

---

## Implementation Plan

### Phase 1: High-Impact Fixes (2-3 hours)
1. Standardize SSH logging (affects multiple files, 2 hours)
2. Add async context timeout (1 hour)

### Phase 2: Safety Improvements (1 hour)
1. Fix slice bounds checking (30 min)
2. Document intentional errors (30 min)

### Phase 3: Documentation (30 minutes)
1. Add context usage guidelines to DEVELOPMENT.md
2. Document logging patterns

### Phase 4: Optional Improvements (1-2 hours)
1. Move API constants to configuration
2. Improve error documentation

---

## Testing Plan

After each fix:

1. **Logging Changes**
   ```bash
   # Run affected tests
   go test ./internal/ssh/... -v

   # Verify log format in output
   go test ./cmd/lumo-agent/... -v
   ```

2. **Context Changes**
   ```bash
   # Run API handler tests
   go test ./internal/api/handlers/... -v

   # Check for context cancellation
   go test ./internal/api/... -race
   ```

3. **Bounds Check**
   ```bash
   # Run remediation tests
   go test ./internal/remediation/... -v
   ```

4. **Full CI Pipeline**
   ```bash
   make ci
   ```

---

## Verification Checklist

- [ ] All logging uses `logrus.WithFields()`
- [ ] Async operations have explicit timeouts
- [ ] No slice access without bounds check
- [ ] All error types documented
- [ ] Tests pass with `go test -race ./...`
- [ ] All changes formatted with `go fmt`
- [ ] No new lint warnings with `golangci-lint`
- [ ] Security scan passes with `govulncheck`
- [ ] Documentation updated

---

## Estimated Time by Fix

| Fix | Time | Priority | Impact |
|-----|------|----------|--------|
| SSH Logging | 2 hrs | HIGH | Operational |
| Async Context | 1 hr | HIGH | Reliability |
| Slice Bounds | 0.5 hr | LOW | Safety |
| Error Docs | 0.5 hr | LOW | Maintainability |
| Config Constants | 1 hr | OPTIONAL | Flexibility |
| **TOTAL** | **5 hrs** | - | - |

---

## Deployment Impact

- ✅ No breaking changes
- ✅ No performance impact
- ✅ Backward compatible
- ✅ Can be deployed incrementally
- ✅ No schema changes needed

---

## Post-Deployment Monitoring

1. Monitor logs for proper structured format
2. Verify async diagnostics complete within timeout
3. Check for any unexpected errors in remediation rollback
4. Monitor test coverage metrics

---

## Questions & Answers

**Q: Will these changes affect performance?**
A: No. Logging and context changes are purely structural with minimal overhead.

**Q: Do we need to update configuration files?**
A: Only the optional improvement (config constants) would require config file updates.

**Q: Can these be done incrementally?**
A: Yes. Each fix is independent and can be deployed separately.

**Q: Will this break existing deployments?**
A: No. All changes are backward compatible.

**Q: How long will code review take?**
A: Estimated 1-2 hours per PR given the straightforward nature of changes.

---

## Related Documentation

- See `code-quality-review.md` for detailed analysis
- See `code-quality-summary.txt` for executive summary
- See CLAUDE.md for project conventions

---

**Next Step:** Create a GitHub issue for tracking these fixes and assign to team.
