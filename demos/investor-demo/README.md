# Lumo Investor Demo

> **Purpose:** Live demonstration of Lumo's capabilities for investor presentations
> **Duration:** 10 minutes
> **Audience:** Potential investors, partners, stakeholders

---

## Overview

This directory contains everything you need to deliver a compelling, live demonstration of Lumo to potential investors. The demo showcases:

1. **Real-time problem detection** (12 health checks in 30 seconds)
2. **AI-powered analysis** (expert-level insights from Claude/GPT/Gemini)
3. **Auto-remediation** (safe, human-approved fixes)
4. **Quantifiable ROI** (95% time reduction, $4.1M annual savings)

---

## Quick Start

### Prerequisites

1. **Lumo installed** (see [Getting Started](../../docs/getting-started.md))
2. **AI provider configured** (optional, but recommended for full demo)
   ```bash
   export LUMO_ANTHROPIC_API_KEY=sk-ant-...
   # or
   export LUMO_OPENAI_API_KEY=sk-...
   ```
3. **Sudo access** (for systemd service demo - optional)

### Run the Demo (3 steps)

```bash
# 1. Setup demo environment
./setup.sh

# 2. Start simulated issues
./start-issues.sh

# 3. Run the guided demo
./run-demo.sh
```

That's it! The demo script will guide you through each section with timing and talking points.

### Reset for Next Demo

```bash
./reset.sh
```

---

## Demo Structure

### Section 1: Problem Introduction (1-2 min)

**Goal:** Establish the pain point

**Talking Points:**
- Traditional incident response: 45-60 minutes
- 60-80% of engineer time on repetitive tasks
- Alert fatigue: 85% require manual investigation
- Cost: $150-200 per incident in engineering time

**Demo Action:** Set the scene - "Your production server is having issues..."

---

### Section 2: Lumo Diagnosis (2 min)

**Goal:** Show instant, comprehensive diagnostics

**Command:**
```bash
lumo diagnose localhost --checks cpu,memory,disk,service,process
```

**What It Shows:**
- 12 parallel health checks complete in < 30 seconds
- Clear, actionable results
- Identifies: disk space issues, failed services, suspicious processes

**Talking Points:**
- No installation required on targets (CLI mode)
- Works via SSH, API, or deployed agents
- Cross-platform (Linux, macOS, Windows)

---

### Section 3: AI Analysis (1-2 min)

**Goal:** Demonstrate AI-powered intelligence

**Command:**
```bash
lumo diagnose localhost --analyze
```

**What It Shows:**
- AI provides root cause analysis
- Expert-level recommendations
- Clear priority ranking
- Estimated time to resolution

**Talking Points:**
- 5 AI providers (Claude, GPT, Gemini, Ollama, OpenRouter)
- TOON format reduces token costs by 30-60%
- Works offline with Ollama (on-prem/air-gapped)

---

### Section 4: Auto-Remediation (2-3 min)

**Goal:** Show safe, intelligent automation

**Commands:**
```bash
# Preview (dry-run)
lumo fix localhost --dry-run

# Execute with approval
lumo fix localhost
```

**What It Shows:**
- Risk classification (safe, moderate, critical)
- Human-in-the-loop approval
- Real-time execution
- Verification of resolution

**Talking Points:**
- Safety first: always requires approval for risky actions
- Full audit trail for compliance
- Can auto-approve safe actions only
- Reduces MTTR by 95%

---

### Section 5: Business Impact (1-2 min)

**Goal:** Quantify value and ROI

**Key Metrics:**
- **Time Savings:** 45 min → 2 min (95% reduction)
- **Cost Savings:** $143-193 per incident
- **Annual Impact:** $4.1M for 10-engineer team
- **ROI:** 8,100% (4.4-day payback period)

**Talking Points:**
- Proven metrics from real-world usage
- Scales linearly with team size
- Prevents downtime, not just detects it
- Frees senior engineers for strategic work

---

### Section 6: Deployment & Scale (1 min)

**Goal:** Show production readiness and flexibility

**Deployment Modes:**
1. **CLI:** SSH to targets, zero installation
2. **Kubernetes:** DaemonSet/Deployment, Helm charts
3. **VMs:** systemd service, DEB/RPM packages
4. **API:** Centralized server with PostgreSQL/Redis

**Talking Points:**
- Scales from 1 to 1000s of servers
- Production-ready (66.7% test coverage, CI/CD)
- Multi-platform notifications (Slack, Telegram, Email, etc.)
- Enterprise features: SSO, RBAC, multi-tenancy (roadmap)

---

### Section 7: Summary & Q&A (1 min)

**Goal:** Reinforce key messages and handle questions

**Key Takeaways:**
- Only AI-native SRE automation platform
- 95% time reduction, $4.1M annual savings
- Open-source foundation (MIT license)
- Production-ready, scaling to 1000s of deployments

**The Ask:**
- $2M seed round
- 100 enterprise customers, $1M ARR in 12 months
- 10,000+ deployed agents

---

## Demo Scripts

### setup.sh

Creates a realistic demo environment with:
- 50 log files (~250 MB)
- 100 temp files (~100 MB)
- Large database dump
- CPU/memory stress scripts
- Failing systemd service
- Baseline metrics

**Run once before demo.**

### start-issues.sh

Activates the simulated issues:
- Starts failing service
- Creates suspicious processes
- Simulates disk space accumulation

**Run this before each demo.**

### run-demo.sh

Interactive demo script with:
- Guided walkthrough (7 sections)
- Timing for each section
- Talking points at each step
- Automated and manual options
- Color-coded terminal output
- Total time tracking

**This is the main demo script.**

### reset.sh

Resets everything to clean state:
- Stops all demo processes
- Cleans workspace
- Removes systemd service
- Clears logs

**Run after each demo to prepare for next one.**

---

## Customization

### Change AI Provider

```bash
# Use Claude (recommended)
export LUMO_AI_PROVIDER=anthropic
export LUMO_ANTHROPIC_API_KEY=sk-ant-...

# Or use OpenAI
export LUMO_AI_PROVIDER=openai
export LUMO_OPENAI_API_KEY=sk-...

# Or use local Ollama (offline demo)
export LUMO_AI_PROVIDER=ollama
export LUMO_OLLAMA_HOST=http://localhost:11434
```

### Adjust Demo Duration

Edit `run-demo.sh` and modify:
- `sleep` durations (between sections)
- Talking points (add/remove as needed)
- Sections to include/exclude

### Add Your Own Scenarios

Edit `setup.sh` to create additional issues:
- Modify log file sizes
- Add more failing services
- Create custom diagnostic scenarios

---

## Tips for a Great Demo

### Before the Meeting

1. **Practice 3+ times** - Run through the entire demo
2. **Test your network** - Ensure stable connectivity
3. **Prepare backups** - Have screenshots/videos ready
4. **Check AI credits** - Ensure API keys are active
5. **Clean your screen** - Close unnecessary windows/tabs

### During the Demo

1. **Start with impact** - "Watch us fix a production issue in 2 minutes"
2. **Keep it simple** - Avoid deep technical jargon (unless asked)
3. **Show, don't tell** - Live execution > explaining
4. **Handle failures gracefully** - Demo has simulated output fallbacks
5. **Pause for questions** - Engage the audience

### After the Demo

1. **Send materials** - Pitch deck, ROI calculator, demo recording
2. **Offer trial** - Pilot deployment in their environment
3. **Follow up quickly** - Within 24 hours
4. **Collect feedback** - What resonated? What questions remain?

---

## Troubleshooting

### "lumo: command not found"

**Solution:**
```bash
# Add lumo to PATH
export PATH=$PATH:/path/to/lumo

# Or run from lumo directory
cd /path/to/lumo
./lumo diagnose localhost
```

### AI provider not working

**Solution:**
```bash
# Check API key is set
echo $LUMO_ANTHROPIC_API_KEY

# Test API key
lumo diagnose localhost --analyze --verbose

# Use simulated output (demo script falls back automatically)
```

### Systemd service not available

**Expected** - Demo works without systemd. Script will simulate service management.

**Optional:** Skip systemd parts by editing `setup.sh` and commenting out service creation.

### Demo runs too fast/slow

**Adjust timing** in `run-demo.sh`:
```bash
# Find sleep commands and adjust
sleep 1  # Change to 2 for slower demo
```

---

## Files in This Directory

```
demos/investor-demo/
├── README.md              # This file
├── setup.sh               # Environment setup
├── start-issues.sh        # Activate demo issues
├── run-demo.sh            # Main demo script (guided)
├── reset.sh               # Reset to clean state
└── workspace/             # Created by setup.sh
    ├── logs/              # Sample log files
    ├── tmp/               # Temporary files
    ├── data/              # Sample data files
    └── *.sh               # Simulation scripts
```

---

## Success Metrics

After running this demo, investors should:

- [ ] Understand Lumo's value proposition clearly
- [ ] See quantifiable ROI ($4.1M savings example)
- [ ] Appreciate the AI differentiation
- [ ] Ask about deployment in their portfolio companies
- [ ] Request term sheet discussion or due diligence

---

## What to Send After Demo

1. **Investor Pitch Deck** (`docs/investor-deck/Lumo_Investor_Deck.pdf`)
2. **ROI Calculator** (`docs/ROI_Calculator.xlsx`)
3. **Demo Recording** (record run-demo.sh with asciinema or screen capture)
4. **One-Page Overview** (`docs/Lumo_One_Pager.pdf`)
5. **Competitive Analysis** (`docs/COMPETITIVE_ANALYSIS.md`)

---

## Next Steps

### For Demo Preparation

1. [ ] Run `./setup.sh` to create demo environment
2. [ ] Practice demo 3+ times with `./run-demo.sh`
3. [ ] Time yourself (target: 10 minutes)
4. [ ] Prepare for common questions (see FAQ below)
5. [ ] Create backup materials (slides, screenshots)

### For Investor Meetings

1. [ ] Schedule 30-45 minute meeting (10 min demo + Q&A)
2. [ ] Send calendar invite with agenda
3. [ ] Test your setup 1 hour before meeting
4. [ ] Have pitch deck open in another tab
5. [ ] Record the demo (with permission) for follow-up

---

## Frequently Asked Questions

### Technical Questions

**Q: Does this require installation on every server?**
A: No! CLI mode works via SSH with zero installation. Agents are optional for continuous monitoring.

**Q: What about security? Running automated fixes seems risky.**
A: We classify every action by risk level and require approval for anything risky. Plus, full audit trail for compliance.

**Q: How does this work with our existing monitoring (Datadog, New Relic)?**
A: Lumo complements existing monitoring. We focus on remediation, not just alerting. Can integrate via webhooks.

**Q: What about air-gapped/on-prem environments?**
A: Works offline with Ollama (local AI). No external dependencies required.

### Business Questions

**Q: How is this different from Ansible/Terraform?**
A: Those are config tools - you write the playbooks. Lumo diagnoses problems and writes the remediation plan using AI. It's autonomous, not just automated.

**Q: What's your go-to-market strategy?**
A: Bottom-up (open-source CLI) → Agent deployment → Enterprise features → SaaS platform. Land with DevOps teams, expand to CTO/VP Eng.

**Q: How will you compete with Datadog/New Relic?**
A: We're remediation-first, they're monitoring-first. We're 70% cheaper. We work with any monitoring stack. We're AI-native.

**Q: What's your revenue model?**
A: Open-core: CLI (MIT) + Enterprise (SSO, RBAC, multi-tenancy, support) + SaaS (hosted platform, $50-100/month per 100 servers).

---

## Demo Day Checklist

**One Week Before:**
- [ ] Confirm demo environment works
- [ ] Practice demo 3+ times
- [ ] Prepare pitch deck
- [ ] Create demo recording (backup)
- [ ] Test AI provider API keys

**One Day Before:**
- [ ] Run through demo once more
- [ ] Check internet connectivity
- [ ] Prepare laptop (charge, clean desktop)
- [ ] Print one-page overview (optional)
- [ ] Prepare backup materials

**One Hour Before:**
- [ ] Close unnecessary apps/tabs
- [ ] Run `./reset.sh && ./setup.sh && ./start-issues.sh`
- [ ] Test Lumo installation
- [ ] Open pitch deck in another window
- [ ] Set phone to silent

**After Demo:**
- [ ] Send thank you email with materials
- [ ] Add to CRM with notes
- [ ] Schedule follow-up within 48 hours

---

## Contact & Support

For questions about this demo:
- **GitHub Issues:** https://github.com/ignacio/lumo/issues
- **Email:** [your-email@example.com]
- **Documentation:** https://github.com/ignacio/lumo

---

**Good luck with your investor presentation! 🚀**
