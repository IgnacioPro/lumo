# Example 4: Auto-Remediation

This example demonstrates Lumo's auto-remediation capabilities with human-in-the-loop approval.

## What You'll Learn

- Automatic issue detection and fixes
- Human-in-the-loop approval workflow
- Risk levels (safe, moderate, critical)
- Dry-run mode for testing
- Audit logging

## Prerequisites

- Lumo configured
- Sudo/root access for system modifications (when needed)
- AI provider configured (recommended for smart remediation)

## Safety First

Lumo uses a risk-based approval system:

- **Safe Actions**: Low-risk (restart service, clear cache)
- **Moderate Actions**: Some risk (kill process, delete files)
- **Critical Actions**: High risk (system configuration changes)

## Basic Remediation

### Dry-Run Mode (Recommended First Step)

Preview what would be fixed without making changes:

```bash
# Local dry-run
lumo fix localhost --dry-run

# Remote dry-run
lumo fix user@server --use-agent --dry-run
```

**Example Output:**
```
=== Remediation Plan (DRY-RUN MODE) ===

Issues Found: 3

1. [SAFE] Disk Space Critical on /var/log
   Action: Clean old log files (>30 days)
   Files to delete: 45 files, 2.3 GB
   Risk: Low - only affects old logs

2. [MODERATE] Unresponsive Service: nginx
   Action: Restart nginx service
   Risk: Moderate - brief downtime (~2s)

3. [CRITICAL] Kernel Updates Available
   Action: Install security updates and reboot
   Risk: High - requires system reboot

🔵 Would execute 3 actions (DRY-RUN, no changes made)
```

### Interactive Approval (Recommended)

Review and approve each action:

```bash
lumo fix localhost
```

**Interactive Session:**
```
=== Remediation Session ===

Issue #1: Disk space critical on /var/log (92% full)
Proposed Action: Delete log files older than 30 days
Risk Level: SAFE
Files: 45 files, 2.3 GB

? Approve this action? (y/n/s/q)
  y - Yes, execute
  n - No, skip
  s - Show details
  q - Quit

Your choice: y

✓ Executed: Cleaned 45 log files (2.3 GB freed)

Issue #2: Service 'nginx' not responding
Proposed Action: Restart nginx service
Risk Level: MODERATE
Impact: 2-3 seconds downtime

? Approve this action? (y/n/s/q): y

✓ Executed: Service nginx restarted successfully

Issue #3: Critical security updates available
Proposed Action: Install updates and reboot
Risk Level: CRITICAL
Updates: 15 security patches
Impact: System reboot required (~3 min downtime)

? Approve this action? (y/n/s/q): n

⊘ Skipped: Security updates not applied

=== Summary ===
✓ Executed: 2 actions
⊘ Skipped: 1 action
✗ Failed: 0 actions
```

### Auto-Approve Safe Actions

Automatically approve low-risk actions:

```bash
lumo fix localhost --auto-approve-safe
```

Only safe actions execute automatically; moderate/critical still require approval.

### Auto-Approve All (Use with Caution)

Auto-approve everything (not recommended for production):

```bash
lumo fix localhost --auto-approve-all

# Or specific risk levels
lumo fix localhost --auto-approve safe,moderate
```

## Common Remediation Scenarios

### Scenario 1: Clean Up Disk Space

```bash
lumo fix localhost --issues disk-space
```

**Actions Taken:**
- Remove old log files
- Clean package manager cache
- Remove temp files
- Clear old crash dumps
- Optimize Docker images (if Docker installed)

### Scenario 2: Fix Unresponsive Services

```bash
lumo fix localhost --issues services
```

**Actions Taken:**
- Restart failed services
- Kill hung processes
- Clear service cache
- Check and fix dependencies

### Scenario 3: Apply Security Updates

```bash
lumo fix localhost --issues security --dry-run

# Review, then apply
lumo fix localhost --issues security
```

**Actions Taken:**
- Install security patches
- Disable insecure SSH settings
- Close unnecessary open ports
- Update SSL/TLS certificates

### Scenario 4: Performance Optimization

```bash
lumo fix localhost --issues performance --analyze
```

**Actions Taken (with AI analysis):**
- Kill runaway processes
- Adjust OOM settings
- Clear page cache if safe
- Optimize service configurations

## Advanced Usage

### Custom Remediation Policies

Create a policy file:

```yaml
# remediation-policy.yaml

policies:
  disk_space:
    auto_approve: true
    thresholds:
      warning: 80
      critical: 90
    actions:
      - clean_logs: true
        retention_days: 30
      - clean_tmp: true
      - clean_cache: true
      - alert_only: false  # Actually fix, don't just alert

  services:
    auto_approve: false  # Always require approval
    actions:
      - restart: true
      - kill_hung: true
        timeout: 300  # 5 minutes

  security:
    auto_approve: false
    require_maintenance_window: true
    actions:
      - patch: true
      - reboot_if_needed: true
```

Use it:

```bash
lumo fix localhost --policy remediation-policy.yaml
```

### Scheduled Remediation

Set up cron for automatic fixes:

```bash
# crontab -e

# Every night at 2 AM, auto-fix safe issues
0 2 * * * /usr/local/bin/lumo fix localhost --auto-approve-safe --quiet >> /var/log/lumo-fix.log 2>&1

# Weekly full fix (with approval if issues found)
0 3 * * 0 /usr/local/bin/lumo fix localhost --email-approval ops@example.com
```

### Remediation with Monitoring

```bash
#!/bin/bash
# fix-with-monitoring.sh

# Pre-fix metrics
echo "Before:"
lumo diagnose localhost --checks cpu,memory,disk --format json > before.json

# Apply fixes
lumo fix localhost --auto-approve-safe

# Wait for system to stabilize
sleep 60

# Post-fix metrics
echo "After:"
lumo diagnose localhost --checks cpu,memory,disk --format json > after.json

# Compare
echo "Changes:"
diff before.json after.json
```

### Remote Fleet Remediation

```bash
#!/bin/bash
# fix-fleet.sh

SERVERS=(
  "web01.example.com"
  "web02.example.com"
  "app01.example.com"
)

for server in "${SERVERS[@]}"; do
  echo "=== Remediating $server ==="

  # Dry-run first
  lumo fix "admin@$server" --use-agent --dry-run > "reports/$server-plan.txt"

  # Review plan
  cat "reports/$server-plan.txt"

  read -p "Proceed with $server? (y/n) " -n 1 -r
  echo

  if [[ $REPLY =~ ^[Yy]$ ]]; then
    lumo fix "admin@$server" --use-agent --auto-approve-safe | tee "reports/$server-result.txt"
  fi
done
```

## Audit Logging

All remediation actions are logged:

```bash
# View audit log
cat ~/.lumo/audit.log

# Or in JSON format
cat ~/.lumo/audit.json
```

**Example Audit Entry:**
```json
{
  "timestamp": "2025-01-20T15:30:45Z",
  "target": "localhost",
  "issue": "disk_space_critical",
  "action": "clean_old_logs",
  "risk_level": "safe",
  "approved_by": "user",
  "approval_method": "interactive",
  "result": "success",
  "details": {
    "files_deleted": 45,
    "space_freed_bytes": 2470000000,
    "execution_time_ms": 1250
  }
}
```

Query audit log:

```bash
# Failed actions
jq '.[] | select(.result == "failed")' ~/.lumo/audit.json

# Actions by user
jq '.[] | select(.approved_by == "john")' ~/.lumo/audit.json

# Critical actions
jq '.[] | select(.risk_level == "critical")' ~/.lumo/audit.json
```

## Safety Features

### Rollback Support

Some actions support automatic rollback:

```bash
lumo fix localhost --enable-rollback
```

If a fix causes issues, Lumo can revert:

```bash
lumo rollback --last
lumo rollback --action-id abc123
```

### Backup Before Action

Automatically backup before critical changes:

```bash
lumo fix localhost --backup-first
```

### Maintenance Windows

Only fix during specific time windows:

```bash
# Only fix between 2-4 AM
lumo fix localhost --maintenance-window "02:00-04:00"

# Only fix on weekends
lumo fix localhost --maintenance-window "Sat-Sun"
```

## Real-World Examples

### Example 1: Emergency Disk Space Fix

```bash
# Server running out of disk space
ssh user@server df -h
# /dev/sda1  98% full

# Quick fix
lumo fix user@server --use-agent --issues disk-space --auto-approve-safe

# Verify
ssh user@server df -h
# /dev/sda1  82% full
```

### Example 2: Fix Multiple Failing Services

```bash
# Multiple services failed after update
lumo diagnose prod-app --checks service
# Found: 3 failed services (redis, nginx, app-worker)

# Fix with approval
lumo fix prod-app --use-agent --issues services
# Approve restart of each service

# Verify
lumo diagnose prod-app --checks service
# All services running ✓
```

### Example 3: Automated Nightly Cleanup

```bash
#!/bin/bash
# /etc/cron.daily/lumo-cleanup

# Auto-fix safe issues on all servers
SERVERS=$(cat /etc/lumo/servers.txt)

for server in $SERVERS; do
  lumo fix "$server" \
    --use-agent \
    --auto-approve-safe \
    --issues disk-space,logs,temp-files \
    --quiet \
    >> /var/log/lumo/nightly-cleanup-$(date +%Y%m%d).log 2>&1
done
```

## Troubleshooting

### "Permission denied" for remediation

```bash
# Some actions require root/sudo
sudo lumo fix localhost

# Or configure sudo for specific actions
# /etc/sudoers.d/lumo
lumo ALL=(ALL) NOPASSWD: /bin/systemctl restart *
lumo ALL=(ALL) NOPASSWD: /usr/bin/apt-get update
```

### "Action failed"

Check audit log:

```bash
# View failures
grep "failed" ~/.lumo/audit.log

# Detailed error
lumo fix localhost --verbose
```

### Rollback a Fix

```bash
# View recent actions
lumo audit --recent 10

# Rollback specific action
lumo rollback --action-id <id>

# Rollback last action
lumo rollback --last
```

## Best Practices

1. **Always dry-run first** in production
2. **Use interactive approval** for moderate/critical actions
3. **Test in staging** before production
4. **Enable audit logging** (on by default)
5. **Set up rollback** for critical systems
6. **Use maintenance windows** for scheduled fixes
7. **Monitor after fixes** to verify success
8. **Keep backups** before critical changes

## Next Steps

- **[Example 5: Agent Deployment (K8s)](../05-agent-deployment-k8s/)** - Automated remediation at scale
- **[Example 6: Agent Deployment (VMs)](../06-agent-deployment-vms/)** - Remediation on bare metal
- **[Remediation Policies](../../docs/remediation-policies.md)** - Create custom policies
- **[Audit Guide](../../docs/audit.md)** - Track all changes

## Additional Resources

- [Action Catalog](../../docs/actions.md)
- [Risk Assessment](../../docs/risk-levels.md)
- [Rollback Guide](../../docs/rollback.md)
