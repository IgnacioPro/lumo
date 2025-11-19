# Example 1: Local Diagnostics

This example demonstrates the most basic use case: running diagnostics on your local machine.

## What You'll Learn

- How to run a basic diagnostic
- Understanding diagnostic output
- Selecting specific checks
- Different output formats

## Prerequisites

- Lumo installed
- Configuration set up (run `lumo init` if you haven't)

## Basic Usage

### Run All Checks

```bash
lumo diagnose localhost
```

This runs all enabled diagnostic checks on your local machine and displays results in a human-readable format.

**Expected Output:**
```
[INFO] Diagnosing localhost
[INFO] Running CPU check...
[INFO] Running Memory check...
[INFO] Running Disk check...
[INFO] Running Process check...
[INFO] Running Service check...
[INFO] Running Network check...

=== Diagnostic Report ===

CPU:
  Architecture: x86_64
  Cores: 8 (4 physical)
  Load Average (1/5/15): 1.23 / 1.45 / 1.32
  Usage: 15%
  Status: ✓ OK

Memory:
  Total: 16.0 GB
  Used: 8.2 GB (51%)
  Available: 7.8 GB
  Swap: 2.0 GB (0.1 GB used)
  Status: ✓ OK

Disk:
  /dev/sda1 (/): 45% used (120 GB / 250 GB)
  /dev/sdb1 (/data): 78% used (780 GB / 1 TB)
  Status: ⚠ WARNING (high usage on /data)

... (more output)
```

### Run Specific Checks

Select only the checks you need:

```bash
# CPU and memory only
lumo diagnose localhost --checks cpu,memory

# All core checks
lumo diagnose localhost --checks cpu,memory,disk,process,service,network

# Only security checks
lumo diagnose localhost --checks patch,ssh-security,ports,auth-failures
```

### Different Output Formats

#### Text (Default)
Human-readable format:
```bash
lumo diagnose localhost --format text
```

#### JSON
Machine-readable format for scripting:
```bash
lumo diagnose localhost --format json
```

**Example JSON Output:**
```json
{
  "target": "localhost",
  "timestamp": "2025-01-20T10:30:00Z",
  "checks": {
    "cpu": {
      "status": "ok",
      "load_avg": {
        "1min": 1.23,
        "5min": 1.45,
        "15min": 1.32
      },
      "usage_percent": 15.3
    },
    "memory": {
      "status": "ok",
      "total_bytes": 17179869184,
      "used_bytes": 8791146496,
      "available_bytes": 8388722688
    }
  }
}
```

#### TOON
AI-optimized format (30-60% smaller than JSON):
```bash
lumo diagnose localhost --format toon
```

## Advanced Usage

### Verbose Output

Get detailed diagnostic information:

```bash
lumo diagnose localhost --verbose
```

This shows:
- Debug logs
- Executed commands
- Timing information
- Additional metrics

### Save to File

```bash
# Text format
lumo diagnose localhost > diagnostic-report.txt

# JSON format
lumo diagnose localhost --format json > diagnostic-report.json

# TOON format
lumo diagnose localhost --format toon > diagnostic-report.toon
```

### Combine with Other Tools

```bash
# Monitor diagnostics over time
watch -n 60 'lumo diagnose localhost --checks cpu,memory'

# Check multiple metrics and alert on issues
lumo diagnose localhost --format json | jq '.checks.disk.status' | \
  grep -q warning && echo "ALERT: Disk usage high"

# Generate periodic reports
while true; do
  lumo diagnose localhost --format json >> diagnostics-$(date +%Y%m%d).json
  sleep 3600  # Every hour
done
```

## Understanding Check Results

### Status Levels

- ✓ **OK**: Everything is normal
- ⚠ **WARNING**: Needs attention, but not critical
- ✗ **CRITICAL**: Immediate action required
- ℹ **INFO**: Informational, no action needed

### Common Warnings

**High CPU Usage (>80%):**
```
CPU Usage: 85%
Status: ⚠ WARNING
```
**Action**: Check top processes, investigate resource-heavy applications

**Low Memory (<10% free):**
```
Memory Available: 1.2 GB (8%)
Status: ⚠ WARNING
```
**Action**: Check memory-consuming processes, consider adding RAM

**High Disk Usage (>80%):**
```
/dev/sda1: 92% used
Status: ⚠ WARNING
```
**Action**: Clean up old files, expand storage, investigate large files

## Automation Examples

### Cron Job for Daily Reports

```bash
# Add to crontab (crontab -e)
0 9 * * * /usr/local/bin/lumo diagnose localhost --format json > /var/log/lumo-daily-$(date +\%Y\%m\%d).json
```

### CI/CD Health Check

```yaml
# .github/workflows/health-check.yml
name: Infrastructure Health Check
on:
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours

jobs:
  diagnose:
    runs-on: ubuntu-latest
    steps:
      - name: Run Lumo Diagnostic
        run: |
          lumo diagnose localhost --format json > diagnostic.json

      - name: Check for Critical Issues
        run: |
          if jq -e '.checks | to_entries[] | select(.value.status == "critical")' diagnostic.json; then
            echo "Critical issues found!"
            exit 1
          fi
```

### Monitoring Script

```bash
#!/bin/bash
# monitor.sh - Continuous monitoring with alerts

THRESHOLD_CPU=80
THRESHOLD_MEM=90
THRESHOLD_DISK=85

while true; do
  # Run diagnostic
  lumo diagnose localhost --format json > /tmp/diagnostic.json

  # Check CPU
  CPU=$(jq -r '.checks.cpu.usage_percent' /tmp/diagnostic.json)
  if (( $(echo "$CPU > $THRESHOLD_CPU" | bc -l) )); then
    echo "ALERT: CPU usage high: ${CPU}%"
    # Send alert (e.g., via curl to webhook)
  fi

  # Check Memory
  MEM=$(jq -r '.checks.memory.used_percent' /tmp/diagnostic.json)
  if (( $(echo "$MEM > $THRESHOLD_MEM" | bc -l) )); then
    echo "ALERT: Memory usage high: ${MEM}%"
  fi

  # Wait before next check
  sleep 300  # 5 minutes
done
```

## Troubleshooting

### "Command not found"
Ensure Lumo is in your PATH:
```bash
export PATH="$PATH:/usr/local/bin"
```

### "Permission denied" for certain checks
Some checks require elevated privileges:
```bash
# Option 1: Run with sudo
sudo lumo diagnose localhost

# Option 2: Grant capabilities
sudo setcap cap_net_raw,cap_sys_ptrace+ep $(which lumo)
```

### Checks taking too long
Use specific checks instead of all checks:
```bash
lumo diagnose localhost --checks cpu,memory  # Fast
```

## Next Steps

- **[Example 2: SSH Remote Server](../02-ssh-remote-server/)** - Diagnose remote machines
- **[Example 3: AI Analysis](../03-ai-analysis/)** - Use AI to interpret results
- **[Example 4: Auto-Remediation](../04-auto-remediation/)** - Automatically fix issues

## Additional Resources

- [Configuration Guide](../../docs/configuration.md)
- [Available Checks](../../docs/checks.md)
- [Output Formats](../../docs/output-formats.md)
