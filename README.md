# Lumo

**Lumo** is an intelligent SRE/DevOps agent that automates system diagnostics and remediation tasks. It SSH into remote machines, runs diagnostic commands, uses AI to analyze issues, and performs auto-remediation with human-in-the-loop approval for critical operations.

## Features

- **SSH Connection Management**: Connect to multiple remote servers securely
- **Intelligent Diagnostics**: Automated system health checks (CPU, memory, disk, processes, logs, network)
- **AI-Powered Analysis**: Uses AI to analyze diagnostic results and suggest fixes
- **Auto-Remediation**: Automatically fix common issues with smart approval workflows
- **Human-in-the-Loop**: Critical operations require human approval before execution
- **Multi-Provider AI Support**: Works with Anthropic Claude, OpenAI GPT, and local models
- **Dual Mode**: Works as both a CLI tool and API server
- **Detailed Reporting**: Generate comprehensive reports in markdown, JSON, or YAML

## Architecture

```
┌─────────────┐
│   CLI/API   │  User interface (cobra + gin/echo)
└──────┬──────┘
       │
┌──────▼──────┐
│   Agent     │  AI orchestration & decision-making
└──────┬──────┘
       │
┌──────▼──────┐
│    SSH      │  Connection management & command execution
└──────┬──────┘
       │
┌──────▼──────────────┐
│  Remote Servers     │  Target systems
└─────────────────────┘
```

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/ignacio/lumo.git
cd lumo

# Build
go build -o lumo ./cmd/lumo

# Install globally
go install ./cmd/lumo
```

### Using Go Install

```bash
go install github.com/ignacio/lumo/cmd/lumo@latest
```

## Quick Start

### 1. Configure Lumo

Create a configuration file at `~/.lumo/config.yaml`:

```bash
mkdir -p ~/.lumo
cp configs/config.example.yaml ~/.lumo/config.yaml
```

Edit the configuration file to set your AI provider credentials and preferences.

### 2. Connect to a Server

Lumo supports multiple authentication methods with automatic fallback:

```bash
# Connect using SSH agent (recommended)
lumo connect user@example.com

# Connect with specific key file
lumo connect user@example.com --key ~/.ssh/id_ed25519

# Connect with custom port
lumo connect user@example.com --port 2222

# Connect with password (interactive prompt)
lumo connect user@example.com

# Skip test commands after connecting
lumo connect user@example.com --test=false
```

**Authentication Methods** (tried in order):
1. **SSH Agent** - Most secure, uses ssh-agent
2. **Private Keys** - Auto-discovers keys in ~/.ssh/ (id_ed25519, id_rsa, etc.)
3. **Password** - Interactive prompt (if other methods fail)
4. **Keyboard-Interactive** - For 2FA/MFA scenarios

### 3. Run Diagnostics

```bash
# Run all diagnostic checks
lumo diagnose

# Run specific checks
lumo diagnose --checks cpu,memory,disk
```

### 4. Auto-Remediation

```bash
# Run auto-remediation with approval prompts
lumo fix

# Auto-approve safe operations
lumo fix --auto-approve
```

### 5. Generate Reports

```bash
# Generate markdown report
lumo report

# Export as JSON
lumo report --format json --output report.json
```

### 6. Start API Server

```bash
# Start API server on default port (8080)
lumo serve

# Start with custom port and TLS
lumo serve --port 443 --tls --cert cert.pem --key key.pem
```

## Configuration

Lumo uses a YAML configuration file with support for environment variable overrides.

### Configuration Locations

Lumo searches for configuration files in the following order:
1. Path specified by `--config` flag
2. `./config.yaml` (current directory)
3. `~/.lumo/config.yaml` (home directory)

### Configuration Structure

```yaml
ssh:
  timeout: 30s
  port: 22
  keepalive: 30s
  max_retries: 3
  retry_interval: 5s
  known_hosts_path: ""  # Uses ~/.ssh/known_hosts by default
  strict_host_key_checking: false
  preferred_auth_methods:
    - agent
    - key
    - password
    - interactive
  command_timeout: 5m
  default_key_path: ""  # Auto-discovers in ~/.ssh/

ai:
  provider: anthropic
  models:
    anthropic: claude-sonnet-4-5-20250929
    openai: gpt-4
  timeout: 60s
  max_retries: 3

logging:
  level: info
  format: text
  output: stdout

api:
  port: 8080
  host: 0.0.0.0
  tls: false
```

### Environment Variables

Override configuration with environment variables using the `LUMO_` prefix:

```bash
export LUMO_SSH_PORT=2222
export LUMO_SSH_STRICT_HOST_KEY_CHECKING=true
export LUMO_SSH_DEFAULT_KEY_PATH=~/.ssh/id_ed25519
export LUMO_AI_PROVIDER=openai
export LUMO_LOGGING_LEVEL=debug
```

## SSH Authentication

Lumo provides comprehensive SSH connection management with multiple authentication methods and robust error handling.

### Supported Authentication Methods

1. **SSH Agent (Recommended)**
   ```bash
   # Ensure ssh-agent is running and has keys loaded
   eval $(ssh-agent)
   ssh-add ~/.ssh/id_ed25519

   # Connect using agent
   lumo connect user@example.com
   ```

2. **Private Key Files**
   ```bash
   # Auto-discovery (tries id_ed25519, id_ecdsa, id_rsa, id_dsa)
   lumo connect user@example.com

   # Specify key explicitly
   lumo connect user@example.com --key ~/.ssh/custom_key

   # Encrypted keys will prompt for passphrase
   lumo connect user@example.com --key ~/.ssh/encrypted_key
   ```

3. **Password Authentication**
   ```bash
   # Interactive password prompt
   lumo connect user@example.com
   # You'll be prompted: "Password for user@example.com:"

   # Or specify password (NOT RECOMMENDED for security)
   lumo connect user@example.com --password 'mypassword'
   ```

4. **Keyboard-Interactive**
   - Automatically handles multi-factor authentication
   - Supports challenge-response authentication
   - Works with systems requiring 2FA/MFA

### Connection Features

- **Automatic Retry**: Retries failed connections with exponential backoff (configurable)
- **Keep-Alive**: Background health monitoring prevents connection drops
- **Auto-Reconnect**: Automatically reconnects if connection becomes unhealthy
- **Connection Health Checks**: Periodic validation ensures connection stability
- **Timeout Management**: Configurable timeouts for connections and commands

### Troubleshooting

**Connection Refused**
```bash
# Use verbose mode to see detailed logs
lumo --verbose connect user@example.com
```

**SSH Agent Issues**
```bash
# Check if agent is running
ssh-add -l

# Start agent if needed
eval $(ssh-agent)
ssh-add ~/.ssh/id_ed25519
```

**Key Permission Issues**
```bash
# Fix key permissions (must be 600 or 400)
chmod 600 ~/.ssh/id_ed25519
```

**Known Hosts Verification**
```yaml
# Enable strict host key checking in config.yaml
ssh:
  strict_host_key_checking: true
  known_hosts_path: ~/.ssh/known_hosts
```

## Usage Examples

### Dry Run Mode

Test operations without making changes:

```bash
lumo diagnose --dry-run
lumo fix --dry-run
```

### Verbose Logging

Enable detailed logging for debugging:

```bash
lumo --verbose connect user@example.com
lumo -v diagnose
```

### Custom Configuration

Use a specific configuration file:

```bash
lumo --config /path/to/config.yaml diagnose
```

## API Endpoints

When running in server mode (`lumo serve`), the following endpoints are available:

- `POST /connect` - Establish SSH connection
- `POST /diagnose` - Run diagnostics
- `POST /fix` - Execute remediation
- `GET /reports` - Retrieve reports
- `GET /status` - Agent status
- `WebSocket /logs` - Stream logs in real-time

## Development Roadmap

- [x] **Phase 1**: Foundation & Project Setup
- [x] **Phase 2**: SSH & Connection Management
- [ ] **Phase 3**: Diagnostic System
- [ ] **Phase 4**: AI Integration Layer
- [ ] **Phase 5**: Auto-Remediation & Approval
- [ ] **Phase 6**: Reporting & Logging
- [ ] **Phase 7**: API Server
- [ ] **Phase 8**: Testing & Documentation

## Security Considerations

- SSH keys should be stored securely with appropriate permissions (600)
- Never commit `config.yaml` with API keys or credentials to version control
- Use environment variables for sensitive configuration in production
- Enable TLS for API server in production environments
- Review all remediation actions before approval

## License

MIT License - See LICENSE file for details

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## Support

For issues and questions, please open a GitHub issue at https://github.com/ignacio/lumo/issues
