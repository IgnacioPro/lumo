# Phase 10: Messaging & Notifications - Implementation Plan

> **Created:** 2025-11-17
> **Status:** Planning
> **Estimated Effort:** 5,800+ lines of code + tests
> **Priority:** Medium (Cross-cutting capability for future phases)

---

## Executive Summary

This document outlines the complete implementation plan for **Phase 10: Messaging & Notifications**, a cross-cutting notification system that enables Lumo to send alerts and reports to various communication platforms. This feature will enhance operational awareness by integrating with team chat platforms, incident management systems, and custom webhooks.

### Key Objectives

1. **Multi-Provider Support**: Integrate 10+ notification providers (Slack, Teams, Telegram, Discord, PagerDuty, etc.)
2. **Severity-Based Routing**: Route messages to different channels based on severity levels
3. **Rich Formatting**: Support provider-specific message formats (Slack blocks, Teams cards, Markdown)
4. **Secure Configuration**: All credentials via environment variables only
5. **Integration Ready**: Hooks for diagnostics, remediation, and API events
6. **Testing**: Comprehensive unit and integration tests

---

## Architecture Overview

### Provider Interface Pattern

Following the established AI provider pattern from Phase 4:

```go
// internal/notifications/provider.go
type Provider interface {
    Name() string
    Send(ctx context.Context, msg *Message) error
    Health(ctx context.Context) error
}

type Message struct {
    Severity    diagnostics.Severity
    Title       string
    Body        string
    Fields      map[string]string
    Timestamp   time.Time
    Source      string  // "diagnostics", "remediation", "api"
    HostInfo    *HostInfo
    Attachments []Attachment
    Metadata    map[string]interface{}
}

type HostInfo struct {
    Hostname string
    IP       string
    OS       string
    Platform string
}

type Attachment struct {
    Title    string
    Content  string
    Filename string
    MimeType string
}
```

### Notification Router

```go
// internal/notifications/notifier.go
type Notifier struct {
    config    *config.NotificationConfig
    providers map[string]Provider
    logger    *logrus.Logger
}

func (n *Notifier) Notify(ctx context.Context, msg *Message) error {
    // Route based on severity and configuration
    providers := n.getProvidersForSeverity(msg.Severity)

    var errors []error
    for _, provider := range providers {
        if err := provider.Send(ctx, msg); err != nil {
            errors = append(errors, err)
        }
    }

    if len(errors) > 0 {
        return fmt.Errorf("notification errors: %v", errors)
    }
    return nil
}
```

---

## Provider Implementations

### Provider Priority Matrix

| Provider | Priority | Complexity | LOC Estimate | Use Case |
|----------|----------|------------|--------------|----------|
| **Slack** | High | Medium | 300 | Primary team communication |
| **Microsoft Teams** | High | Medium | 350 | Enterprise environments |
| **Telegram** | High | Low | 200 | Personal alerts, small teams |
| **Discord** | Medium | Low | 200 | Dev teams, community |
| **Email** | High | Medium | 250 | Executive reports, audit trails |
| **PagerDuty** | High | Medium | 300 | Critical incident management |
| **Opsgenie** | Medium | Medium | 300 | Alternative incident management |
| **Webhooks** | High | Low | 150 | Custom integrations |
| **Mattermost** | Low | Low | 150 | Self-hosted (Slack-compatible) |
| **Rocket.Chat** | Low | Low | 150 | Self-hosted alternative |

**Total Estimated LOC: ~2,350 lines**

---

## Detailed Implementation Steps

### Step 1: Core Infrastructure (Week 1)

**Goal**: Build the foundation - interfaces, message types, and router

#### 1.1 Create Package Structure
```bash
mkdir -p internal/notifications/providers
touch internal/notifications/notifier.go
touch internal/notifications/message.go
touch internal/notifications/provider.go
touch internal/notifications/router.go
touch internal/notifications/formatters.go
```

#### 1.2 Define Core Interfaces (150 lines)
**File**: `internal/notifications/provider.go`
- [ ] Define `Provider` interface
- [ ] Define `Message` struct with all fields
- [ ] Define `HostInfo` and `Attachment` structs
- [ ] Add severity constants

#### 1.3 Implement Message Builder (100 lines)
**File**: `internal/notifications/message.go`
- [ ] Create `MessageBuilder` with fluent API
- [ ] Add validation for required fields
- [ ] Implement severity-based defaults
- [ ] Add helper methods for common message types

#### 1.4 Implement Router Logic (250 lines)
**File**: `internal/notifications/router.go`
- [ ] Create `Notifier` struct
- [ ] Implement provider registration
- [ ] Build severity-based routing logic
- [ ] Add retry logic with exponential backoff
- [ ] Implement concurrent notification sending
- [ ] Add error aggregation

#### 1.5 Configuration Integration (150 lines)
**File**: `internal/config/notifications.go`
- [ ] Add `NotificationConfig` struct to main config
- [ ] Define provider-specific config structs
- [ ] Add routing configuration (severity → providers mapping)
- [ ] Implement validation logic
- [ ] Add environment variable support

**Deliverables**: Core infrastructure (~650 lines)

---

### Step 2: High-Priority Providers (Week 2)

**Goal**: Implement the 4 most critical providers

#### 2.1 Slack Provider (300 lines)
**File**: `internal/notifications/providers/slack.go`

**Features**:
- Webhook-based message posting
- Rich formatting with Slack Block Kit
- Thread support for related messages
- Emoji and mention support
- Error handling with rate limit detection

**Implementation**:
```go
type SlackProvider struct {
    webhookURL string
    channel    string
    username   string
    iconEmoji  string
    logger     *logrus.Logger
}

func (s *SlackProvider) Send(ctx context.Context, msg *Message) error {
    payload := s.formatMessage(msg)
    resp, err := s.sendWebhook(ctx, payload)
    if err != nil {
        return fmt.Errorf("slack webhook failed: %w", err)
    }
    return s.handleResponse(resp)
}

func (s *SlackProvider) formatMessage(msg *Message) *SlackMessage {
    blocks := []Block{
        {Type: "header", Text: msg.Title},
        {Type: "section", Text: msg.Body},
    }

    // Add severity indicator
    blocks = append(blocks, s.severityBlock(msg.Severity))

    // Add fields
    if len(msg.Fields) > 0 {
        blocks = append(blocks, s.fieldsBlock(msg.Fields))
    }

    return &SlackMessage{
        Channel:  s.channel,
        Username: s.username,
        Blocks:   blocks,
    }
}
```

**Tasks**:
- [ ] Implement webhook POST logic
- [ ] Build Slack Block Kit formatter
- [ ] Add severity-based color coding
- [ ] Implement rate limit handling
- [ ] Add retry logic (3 attempts with backoff)
- [ ] Write unit tests (100 lines)

#### 2.2 Microsoft Teams Provider (350 lines)
**File**: `internal/notifications/providers/teams.go`

**Features**:
- Webhook connector integration
- Adaptive Card formatting (richer than Slack)
- Theme color based on severity
- Action buttons for interactive messages
- Fact sets for structured data

**Implementation**:
```go
type TeamsProvider struct {
    webhookURL string
    themeColor string
    logger     *logrus.Logger
}

func (t *TeamsProvider) formatMessage(msg *Message) *TeamsCard {
    card := &AdaptiveCard{
        Type:    "AdaptiveCard",
        Version: "1.4",
        Body: []CardElement{
            {Type: "TextBlock", Size: "Large", Weight: "Bolder", Text: msg.Title},
            {Type: "TextBlock", Text: msg.Body, Wrap: true},
        },
    }

    // Add severity container with color
    card.Body = append(card.Body, t.severityContainer(msg.Severity))

    // Add facts (fields)
    if len(msg.Fields) > 0 {
        card.Body = append(card.Body, t.factSet(msg.Fields))
    }

    return &TeamsCard{
        Type:       "message",
        ThemeColor: t.getSeverityColor(msg.Severity),
        Attachments: []Attachment{{
            ContentType: "application/vnd.microsoft.card.adaptive",
            Content:     card,
        }},
    }
}
```

**Tasks**:
- [ ] Implement webhook POST logic
- [ ] Build Adaptive Card formatter
- [ ] Add severity-based theme colors
- [ ] Implement action button support
- [ ] Add retry logic
- [ ] Write unit tests (120 lines)

#### 2.3 Telegram Provider (200 lines)
**File**: `internal/notifications/providers/telegram.go`

**Features**:
- Bot API integration
- Markdown/HTML message formatting
- Inline keyboard buttons
- Photo/document attachments
- Silent notifications for low-priority

**Implementation**:
```go
type TelegramProvider struct {
    botToken  string
    chatID    string
    parseMode string  // "Markdown" or "HTML"
    logger    *logrus.Logger
}

func (t *TelegramProvider) Send(ctx context.Context, msg *Message) error {
    apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

    payload := map[string]interface{}{
        "chat_id":    t.chatID,
        "text":       t.formatMessage(msg),
        "parse_mode": t.parseMode,
    }

    // Add silent mode for info/warning
    if msg.Severity <= diagnostics.SeverityWarning {
        payload["disable_notification"] = true
    }

    return t.sendRequest(ctx, apiURL, payload)
}

func (t *TelegramProvider) formatMessage(msg *Message) string {
    var buf bytes.Buffer

    // Severity emoji
    buf.WriteString(t.severityEmoji(msg.Severity))
    buf.WriteString(" *")
    buf.WriteString(msg.Title)
    buf.WriteString("*\n\n")
    buf.WriteString(msg.Body)

    // Add fields
    if len(msg.Fields) > 0 {
        buf.WriteString("\n\n*Details:*\n")
        for k, v := range msg.Fields {
            buf.WriteString(fmt.Sprintf("• %s: `%s`\n", k, v))
        }
    }

    return buf.String()
}
```

**Tasks**:
- [ ] Implement Bot API client
- [ ] Build Markdown formatter
- [ ] Add severity-based emojis
- [ ] Implement silent mode for low-priority
- [ ] Add retry logic
- [ ] Write unit tests (80 lines)

#### 2.4 Email Provider (SMTP) (250 lines)
**File**: `internal/notifications/providers/email.go`

**Features**:
- SMTP authentication (plain, login, CRAM-MD5)
- HTML + plain text multipart messages
- Attachment support
- TLS encryption
- Template-based formatting

**Implementation**:
```go
type EmailProvider struct {
    smtpHost string
    smtpPort int
    from     string
    to       []string
    auth     smtp.Auth
    logger   *logrus.Logger
}

func (e *EmailProvider) Send(ctx context.Context, msg *Message) error {
    email := e.buildEmail(msg)

    addr := fmt.Sprintf("%s:%d", e.smtpHost, e.smtpPort)
    return smtp.SendMail(addr, e.auth, e.from, e.to, email)
}

func (e *EmailProvider) buildEmail(msg *Message) []byte {
    var buf bytes.Buffer

    // Headers
    buf.WriteString(fmt.Sprintf("From: %s\r\n", e.from))
    buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(e.to, ", ")))
    buf.WriteString(fmt.Sprintf("Subject: [%s] %s\r\n", msg.Severity, msg.Title))
    buf.WriteString("MIME-Version: 1.0\r\n")
    buf.WriteString("Content-Type: multipart/alternative; boundary=\"boundary\"\r\n\r\n")

    // Plain text version
    buf.WriteString("--boundary\r\n")
    buf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n")
    buf.WriteString(e.formatPlainText(msg))

    // HTML version
    buf.WriteString("\r\n--boundary\r\n")
    buf.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n\r\n")
    buf.WriteString(e.formatHTML(msg))

    buf.WriteString("\r\n--boundary--\r\n")

    return buf.Bytes()
}
```

**Tasks**:
- [ ] Implement SMTP client with TLS
- [ ] Build HTML email template
- [ ] Build plain text template
- [ ] Add attachment support
- [ ] Implement authentication methods
- [ ] Write unit tests (90 lines)

**Deliverables**: 4 high-priority providers (~1,100 lines + 390 test lines)

---

### Step 3: Incident Management Providers (Week 3)

**Goal**: Integrate with PagerDuty and Opsgenie for critical alerts

#### 3.1 PagerDuty Provider (300 lines)
**File**: `internal/notifications/providers/pagerduty.go`

**Features**:
- Events API v2 integration
- Incident creation with severity mapping
- Deduplication key support
- Custom incident details
- Link to Lumo diagnostics

**API Integration**:
```go
type PagerDutyProvider struct {
    integrationKey string
    routingKey     string
    logger         *logrus.Logger
}

func (p *PagerDutyProvider) Send(ctx context.Context, msg *Message) error {
    event := &PagerDutyEvent{
        RoutingKey:  p.routingKey,
        EventAction: "trigger",
        DedupKey:    p.generateDedupKey(msg),
        Payload: &Payload{
            Summary:   msg.Title,
            Source:    msg.HostInfo.Hostname,
            Severity:  p.mapSeverity(msg.Severity),
            Timestamp: msg.Timestamp.Format(time.RFC3339),
            CustomDetails: msg.Fields,
        },
    }

    return p.sendEvent(ctx, event)
}

func (p *PagerDutyProvider) mapSeverity(s diagnostics.Severity) string {
    switch s {
    case diagnostics.SeverityCritical:
        return "critical"
    case diagnostics.SeverityError:
        return "error"
    case diagnostics.SeverityWarning:
        return "warning"
    default:
        return "info"
    }
}
```

**Tasks**:
- [ ] Implement Events API v2 client
- [ ] Add deduplication logic
- [ ] Map Lumo severity to PagerDuty severity
- [ ] Add custom incident details
- [ ] Implement retry logic
- [ ] Write unit tests (100 lines)

#### 3.2 Opsgenie Provider (300 lines)
**File**: `internal/notifications/providers/opsgenie.go`

**Features**:
- Alert API integration
- Priority mapping (P1-P5)
- Tag-based routing
- Responder assignment
- Custom alert properties

**Tasks**:
- [ ] Implement Alert API client
- [ ] Add priority mapping
- [ ] Implement tag-based routing
- [ ] Add custom properties
- [ ] Write unit tests (100 lines)

**Deliverables**: 2 incident management providers (~600 lines + 200 test lines)

---

### Step 4: Additional Chat Providers (Week 4)

**Goal**: Add Discord and self-hosted chat platforms

#### 4.1 Discord Provider (200 lines)
**File**: `internal/notifications/providers/discord.go`

**Features**:
- Webhook integration
- Rich embeds with color coding
- Field support for structured data
- Mention support (@role, @user)

**Tasks**:
- [ ] Implement webhook client
- [ ] Build embed formatter
- [ ] Add severity-based colors
- [ ] Write unit tests (80 lines)

#### 4.2 Mattermost Provider (150 lines)
**File**: `internal/notifications/providers/mattermost.go`

**Features**:
- Slack-compatible webhooks
- Markdown formatting
- Attachment support

**Tasks**:
- [ ] Implement webhook client (reuse Slack logic)
- [ ] Test compatibility
- [ ] Write unit tests (60 lines)

#### 4.3 Rocket.Chat Provider (150 lines)
**File**: `internal/notifications/providers/rocketchat.go`

**Features**:
- REST API integration
- Markdown formatting
- Emoji support

**Tasks**:
- [ ] Implement REST API client
- [ ] Build message formatter
- [ ] Write unit tests (60 lines)

#### 4.4 Generic Webhook Provider (150 lines)
**File**: `internal/notifications/providers/webhook.go`

**Features**:
- Custom HTTP endpoint
- Configurable headers
- JSON payload template
- Retry logic

**Implementation**:
```go
type WebhookProvider struct {
    url     string
    method  string
    headers map[string]string
    logger  *logrus.Logger
}

func (w *WebhookProvider) Send(ctx context.Context, msg *Message) error {
    payload := map[string]interface{}{
        "severity":  msg.Severity.String(),
        "title":     msg.Title,
        "body":      msg.Body,
        "fields":    msg.Fields,
        "timestamp": msg.Timestamp.Unix(),
        "host":      msg.HostInfo,
    }

    jsonData, err := json.Marshal(payload)
    if err != nil {
        return fmt.Errorf("failed to marshal payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, w.method, w.url, bytes.NewReader(jsonData))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    // Add custom headers
    for k, v := range w.headers {
        req.Header.Set(k, v)
    }
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("webhook request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("webhook returned error: %d", resp.StatusCode)
    }

    return nil
}
```

**Tasks**:
- [ ] Implement generic HTTP client
- [ ] Add custom header support
- [ ] Add template support for payloads
- [ ] Write unit tests (60 lines)

**Deliverables**: 4 additional providers (~650 lines + 260 test lines)

---

### Step 5: CLI Integration (Week 5)

**Goal**: Add CLI commands and flags for notification management

#### 5.1 Add `notify` Command (200 lines)
**File**: `cmd/lumo/notify.go`

**Features**:
```bash
# Test notification configuration
lumo notify test --provider slack
lumo notify test --provider all

# Send custom message
lumo notify send --provider slack --title "Test" --body "Message"

# List configured providers
lumo notify list

# Check provider health
lumo notify health --provider slack
```

**Implementation**:
```go
var notifyCmd = &cobra.Command{
    Use:   "notify",
    Short: "Manage and test notifications",
    Long:  "Send test notifications, list providers, and check health",
}

var notifyTestCmd = &cobra.Command{
    Use:   "test",
    Short: "Test notification providers",
    Run:   runNotifyTest,
}

var notifySendCmd = &cobra.Command{
    Use:   "send",
    Short: "Send a custom notification",
    Run:   runNotifySend,
}

func runNotifyTest(cmd *cobra.Command, args []string) {
    provider, _ := cmd.Flags().GetString("provider")

    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    notifier := notifications.NewNotifier(cfg.Notifications, log)

    testMsg := &notifications.Message{
        Severity:  diagnostics.SeverityInfo,
        Title:     "Lumo Notification Test",
        Body:      "This is a test notification from Lumo",
        Timestamp: time.Now(),
        Source:    "cli-test",
        Fields: map[string]string{
            "Test Type": "Configuration Validation",
            "Provider":  provider,
        },
    }

    if provider == "all" {
        err = notifier.NotifyAll(context.Background(), testMsg)
    } else {
        err = notifier.NotifyProvider(context.Background(), provider, testMsg)
    }

    if err != nil {
        log.Errorf("Test failed: %v", err)
        os.Exit(1)
    }

    log.Info("✅ Test notification sent successfully")
}
```

**Tasks**:
- [ ] Create `notify` root command
- [ ] Implement `notify test` subcommand
- [ ] Implement `notify send` subcommand
- [ ] Implement `notify list` subcommand
- [ ] Implement `notify health` subcommand
- [ ] Add comprehensive flags
- [ ] Write integration tests (150 lines)

#### 5.2 Add Flags to Existing Commands (100 lines)

**`diagnose` command**:
```bash
lumo diagnose host --notify slack
lumo diagnose host --notify-on critical
lumo diagnose host --notify-on error,critical
```

**`fix` command** (future):
```bash
lumo fix host --notify teams
lumo fix host --interactive --notify slack
```

**Tasks**:
- [ ] Add `--notify` flag to `diagnose` command
- [ ] Add `--notify-on` flag for severity filtering
- [ ] Integrate with diagnostic runner
- [ ] Add notification after analysis complete
- [ ] Write integration tests (100 lines)

**Deliverables**: CLI integration (~300 lines + 250 test lines)

---

### Step 6: Integration Hooks (Week 6)

**Goal**: Integrate notifications with diagnostics, remediation, and API events

#### 6.1 Diagnostics Integration (150 lines)
**File**: `cmd/lumo/diagnose.go` (modifications)

**Hook Points**:
1. After diagnostics complete (if `--notify` specified)
2. When critical issues detected (if `--notify-on critical`)
3. After AI analysis (include AI recommendations)

**Implementation**:
```go
func runDiagnostics(cmd *cobra.Command, args []string) {
    // ... existing diagnostic logic ...

    report, err := runner.Run(ctx, selectedChecks)
    if err != nil {
        log.Fatalf("Diagnostics failed: %v", err)
    }

    // Check if notifications enabled
    notifyProvider, _ := cmd.Flags().GetString("notify")
    notifyOn, _ := cmd.Flags().GetStringSlice("notify-on")

    if notifyProvider != "" || len(notifyOn) > 0 {
        if shouldNotify(report, notifyOn) {
            sendDiagnosticNotification(cfg, report, notifyProvider)
        }
    }
}

func sendDiagnosticNotification(cfg *config.Config, report *diagnostics.Report, provider string) {
    notifier := notifications.NewNotifier(cfg.Notifications, log)

    msg := &notifications.Message{
        Severity:  report.HighestSeverity(),
        Title:     fmt.Sprintf("Diagnostic Report: %s", report.Hostname),
        Body:      buildDiagnosticSummary(report),
        Timestamp: time.Now(),
        Source:    "diagnostics",
        HostInfo: &notifications.HostInfo{
            Hostname: report.Hostname,
            Platform: report.Platform,
        },
        Fields: buildDiagnosticFields(report),
    }

    if provider != "" {
        notifier.NotifyProvider(context.Background(), provider, msg)
    } else {
        notifier.Notify(context.Background(), msg)
    }
}
```

**Tasks**:
- [ ] Add notification hook after diagnostics
- [ ] Build diagnostic summary formatter
- [ ] Add severity filtering logic
- [ ] Include AI analysis results if available
- [ ] Write integration tests (80 lines)

#### 6.2 Future Remediation Integration (100 lines)
**File**: `cmd/lumo/fix.go` (future implementation)

**Hook Points**:
1. Before remediation (approval request)
2. After remediation (success/failure notification)
3. On rollback

**Placeholder Implementation**:
```go
// Notification messages for remediation phase
func notifyRemediationProposed(notifier *notifications.Notifier, action *remediation.Action) {
    msg := &notifications.Message{
        Severity:  diagnostics.SeverityWarning,
        Title:     "🔧 Remediation Action Proposed",
        Body:      fmt.Sprintf("Lumo proposes to %s", action.Description),
        Source:    "remediation",
        Fields: map[string]string{
            "Action":   action.Type,
            "Risk":     action.Risk.String(),
            "Hostname": action.Hostname,
        },
    }
    notifier.Notify(context.Background(), msg)
}

func notifyRemediationComplete(notifier *notifications.Notifier, result *remediation.Result) {
    severity := diagnostics.SeverityInfo
    if result.Success {
        severity = diagnostics.SeverityOK
    } else {
        severity = diagnostics.SeverityError
    }

    msg := &notifications.Message{
        Severity:  severity,
        Title:     "✅ Remediation Complete",
        Body:      result.Summary,
        Source:    "remediation",
        Fields:    result.Details,
    }
    notifier.Notify(context.Background(), msg)
}
```

**Tasks**:
- [ ] Define notification messages for remediation
- [ ] Add approval request formatting
- [ ] Add success/failure formatting
- [ ] Document integration points

#### 6.3 Future API Server Integration (100 lines)
**File**: `cmd/lumo/serve.go` (future implementation)

**Hook Points**:
1. API server start/stop
2. Critical API errors
3. Webhook events
4. Scheduled diagnostic runs

**Tasks**:
- [ ] Define API event messages
- [ ] Document integration points

**Deliverables**: Integration hooks (~350 lines + 80 test lines)

---

### Step 7: Configuration & Documentation (Week 7)

**Goal**: Complete configuration integration and documentation

#### 7.1 Update Configuration Files (200 lines)

**Update `configs/config.example.yaml`**:
```yaml
# Notification Configuration
notifications:
  enabled: true

  # Severity-based routing: which providers get which severity levels
  routing:
    critical: [pagerduty, slack, email]  # Page on-call + notify team
    error: [slack, email]                # Team notification
    warning: [slack]                     # Passive notification
    info: []                             # No notifications

  # Provider configurations
  providers:
    # Slack (Webhooks)
    slack:
      enabled: true
      webhook_url: env:LUMO_SLACK_WEBHOOK_URL  # Required
      channel: "#lumo-alerts"                  # Optional, override webhook default
      username: "Lumo Bot"                     # Optional
      icon_emoji: ":robot_face:"               # Optional

    # Microsoft Teams (Webhooks)
    teams:
      enabled: true
      webhook_url: env:LUMO_TEAMS_WEBHOOK_URL  # Required
      theme_color: "0078D7"                    # Optional, hex color

    # Telegram (Bot API)
    telegram:
      enabled: true
      bot_token: env:LUMO_TELEGRAM_BOT_TOKEN   # Required
      chat_id: env:LUMO_TELEGRAM_CHAT_ID       # Required
      parse_mode: "Markdown"                   # Optional: Markdown or HTML

    # Discord (Webhooks)
    discord:
      enabled: false
      webhook_url: env:LUMO_DISCORD_WEBHOOK_URL

    # Email (SMTP)
    email:
      enabled: true
      smtp_host: smtp.gmail.com
      smtp_port: 587
      from: env:LUMO_EMAIL_FROM
      to:
        - oncall@company.com
        - team@company.com
      auth:
        username: env:LUMO_SMTP_USER
        password: env:LUMO_SMTP_PASSWORD

    # PagerDuty (Events API v2)
    pagerduty:
      enabled: true
      integration_key: env:LUMO_PAGERDUTY_INTEGRATION_KEY
      routing_key: env:LUMO_PAGERDUTY_ROUTING_KEY  # Optional

    # Opsgenie (Alert API)
    opsgenie:
      enabled: false
      api_key: env:LUMO_OPSGENIE_API_KEY
      api_url: https://api.opsgenie.com  # Optional, EU: https://api.eu.opsgenie.com

    # Mattermost (Slack-compatible webhooks)
    mattermost:
      enabled: false
      webhook_url: env:LUMO_MATTERMOST_WEBHOOK_URL

    # Rocket.Chat (REST API)
    rocketchat:
      enabled: false
      webhook_url: env:LUMO_ROCKETCHAT_WEBHOOK_URL

    # Generic Webhook (Custom integrations)
    webhook:
      enabled: false
      url: env:LUMO_WEBHOOK_URL
      method: POST
      headers:
        Authorization: "Bearer ${LUMO_WEBHOOK_TOKEN}"
        X-Custom-Header: "value"
```

**Update `internal/config/config.go`**:
```go
type Config struct {
    SSH           SSHConfig
    AI            AIConfig
    Logging       LoggingConfig
    API           APIConfig
    Diagnostics   DiagnosticsConfig
    Notifications NotificationConfig  // NEW
}

type NotificationConfig struct {
    Enabled   bool
    Routing   map[string][]string  // severity -> providers
    Providers ProviderConfigs
}

type ProviderConfigs struct {
    Slack       *SlackConfig
    Teams       *TeamsConfig
    Telegram    *TelegramConfig
    Discord     *DiscordConfig
    Email       *EmailConfig
    PagerDuty   *PagerDutyConfig
    Opsgenie    *OpsgenieConfig
    Mattermost  *MattermostConfig
    RocketChat  *RocketChatConfig
    Webhook     *WebhookConfig
}

type SlackConfig struct {
    Enabled    bool
    WebhookURL string `mapstructure:"webhook_url"`
    Channel    string
    Username   string
    IconEmoji  string `mapstructure:"icon_emoji"`
}

// ... similar structs for other providers
```

**Tasks**:
- [ ] Update config.example.yaml with all providers
- [ ] Add NotificationConfig to main config struct
- [ ] Define all provider config structs
- [ ] Add validation logic for required fields
- [ ] Write config tests (100 lines)

#### 7.2 Update DEVELOPMENT.md (300 lines)

Add comprehensive guide for:
- [ ] Notification configuration examples
- [ ] Provider-specific setup instructions
- [ ] Testing notifications
- [ ] Troubleshooting common issues
- [ ] Security best practices

#### 7.3 Create Provider Documentation (400 lines)

**New file**: `docs/NOTIFICATIONS.md`

Sections:
- [ ] Overview and architecture
- [ ] Provider comparison matrix
- [ ] Setup guide for each provider
- [ ] Configuration examples
- [ ] CLI usage examples
- [ ] API integration examples
- [ ] Troubleshooting guide

**Deliverables**: Configuration and docs (~900 lines)

---

### Step 8: Testing (Week 8)

**Goal**: Achieve 70%+ test coverage

#### 8.1 Unit Tests (~1,200 lines)

**Core Infrastructure Tests** (300 lines):
- `internal/notifications/notifier_test.go`
  - [ ] Provider registration
  - [ ] Routing logic
  - [ ] Severity filtering
  - [ ] Error handling
  - [ ] Concurrent notifications

**Provider Tests** (900 lines total, ~90 per provider):
- `internal/notifications/providers/slack_test.go`
- `internal/notifications/providers/teams_test.go`
- `internal/notifications/providers/telegram_test.go`
- `internal/notifications/providers/discord_test.go`
- `internal/notifications/providers/email_test.go`
- `internal/notifications/providers/pagerduty_test.go`
- `internal/notifications/providers/opsgenie_test.go`
- `internal/notifications/providers/mattermost_test.go`
- `internal/notifications/providers/rocketchat_test.go`
- `internal/notifications/providers/webhook_test.go`

**Test Pattern**:
```go
func TestSlackProvider_Send(t *testing.T) {
    tests := []struct {
        name       string
        msg        *Message
        mockStatus int
        wantErr    bool
    }{
        {
            name: "successful send",
            msg: &Message{
                Severity: diagnostics.SeverityCritical,
                Title:    "Test Alert",
                Body:     "Test body",
            },
            mockStatus: 200,
            wantErr:    false,
        },
        {
            name: "rate limited",
            msg: &Message{
                Severity: diagnostics.SeverityError,
                Title:    "Test",
            },
            mockStatus: 429,
            wantErr:    true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                w.WriteHeader(tt.mockStatus)
            }))
            defer server.Close()

            provider := &SlackProvider{
                webhookURL: server.URL,
                logger:     logrus.New(),
            }

            err := provider.Send(context.Background(), tt.msg)
            if (err != nil) != tt.wantErr {
                t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

#### 8.2 Integration Tests (800 lines)

**CLI Integration Tests** (400 lines):
- `cmd/lumo/notify_test.go`
  - [ ] Test `notify test` command
  - [ ] Test `notify send` command
  - [ ] Test `notify list` command
  - [ ] Test provider health checks

**Diagnostic Integration Tests** (400 lines):
- `cmd/lumo/diagnose_notification_test.go`
  - [ ] Test `--notify` flag
  - [ ] Test `--notify-on` severity filtering
  - [ ] Test notification formatting
  - [ ] Test error handling

**Mock Testing Infrastructure**:
```go
// internal/notifications/testing.go
type MockProvider struct {
    SendFunc   func(ctx context.Context, msg *Message) error
    HealthFunc func(ctx context.Context) error
    name       string
}

func (m *MockProvider) Name() string {
    return m.name
}

func (m *MockProvider) Send(ctx context.Context, msg *Message) error {
    if m.SendFunc != nil {
        return m.SendFunc(ctx, msg)
    }
    return nil
}

func (m *MockProvider) Health(ctx context.Context) error {
    if m.HealthFunc != nil {
        return m.HealthFunc(ctx)
    }
    return nil
}
```

**Deliverables**: Comprehensive tests (~2,000 lines)

---

## Implementation Timeline

### 8-Week Sprint Plan

| Week | Focus | Deliverables | LOC |
|------|-------|--------------|-----|
| **1** | Core Infrastructure | Interfaces, router, config | 650 |
| **2** | High-Priority Providers | Slack, Teams, Telegram, Email | 1,100 |
| **3** | Incident Management | PagerDuty, Opsgenie | 600 |
| **4** | Additional Providers | Discord, Mattermost, Rocket.Chat, Webhooks | 650 |
| **5** | CLI Integration | notify command, flags | 300 |
| **6** | Integration Hooks | Diagnostics, remediation, API | 350 |
| **7** | Configuration & Docs | Config files, documentation | 900 |
| **8** | Testing | Unit + integration tests | 2,000 |
| **Total** | | | **6,550** |

---

## File Structure (Complete)

```
internal/notifications/
├── notifier.go              # Core notifier with routing (250 lines)
├── message.go               # Message types and builders (100 lines)
├── provider.go              # Provider interface (100 lines)
├── router.go                # Routing logic (200 lines)
├── formatters.go            # Common formatters (150 lines)
├── testing.go               # Mock provider for tests (100 lines)
├── notifier_test.go         # Core tests (300 lines)
└── providers/
    ├── slack.go             # Slack provider (300 lines)
    ├── slack_test.go        # Slack tests (90 lines)
    ├── teams.go             # Microsoft Teams (350 lines)
    ├── teams_test.go        # Teams tests (100 lines)
    ├── telegram.go          # Telegram Bot API (200 lines)
    ├── telegram_test.go     # Telegram tests (80 lines)
    ├── discord.go           # Discord webhooks (200 lines)
    ├── discord_test.go      # Discord tests (80 lines)
    ├── email.go             # SMTP email (250 lines)
    ├── email_test.go        # Email tests (90 lines)
    ├── pagerduty.go         # PagerDuty Events API (300 lines)
    ├── pagerduty_test.go    # PagerDuty tests (100 lines)
    ├── opsgenie.go          # Opsgenie Alert API (300 lines)
    ├── opsgenie_test.go     # Opsgenie tests (100 lines)
    ├── mattermost.go        # Mattermost webhooks (150 lines)
    ├── mattermost_test.go   # Mattermost tests (60 lines)
    ├── rocketchat.go        # Rocket.Chat API (150 lines)
    ├── rocketchat_test.go   # Rocket.Chat tests (60 lines)
    ├── webhook.go           # Generic webhooks (150 lines)
    └── webhook_test.go      # Webhook tests (60 lines)

internal/config/
├── config.go                # Add NotificationConfig (100 lines)
├── notifications.go         # Provider configs (200 lines)
└── notifications_test.go    # Config tests (100 lines)

cmd/lumo/
├── notify.go                        # notify command (200 lines)
├── notify_test.go                   # notify tests (150 lines)
├── diagnose.go                      # Add --notify flags (50 lines)
└── diagnose_notification_test.go    # Integration tests (250 lines)

configs/
└── config.example.yaml              # Add notification config (150 lines)

docs/
└── NOTIFICATIONS.md                 # Provider documentation (400 lines)

Total Production Code: ~3,800 lines
Total Test Code: ~2,000 lines
Total Documentation: ~900 lines
Grand Total: ~6,700 lines
```

---

## Testing Strategy

### Test Coverage Goals

| Package | Target Coverage | Priority |
|---------|----------------|----------|
| `internal/notifications` | 80%+ | High |
| `internal/notifications/providers` | 70%+ | High |
| `internal/config` (notifications) | 80%+ | Medium |
| `cmd/lumo` (notify) | 70%+ | Medium |

### Test Types

**1. Unit Tests**:
- Mock HTTP servers for webhook testing
- Table-driven tests for message formatting
- Error path coverage
- Edge cases (empty fields, nil values)

**2. Integration Tests**:
- End-to-end CLI command testing
- Diagnostic integration testing
- Config loading and validation

**3. Manual Testing Checklist**:
- [ ] Test each provider with real credentials
- [ ] Verify message formatting in each platform
- [ ] Test severity-based routing
- [ ] Verify retry logic with network failures
- [ ] Test concurrent notifications
- [ ] Verify error handling and logging

---

## Security Considerations

### Credential Management

**✅ DO**:
- Store all credentials in environment variables
- Use provider-specific env vars (e.g., `LUMO_SLACK_WEBHOOK_URL`)
- Validate webhook URLs before use
- Redact credentials in logs
- Use TLS for all HTTP communications

**❌ DON'T**:
- Never store credentials in config files
- Never log full webhook URLs
- Never commit example credentials
- Never expose credentials in error messages

### Input Validation

**Message Content**:
- Sanitize user-provided content
- Limit message size to prevent abuse
- Validate URLs in custom fields
- Escape special characters for each provider

### Rate Limiting

**Provider Limits**:
- Slack: 1 message per second per webhook
- Teams: No official limit, recommend 1/sec
- Telegram: 30 messages per second per bot
- PagerDuty: 120 requests per minute
- Email: SMTP server dependent

**Implementation**:
```go
type RateLimiter struct {
    limiters map[string]*rate.Limiter
}

func (r *RateLimiter) Wait(ctx context.Context, provider string) error {
    limiter := r.limiters[provider]
    return limiter.Wait(ctx)
}
```

---

## Dependencies

### New Dependencies Required

| Library | Purpose | License | Size |
|---------|---------|---------|------|
| None required | All providers use net/http | - | Built-in |

**Note**: This implementation uses only Go standard library (`net/http`, `net/smtp`), no external dependencies needed.

---

## Migration & Rollout Plan

### Phase 1: Internal Testing (Week 1-2)
- Implement core + 2 providers (Slack, Email)
- Internal team testing
- Gather feedback

### Phase 2: Beta Release (Week 3-6)
- Implement remaining providers
- Beta testers validate configurations
- Fix issues, improve docs

### Phase 3: Production Release (Week 7-8)
- Complete testing
- Final documentation
- Release with Phase 10 tag

---

## Success Metrics

### Implementation Metrics
- [ ] All 10 providers implemented
- [ ] 70%+ test coverage
- [ ] Zero security vulnerabilities
- [ ] Complete documentation

### Usage Metrics (Post-Release)
- Monitor notification success rate (target: >99%)
- Track provider usage distribution
- Measure notification latency (target: <2 seconds)
- Count configuration errors

---

## Open Questions & Decisions

### To Decide:

1. **Message Persistence**: Should failed notifications be queued and retried later?
   - **Recommendation**: Yes, implement SQLite-based queue for failed messages

2. **Notification Templates**: Should we support custom message templates?
   - **Recommendation**: Phase 11 feature, start with hardcoded formats

3. **Interactive Notifications**: Support for buttons/actions in Slack/Teams?
   - **Recommendation**: Yes for Phase 6 (remediation approval), plan ahead

4. **Notification History**: Should we log all sent notifications?
   - **Recommendation**: Yes, store in SQLite with 30-day retention

5. **Batch Notifications**: Send digest of multiple issues vs individual messages?
   - **Recommendation**: Start individual, add batching in Phase 11

---

## Risk Assessment

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Provider API changes | High | Medium | Version lock, comprehensive tests |
| Rate limiting | Medium | High | Implement rate limiters, retry logic |
| Credential leaks | Critical | Low | Strict env var policy, code review |
| Message size limits | Low | Medium | Truncation logic, attachment support |
| Network failures | Medium | High | Retry logic, exponential backoff |

---

## Appendix A: Provider API References

- **Slack**: https://api.slack.com/messaging/webhooks
- **Microsoft Teams**: https://docs.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/
- **Telegram**: https://core.telegram.org/bots/api
- **Discord**: https://discord.com/developers/docs/resources/webhook
- **PagerDuty**: https://developer.pagerduty.com/docs/events-api-v2/overview/
- **Opsgenie**: https://docs.opsgenie.com/docs/alert-api

---

## Appendix B: Example Notification Messages

### Slack Example (Critical Alert)
```json
{
  "channel": "#lumo-alerts",
  "username": "Lumo Bot",
  "icon_emoji": ":robot_face:",
  "blocks": [
    {
      "type": "header",
      "text": {
        "type": "plain_text",
        "text": "🚨 Critical Alert: High Memory Usage"
      }
    },
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "Memory usage at 95% on production-web-01"
      }
    },
    {
      "type": "section",
      "fields": [
        {"type": "mrkdwn", "text": "*Severity:*\nCritical"},
        {"type": "mrkdwn", "text": "*Hostname:*\nproduction-web-01"},
        {"type": "mrkdwn", "text": "*Memory:*\n95.2%"},
        {"type": "mrkdwn", "text": "*Detected:*\n2025-11-17 14:23:45"}
      ]
    },
    {
      "type": "context",
      "elements": [
        {
          "type": "mrkdwn",
          "text": "🤖 Automated alert from Lumo | Source: diagnostics"
        }
      ]
    }
  ]
}
```

### Microsoft Teams Example (Warning)
```json
{
  "@type": "MessageCard",
  "themeColor": "FFA500",
  "summary": "Warning: Disk Space Low",
  "sections": [{
    "activityTitle": "⚠️ Warning: Disk Space Low",
    "activitySubtitle": "Database server disk at 85%",
    "facts": [
      {"name": "Severity", "value": "Warning"},
      {"name": "Hostname", "value": "db-primary-01"},
      {"name": "Disk Usage", "value": "85.3%"},
      {"name": "Available", "value": "12.4 GB"}
    ],
    "text": "Disk /dev/sda1 approaching capacity threshold"
  }],
  "potentialAction": [{
    "@type": "OpenUri",
    "name": "View Details",
    "targets": [{
      "os": "default",
      "uri": "https://lumo.example.com/reports/latest"
    }]
  }]
}
```

### Email Example (HTML)
```html
<!DOCTYPE html>
<html>
<head>
  <style>
    body { font-family: Arial, sans-serif; }
    .header { background: #dc3545; color: white; padding: 20px; }
    .content { padding: 20px; }
    .field { margin: 10px 0; }
    .label { font-weight: bold; }
  </style>
</head>
<body>
  <div class="header">
    <h1>🚨 Critical Alert: High Memory Usage</h1>
  </div>
  <div class="content">
    <p>Memory usage at 95% on production-web-01</p>

    <div class="field">
      <span class="label">Severity:</span> Critical
    </div>
    <div class="field">
      <span class="label">Hostname:</span> production-web-01
    </div>
    <div class="field">
      <span class="label">Memory Usage:</span> 95.2%
    </div>
    <div class="field">
      <span class="label">Timestamp:</span> 2025-11-17 14:23:45 UTC
    </div>

    <p style="color: #666; margin-top: 20px;">
      🤖 Automated alert from Lumo | Source: diagnostics
    </p>
  </div>
</body>
</html>
```

---

**End of Implementation Plan**

---

## Quick Start Checklist

When beginning implementation:

- [ ] Create branch `claude/phase-10-notifications`
- [ ] Set up directory structure
- [ ] Implement core interfaces (Week 1)
- [ ] Start with Slack provider (easiest to test)
- [ ] Write tests alongside implementation
- [ ] Update config.example.yaml incrementally
- [ ] Document as you go
- [ ] Test with real credentials frequently

**First Command to Implement:**
```bash
lumo notify test --provider slack
```

This will drive the core architecture and provide immediate value for testing.
