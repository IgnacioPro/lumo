# Lumo

**Lumo** is an intelligent SRE/DevOps automation AGENT that connects to remote servers via SSH, runs comprehensive system diagnostics, and uses AI to analyze issues and suggest fixes. It supports multiple AI providers (Anthropic Claude, OpenAI GPT-4, Ollama, Google Gemini) and works on both remote servers and localhost.

## Features

### ✅ Implemented
- **SSH Connection Management**: 4 auth methods (agent, key, password, keyboard-interactive) with retry logic and auto-reconnect
- **Intelligent Diagnostics**:
  - 10 automated health checks (6 core + 4 security)
  - Core: CPU, memory, disk, processes, services, network
  - Security: Patch status, open ports, SSH security, auth failures
  - Kubernetes: Native cluster diagnostics (nodes, pods, deployments, services, etc.)
- **AI-Powered Analysis**: Multi-provider support (Anthropic Claude, OpenAI GPT-4, Ollama, Google Gemini)
- **Localhost Execution**: Run diagnostics on local machine without SSH overhead
- **Cross-Platform Support**: Linux, macOS, BSD compatibility
- **Kubernetes Support**: Native diagnostics using k8s.io/client-go (no kubectl required)

### 🚧 Planned
- **Auto-Remediation**: Automatically fix common issues with smart approval workflows (Phase 5)
- **Detailed Reporting**: Generate comprehensive reports in markdown, JSON, or YAML (Phase 6)
- **API Server**: REST API with WebSocket support (Phase 7)

## Architecture

```
┌─────────────────┐
│   CLI Interface │  Commands: connect, diagnose
└────────┬────────┘
         │
    ┌────▼────┐
    │ Config  │  YAML + Environment Variables
    └────┬────┘
         │
┌────────▼──────────────┐
│  Diagnostics Runner   │  11 Health Checkers:
│                       │  • 6 Core (CPU, Memory, Disk, Process, Service, Network)
│                       │  • 4 Security (Patches, Ports, SSH, Auth)
│                       │  • 1 Kubernetes (Cluster Health)
└────┬─────────┬────┬───┘
     │         │    │
┌────▼────┐ ┌──▼────────┐ ┌──▼──────────┐
│  Local  │ │    SSH    │ │ Kubernetes  │
│ Executor│ │  Executor │ │   Client    │
└─────────┘ └──────┬────┘ └──────┬──────┘
                   │             │
            ┌──────▼──────┐ ┌────▼─────────┐
            │   Remote    │ │  K8s Cluster │
            │   Servers   │ │  (via API)   │
            └─────────────┘ └──────────────┘
                   │
            ┌──────▼──────────┐
            │  AI Analysis    │  4 Providers
            │  (Optional)     │  Streaming Support
            └─────────────────┘
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
# Diagnose localhost (no SSH required)
lumo diagnose localhost

# Diagnose remote server
lumo diagnose user@example.com

# Run specific checks only
lumo diagnose user@example.com --checks cpu,memory,disk

# Run Kubernetes cluster diagnostics (requires kubeconfig)
lumo diagnose --checks kubernetes

# Run all diagnostics including Kubernetes
lumo diagnose localhost

# Get AI-powered analysis (requires API key)
export LUMO_ANTHROPIC_API_KEY=sk-ant-...
lumo diagnose localhost --analyze

# Kubernetes diagnostics with AI analysis
lumo diagnose --checks kubernetes --analyze

# Use different AI provider
export LUMO_OPENAI_API_KEY=sk-...
lumo diagnose localhost --analyze --ai-provider openai

# Output in JSON format
lumo diagnose localhost --format json
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
  provider: anthropic  # Options: anthropic, openai, ollama, gemini
  models:
    anthropic: claude-sonnet-4-5-20250929
    openai: gpt-4-turbo-preview
    ollama: llama3.1:8b
    gemini: gemini-2.0-flash-exp
  timeout: 60s
  max_retries: 3
  temperature: 1.0

logging:
  level: info
  format: text
  output: stdout

api:
  port: 8080
  host: 0.0.0.0
  tls: false

diagnostics:
  kubernetes:
    enabled: false  # Enable Kubernetes diagnostics
    kubeconfig_path: ""  # Uses ~/.kube/config by default
    context: ""  # Use specific cluster context
    namespaces: []  # Check all namespaces (or specify list)
    check_nodes: true
    check_pods: true
    check_deployments: true
    check_statefulsets: true
    check_daemonsets: true
    check_services: true
    check_pvcs: true
    check_events: true
    event_lookback_mins: 30
```

### Environment Variables

Override configuration with environment variables using the `LUMO_` prefix:

```bash
# SSH Configuration
export LUMO_SSH_PORT=2222
export LUMO_SSH_STRICT_HOST_KEY_CHECKING=true
export LUMO_SSH_DEFAULT_KEY_PATH=~/.ssh/id_ed25519

# AI Configuration
export LUMO_AI_PROVIDER=openai

# AI API Keys (Provider-Specific - Recommended)
export LUMO_ANTHROPIC_API_KEY=sk-ant-...   # For Anthropic Claude
export LUMO_OPENAI_API_KEY=sk-...          # For OpenAI GPT
export LUMO_GEMINI_API_KEY=...             # For Google Gemini
export LUMO_OLLAMA_API_KEY=...             # For Ollama (usually not needed)

# AI API Key (Generic Fallback - works but not recommended)
export LUMO_AI_API_KEY=sk-...              # Fallback if provider-specific not set

# Logging
export LUMO_LOGGING_LEVEL=debug

# Kubernetes Diagnostics
export LUMO_DIAGNOSTICS_KUBERNETES_ENABLED=true
export LUMO_DIAGNOSTICS_KUBERNETES_CONTEXT=production-cluster
export LUMO_DIAGNOSTICS_KUBERNETES_KUBECONFIG_PATH=/path/to/kubeconfig
export LUMO_DIAGNOSTICS_KUBERNETES_EVENT_LOOKBACK_MINS=60
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

## Kubernetes Diagnostics

Lumo includes native Kubernetes cluster diagnostics using the official `k8s.io/client-go` library (no `kubectl` required).

### Features

- **8 Resource Types**: Nodes, Pods, Deployments, StatefulSets, DaemonSets, Services, PVCs, Events
- **Native Client**: Direct Kubernetes API access via k8s.io/client-go
- **Granular Control**: Enable/disable individual checks
- **Namespace Filtering**: Check all or specific namespaces
- **Read-Only**: No modifications to your cluster
- **RBAC-Aware**: Requires minimal permissions (get/list)

### Prerequisites

1. **Kubeconfig**: Ensure `~/.kube/config` exists or specify custom path
2. **RBAC Permissions**: Read-only access to cluster resources
3. **Enable in Config**: Set `diagnostics.kubernetes.enabled: true`

### Quick Start

```bash
# Enable Kubernetes diagnostics
export LUMO_DIAGNOSTICS_KUBERNETES_ENABLED=true

# Run Kubernetes diagnostics only
lumo diagnose --checks kubernetes

# Run with AI analysis
lumo diagnose --checks kubernetes --analyze

# Use specific cluster context
export LUMO_DIAGNOSTICS_KUBERNETES_CONTEXT=production
lumo diagnose --checks kubernetes

# Check specific namespaces only
# (Set in config.yaml: namespaces: ["production", "staging"])
lumo diagnose --checks kubernetes
```

### Health Checks Performed

| Check | Metrics Collected |
|-------|------------------|
| **Nodes** | Total, ready, not-ready, conditions |
| **Pods** | Phase distribution, restart counts, problem pods |
| **Deployments** | Replica health, desired vs ready |
| **StatefulSets** | Replica readiness |
| **DaemonSets** | Node coverage, desired vs current |
| **Services** | Endpoint validation |
| **PVCs** | Binding status, pending/lost volumes |
| **Events** | Recent warnings/errors (configurable lookback) |

### Sample Output

```
=== Kubernetes Cluster Diagnostics ===

[✓] Nodes: All 3 nodes ready
[✓] Pods (production): 25 total, 25 running
[!] Deployments (production): 1 unhealthy
    - api-server: Desired=3, Ready=1
[✓] Services: All services have endpoints

Overall Status: WARNING
```

### Configuration

See [Configuration](#configuration) section for full Kubernetes configuration options.

For detailed implementation documentation, see [REPORTS/kubernetes-diagnostics-implementation-2025-11-16.md](REPORTS/kubernetes-diagnostics-implementation-2025-11-16.md).

## Usage Examples

### Verbose Logging

Enable detailed logging for debugging:

```bash
lumo --verbose connect user@example.com
lumo --verbose diagnose localhost
lumo -v diagnose user@example.com --analyze
```

### Multiple Checks

Run specific diagnostic checks:

```bash
# Single check
lumo diagnose localhost --checks cpu

# Multiple checks
lumo diagnose user@example.com --checks cpu,memory,disk

# All checks (default)
lumo diagnose localhost
```

### Custom Configuration

Use a specific configuration file:

```bash
lumo --config /path/to/config.yaml diagnose
```

## API Endpoints (Planned - Phase 7)

When the API server is implemented (`lumo serve`), the following endpoints will be available:

- `POST /connect` - Establish SSH connection
- `POST /diagnose` - Run diagnostics
- `POST /fix` - Execute remediation
- `GET /reports` - Retrieve reports
- `GET /status` - Agent status
- `WebSocket /logs` - Stream logs in real-time

## Development Roadmap

- [x] **Phase 1**: Foundation & Project Setup
- [x] **Phase 2**: SSH & Connection Management (4 auth methods, retry logic, auto-reconnect)
- [x] **Phase 3**: Diagnostic System (6 core checkers: CPU, Memory, Disk, Process, Service, Network)
- [x] **Phase 4**: AI Integration (4 providers: Anthropic, OpenAI, Ollama, Gemini)
- [x] **Phase 5**: Enhanced Diagnostics
  - [x] Security Diagnostics (4 checkers: Patch Status, Open Ports, SSH Security, Auth Failures)
  - [x] Kubernetes Diagnostics (Native cluster health monitoring)
  - [x] Enhanced Memory Metrics (Page faults, pressure indicators, top consumers)
- [x] **Phase 8**: Testing & Documentation (50.4% coverage, 25 test files, 11,059 lines)
- [ ] **Phase 6**: Auto-Remediation & Approval (Planned)
- [ ] **Phase 7**: Reporting & Logging (Planned)
- [ ] **Phase 9**: API Server (Planned)

## Testing

**Overall Coverage**: 50.4% (11,059 lines of test code across 25 test files)

### Coverage by Package

| Package | Coverage | Status |
|---------|----------|--------|
| internal/diagnostics/formatters | 98.1% | ✅ Complete |
| internal/diagnostics | 87.6% | ✅ Excellent |
| internal/config | 68.8% | ✅ Good |
| internal/diagnostics/checkers | 61.4% | ✅ Good |
| cmd/lumo | 61.6% | ✅ Good |
| internal/ssh | 30.5% | ⚠️ In Progress |
| internal/ai | 27.1% | ⚠️ In Progress |

### Run Tests

```bash
# All tests
go test ./...

# With coverage report
go test -cover ./...

# Specific package
go test -v ./internal/diagnostics

# HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Security Considerations

- **SSH Keys**: Store with proper permissions (`chmod 600 ~/.ssh/id_rsa`)
- **API Keys**: Use provider-specific environment variables (LUMO_ANTHROPIC_API_KEY, LUMO_OPENAI_API_KEY, etc.)
- **Configuration**: Never commit `config.yaml` with secrets to version control
- **Credentials**: Use environment variables for all sensitive data in production
- **Localhost**: Commands run with your user permissions when using localhost execution

## License

MIT License - See LICENSE file for details

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## Support

For issues and questions, please open a GitHub issue at https://github.com/ignacio/lumo/issues
