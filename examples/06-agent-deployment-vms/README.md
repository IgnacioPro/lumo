# Example 6: Agent Deployment on VMs/Bare Metal

This example demonstrates deploying Lumo agents on VMs and bare metal servers using systemd.

## What You'll Learn

- Install agent as systemd service
- Configure agent for VM environments
- Manage agent lifecycle (start, stop, restart)
- Monitor agent health and logs
- Deploy at scale with automation tools
- Security hardening for production

## Prerequisites

- Linux system with systemd
- Sudo/root access
- Lumo API server running (or use `lumo serve`)
- API key for agent authentication

## Architecture

```
┌──────────────────────┐
│   VM/Bare Metal      │
│                      │
│  ┌────────────────┐  │
│  │  lumo-agent    │  │
│  │  (systemd)     │  │
│  │                │  │
│  │  - Scheduler   │  │
│  │  - Diagnostics │  │
│  │  - Reporter    │  │
│  │  - Health API  │  │
│  │  - Metrics     │  │
│  └────────┬───────┘  │
└───────────┼──────────┘
            │
    ┌───────▼────────┐
    │  Lumo API      │
    │  Server        │
    └────────────────┘
```

## Quick Start

### 1. Start API Server

```bash
# Local development
docker-compose up -d
lumo serve --port 8080

# Or configure existing API
export LUMO_API_ENDPOINT=https://lumo-api.example.com
export LUMO_AGENT_TOKEN=your-agent-token
```

### 2. Install Agent

```bash
# Download and run install script
curl -sSL https://raw.githubusercontent.com/ignacio/lumo/main/deployments/systemd/install.sh | sudo bash

# Or manually
git clone https://github.com/ignacio/lumo.git
cd lumo/deployments/systemd
sudo ./install.sh
```

### 3. Configure Agent

Edit `/etc/lumo/agent-config.yaml`:

```yaml
agent:
  mode: hybrid
  schedule: "*/5 * * * *"
  api_endpoint: http://lumo-api.example.com:8080
  token: your-agent-token
  enabled_checks:
    - cpu
    - memory
    - disk
    - process
    - service
    - network
  report_format: toon
  offline_mode: true
  health_check_port: 8080
  metrics_port: 9090

logging:
  level: info
  format: json
```

### 4. Start Agent

```bash
sudo systemctl enable lumo-agent
sudo systemctl start lumo-agent
```

### 5. Verify

```bash
# Check status
sudo systemctl status lumo-agent

# Check logs
journalctl -u lumo-agent -f

# Health check
curl http://localhost:8080/health

# Metrics
curl http://localhost:9090/metrics
```

## Manual Installation

### Build Binary

```bash
# Clone repository
git clone https://github.com/ignacio/lumo.git
cd lumo

# Build agent binary
go build -o lumo-agent ./cmd/lumo-agent

# Install
sudo mv lumo-agent /usr/local/bin/
sudo chmod +x /usr/local/bin/lumo-agent
```

### Create Configuration

```bash
# Create config directory
sudo mkdir -p /etc/lumo

# Create config file
sudo cat > /etc/lumo/agent-config.yaml <<EOF
agent:
  mode: hybrid
  schedule: "*/5 * * * *"
  api_endpoint: http://localhost:8080
  token: your-token-here
  enabled_checks: [cpu, memory, disk, process, service, network]
  report_format: toon

logging:
  level: info
  format: json
EOF

# Secure permissions
sudo chmod 600 /etc/lumo/agent-config.yaml
```

### Create systemd Service

```bash
sudo cat > /etc/systemd/system/lumo-agent.service <<EOF
[Unit]
Description=Lumo Agent - System Diagnostics and Monitoring
Documentation=https://github.com/ignacio/lumo
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=lumo
Group=lumo
ExecStart=/usr/local/bin/lumo-agent --config /etc/lumo/agent-config.yaml
Restart=always
RestartSec=10

# Security hardening
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
NoNewPrivileges=yes
ReadWritePaths=/var/lib/lumo /var/log/lumo

# Capabilities
AmbientCapabilities=CAP_NET_RAW CAP_SYS_PTRACE CAP_DAC_READ_SEARCH

# Resource limits
MemoryLimit=512M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
EOF
```

### Create User

```bash
sudo useradd -r -s /bin/false -d /var/lib/lumo lumo
sudo mkdir -p /var/lib/lumo /var/log/lumo
sudo chown -R lumo:lumo /var/lib/lumo /var/log/lumo
```

### Enable and Start

```bash
sudo systemctl daemon-reload
sudo systemctl enable lumo-agent
sudo systemctl start lumo-agent
```

## Package Installation

### RPM (RHEL/CentOS/Fedora)

```bash
# Download RPM
wget https://github.com/ignacio/lumo/releases/latest/download/lumo-agent.rpm

# Install
sudo rpm -i lumo-agent.rpm

# Configure
sudo vi /etc/lumo/agent-config.yaml

# Start
sudo systemctl enable --now lumo-agent
```

### DEB (Debian/Ubuntu)

```bash
# Download DEB
wget https://github.com/ignacio/lumo/releases/latest/download/lumo-agent.deb

# Install
sudo dpkg -i lumo-agent.deb

# Configure
sudo vi /etc/lumo/agent-config.yaml

# Start
sudo systemctl enable --now lumo-agent
```

## Agent Management

### Basic Commands

```bash
# Start
sudo systemctl start lumo-agent

# Stop
sudo systemctl stop lumo-agent

# Restart
sudo systemctl restart lumo-agent

# Status
sudo systemctl status lumo-agent

# Enable on boot
sudo systemctl enable lumo-agent

# Disable
sudo systemctl disable lumo-agent
```

### Logs

```bash
# Follow logs
journalctl -u lumo-agent -f

# Last 100 lines
journalctl -u lumo-agent -n 100

# Since 1 hour ago
journalctl -u lumo-agent --since "1 hour ago"

# Export logs
journalctl -u lumo-agent --since today > lumo-agent-logs.txt
```

### Health Checks

```bash
# HTTP health endpoint
curl http://localhost:8080/health

# Detailed status
curl http://localhost:8080/status

# Check if ready
curl http://localhost:8080/ready
```

### Metrics

```bash
# Prometheus metrics
curl http://localhost:9090/metrics

# Specific metrics
curl -s http://localhost:9090/metrics | grep lumo_agent

# Integration with Prometheus
# prometheus.yml:
# - job_name: 'lumo-agent'
#   static_configs:
#   - targets: ['server1:9090', 'server2:9090']
```

## Configuration

### Agent Modes

**Scheduled Mode:**
```yaml
agent:
  mode: scheduled
  schedule: "*/10 * * * *"  # Every 10 minutes
```

**On-Demand Mode:**
```yaml
agent:
  mode: on-demand  # Waits for API triggers
```

**Continuous Mode:**
```yaml
agent:
  mode: continuous  # Runs checks continuously
```

**Hybrid Mode (Recommended):**
```yaml
agent:
  mode: hybrid  # Scheduled + on-demand + alerts
  schedule: "*/5 * * * *"
```

### Enabled Checks

```yaml
agent:
  enabled_checks:
    # Core checks
    - cpu
    - memory
    - disk
    - process
    - service
    - network

    # Security checks
    - patch
    - ssh-security
    - ports
    - auth-failures

    # Specialized (if applicable)
    - kubernetes  # If running on K8s node
    - proxmox     # If Proxmox host
```

### Offline Mode

Continue working when API is unavailable:

```yaml
agent:
  offline_mode: true
  cache_dir: /var/lib/lumo/cache
  max_cache_size: 100  # MB
```

### Notifications

```yaml
notifications:
  enabled: true
  providers:
    - slack
    - email

  slack:
    webhook_url: https://hooks.slack.com/services/YOUR/WEBHOOK/URL
    channel: "#alerts"

  email:
    smtp_host: smtp.gmail.com
    smtp_port: 587
    from: lumo-agent@example.com
    to: ops@example.com
```

## Automation & Fleet Management

### Ansible Playbook

```yaml
# deploy-lumo-agent.yml
---
- name: Deploy Lumo Agent
  hosts: all
  become: yes
  vars:
    lumo_api_endpoint: https://lumo-api.example.com
    lumo_agent_token: "{{ vault_lumo_token }}"

  tasks:
    - name: Download Lumo agent binary
      get_url:
        url: https://github.com/ignacio/lumo/releases/latest/download/lumo-agent-linux-amd64
        dest: /usr/local/bin/lumo-agent
        mode: '0755'

    - name: Create lumo user
      user:
        name: lumo
        system: yes
        create_home: yes
        home: /var/lib/lumo
        shell: /bin/false

    - name: Create directories
      file:
        path: "{{ item }}"
        state: directory
        owner: lumo
        group: lumo
      loop:
        - /etc/lumo
        - /var/lib/lumo
        - /var/log/lumo

    - name: Deploy configuration
      template:
        src: agent-config.yaml.j2
        dest: /etc/lumo/agent-config.yaml
        owner: lumo
        group: lumo
        mode: '0600'

    - name: Deploy systemd service
      copy:
        src: lumo-agent.service
        dest: /etc/systemd/system/lumo-agent.service
        owner: root
        group: root
        mode: '0644'
      notify: Reload systemd

    - name: Enable and start service
      systemd:
        name: lumo-agent
        enabled: yes
        state: started

  handlers:
    - name: Reload systemd
      systemd:
        daemon_reload: yes
```

Run playbook:

```bash
ansible-playbook -i inventory.ini deploy-lumo-agent.yml
```

### Terraform

```hcl
# main.tf

resource "null_resource" "install_lumo_agent" {
  count = length(var.vm_instances)

  connection {
    type        = "ssh"
    host        = var.vm_instances[count.index].public_ip
    user        = "ubuntu"
    private_key = file(var.ssh_private_key)
  }

  provisioner "remote-exec" {
    inline = [
      # Download and install
      "curl -sSL https://raw.githubusercontent.com/ignacio/lumo/main/deployments/systemd/install.sh | sudo bash",

      # Configure
      "sudo tee /etc/lumo/agent-config.yaml > /dev/null <<EOF",
      templatefile("${path.module}/agent-config.yaml.tpl", {
        api_endpoint = var.lumo_api_endpoint
        agent_token  = var.lumo_agent_token
      }),
      "EOF",

      # Start
      "sudo systemctl enable --now lumo-agent"
    ]
  }
}
```

### Salt Stack

```yaml
# lumo-agent.sls

lumo-agent:
  pkg.installed:
    - sources:
      - lumo-agent: https://github.com/ignacio/lumo/releases/latest/download/lumo-agent.rpm

  file.managed:
    - name: /etc/lumo/agent-config.yaml
    - source: salt://lumo/agent-config.yaml
    - template: jinja
    - user: lumo
    - group: lumo
    - mode: 600

  service.running:
    - enable: True
    - watch:
      - file: /etc/lumo/agent-config.yaml
```

## Monitoring & Observability

### Prometheus Integration

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'lumo-agents'
    static_configs:
      - targets:
        - 'server1.example.com:9090'
        - 'server2.example.com:9090'
        - 'server3.example.com:9090'
    relabel_configs:
      - source_labels: [__address__]
        target_label: instance
        regex: '([^:]+):.*'
        replacement: '$1'
```

### Grafana Dashboard

Key metrics to monitor:
- `lumo_agent_health{status}` - Agent health status
- `lumo_diagnostics_total` - Total diagnostics run
- `lumo_diagnostics_duration_seconds` - Execution time
- `lumo_api_requests_total{status}` - API request metrics
- `lumo_cache_hits_total` - Cache performance

### Log Aggregation

**rsyslog:**
```bash
# /etc/rsyslog.d/lumo.conf
if $programname == 'lumo-agent' then /var/log/lumo/lumo-agent.log
& stop
```

**Logrotate:**
```bash
# /etc/logrotate.d/lumo-agent
/var/log/lumo/lumo-agent.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    postrotate
        systemctl reload lumo-agent > /dev/null 2>&1 || true
    endscript
}
```

## Security Hardening

### Capabilities

Grant minimal capabilities:

```bash
# Set capabilities on binary
sudo setcap cap_net_raw,cap_sys_ptrace,cap_dac_read_search+ep /usr/local/bin/lumo-agent
```

### SELinux Policy

```bash
# Create custom policy if needed
sudo audit2allow -a -M lumo-agent
sudo semodule -i lumo-agent.pp
```

### AppArmor Profile

```bash
# /etc/apparmor.d/usr.local.bin.lumo-agent
#include <tunables/global>

/usr/local/bin/lumo-agent {
  #include <abstractions/base>
  #include <abstractions/nameservice>

  capability net_raw,
  capability sys_ptrace,
  capability dac_read_search,

  /usr/local/bin/lumo-agent mr,
  /etc/lumo/** r,
  /var/lib/lumo/** rw,
  /var/log/lumo/** w,
  /proc/** r,
  /sys/** r,
}
```

## Troubleshooting

### Agent Won't Start

```bash
# Check logs
journalctl -u lumo-agent -n 50

# Test configuration
sudo /usr/local/bin/lumo-agent --config /etc/lumo/agent-config.yaml --validate

# Check permissions
ls -la /usr/local/bin/lumo-agent
ls -la /etc/lumo/agent-config.yaml

# Check user
id lumo
```

### Can't Connect to API

```bash
# Test connectivity
curl -v http://lumo-api.example.com:8080/api/v1/health

# Check DNS
nslookup lumo-api.example.com

# Check firewall
sudo iptables -L -n
sudo firewall-cmd --list-all
```

### High Resource Usage

```bash
# Check agent metrics
curl http://localhost:9090/metrics | grep -E "(cpu|memory)"

# Reduce check frequency
# Edit /etc/lumo/agent-config.yaml
# Change schedule to "*/15 * * * *"

# Disable expensive checks
# Remove process, kubernetes from enabled_checks
```

## Production Best Practices

1. **Use packages (RPM/DEB)** instead of manual installation
2. **Configure proper resource limits** in systemd
3. **Enable security hardening** (AppArmor/SELinux)
4. **Monitor agent health** via Prometheus/Grafana
5. **Set up log rotation** to prevent disk filling
6. **Use offline mode** for reliability
7. **Secure API keys** with proper file permissions
8. **Test in staging** before production rollout
9. **Automate deployment** with Ansible/Terraform/Salt
10. **Document your setup** for your team

## Uninstallation

```bash
# Stop and disable service
sudo systemctl stop lumo-agent
sudo systemctl disable lumo-agent

# Remove service file
sudo rm /etc/systemd/system/lumo-agent.service
sudo systemctl daemon-reload

# Remove binary and config
sudo rm /usr/local/bin/lumo-agent
sudo rm -rf /etc/lumo

# Remove data (optional)
sudo rm -rf /var/lib/lumo /var/log/lumo

# Remove user
sudo userdel -r lumo
```

Or use the uninstall script:

```bash
cd deployments/systemd
sudo ./uninstall.sh
```

## Next Steps

- **[Example 5: K8s Agent Deployment](../05-agent-deployment-k8s/)** - Event-driven Kubernetes monitoring
- **[Example 7: Querying Events](../07-events-query/)** - Query and analyze events
- **[API Server Setup](../../docs/api-server.md)** - Set up central API server
- **[Monitoring Guide](../../docs/monitoring.md)** - Comprehensive monitoring setup
- **[Fleet Management](../../docs/fleet-management.md)** - Manage large agent deployments

## Additional Resources

- [systemd Documentation](../../deployments/systemd/README.md)
- [Configuration Reference](../../docs/configuration.md)
- [Troubleshooting Guide](../../docs/troubleshooting.md)
