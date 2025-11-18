%global debug_package %{nil}
%define _build_id_links none

Name:           lumo-agent
Version:        1.0.0
Release:        1%{?dist}
Summary:        Lumo Agent - Intelligent SRE/DevOps Automation Agent

License:        MIT
URL:            https://github.com/ignacio/lumo
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  systemd-rpm-macros
Requires:       systemd
Requires(pre):  shadow-utils
Requires(post): systemd
Requires(preun): systemd
Requires(postun): systemd

%description
Lumo Agent is an intelligent SRE/DevOps automation platform that provides
system diagnostics, AI-powered analysis, and auto-remediation capabilities.

The agent runs as a systemd daemon and can operate in multiple modes:
- Scheduled: Periodic diagnostics based on cron expressions
- On-demand: API-triggered diagnostics
- Continuous: Real-time monitoring with WebSocket streaming
- Hybrid: Combination of scheduled, on-demand, and alerts

Features:
- System diagnostics (CPU, memory, disk, processes, services, network)
- Security checks (patches, ports, SSH security, auth failures)
- Kubernetes and Proxmox monitoring
- AI analysis via multiple providers (Anthropic, OpenAI, Ollama, Gemini)
- TOON format for 30-60%% token reduction
- Auto-remediation with human-in-the-loop approval
- Prometheus metrics and health endpoints

%prep
%setup -q

%build
# Binary is pre-built and included in the source tarball
# For building from source, use:
# cd cmd/lumo-agent
# CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=%{version}" -o lumo-agent

%install
# Create directories
install -d %{buildroot}%{_bindir}
install -d %{buildroot}%{_sysconfdir}/lumo-agent
install -d %{buildroot}%{_sharedstatedir}/lumo-agent
install -d %{buildroot}%{_localstatedir}/log/lumo-agent
install -d %{buildroot}%{_unitdir}

# Install binary
install -p -m 755 lumo-agent %{buildroot}%{_bindir}/lumo-agent

# Install systemd service
install -p -m 644 lumo-agent.service %{buildroot}%{_unitdir}/lumo-agent.service

# Install default configuration
cat > %{buildroot}%{_sysconfdir}/lumo-agent/config.yaml << 'EOF'
# Lumo Agent Configuration
# See https://github.com/ignacio/lumo for documentation

agent:
  mode: hybrid
  schedule: "*/5 * * * *"
  api_endpoint: ""  # Set to your Lumo API server URL
  token: ""         # Set via LUMO_AGENT_TOKEN environment variable
  tls_enabled: true
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

diagnostics:
  timeout: 5m
  max_concurrent: 5
EOF

%pre
# Create system user and group
getent group lumo-agent >/dev/null || groupadd -r lumo-agent
getent passwd lumo-agent >/dev/null || \
    useradd -r -g lumo-agent -d %{_sharedstatedir}/lumo-agent -s /sbin/nologin \
    -c "Lumo Agent Service User" lumo-agent
exit 0

%post
%systemd_post lumo-agent.service

# Set permissions on data and log directories
chown -R lumo-agent:lumo-agent %{_sharedstatedir}/lumo-agent
chown -R lumo-agent:lumo-agent %{_localstatedir}/log/lumo-agent
chmod 750 %{_sharedstatedir}/lumo-agent
chmod 750 %{_localstatedir}/log/lumo-agent

# Set permissions on config directory
chown -R root:lumo-agent %{_sysconfdir}/lumo-agent
chmod 750 %{_sysconfdir}/lumo-agent
chmod 640 %{_sysconfdir}/lumo-agent/config.yaml

cat << 'BANNER'
═══════════════════════════════════════════════════════════════
  Lumo Agent Installation Complete
═══════════════════════════════════════════════════════════════

Configuration:
  Config file: /etc/lumo-agent/config.yaml
  Data dir:    /var/lib/lumo-agent
  Log dir:     /var/log/lumo-agent

Next Steps:
  1. Edit the configuration file:
     sudo vi /etc/lumo-agent/config.yaml

  2. Set required environment variables:
     - LUMO_AGENT_API_ENDPOINT
     - LUMO_AGENT_TOKEN

     You can set these in: /etc/systemd/system/lumo-agent.service.d/override.conf

  3. Start the service:
     sudo systemctl start lumo-agent

  4. Enable auto-start on boot:
     sudo systemctl enable lumo-agent

  5. Check status:
     sudo systemctl status lumo-agent

  6. View logs:
     sudo journalctl -u lumo-agent -f

Documentation: https://github.com/ignacio/lumo

═══════════════════════════════════════════════════════════════
BANNER

%preun
%systemd_preun lumo-agent.service

%postun
%systemd_postun_with_restart lumo-agent.service

# Remove user and group on complete uninstall
if [ $1 -eq 0 ]; then
    userdel lumo-agent 2>/dev/null || :
    groupdel lumo-agent 2>/dev/null || :
fi

%files
%license LICENSE
%doc README.md CLAUDE.md DEVELOPMENT.md
%{_bindir}/lumo-agent
%{_unitdir}/lumo-agent.service
%dir %attr(0750,root,lumo-agent) %{_sysconfdir}/lumo-agent
%config(noreplace) %attr(0640,root,lumo-agent) %{_sysconfdir}/lumo-agent/config.yaml
%dir %attr(0750,lumo-agent,lumo-agent) %{_sharedstatedir}/lumo-agent
%dir %attr(0750,lumo-agent,lumo-agent) %{_localstatedir}/log/lumo-agent

%changelog
* Mon Nov 18 2024 Lumo Team <lumo@example.com> - 1.0.0-1
- Initial RPM release
- Agent daemon with multiple operational modes
- System diagnostics and AI analysis
- Prometheus metrics and health endpoints
- Security hardening with minimal capabilities
- Support for Kubernetes and VM deployments
