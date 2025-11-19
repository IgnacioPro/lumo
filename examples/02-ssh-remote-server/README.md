# Example 2: SSH Remote Server Diagnostics

This example demonstrates how to diagnose remote servers via SSH.

## What You'll Learn

- SSH connection methods (password, key, agent, interactive)
- Diagnosing multiple remote servers
- SSH configuration best practices
- Troubleshooting SSH connection issues

## Prerequisites

- Lumo installed and configured
- SSH access to remote server
- SSH credentials (password or private key)

## SSH Authentication Methods

### Method 1: SSH Agent (Recommended)

Use your existing SSH agent:

```bash
# Ensure SSH agent is running
eval $(ssh-agent)
ssh-add ~/.ssh/id_rsa

# Diagnose using agent authentication
lumo diagnose user@server.example.com --use-agent
```

**Benefits:**
- Most secure (no credentials stored)
- Works with existing SSH setup
- Supports multiple keys
- Key passphrase handled by agent

### Method 2: Private Key File

Specify a private key file:

```bash
# Use specific key
lumo diagnose user@server.example.com --key ~/.ssh/id_rsa

# Use different key
lumo diagnose user@server.example.com --key ~/.ssh/production_key

# With custom port
lumo diagnose user@server.example.com --key ~/.ssh/id_rsa --port 2222
```

### Method 3: Password Authentication

Interactive password prompt:

```bash
# Lumo will prompt for password
lumo diagnose user@server.example.com --password
```

**Note:** Key-based authentication is recommended for security and automation.

### Method 4: Interactive Authentication

For complex authentication (2FA, challenge-response):

```bash
lumo diagnose user@server.example.com --interactive
```

## Basic Remote Diagnostics

### Single Server

```bash
# Using SSH agent
lumo diagnose user@webserver01.example.com --use-agent

# Using key file
lumo diagnose root@database.example.com --key ~/.ssh/production_key

# Custom SSH port
lumo diagnose admin@firewall.example.com --key ~/.ssh/id_rsa --port 2222

# With specific checks
lumo diagnose user@appserver.example.com --use-agent --checks cpu,memory,disk
```

**Example Output:**
```
[INFO] Diagnosing webserver01.example.com
[INFO] Establishing SSH connection...
[INFO] Connected successfully (authenticated via agent)
[INFO] Running diagnostic checks...

=== Diagnostic Report for webserver01.example.com ===

CPU:
  Model: Intel Xeon E5-2680 v4
  Cores: 28 (14 physical)
  Load Average: 12.5 / 11.2 / 10.8
  Usage: 45%
  Status: ✓ OK

Memory:
  Total: 128 GB
  Used: 64 GB (50%)
  Cached: 32 GB
  Status: ✓ OK

... (more output)
```

### Multiple Servers

Create a script to diagnose multiple servers:

```bash
#!/bin/bash
# diagnose-fleet.sh

SERVERS=(
  "web01.example.com"
  "web02.example.com"
  "db01.example.com"
  "cache01.example.com"
)

for server in "${SERVERS[@]}"; do
  echo "=== Diagnosing $server ==="
  lumo diagnose "admin@$server" --use-agent --format json > "reports/$server-$(date +%Y%m%d).json"
  echo "Done: reports/$server-$(date +%Y%m%d).json"
  echo ""
done
```

## SSH Configuration

### Using SSH Config File

Create `~/.ssh/config` for easier access:

```ssh-config
# ~/.ssh/config

Host webserver
  HostName webserver01.example.com
  User admin
  Port 22
  IdentityFile ~/.ssh/production_key
  StrictHostKeyChecking yes

Host database
  HostName db01.example.com
  User dbadmin
  Port 2222
  IdentityFile ~/.ssh/database_key
  ForwardAgent yes

Host *.production
  User admin
  IdentityFile ~/.ssh/production_key
  ProxyJump bastion.example.com
```

Then use short names:

```bash
# Uses configuration from ~/.ssh/config
lumo diagnose webserver --use-agent
lumo diagnose database --use-agent
```

### Jump Host / Bastion

Diagnose servers through a bastion/jump host:

```ssh-config
# ~/.ssh/config

Host bastion
  HostName bastion.example.com
  User jump-user
  IdentityFile ~/.ssh/bastion_key

Host private-server
  HostName 10.0.1.50
  User admin
  IdentityFile ~/.ssh/private_key
  ProxyJump bastion
```

```bash
lumo diagnose private-server --use-agent
```

## Advanced Usage

### Parallel Diagnostics

Use GNU Parallel or xargs for concurrent diagnostics:

```bash
# Using GNU parallel
cat servers.txt | parallel -j 5 'lumo diagnose {} --use-agent --format json > reports/{}.json'

# Using xargs
cat servers.txt | xargs -P 5 -I {} sh -c 'lumo diagnose {} --use-agent > reports/{}.txt'
```

### Scheduled Remote Diagnostics

Set up cron job for periodic checks:

```bash
# crontab -e

# Every hour, diagnose production servers
0 * * * * /usr/local/bin/lumo diagnose user@prod-web01 --use-agent --format json >> /var/log/lumo/prod-web01.json

# Every 6 hours, full diagnostic of database server
0 */6 * * * /usr/local/bin/lumo diagnose user@prod-db01 --use-agent --analyze > /var/log/lumo/prod-db01-$(date +\%Y\%m\%d-\%H).txt
```

### Ansible Integration

Use Lumo with Ansible for infrastructure diagnostics:

```yaml
# playbook.yml
---
- name: Run Lumo Diagnostics
  hosts: all
  tasks:
    - name: Run diagnostic
      shell: lumo diagnose localhost --format json
      register: diagnostic_result

    - name: Save diagnostic report
      copy:
        content: "{{ diagnostic_result.stdout }}"
        dest: "/tmp/lumo-{{ inventory_hostname }}.json"

    - name: Fetch reports
      fetch:
        src: "/tmp/lumo-{{ inventory_hostname }}.json"
        dest: "reports/{{ inventory_hostname }}.json"
        flat: yes
```

### Terraform Integration

Run diagnostics during Terraform provisioning:

```hcl
# main.tf

resource "null_resource" "lumo_diagnostic" {
  depends_on = [aws_instance.server]

  provisioner "remote-exec" {
    inline = [
      "curl -sSL https://get.lumo.sh | bash",
      "lumo diagnose localhost --format json > /tmp/initial-diagnostic.json"
    ]

    connection {
      type        = "ssh"
      host        = aws_instance.server.public_ip
      user        = "ubuntu"
      private_key = file("~/.ssh/terraform_key")
    }
  }

  provisioner "local-exec" {
    command = "scp -i ~/.ssh/terraform_key ubuntu@${aws_instance.server.public_ip}:/tmp/initial-diagnostic.json reports/${aws_instance.server.id}.json"
  }
}
```

## Troubleshooting

### Connection Refused

```
Error: Failed to establish SSH connection: connection refused
```

**Solutions:**
```bash
# Check if SSH server is running
ssh user@server systemctl status sshd

# Verify port
nmap -p 22 server.example.com

# Try different port
lumo diagnose user@server --port 2222 --use-agent
```

### Permission Denied

```
Error: SSH authentication failed: permission denied
```

**Solutions:**
```bash
# Verify key works manually
ssh -i ~/.ssh/id_rsa user@server

# Check key permissions
chmod 600 ~/.ssh/id_rsa

# Add key to agent
ssh-add ~/.ssh/id_rsa
```

### Host Key Verification Failed

```
Error: Host key verification failed
```

**Solutions:**
```bash
# Remove old host key
ssh-keygen -R server.example.com

# Accept new host key
ssh user@server  # Type 'yes' to accept

# Then retry Lumo
lumo diagnose user@server --use-agent
```

### Timeout Issues

```
Error: SSH connection timeout after 30s
```

**Solutions:**
```bash
# Check network connectivity
ping server.example.com
telnet server.example.com 22

# Use verbose mode for debugging
lumo diagnose user@server --use-agent --verbose
```

## Security Best Practices

### 1. Use SSH Keys (Not Passwords)

```bash
# Generate dedicated key for Lumo
ssh-keygen -t ed25519 -f ~/.ssh/lumo_key -C "lumo-diagnostics"

# Copy to servers
ssh-copy-id -i ~/.ssh/lumo_key user@server

# Use it
lumo diagnose user@server --key ~/.ssh/lumo_key
```

### 2. Restrict SSH Key Permissions

On the remote server, limit what the key can do:

```bash
# ~/.ssh/authorized_keys
command="/usr/local/bin/lumo diagnose localhost --format json" ssh-ed25519 AAAA...
```

### 3. Use Dedicated User

Create a dedicated user for diagnostics:

```bash
# On remote server
sudo useradd -m -s /bin/bash lumodiag
sudo usermod -aG systemd-journal lumodiag  # For log access

# Grant minimal sudo permissions if needed
# /etc/sudoers.d/lumodiag
lumodiag ALL=(ALL) NOPASSWD: /bin/systemctl status *
```

### 4. Enable SSH Connection Multiplexing

Reduces connection overhead:

```ssh-config
# ~/.ssh/config
Host *
  ControlMaster auto
  ControlPath ~/.ssh/sockets/%r@%h:%p
  ControlPersist 600
```

```bash
mkdir -p ~/.ssh/sockets
```

## Real-World Examples

### Example 1: Web Server Fleet Health Check

```bash
#!/bin/bash
# Check all web servers and alert if issues found

WEB_SERVERS="web01 web02 web03 web04"
ALERT_EMAIL="ops@example.com"

for server in $WEB_SERVERS; do
  echo "Checking $server..."

  # Run diagnostic
  lumo diagnose "admin@$server" --use-agent --format json > "/tmp/$server.json"

  # Check for critical issues
  if jq -e '.checks | to_entries[] | select(.value.status == "critical")' "/tmp/$server.json"; then
    echo "CRITICAL: Issues found on $server" | mail -s "Alert: $server" "$ALERT_EMAIL"
  fi
done
```

### Example 2: Database Server Pre-Maintenance Check

```bash
#!/bin/bash
# Run comprehensive diagnostics before maintenance window

lumo diagnose db-master --use-agent --checks cpu,memory,disk,process > "pre-maintenance-$(date +%Y%m%d).txt"

# Check for concerning issues
grep -E "CRITICAL|WARNING" "pre-maintenance-$(date +%Y%m%d).txt" && \
  echo "Review warnings before proceeding with maintenance"
```

### Example 3: New Server Validation

```bash
#!/bin/bash
# Validate newly provisioned server meets requirements

SERVER=$1

echo "Validating $SERVER..."

# Run comprehensive diagnostic
lumo diagnose "admin@$SERVER" --use-agent --format json > validation.json

# Check requirements
CPU_CORES=$(jq -r '.checks.cpu.cores' validation.json)
MEMORY_GB=$(jq -r '.checks.memory.total_bytes / 1073741824 | floor' validation.json)
DISK_GB=$(jq -r '.checks.disk.filesystems[0].total_bytes / 1073741824 | floor' validation.json)

echo "Specs: ${CPU_CORES} cores, ${MEMORY_GB} GB RAM, ${DISK_GB} GB disk"

# Validate against requirements
if [ "$CPU_CORES" -lt 4 ] || [ "$MEMORY_GB" -lt 8 ] || [ "$DISK_GB" -lt 100 ]; then
  echo "ERROR: Server does not meet minimum requirements"
  exit 1
fi

echo "✓ Server validated successfully"
```

## Next Steps

- **[Example 3: AI Analysis](../03-ai-analysis/)** - Get AI-powered insights on remote servers
- **[Example 4: Auto-Remediation](../04-auto-remediation/)** - Fix issues on remote servers automatically
- **[Example 5: Agent Deployment (K8s)](../05-agent-deployment-k8s/)** - Deploy agents instead of SSH

## Additional Resources

- [SSH Configuration Guide](../../docs/ssh-configuration.md)
- [Security Best Practices](../../docs/security.md)
- [Fleet Management](../../docs/fleet-management.md)
