<div align="center">
<img src=".github/images/lumo-logo.png" alt="Lumo Logo" width="300"/><br>
<h1>🔦 Lumo</h1>
<p><strong>Intelligent SRE/DevOps Automation Agent</strong></p>

[![CI](https://github.com/IgnacioPro/lumo/actions/workflows/ci.yml/badge.svg)](https://github.com/IgnacioPro/lumo/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Coverage](https://img.shields.io/badge/coverage-50.4%25-yellow.svg)](https://github.com/IgnacioPro/lumo)
[![Release](https://img.shields.io/badge/version-0.8.0-brightgreen.svg)](https://github.com/IgnacioPro/lumo/releases)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20BSD-lightgrey.svg)](https://github.com/IgnacioPro/lumo)

**AI-powered diagnostics + SSH automation + Kubernetes monitoring = Better SRE workflows**

[Features](#-features) • [Quick Start](#-quick-start) • [Installation](#-installation) • [Documentation](#-documentation) • [Contributing](#-contributing)

</div>

---

## 📖 Overview

**Lumo** is a modern, intelligent automation agent that connects to your infrastructure, performs comprehensive diagnostics, and leverages AI to help you understand and fix issues faster. Think of it as your AI-powered SRE companion.

### What Makes Lumo Different?

- 🤖 **Multi-Provider AI Analysis** - Claude, GPT-4, Gemini, Ollama, or OpenRouter
- 🔌 **Zero-Config Localhost** - Instant diagnostics without SSH overhead
- ☸️ **Native Kubernetes** - Direct API integration (no kubectl required)
- 🏗️ **Agent Architecture** - REST API + Agent daemon with hybrid push/pull model (Phases 7-8)
- 🔐 **Security-First** - Built-in security diagnostics and audit trails
- 🎯 **Token-Optimized** - TOON format reduces AI costs by 30-60%
- 🚀 **Production-Ready** - 50%+ test coverage, CI/CD, cross-platform

---

## ✨ Features

### Core Capabilities

<table>
<tr>
<td width="50%">

#### 🔍 Comprehensive Diagnostics

**6 Core Checks**
- CPU (load average, usage, cores)
- Memory (usage, pressure, top consumers)
- Disk (space, inodes, multi-filesystem)
- Processes (count, zombies, resource hogs)
- Services (systemd, init, launchd)
- Network (interfaces, connectivity, latency)

**4 Security Checks**
- Patch status (security updates)
- Open ports (unexpected listeners)
- SSH security (config audit)
- Auth failures (brute force detection)

</td>
<td width="50%">

#### 🤖 AI-Powered Analysis

**5 Provider Support**
- Anthropic Claude (Sonnet 4.5)
- OpenAI (GPT-4 Turbo, o1, o3)
- Google Gemini (2.0 Flash)
- Ollama (local models)
- OpenRouter (multi-model routing)

**Smart Features**
- Streaming responses
- Token usage tracking
- TOON format (30-60% cost savings)
- Reasoning effort control (OpenAI)

</td>
</tr>
</table>

### Platform Support

| Platform | SSH Diagnostics | Kubernetes | Proxmox VE |
|----------|----------------|------------|------------|
| **Linux** | ✅ Full support | ✅ Native client | ✅ Cluster monitoring |
| **macOS** | ✅ Full support | ✅ Native client | ❌ N/A |
| **BSD** | ✅ Full support | ✅ Native client | ❌ N/A |
| **Windows** | ⏳ Planned | ✅ Native client | ❌ N/A |

### Kubernetes Diagnostics

Native cluster monitoring using `k8s.io/client-go`:

- ✅ **8 Resource Types** - Nodes, Pods, Deployments, StatefulSets, DaemonSets, Services, PVCs, Events
- ✅ **Read-Only** - No modifications to your cluster
- ✅ **Namespace Filtering** - Check all or specific namespaces
- ✅ **RBAC-Aware** - Minimal permissions required (get/list)
- ✅ **Context Switching** - Work with multiple clusters

---

## 🚀 Quick Start

### Prerequisites

- Go 1.23+ (for building from source)
- SSH access to target servers (or use localhost)
- Optional: AI provider API key for analysis
- Optional: kubeconfig for Kubernetes diagnostics

### Installation

#### Option 1: Go Install (Recommended)

```bash
go install github.com/ignacio/lumo/cmd/lumo@latest
```

#### Option 2: Build from Source

```bash
git clone https://github.com/IgnacioPro/lumo.git
cd lumo
go build -o lumo ./cmd/lumo

# Optional: Install globally
sudo mv lumo /usr/local/bin/
```

#### Option 3: Download Binary (Coming Soon)

Pre-built binaries for Linux, macOS, and Windows will be available in [Releases](https://github.com/IgnacioPro/lumo/releases).

### First Run

```bash
# 1. Diagnose your local machine (no configuration needed)
lumo diagnose localhost

# 2. Connect to a remote server
lumo connect user@server.com

# 3. Run remote diagnostics
lumo diagnose user@server.com

# 4. Add AI-powered analysis
export LUMO_ANTHROPIC_API_KEY=sk-ant-...
lumo diagnose localhost --analyze
```

### Example Output

```
=== System Diagnostics for localhost ===

[✓] CPU Check (OK)
    Load Average: 1.23, 1.45, 1.67 (8 cores)
    Usage: 15.3%

[!] Memory Check (WARNING)
    RAM: 71.2% used (22.7/31.9 GB)
    Swap: 12.4% used (2.0/16.0 GB)
    Top Consumers:
      1. chrome (3.2 GB)
      2. docker (2.1 GB)
      3. postgres (1.8 GB)

[✓] Disk Check (OK)
    /: 45% used (225/500 GB)
    /home: 62% used (310/500 GB)

[✓] Network Check (OK)
    Interfaces: eth0 (UP), lo (UP)
    Connectivity: All targets reachable

Overall Status: WARNING (1 check needs attention)

🤖 AI Analysis:
Your system is healthy overall, but memory usage is approaching 75%.
Consider:
1. Closing unnecessary Chrome tabs (3.2 GB usage)
2. Reviewing Docker container memory limits
3. Tuning PostgreSQL shared_buffers if not needed

Estimated Impact: Moderate
Risk Level: Low
```

---

## 📚 Documentation

### Command Reference

#### `diagnose` - Run System Diagnostics

```bash
# Localhost (no SSH)
lumo diagnose localhost

# Remote server
lumo diagnose user@server.com

# Specific checks only
lumo diagnose localhost --checks cpu,memory,disk

# With AI analysis
lumo diagnose localhost --analyze

# Different AI provider
lumo diagnose localhost --analyze --ai-provider openai

# JSON output
lumo diagnose localhost --format json

# TOON format (AI-optimized, 30-60% token reduction)
lumo diagnose localhost --format toon

# Kubernetes cluster
lumo diagnose --checks kubernetes
```

#### `connect` - Establish SSH Connection

```bash
# Auto-discover authentication
lumo connect user@server.com

# Specific key file
lumo connect user@server.com --key ~/.ssh/id_ed25519

# Custom port
lumo connect user@server.com --port 2222

# Skip test commands
lumo connect user@server.com --test=false
```

### Configuration

#### Configuration File Locations (searched in order)

1. `--config` flag path
2. `./config.yaml`
3. `~/.lumo/config.yaml`

#### Quick Setup

```bash
# Copy example configuration
mkdir -p ~/.lumo
cp configs/config.example.yaml ~/.lumo/config.yaml

# Edit with your preferences
vim ~/.lumo/config.yaml
```

#### Environment Variables

```bash
# SSH Configuration
export LUMO_SSH_PORT=2222
export LUMO_SSH_STRICT_HOST_KEY_CHECKING=true

# AI Provider Selection
export LUMO_AI_PROVIDER=openai  # anthropic, openai, ollama, gemini, openrouter

# AI API Keys (Provider-Specific)
export LUMO_ANTHROPIC_API_KEY=sk-ant-...
export LUMO_OPENAI_API_KEY=sk-...
export LUMO_GEMINI_API_KEY=...
export LUMO_OPENROUTER_API_KEY=sk-or-...

# AI Settings
export LUMO_AI_TEMPERATURE=1.0
export LUMO_AI_MAX_TOKENS=4096
export LUMO_AI_REASONING_EFFORT=medium  # low, medium, high (OpenAI reasoning models)

# Logging
export LUMO_LOGGING_LEVEL=debug

# Kubernetes
export LUMO_DIAGNOSTICS_KUBERNETES_ENABLED=true
export LUMO_DIAGNOSTICS_KUBERNETES_CONTEXT=production
```

### SSH Authentication Methods

Lumo tries authentication methods in this order:

1. **SSH Agent** (most secure) → Uses `ssh-agent` with loaded keys
2. **Private Keys** → Auto-discovers: `id_ed25519`, `id_ecdsa`, `id_rsa`, `id_dsa`
3. **Password** → Interactive prompt (secure, not stored)
4. **Keyboard-Interactive** → Supports 2FA/MFA

```bash
# Ensure SSH agent is running
eval $(ssh-agent)
ssh-add ~/.ssh/id_ed25519

# Verify keys loaded
ssh-add -l
```

### Kubernetes Setup

```bash
# 1. Enable Kubernetes diagnostics
export LUMO_DIAGNOSTICS_KUBERNETES_ENABLED=true

# 2. (Optional) Use specific kubeconfig
export LUMO_DIAGNOSTICS_KUBERNETES_KUBECONFIG_PATH=/path/to/kubeconfig

# 3. (Optional) Use specific context
export LUMO_DIAGNOSTICS_KUBERNETES_CONTEXT=production

# 4. Run diagnostics
lumo diagnose --checks kubernetes

# 5. With AI analysis
lumo diagnose --checks kubernetes --analyze
```

#### RBAC Requirements

Lumo needs minimal read-only permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: lumo-reader
rules:
- apiGroups: [""]
  resources: ["nodes", "pods", "services", "persistentvolumeclaims", "events"]
  verbs: ["get", "list"]
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets"]
  verbs: ["get", "list"]
```

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    CLI Interface                        │
│         (connect, diagnose, fix, report, serve)         │
└────────────────────────┬────────────────────────────────┘
                         │
                    ┌────▼────┐
                    │ Config  │  YAML + Environment Variables
                    └────┬────┘
                         │
        ┌────────────────┼────────────────┐
        │                │                │
   ┌────▼────┐    ┌──────▼──────┐   ┌────▼────────┐
   │  Local  │    │     SSH     │   │ Kubernetes  │
   │Executor │    │  Executor   │   │   Client    │
   └────┬────┘    └──────┬──────┘   └──────┬──────┘
        │                │                 │
        └────────────────┼─────────────────┘
                         │
        ┌────────────────▼────────────────┐
        │    Diagnostics Runner (12)       │
        │  • 6 Core (CPU, Mem, Disk...)   │
        │  • 4 Security (Patches, Ports...)│
        │  • 2 Specialized (K8s, Proxmox) │
        └────────────────┬─────────────────┘
                         │
                    ┌────▼────┐
                    │Formatters│  Text, JSON, TOON
                    └────┬────┘
                         │
                    ┌────▼────────┐
                    │AI Analysis  │  5 Providers
                    │ (Optional)  │  Streaming
                    └─────────────┘
```

---

## 🧪 Testing

**Overall Coverage**: 50.4% (11,059 lines of test code)

### Coverage by Package

| Package | Coverage | Status |
|---------|----------|--------|
| `internal/diagnostics/formatters` | 98.1% | ✅ Excellent |
| `internal/diagnostics` | 87.6% | ✅ Excellent |
| `internal/config` | 68.8% | ✅ Good |
| `internal/diagnostics/checkers` | 61.4% | ✅ Good |
| `cmd/lumo` | 61.6% | ✅ Good |
| `internal/ssh` | 30.5% | ⚠️ In Progress |
| `internal/ai` | 27.1% | ⚠️ In Progress |

### Run Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Specific package with verbose output
go test -v ./internal/diagnostics

# HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## 🗺️ Roadmap

### ✅ Completed Phases

- [x] **Phase 1-2**: Foundation & SSH (CLI framework, 4 auth methods, health monitoring)
- [x] **Phase 3**: Core Diagnostics (6 checkers: CPU, Memory, Disk, Process, Service, Network)
- [x] **Phase 4**: AI Integration (5 providers: Claude, GPT-4, Gemini, Ollama, OpenRouter)
- [x] **Phase 5**: Security & Specialized Diagnostics (4 security checkers + Kubernetes + Proxmox)
- [x] **Phase 6**: Auto-Remediation (Human-in-the-loop approval, risk classification, audit logging)
- [x] **Phase 7**: API Server Foundation (REST API, PostgreSQL, Redis, Agent registration)
- [x] **Phase 8**: Agent Daemon (Scheduled diagnostics, API reporter, offline mode, health/metrics) ✨ **NEW**

### 🚧 In Progress

- [ ] **Phase 9**: Kubernetes Deployment (DaemonSet, Deployment, Helm charts)
- [ ] **Phase 10**: VM Deployment (systemd units, RPM/DEB packages)
- [ ] **Phase 11**: Messaging Integration (NATS, Kafka, RabbitMQ, Redis)

### 🔮 Future Phases

- [ ] **Phase 12**: Security Hardening (mTLS, JWT, cert rotation, penetration testing)
- [ ] **Phase 13**: Production Readiness (Performance, monitoring, dashboards, load testing)
- [ ] **Phase 14**: Advanced Reporting (Markdown, HTML, PDF, trends, forecasting)
- [ ] **Phase 15**: Advanced Features (Multi-cluster, ML-based anomaly detection)

---

## 🤝 Contributing

We welcome contributions! Here's how you can help:

### Ways to Contribute

- 🐛 **Report bugs** - Open an issue with reproduction steps
- 💡 **Suggest features** - Share your ideas for improvements
- 📖 **Improve docs** - Fix typos, add examples, clarify usage
- 🧪 **Write tests** - Help us reach 80% coverage
- 🔧 **Submit PRs** - Fix bugs or implement features

### Development Setup

```bash
# 1. Fork and clone
git clone https://github.com/YOUR_USERNAME/lumo.git
cd lumo

# 2. Create a branch
git checkout -b feature/my-feature

# 3. Make changes and test
go test ./...
go vet ./...
go fmt ./...

# 4. Commit and push
git add .
git commit -m "feat: add amazing feature"
git push origin feature/my-feature

# 5. Open a Pull Request
```

### Code Standards

- ✅ Write tests for new functionality
- ✅ Follow existing code patterns
- ✅ Update documentation
- ✅ Pass CI checks (format, vet, tests, build)
- ✅ Use semantic commit messages

### Development Resources

- **Main Guide**: [CLAUDE.md](CLAUDE.md) - Comprehensive development documentation
- **Examples**: [DEVELOPMENT.md](DEVELOPMENT.md) - Tutorials and examples
- **Reports**: [REPORTS/](REPORTS/) - Implementation details and audits

---

## 🔐 Security

### Security Practices

- ✅ **Host Key Verification** - Enabled by default (`StrictHostKeyChecking: true`)
- ✅ **No Password Flags** - Passwords only via secure prompts
- ✅ **API Keys** - Environment variables only, never in config files
- ✅ **Command Injection Prevention** - Path sanitization and validation
- ✅ **File Permissions** - Validates SSH key permissions (600/400)

### Reporting Security Issues

Please report security vulnerabilities to the maintainers privately via GitHub Security Advisories.

### Security Audit

Last audit: 2025-11-16 - All CRITICAL issues resolved
- See [REPORTS/security-audit-2025-11-15.md](REPORTS/security-audit-2025-11-15.md)

---

## 📜 License

MIT License - See [LICENSE](LICENSE) file for details.

Copyright (c) 2025 Lumo Contributors

---

## 🙏 Acknowledgments

Built with these amazing technologies:

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [Logrus](https://github.com/sirupsen/logrus) - Structured logging
- [k8s.io/client-go](https://github.com/kubernetes/client-go) - Kubernetes client
- [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh) - SSH implementation

AI Providers:
- [Anthropic Claude](https://www.anthropic.com/) - Leading AI safety and research
- [OpenAI](https://openai.com/) - GPT-4 and advanced reasoning models
- [Google Gemini](https://deepmind.google/technologies/gemini/) - Multimodal AI
- [Ollama](https://ollama.ai/) - Run LLMs locally
- [OpenRouter](https://openrouter.ai/) - Multi-provider AI routing

---

## 📞 Support

- 📖 **Documentation**: [CLAUDE.md](CLAUDE.md) | [DEVELOPMENT.md](DEVELOPMENT.md)
- 🐛 **Issues**: [GitHub Issues](https://github.com/IgnacioPro/lumo/issues)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/IgnacioPro/lumo/discussions)
- ⭐ **Star us** on GitHub if you find Lumo useful!

---

<div align="center">

**[⬆ Back to Top](#-lumo)**

Made with ❤️ by the Lumo community

</div>
