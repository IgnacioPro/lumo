# Lumo Agent - VM/Systemd Deployment

Complete guide for deploying Lumo Agent as a systemd service on virtual machines and bare-metal servers.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Installation Methods](#installation-methods)
- [Configuration](#configuration)
- [Operation](#operation)
- [Package Building](#package-building)
- [Security](#security)
- [Troubleshooting](#troubleshooting)
- [Uninstallation](#uninstallation)

---

## Overview

Lumo Agent runs as a systemd daemon on Linux systems, providing:

- **System Diagnostics**: CPU, memory, disk, processes, services, network monitoring
- **Security Checks**: Patch status, open ports, SSH security, authentication failures
- **Specialized Monitoring**: Kubernetes and Proxmox support
- **AI Analysis**: Integration with multiple AI providers (Anthropic, OpenAI, Ollama, Gemini, OpenRouter)
- **Auto-Remediation**: Human-in-the-loop approval system
- **Multiple Modes**: Scheduled, on-demand, continuous, and hybrid operation
- **Observability**: Prometheus metrics and health endpoints

### Architecture

```
┌─────────────────────────────────────────────┐
│         Lumo Agent (systemd daemon)         │
│                                             │
│  ┌─────────────┐  ┌────────────────────┐  │
│  │  Scheduler  │  │  Diagnostic Engine │  │
│  │  (cron)     │  │  (12+ checkers)    │  │
│  └─────────────┘  └────────────────────┘  │
│                                             │
│  ┌─────────────┐  ┌────────────────────┐  │
│  │  Reporter   │  │  Health/Metrics    │  │
│  │  (API)      │  │  (:8080/:9090)     │  │
│  └─────────────┘  └────────────────────┘  │
└─────────────────────────────────────────────┘
           │                    │
           ▼                    ▼
    Lumo API Server      Prometheus/Grafana
```

### System Requirements

- **OS**: Linux (kernel 3.10+)
  - RHEL/CentOS/Fedora 7+
  - Ubuntu 16.04+ / Debian 8+
  - Other systemd-based distributions
- **Memory**: 64-128 MB baseline, 256 MB peak
- **CPU**: <5% average, 50% peak during diagnostics
- **Disk**: 100 MB binary + 1 GB for cache/logs
- **Network**: Outbound HTTPS (443) to Lumo API server

---

## Quick Start

### Method 1: Installation Script (Recommended)

```bash
# Download the latest release
wget https://github.com/ignacio/lumo/releases/latest/download/lumo-agent-linux-amd64.tar.gz

# Extract
tar xzf lumo-agent-linux-amd64.tar.gz
cd lumo-agent-*/

# Run installation script
sudo ./install.sh

# Edit configuration
sudo nano /etc/lumo-agent/config.yaml

# Start service
sudo systemctl start lumo-agent
sudo systemctl enable lumo-agent

# Check status
sudo systemctl status lumo-agent
```

### Method 2: Package Manager

**RPM (RHEL/CentOS/Fedora):**
```bash
# Download package
wget https://github.com/ignacio/lumo/releases/latest/download/lumo-agent-1.0.0-1.x86_64.rpm

# Install
sudo rpm -ivh lumo-agent-1.0.0-1.x86_64.rpm

# Or with yum
sudo yum localinstall lumo-agent-1.0.0-1.x86_64.rpm

# Start service
sudo systemctl start lumo-agent
sudo systemctl enable lumo-agent
```

**DEB (Ubuntu/Debian):**
```bash
# Download package
wget https://github.com/ignacio/lumo/releases/latest/download/lumo-agent_1.0.0-1_amd64.deb

# Install
sudo dpkg -i lumo-agent_1.0.0-1_amd64.deb

# Install dependencies if needed
sudo apt-get install -f

# Start service
sudo systemctl start lumo-agent
sudo systemctl enable lumo-agent
```

---

## Installation Methods

### 1. Installation Script

The installation script (`install.sh`) provides the most flexible installation method.

**Options:**
```bash
sudo ./install.sh [OPTIONS]

Options:
  --binary PATH       Path to lumo-agent binary (default: auto-detect)
  --config PATH       Path to config file (optional)
  --user USER         Service user (default: lumo-agent)
  --group GROUP       Service group (default: lumo-agent)
  --skip-systemd      Skip systemd service installation
  --help              Show help message
```

**Examples:**
```bash
# Standard installation
sudo ./install.sh

# Custom binary location
sudo ./install.sh --binary /path/to/lumo-agent

# With custom config
sudo ./install.sh --config /path/to/config.yaml

# Custom service user
sudo ./install.sh --user myuser --group mygroup
```

**What it does:**
1. Creates system user and group (`lumo-agent`)
2. Creates directories (`/etc/lumo-agent`, `/var/lib/lumo-agent`, `/var/log/lumo-agent`)
3. Installs binary to `/usr/local/bin/lumo-agent`
4. Installs systemd service
5. Sets proper permissions and ownership

### 2. RPM Package

**Build from source:**
```bash
cd deployments/systemd/packaging/rpm
./build-rpm.sh
```

**Install:**
```bash
sudo rpm -ivh build/RPMS/x86_64/lumo-agent-*.rpm
```

**Features:**
- Automatic user/group creation
- Systemd integration
- Config file preservation on upgrade
- Clean uninstallation

### 3. DEB Package

**Build from source:**
```bash
cd deployments/systemd/packaging/deb
./build-deb.sh
```

**Install:**
```bash
sudo dpkg -i build/lumo-agent_*.deb
sudo apt-get install -f  # Install dependencies
```

### 4. Manual Installation

```bash
# 1. Create user and group
sudo groupadd --system lumo-agent
sudo useradd --system --gid lumo-agent --no-create-home \
    --home-dir /var/lib/lumo-agent --shell /usr/sbin/nologin \
    lumo-agent

# 2. Create directories
sudo mkdir -p /etc/lumo-agent /var/lib/lumo-agent /var/log/lumo-agent

# 3. Install binary
sudo cp lumo-agent /usr/local/bin/
sudo chmod 755 /usr/local/bin/lumo-agent

# 4. Install systemd service
sudo cp lumo-agent.service /etc/systemd/system/
sudo systemctl daemon-reload

# 5. Set permissions
sudo chown -R lumo-agent:lumo-agent /var/lib/lumo-agent /var/log/lumo-agent
sudo chown -R root:lumo-agent /etc/lumo-agent
sudo chmod 750 /var/lib/lumo-agent /var/log/lumo-agent /etc/lumo-agent

# 6. Create config
sudo cp config.example.yaml /etc/lumo-agent/config.yaml
sudo chmod 640 /etc/lumo-agent/config.yaml

# 7. Enable and start
sudo systemctl enable lumo-agent
sudo systemctl start lumo-agent
```

---

## Configuration

### Configuration File

Location: `/etc/lumo-agent/config.yaml`

```yaml
# Lumo Agent Configuration
agent:
  mode: hybrid                  # scheduled|on-demand|continuous|hybrid
  schedule: "*/5 * * * *"       # Cron expression for scheduled mode
  api_endpoint: ""              # Lumo API server URL
  token: ""                     # Authentication token (prefer env var)
  tls_enabled: true
  enabled_checks:
    - cpu
    - memory
    - disk
    - process
    - service
    - network
  report_format: toon           # text|json|toon (30-60% token reduction)
  offline_mode: true            # Continue if API unavailable
  health_check_port: 8080
  metrics_port: 9090

logging:
  level: info                   # debug|info|warn|error
  format: json                  # text|json

diagnostics:
  timeout: 5m
  max_concurrent: 5
```

### Environment Variables

Set environment variables in systemd service:

```bash
# Create override file
sudo systemctl edit lumo-agent
```

Add:
```ini
[Service]
Environment="LUMO_AGENT_API_ENDPOINT=https://lumo-api.example.com"
Environment="LUMO_AGENT_TOKEN=your-jwt-token-here"
Environment="LUMO_ANTHROPIC_API_KEY=sk-ant-..."
```

Or set in `/etc/environment`:
```bash
LUMO_AGENT_API_ENDPOINT=https://lumo-api.example.com
LUMO_AGENT_TOKEN=your-jwt-token-here
```

### Operational Modes

**Scheduled Mode** (periodic diagnostics):
```yaml
agent:
  mode: scheduled
  schedule: "*/5 * * * *"  # Every 5 minutes
```

**On-Demand Mode** (API-triggered only):
```yaml
agent:
  mode: on-demand
  api_endpoint: https://lumo-api.example.com
```

**Continuous Mode** (real-time monitoring):
```yaml
agent:
  mode: continuous
  # High-frequency checks, use with caution
```

**Hybrid Mode** (recommended - combines all):
```yaml
agent:
  mode: hybrid
  schedule: "*/5 * * * *"
  api_endpoint: https://lumo-api.example.com
```

---

## Operation

### Service Management

```bash
# Start service
sudo systemctl start lumo-agent

# Stop service
sudo systemctl stop lumo-agent

# Restart service
sudo systemctl restart lumo-agent

# Enable auto-start on boot
sudo systemctl enable lumo-agent

# Disable auto-start
sudo systemctl disable lumo-agent

# Check status
sudo systemctl status lumo-agent

# View logs
sudo journalctl -u lumo-agent -f

# View logs since boot
sudo journalctl -u lumo-agent -b

# View last 100 lines
sudo journalctl -u lumo-agent -n 100
```

### Health Checks

```bash
# Health endpoint
curl http://localhost:8080/health

# Ready endpoint
curl http://localhost:8080/ready

# Live endpoint
curl http://localhost:8080/live

# Detailed status
curl http://localhost:8080/status
```

### Metrics

Prometheus metrics available at `http://localhost:9090/metrics`:

```bash
# View metrics
curl http://localhost:9090/metrics

# Key metrics:
# - lumo_agent_diagnostics_runs_total
# - lumo_agent_diagnostics_duration_seconds
# - lumo_agent_diagnostics_errors_total
# - lumo_agent_api_requests_total
# - lumo_agent_cache_hits_total
# - lumo_agent_cache_size_bytes
```

### Logs

```bash
# Follow logs in real-time
sudo journalctl -u lumo-agent -f

# Filter by priority
sudo journalctl -u lumo-agent -p err  # Errors only
sudo journalctl -u lumo-agent -p warning  # Warnings and above

# Since specific time
sudo journalctl -u lumo-agent --since "1 hour ago"
sudo journalctl -u lumo-agent --since "2024-11-18 10:00:00"

# Export to file
sudo journalctl -u lumo-agent > /tmp/lumo-agent.log
```

### Configuration Reload

```bash
# Reload configuration without restarting
sudo systemctl reload lumo-agent

# Or send HUP signal
sudo systemctl kill -s HUP lumo-agent
```

---

## Package Building

### Cross-Platform Binaries

Build binaries for all supported platforms:

```bash
cd deployments/systemd
./build.sh

# Build specific targets
./build.sh --targets linux/amd64,linux/arm64

# Custom version
./build.sh --version 1.2.0

# Skip package building
./build.sh --skip-packages

# Clean build
./build.sh --clean
```

**Supported platforms:**
- linux/amd64
- linux/arm64
- linux/arm
- darwin/amd64 (macOS Intel)
- darwin/arm64 (macOS Apple Silicon)
- windows/amd64

### RPM Package

```bash
cd deployments/systemd/packaging/rpm
./build-rpm.sh

# Custom version
VERSION=1.2.0 RELEASE=2 ./build-rpm.sh

# Output: build/RPMS/x86_64/lumo-agent-*.rpm
```

### DEB Package

```bash
cd deployments/systemd/packaging/deb
./build-deb.sh

# Custom version
VERSION=1.2.0 ./build-deb.sh

# Output: build/lumo-agent_*.deb
```

---

## Security

### Service Hardening

The systemd service includes comprehensive security hardening:

**Filesystem Protection:**
- `ProtectSystem=strict` - Read-only filesystem
- `ProtectHome=true` - No access to home directories
- `PrivateTmp=true` - Private /tmp namespace
- `ReadWritePaths` - Limited to `/var/lib/lumo-agent` and `/var/log/lumo-agent`

**Process Isolation:**
- `PrivateDevices=false` - Access to devices for diagnostics
- `ProtectKernelTunables=true` - No kernel parameter changes
- `ProtectKernelModules=true` - No module loading
- `NoNewPrivileges=true` - Prevents privilege escalation

**Capabilities:**
Minimal capabilities for diagnostics:
- `CAP_NET_RAW` - Network diagnostics (ping, traceroute)
- `CAP_SYS_PTRACE` - Process monitoring
- `CAP_DAC_READ_SEARCH` - Read system files

**System Call Filtering:**
- Whitelist-based syscall filter
- Blocks privileged and dangerous syscalls

### File Permissions

```
/usr/local/bin/lumo-agent           755  root:root
/etc/lumo-agent/                    750  root:lumo-agent
/etc/lumo-agent/config.yaml         640  root:lumo-agent
/var/lib/lumo-agent/                750  lumo-agent:lumo-agent
/var/log/lumo-agent/                750  lumo-agent:lumo-agent
```

### TLS Configuration

For production deployments, enable TLS:

```yaml
agent:
  tls_enabled: true
  api_endpoint: https://lumo-api.example.com
```

Configure certificates:
```bash
# System certificates (recommended)
sudo update-ca-certificates

# Custom CA
sudo cp ca.crt /etc/lumo-agent/
```

---

## Troubleshooting

### Service won't start

```bash
# Check service status
sudo systemctl status lumo-agent

# Check logs
sudo journalctl -u lumo-agent -n 50

# Verify binary
/usr/local/bin/lumo-agent version

# Check config
sudo lumo-agent --config /etc/lumo-agent/config.yaml --dry-run
```

### Permission issues

```bash
# Reset permissions
sudo chown -R lumo-agent:lumo-agent /var/lib/lumo-agent /var/log/lumo-agent
sudo chown -R root:lumo-agent /etc/lumo-agent
sudo chmod 750 /var/lib/lumo-agent /var/log/lumo-agent /etc/lumo-agent
sudo chmod 640 /etc/lumo-agent/config.yaml
```

### High CPU/Memory usage

```bash
# Check resource usage
systemctl status lumo-agent

# Adjust limits in service file
sudo systemctl edit lumo-agent

[Service]
CPUQuota=25%
MemoryMax=256M
```

### Connection issues

```bash
# Test API endpoint
curl -v https://lumo-api.example.com/health

# Check firewall
sudo firewall-cmd --list-all

# Check TLS certificates
openssl s_client -connect lumo-api.example.com:443
```

### Offline mode

If API is unavailable, agent continues with local caching:

```yaml
agent:
  offline_mode: true  # Enable offline operation
```

```bash
# Check cache status
curl http://localhost:8080/status | jq '.cache'
```

---

## Uninstallation

### Using uninstall script

```bash
cd /path/to/lumo-agent
sudo ./uninstall.sh

# Keep configuration
sudo ./uninstall.sh --keep-config

# Complete removal (purge everything)
sudo ./uninstall.sh --purge --yes
```

### Manual removal

```bash
# 1. Stop and disable service
sudo systemctl stop lumo-agent
sudo systemctl disable lumo-agent

# 2. Remove service file
sudo rm /etc/systemd/system/lumo-agent.service
sudo systemctl daemon-reload

# 3. Remove binary
sudo rm /usr/local/bin/lumo-agent

# 4. Remove directories (optional)
sudo rm -rf /etc/lumo-agent
sudo rm -rf /var/lib/lumo-agent
sudo rm -rf /var/log/lumo-agent

# 5. Remove user and group
sudo userdel lumo-agent
sudo groupdel lumo-agent
```

### Package removal

**RPM:**
```bash
# Remove package (keep config)
sudo rpm -e lumo-agent

# Complete removal
sudo rpm -e --allmatches lumo-agent
sudo rm -rf /etc/lumo-agent /var/lib/lumo-agent /var/log/lumo-agent
```

**DEB:**
```bash
# Remove package (keep config)
sudo apt-get remove lumo-agent

# Purge (complete removal)
sudo apt-get purge lumo-agent
```

---

## Additional Resources

- **Main Documentation**: [CLAUDE.md](../../CLAUDE.md)
- **Development Guide**: [DEVELOPMENT.md](../../DEVELOPMENT.md)
- **Kubernetes Deployment**: [deployments/kubernetes/README.md](../kubernetes/README.md)
- **GitHub Repository**: https://github.com/ignacio/lumo
- **Issue Tracker**: https://github.com/ignacio/lumo/issues

---

## License

MIT License - See [LICENSE](../../LICENSE) for details
