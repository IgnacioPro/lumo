# Example 3: AI-Powered Analysis

This example demonstrates how to use AI to analyze diagnostic results and get actionable insights.

## What You'll Learn

- Using `--analyze` flag for AI analysis
- Choosing AI providers (Claude, GPT, Gemini, Ollama, OpenRouter)
- Understanding AI recommendations
- Cost optimization with TOON format
- Advanced prompting for specific scenarios

## Prerequisites

- Lumo configured with AI provider
- API key set in environment variable:
  ```bash
  export LUMO_ANTHROPIC_API_KEY=your-key  # Or your chosen provider
  ```

## Basic AI Analysis

### Simple Analysis

Add `--analyze` to any diagnostic command:

```bash
# Local analysis
lumo diagnose localhost --analyze

# Remote server analysis
lumo diagnose user@server --use-agent --analyze

# Specific checks with analysis
lumo diagnose localhost --checks cpu,memory,disk --analyze
```

**Example Output:**
```
=== Diagnostic Report ===

CPU: Usage: 85%, Load: 4.2/3.8/3.5
Memory: 14.2 GB / 16 GB used (89%)
Disk: /dev/sda1 92% full

=== AI Analysis ===

🤖 Analysis by Claude (Anthropic)

**Summary:** Your system is experiencing resource pressure across CPU, memory, and disk.

**Issues Identified:**

1. **HIGH CPU Usage (85%)**
   - Severity: ⚠️ WARNING
   - Impact: Performance degradation, slow response times
   - Root Cause: Multiple resource-intensive processes running

2. **HIGH Memory Usage (89%)**
   - Severity: ⚠️ WARNING
   - Impact: Risk of OOM kills, swapping
   - Root Cause: Memory-hungry applications + insufficient RAM

3. **CRITICAL Disk Usage (92%)**
   - Severity: ✗ CRITICAL
   - Impact: System may become unresponsive when disk fills
   - Root Cause: Log files, temporary data accumulation

**Recommendations:**

Priority 1 (Immediate):
- Free up disk space: clean logs, remove old files
- Identify and investigate top disk consumers
- Consider expanding disk capacity

Priority 2 (Short-term):
- Review and optimize high-CPU processes
- Investigate memory leaks in applications
- Consider adding more RAM

Priority 3 (Long-term):
- Implement log rotation policies
- Set up monitoring and alerting
- Plan for horizontal scaling

**Commands to Run:**
```bash
# Find large files
du -sh /* | sort -rh | head -10

# Check log sizes
du -sh /var/log/*

# Top disk consumers
ncdu /

# Top CPU processes
htop
```

**Estimated Time to Resolve:** 30-60 minutes for immediate actions
```

## AI Provider Comparison

### Anthropic Claude (Recommended)

```bash
export LUMO_AI_PROVIDER=anthropic
export LUMO_ANTHROPIC_API_KEY=sk-ant-...

lumo diagnose localhost --analyze
```

**Strengths:**
- Best balance of quality and cost
- Excellent at root cause analysis
- Clear, actionable recommendations
- Good context understanding

**Models:**
- `claude-3-5-sonnet-20241022` (default) - Best overall
- `claude-3-5-haiku-20241022` - Fastest, cheapest
- `claude-3-opus-20240229` - Most thorough

### OpenAI GPT

```bash
export LUMO_AI_PROVIDER=openai
export LUMO_OPENAI_API_KEY=sk-...

lumo diagnose localhost --analyze
```

**Strengths:**
- Fast response times
- Good for pattern recognition
- Wide model selection

**Models:**
- `gpt-4o` (default) - Best quality
- `gpt-4o-mini` - Faster, cheaper
- `o1` - Advanced reasoning (for complex issues)

### Google Gemini

```bash
export LUMO_AI_PROVIDER=gemini
export LUMO_GEMINI_API_KEY=...

lumo diagnose localhost --analyze
```

**Strengths:**
- Free tier available
- Fast inference
- Good for quick checks

**Models:**
- `gemini-2.0-flash-exp` (default)
- `gemini-1.5-pro`

### Ollama (Local/Self-Hosted)

```bash
export LUMO_AI_PROVIDER=ollama
export LUMO_OLLAMA_HOST=http://localhost:11434

lumo diagnose localhost --analyze
```

**Strengths:**
- No API costs
- Complete privacy
- Works offline
- Customizable models

**Models:**
- `llama3.2` (default)
- `mistral`
- `codellama`

### OpenRouter (Multi-Model Access)

```bash
export LUMO_AI_PROVIDER=openrouter
export LUMO_OPENROUTER_API_KEY=sk-or-...

lumo diagnose localhost --analyze
```

**Strengths:**
- Access to 100+ models
- Model fallback/routing
- Cost optimization

## Cost Optimization

### Use TOON Format (30-60% Savings)

TOON format reduces AI tokens by 30-60%:

```bash
# Instead of this (uses JSON internally)
lumo diagnose localhost --analyze

# Use TOON for analysis (smaller tokens)
lumo diagnose localhost --format toon --analyze
```

**Token Comparison:**
- JSON: ~2,500 tokens
- TOON: ~1,000 tokens (60% reduction)
- Cost savings: $0.0015 → $0.0006 per analysis (Claude)

### Use Cheaper Models

```bash
# Premium model (~$0.003 per analysis)
export LUMO_AI_MODEL=claude-3-opus-20240229
lumo diagnose localhost --analyze

# Balanced model (~$0.0006 per analysis)
export LUMO_AI_MODEL=claude-3-5-sonnet-20241022
lumo diagnose localhost --analyze

# Budget model (~$0.0001 per analysis)
export LUMO_AI_MODEL=claude-3-5-haiku-20241022
lumo diagnose localhost --analyze
```

### Selective Analysis

Only analyze when needed:

```bash
# Run diagnostic without analysis (free)
lumo diagnose localhost > report.txt

# Review the report, then analyze if concerning
cat report.txt | lumo analyze  # (Future feature)

# Or be selective about checks
lumo diagnose localhost --checks cpu,memory --analyze  # Cheaper than all checks
```

## Advanced Use Cases

### Security-Focused Analysis

```bash
lumo diagnose server --checks patch,ssh-security,ports,auth-failures --analyze
```

AI will focus on security implications:
- Unpatched vulnerabilities
- Weak SSH configurations
- Unnecessary open ports
- Brute force attempts

### Performance Analysis

```bash
lumo diagnose server --checks cpu,memory,disk,process,network --analyze
```

AI will analyze:
- Performance bottlenecks
- Resource contention
- Optimization opportunities

### Capacity Planning

```bash
# Run diagnostics over time
for i in {1..10}; do
  lumo diagnose server --format json >> capacity-data.json
  sleep 3600  # Hourly
done

# Analyze trends (future feature)
lumo analyze capacity-data.json --mode capacity-planning
```

## Real-World Examples

### Example 1: Investigating Slow Application

```bash
# Full diagnostic with AI analysis
lumo diagnose app-server --use-agent --analyze > investigation-$(date +%Y%m%d).txt

# Focus on likely culprits
lumo diagnose app-server --checks cpu,memory,process,network --analyze
```

**Typical AI Output:**
```
🤖 Analysis: Application Performance Issue

Root Cause: High CPU usage (92%) caused by:
- nginx worker processes consuming 40% CPU
- Java application consuming 35% CPU
- Multiple Python processes (12% CPU combined)

Recommendations:
1. Check nginx for slow backends (may indicate app server issues)
2. Profile Java application (possible infinite loop or memory leak)
3. Review Python processes - may be stuck workers

Next Steps:
- Check nginx access logs for slow requests
- Get Java thread dump: jstack <pid>
- Review application logs for errors
```

### Example 2: Post-Incident Analysis

```bash
# Capture current state
lumo diagnose prod-db --use-agent --analyze > post-incident-$(date +%Y%m%d-%H%M).txt

# Compare with baseline
diff baseline-diagnostic.txt post-incident-*.txt
```

### Example 3: Scheduled Analysis

```bash
#!/bin/bash
# daily-analysis.sh - Run daily with AI analysis

lumo diagnose prod-web01 --use-agent --format toon --analyze > \
  "reports/prod-web01-$(date +%Y%m%d).txt"

# Alert if critical issues found
if grep -q "CRITICAL" "reports/prod-web01-$(date +%Y%m%d).txt"; then
  mail -s "Critical Issues on prod-web01" ops@example.com < \
    "reports/prod-web01-$(date +%Y%m%d).txt"
fi
```

## Interpreting AI Recommendations

### Severity Levels

AI will categorize issues:

- 🔴 **CRITICAL**: Immediate action required (system failure imminent)
- 🟡 **WARNING**: Needs attention soon (degraded performance)
- 🔵 **INFO**: Optimization opportunities (no immediate risk)

### Confidence Levels

Some AI responses include confidence:

```
**Root Cause (90% confidence):** Memory leak in application
**Alternative Explanation (10%):** Sudden traffic spike

Recommendation: Check both, but focus on memory leak first
```

### Actionable vs. Informational

**Actionable:**
```
✓ Run: sudo systemctl restart nginx
✓ Investigate: /var/log/application.log
✓ Consider: Upgrade to 16 GB RAM
```

**Informational:**
```
ℹ System is running normally
ℹ Resource usage is typical for this workload
ℹ No immediate action needed
```

## Troubleshooting

### "AI provider not configured"

```bash
# Set provider and API key
export LUMO_AI_PROVIDER=anthropic
export LUMO_ANTHROPIC_API_KEY=your-key

# Verify
lumo diagnose localhost --analyze
```

### "API key invalid"

```bash
# Check key format
echo $LUMO_ANTHROPIC_API_KEY  # Should start with sk-ant-

# Test manually
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $LUMO_ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-3-5-sonnet-20241022","max_tokens":10,"messages":[{"role":"user","content":"Hi"}]}'
```

### "Rate limit exceeded"

```bash
# Wait and retry, or use different model
export LUMO_AI_MODEL=claude-3-5-haiku-20241022  # Higher rate limits
lumo diagnose localhost --analyze

# Or use Ollama (no rate limits)
export LUMO_AI_PROVIDER=ollama
lumo diagnose localhost --analyze
```

## Best Practices

1. **Use TOON format** for cost savings
2. **Start with Haiku/mini models** for routine checks
3. **Reserve premium models** for complex issues
4. **Cache diagnostic results** and analyze offline
5. **Review AI recommendations** before executing
6. **Use selective checks** to reduce token usage
7. **Set up Ollama** for development/testing

## Next Steps

- **[Example 4: Auto-Remediation](../04-auto-remediation/)** - Automatically fix issues identified by AI
- **[Advanced AI Prompting](../../docs/ai-prompting.md)** - Customize AI analysis
- **[Cost Optimization Guide](../../docs/cost-optimization.md)** - Minimize AI costs

## Additional Resources

- [AI Provider Comparison](../../docs/ai-providers.md)
- [TOON Format Specification](../../docs/toon-format.md)
- [Pricing Calculator](../../docs/pricing-calculator.md)
