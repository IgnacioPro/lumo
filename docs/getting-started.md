# Getting Started with Lumo

Welcome to Lumo! This guide will help you get up and running in just a few minutes.

## What is Lumo?

Lumo is an intelligent SRE/DevOps automation platform that helps you:
- 🔍 **Diagnose** system issues (CPU, memory, disk, processes, services, network)
- 🤖 **Analyze** problems with AI (Claude, GPT, Gemini, Ollama, OpenRouter)
- 🔧 **Fix** issues automatically with human-in-the-loop approval
- 📊 **Monitor** infrastructure via CLI, API, or deployed agents
- 🔔 **Alert** teams via Slack, Telegram, Discord, Teams, or Email

## Quick Start (5 minutes)

### 1. Install Lumo

Choose your preferred installation method:

#### Option A: Quick Install Script (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/ignacio/lumo/main/scripts/quickstart.sh | bash
```

#### Option B: Download Pre-built Binary

Visit the [releases page](https://github.com/ignacio/lumo/releases) and download the binary for your platform:

**Linux:**
```bash
# AMD64
wget https://github.com/ignacio/lumo/releases/latest/download/lumo-linux-amd64
chmod +x lumo-linux-amd64
sudo mv lumo-linux-amd64 /usr/local/bin/lumo

# ARM64 (Raspberry Pi, Apple Silicon)
wget https://github.com/ignacio/lumo/releases/latest/download/lumo-linux-arm64
chmod +x lumo-linux-arm64
sudo mv lumo-linux-arm64 /usr/local/bin/lumo
```

**macOS:**
```bash
# Intel
wget https://github.com/ignacio/lumo/releases/latest/download/lumo-darwin-amd64
chmod +x lumo-darwin-amd64
sudo mv lumo-darwin-amd64 /usr/local/bin/lumo

# Apple Silicon (M1/M2/M3)
wget https://github.com/ignacio/lumo/releases/latest/download/lumo-darwin-arm64
chmod +x lumo-darwin-arm64
sudo mv lumo-darwin-arm64 /usr/local/bin/lumo
```

#### Option C: Homebrew (Coming Soon)

```bash
brew install ignacio/lumo/lumo
```

#### Option D: Build from Source

```bash
git clone https://github.com/ignacio/lumo.git
cd lumo
go build -o lumo ./cmd/lumo
sudo mv lumo /usr/local/bin/
```

### 2. Verify Installation

```bash
lumo version
```

You should see output like:
```
Lumo v0.9.1
```

### 3. Run Setup Wizard

```bash
lumo init
```

This interactive wizard will guide you through:
- Choosing an AI provider (Anthropic, OpenAI, Gemini, Ollama, OpenRouter)
- Setting up your API key
- Configuring basic settings
- Creating your config file

**Example session:**
```
╦  ╦ ╦╔╦╗╔═╗
║  ║ ║║║║║ ║
╩═╝╚═╝╩ ╩╚═╝

Welcome to Lumo Setup Wizard!
==============================

? Choose an AI provider: anthropic (Claude - Recommended)
? Enter your ANTHROPIC API key: ****************
? Choose logging level: info
? Choose default output format: text (Human-readable)
? Enable SSH for remote diagnostics? Yes
? Configure as agent mode? No
? Enable notifications? No

✅ Configuration saved successfully!
   Location: /home/user/.lumo/config.yaml
```

### 4. Set Your API Key (Important!)

The wizard will show you the command to set your API key securely:

```bash
# For Anthropic (Claude)
export LUMO_ANTHROPIC_API_KEY='your-api-key-here'

# Or for OpenAI
export LUMO_OPENAI_API_KEY='your-api-key-here'
```

**Make it permanent** by adding to your shell config:

```bash
# For bash
echo 'export LUMO_ANTHROPIC_API_KEY=your-key' >> ~/.bashrc
source ~/.bashrc

# For zsh
echo 'export LUMO_ANTHROPIC_API_KEY=your-key' >> ~/.zshrc
source ~/.zshrc
```

### 5. Your First Diagnostic

Run a diagnostic on your local machine:

```bash
lumo diagnose localhost
```

You'll see output like:
```
[INFO] Diagnosing localhost
[INFO] Running CPU check...
[INFO] Running Memory check...
[INFO] Running Disk check...
...

=== Diagnostic Report ===

CPU:
  Load Average: 1.2, 1.5, 1.3
  Status: ✓ OK

Memory:
  Total: 16 GB
  Used: 8.2 GB (51%)
  Status: ✓ OK

Disk:
  /dev/sda1: 45% used (120 GB / 250 GB)
  Status: ✓ OK
```

### 6. Try AI Analysis

Add the `--analyze` flag to get AI-powered insights:

```bash
lumo diagnose localhost --analyze
```

The AI will:
- Interpret the diagnostic results
- Identify potential issues
- Suggest optimizations
- Provide actionable recommendations

## What's Next?

### Basic Usage

#### Diagnose a Remote Server

```bash
# Via SSH (password)
lumo diagnose user@server.example.com

# Via SSH (key-based auth)
lumo diagnose user@server.example.com --key ~/.ssh/id_rsa

# Via SSH agent
lumo diagnose user@server.example.com --use-agent
```

#### Get Specific Checks

```bash
# Only CPU and memory
lumo diagnose localhost --checks cpu,memory

# Only security checks
lumo diagnose localhost --checks patch,ssh-security,auth-failures
```

#### Different Output Formats

```bash
# JSON (for scripts/automation)
lumo diagnose localhost --format json

# TOON (AI-optimized, 30-60% token reduction)
lumo diagnose localhost --format toon
```

#### Auto-Remediation

```bash
# Dry-run (see what would be fixed)
lumo fix localhost --dry-run

# Interactive approval
lumo fix localhost

# Auto-approve safe actions only
lumo fix localhost --auto-approve-safe
```

### Common Use Cases

#### 1. Check Server Health

```bash
lumo diagnose production-server --analyze
```

#### 2. Investigate High CPU

```bash
lumo diagnose server --checks cpu,process --analyze
```

#### 3. Check Security Posture

```bash
lumo diagnose server --checks patch,ssh-security,ports,auth-failures --analyze
```

#### 4. Diagnose Kubernetes Cluster

```bash
# Set kubeconfig
export KUBECONFIG=~/.kube/config

# Run Kubernetes diagnostics
lumo diagnose localhost --checks kubernetes --analyze
```

#### 5. Check Proxmox Cluster

```bash
# Configure Proxmox credentials
export LUMO_PROXMOX_HOST=proxmox.example.com
export LUMO_PROXMOX_USER=root@pam
export LUMO_PROXMOX_PASSWORD=your-password

# Run Proxmox diagnostics
lumo diagnose localhost --checks proxmox --analyze
```

### Advanced Features

#### Run API Server (for Agent Mode)

```bash
# Start PostgreSQL and Redis (development)
docker-compose up -d

# Start API server
lumo serve --port 8080
```

See [Agent Deployment Guide](agent-deployment.md) for production setup.

#### Deploy Agents

**Kubernetes:**
```bash
# DaemonSet (per-node monitoring)
kubectl apply -f deployments/kubernetes/base/daemonset.yaml

# Or use Helm
helm install lumo-agent deployments/kubernetes/helm/lumo-agent
```

**VMs/Bare Metal:**
```bash
# Install systemd service
sudo ./deployments/systemd/install.sh

# Check status
sudo systemctl status lumo-agent
```

See [Deployment Documentation](../deployments/) for details.

## Configuration

### Config File Location

Lumo searches for config in this order:
1. `--config` flag
2. `./config.yaml` (current directory)
3. `~/.lumo/config.yaml` (user config)

### Essential Configuration

The `lumo init` wizard creates a minimal config. Here's what it looks like:

```yaml
# ~/.lumo/config.yaml

ai:
  provider: anthropic
  model: claude-3-5-sonnet-20241022

logging:
  level: info
  format: text

diagnostics:
  output_format: text
  enabled_checks:
    - cpu
    - memory
    - disk
    - process
    - service
    - network
```

### Configuration via Environment Variables

You can override any setting with environment variables:

```bash
# AI Provider
export LUMO_AI_PROVIDER=anthropic
export LUMO_ANTHROPIC_API_KEY=sk-ant-...

# Logging
export LUMO_LOG_LEVEL=debug

# Output
export LUMO_DIAGNOSTICS_OUTPUT_FORMAT=json
```

### Full Configuration Options

For all available options, see [config.example.yaml](../configs/config.example.yaml).

## Getting Help

### Command Help

```bash
# General help
lumo --help

# Command-specific help
lumo diagnose --help
lumo fix --help
lumo serve --help
```

### Examples

```bash
# View usage examples
lumo examples
```

### Documentation

- **GitHub**: https://github.com/ignacio/lumo
- **Issues**: https://github.com/ignacio/lumo/issues
- **Discussions**: https://github.com/ignacio/lumo/discussions

### Troubleshooting

#### "AI provider not configured"

Set your AI provider and API key:
```bash
export LUMO_AI_PROVIDER=anthropic
export LUMO_ANTHROPIC_API_KEY=your-key
```

#### "SSH connection failed"

Check your SSH credentials:
```bash
# Test SSH manually
ssh user@server.example.com

# Use verbose mode for debugging
lumo diagnose user@server --verbose
```

#### "Command not found: lumo"

Add Lumo to your PATH:
```bash
# Check where lumo is installed
which lumo

# If not in PATH, add it
export PATH="$PATH:$HOME/.local/bin"

# Make it permanent
echo 'export PATH="$PATH:$HOME/.local/bin"' >> ~/.bashrc
```

#### "Permission denied"

Lumo may need certain capabilities for diagnostics:
```bash
# Run with sudo (not recommended for regular use)
sudo lumo diagnose localhost

# Or grant specific capabilities
sudo setcap cap_net_raw,cap_sys_ptrace+ep $(which lumo)
```

## AI Provider Setup

### Anthropic (Claude) - Recommended

1. Sign up at https://console.anthropic.com/
2. Create an API key
3. Set it in your environment:
   ```bash
   export LUMO_ANTHROPIC_API_KEY=sk-ant-...
   ```

**Models:**
- `claude-3-5-sonnet-20241022` (default, best balance)
- `claude-3-5-haiku-20241022` (fast, cheaper)
- `claude-3-opus-20240229` (most capable)

### OpenAI (GPT)

1. Sign up at https://platform.openai.com/
2. Create an API key at https://platform.openai.com/api-keys
3. Set it in your environment:
   ```bash
   export LUMO_OPENAI_API_KEY=sk-...
   ```

**Models:**
- `gpt-4o` (default)
- `gpt-4o-mini` (faster, cheaper)
- `o1` (reasoning model)

### Google Gemini

1. Get an API key from https://makersuite.google.com/app/apikey
2. Set it in your environment:
   ```bash
   export LUMO_GEMINI_API_KEY=...
   ```

**Models:**
- `gemini-2.0-flash-exp` (default)
- `gemini-1.5-pro`

### Ollama (Local/Self-Hosted)

1. Install Ollama: https://ollama.ai/
2. Pull a model:
   ```bash
   ollama pull llama3.2
   ```
3. Configure Lumo:
   ```bash
   export LUMO_AI_PROVIDER=ollama
   export LUMO_OLLAMA_HOST=http://localhost:11434
   ```

**Models:**
- `llama3.2` (default)
- `mistral`
- `codellama`

### OpenRouter (Multi-Model Access)

1. Sign up at https://openrouter.ai/
2. Create an API key at https://openrouter.ai/keys
3. Set it in your environment:
   ```bash
   export LUMO_OPENROUTER_API_KEY=sk-or-...
   ```

Gives you access to 100+ models from multiple providers.

## Next Steps

Now that you're up and running, explore:

1. **[Configuration Guide](configuration.md)** - Detailed configuration options
2. **[Examples](examples/)** - Real-world usage scenarios
3. **[Agent Deployment](agent-deployment.md)** - Deploy agents to your infrastructure
4. **[API Reference](../api/README.md)** - Use Lumo programmatically
5. **[Development Guide](../DEVELOPMENT.md)** - Contribute to Lumo

## Quick Reference

### Most Useful Commands

```bash
# Setup
lumo init                                    # Interactive setup
lumo version                                 # Check version
lumo --help                                  # Get help

# Diagnostics
lumo diagnose localhost                      # Local diagnostic
lumo diagnose localhost --analyze            # With AI analysis
lumo diagnose user@server                    # Remote diagnostic
lumo diagnose server --checks cpu,memory     # Specific checks
lumo diagnose server --format json           # JSON output

# Remediation
lumo fix localhost --dry-run                 # Preview fixes
lumo fix localhost                           # Interactive fixes
lumo fix localhost --auto-approve-safe       # Auto-approve safe fixes

# Agent Mode
lumo serve                                   # Start API server
lumo-agent                                   # Start agent daemon

# Examples & Help
lumo examples                                # View examples
lumo diagnose --help                         # Command help
```

### Available Checks

**Core Checks:**
- `cpu` - CPU usage, load average
- `memory` - Memory usage, swap, pressure
- `disk` - Disk usage, I/O stats
- `process` - Running processes, resource usage
- `service` - Systemd services status
- `network` - Network interfaces, connectivity

**Security Checks:**
- `patch` - Patch status, updates needed
- `ports` - Open ports, listening services
- `ssh-security` - SSH configuration security
- `auth-failures` - Failed authentication attempts

**Specialized Checks:**
- `kubernetes` - K8s cluster health (pods, nodes, deployments)
- `proxmox` - Proxmox VE cluster status

Use `--checks` to select specific checks:
```bash
lumo diagnose localhost --checks cpu,memory,disk
```

---

**Happy Diagnosing! 🚀**

If you have questions or run into issues, please:
- Check the [FAQ](faq.md)
- Search [existing issues](https://github.com/ignacio/lumo/issues)
- Ask in [Discussions](https://github.com/ignacio/lumo/discussions)
- [Open an issue](https://github.com/ignacio/lumo/issues/new)
