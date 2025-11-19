# Notifications Package

The notifications package provides a unified interface for sending notifications to multiple platforms including Slack, Telegram, Discord, Microsoft Teams, and email.

## Overview

The package implements a `Notifier` interface that allows you to send notifications through various channels with a consistent API. All notifiers support:

- Multiple notification levels (info, warning, error, critical, success)
- Rich message formatting with fields and tags
- Timeout control
- Health checks
- Concurrent-safe operations

## Supported Providers

### 1. Slack (Webhook)
Sends notifications via Slack incoming webhooks with rich attachments.

**Features:**
- Color-coded messages by severity level
- Structured fields display
- Timestamp footer

**Configuration:**
```yaml
notifications:
  enabled: true
  notifiers:
    - name: ops-slack
      type: slack
      enabled: true
      webhook_url: https://hooks.slack.com/services/YOUR/WEBHOOK/URL
      timeout: 30
```

**Environment Variable:**
```bash
export SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
```

### 2. Telegram (Bot API)
Sends notifications via Telegram Bot API with Markdown formatting.

**Features:**
- Markdown-formatted messages
- Automatic special character escaping
- Support for fields and structured data

**Configuration:**
```yaml
notifications:
  enabled: true
  notifiers:
    - name: alerts-telegram
      type: telegram
      enabled: true
      bot_token: ${TELEGRAM_BOT_TOKEN}
      chat_id: "-1001234567890"
      timeout: 30
```

**Environment Variables:**
```bash
export TELEGRAM_BOT_TOKEN=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz
export TELEGRAM_CHAT_ID=-1001234567890
```

**Getting Started:**
1. Create a bot via [@BotFather](https://t.me/BotFather)
2. Get your chat ID by messaging [@userinfobot](https://t.me/userinfobot)
3. Add the bot to your channel/group (for group notifications)

### 3. Generic Webhooks (Discord, Teams, Mattermost)
Sends notifications via generic webhooks with platform detection.

**Supported Platforms:**
- Discord (auto-detected via webhook URL)
- Microsoft Teams (auto-detected via webhook URL)
- Mattermost
- Any custom webhook endpoint

**Features:**
- Automatic platform detection from URL
- Custom HTTP headers
- Configurable HTTP method (GET, POST, PUT, etc.)

**Configuration:**

**Discord:**
```yaml
notifications:
  enabled: true
  notifiers:
    - name: discord-alerts
      type: webhook
      enabled: true
      webhook_url: https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_TOKEN
      timeout: 30
```

**Microsoft Teams:**
```yaml
notifications:
  enabled: true
  notifiers:
    - name: teams-notifications
      type: webhook
      enabled: true
      webhook_url: https://outlook.office.com/webhook/YOUR_WEBHOOK_URL
      timeout: 30
```

**Custom Webhook:**
```yaml
notifications:
  enabled: true
  notifiers:
    - name: custom-webhook
      type: webhook
      enabled: true
      webhook_url: https://your-api.example.com/webhook
      method: POST
      headers:
        Authorization: Bearer your-token
        X-Custom-Header: custom-value
      timeout: 30
```

### 4. Email (SMTP)
Sends HTML-formatted email notifications via SMTP.

**Features:**
- HTML formatted emails with tables
- TLS/SSL support
- SMTP authentication
- Multiple recipients
- IPv6 compatible

**Configuration:**
```yaml
notifications:
  enabled: true
  notifiers:
    - name: email-alerts
      type: email
      enabled: true
      smtp_host: smtp.gmail.com
      smtp_port: 587
      smtp_user: your-email@gmail.com
      smtp_pass: ${SMTP_PASSWORD}
      from: alerts@example.com
      to:
        - ops-team@example.com
        - admin@example.com
      use_tls: true
      timeout: 30
```

**Environment Variable:**
```bash
export SMTP_PASSWORD=your-app-password
```

**Gmail Configuration:**
1. Enable 2-factor authentication
2. Generate an [App Password](https://myaccount.google.com/apppasswords)
3. Use `smtp.gmail.com:587` with TLS

**Common SMTP Providers:**
- Gmail: `smtp.gmail.com:587` (TLS)
- Outlook: `smtp-mail.outlook.com:587` (TLS)
- SendGrid: `smtp.sendgrid.net:587` (TLS)
- AWS SES: `email-smtp.us-east-1.amazonaws.com:587` (TLS)

## Usage Examples

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/ignacio/lumo/internal/notifications"
    "github.com/sirupsen/logrus"
)

func main() {
    logger := logrus.New()

    // Create Slack notifier
    config := &notifications.NotifierConfig{
        Name:       "slack-alerts",
        Type:       notifications.NotifierSlack,
        Enabled:    true,
        WebhookURL: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL",
        Timeout:    30,
    }

    notifier, err := notifications.NewNotifier(config, logger)
    if err != nil {
        log.Fatal(err)
    }

    // Create notification
    notification := notifications.NewNotification(
        "Service Alert",
        "High CPU usage detected on server-01",
        notifications.LevelWarning,
    )

    // Add fields
    notification.
        WithField("Host", "server-01").
        WithField("CPU", "95%").
        WithField("Memory", "80%").
        WithTag("production").
        WithTag("infrastructure")

    // Send notification
    ctx := context.Background()
    if err := notifier.Send(ctx, notification); err != nil {
        log.Printf("Failed to send notification: %v", err)
    }
}
```

### Multiple Notifiers

```go
package main

import (
    "context"
    "github.com/ignacio/lumo/internal/config"
    "github.com/ignacio/lumo/internal/notifications"
    "github.com/sirupsen/logrus"
)

func main() {
    logger := logrus.New()

    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        panic(err)
    }

    // Create notifiers from config
    var notifiers []notifications.Notifier
    for _, notifierCfg := range cfg.Notifications.Notifiers {
        if !notifierCfg.Enabled {
            continue
        }

        notifier, err := notifications.NewNotifier(&notifierCfg, logger)
        if err != nil {
            logger.Errorf("Failed to create notifier %s: %v", notifierCfg.Name, err)
            continue
        }

        notifiers = append(notifiers, notifier)
    }

    // Send to all notifiers
    notification := notifications.NewNotification(
        "Deployment Complete",
        "Version v2.0.0 deployed successfully",
        notifications.LevelSuccess,
    )

    ctx := context.Background()
    for _, notifier := range notifiers {
        if err := notifier.Send(ctx, notification); err != nil {
            logger.Errorf("Failed to send via %s: %v", notifier.Name(), err)
        }
    }
}
```

### Health Checks

```go
package main

import (
    "context"
    "github.com/ignacio/lumo/internal/notifications"
)

func checkNotifierHealth(notifier notifications.Notifier) error {
    ctx := context.Background()
    return notifier.Health(ctx)
}
```

## Notification Levels

The package supports five notification levels:

| Level | Icon | Color | Use Case |
|-------|------|-------|----------|
| `LevelInfo` | ℹ️ | Blue | Informational messages |
| `LevelWarning` | ⚠️ | Orange | Warning conditions |
| `LevelError` | ❌ | Red | Error conditions |
| `LevelCritical` | 🚨 | Dark Red | Critical alerts requiring immediate attention |
| `LevelSuccess` | ✅ | Green | Successful operations |

## Architecture

### Interface

```go
type Notifier interface {
    Name() string
    Send(ctx context.Context, notification *Notification) error
    Health(ctx context.Context) error
}
```

### Components

- **notifier.go**: Core interface and factory
- **types.go**: Notification types and levels
- **slack.go**: Slack webhook implementation
- **telegram.go**: Telegram bot API implementation
- **webhook.go**: Generic webhook implementation
- **email.go**: SMTP email implementation

## Configuration Integration

The notifications package integrates with Lumo's configuration system:

```yaml
# config.yaml
notifications:
  enabled: true
  notifiers:
    - name: slack-ops
      type: slack
      enabled: true
      webhook_url: https://hooks.slack.com/...

    - name: telegram-alerts
      type: telegram
      enabled: true
      bot_token: ${TELEGRAM_BOT_TOKEN}
      chat_id: ${TELEGRAM_CHAT_ID}

    - name: email-team
      type: email
      enabled: true
      smtp_host: smtp.gmail.com
      smtp_port: 587
      smtp_user: alerts@example.com
      smtp_pass: ${SMTP_PASSWORD}
      from: alerts@example.com
      to:
        - team@example.com
      use_tls: true
```

## Environment Variables

Sensitive data should be provided via environment variables:

```bash
# Slack
export SLACK_WEBHOOK_URL=https://hooks.slack.com/services/...

# Telegram
export TELEGRAM_BOT_TOKEN=1234567890:ABC...
export TELEGRAM_CHAT_ID=-1001234567890

# Email
export SMTP_PASSWORD=your-app-password
```

## Error Handling

All notifiers return errors that can be inspected:

```go
if err := notifier.Send(ctx, notification); err != nil {
    // Handle error - notification failed
    log.Printf("Notification failed: %v", err)
}
```

Common errors:
- Network timeouts
- Authentication failures
- Invalid configuration
- Rate limiting

## Testing

Run tests with:

```bash
# All tests
go test ./internal/notifications/...

# With coverage
go test -cover ./internal/notifications/...

# With race detector
go test -race ./internal/notifications/...
```

## Performance

- **Memory**: ~5-10 KB per notifier instance
- **Concurrency**: All notifiers are safe for concurrent use
- **Timeout**: Default 30s, configurable per notifier
- **Retries**: No automatic retries (implement at application level)

## Security Considerations

1. **API Keys**: Never commit webhook URLs or API tokens to version control
2. **TLS**: Always use TLS for SMTP (port 587 or 465)
3. **Validation**: Input validation prevents injection attacks
4. **Rate Limiting**: Implement rate limiting at application level
5. **Error Messages**: Errors may contain sensitive data - sanitize before logging

## Future Enhancements

Potential additions (not yet implemented):
- PagerDuty integration
- Opsgenie integration
- AWS SNS support
- Retry mechanisms with exponential backoff
- Message queuing for reliability
- Template support for messages
- Batch notification sending

## Contributing

When adding new notifiers:

1. Implement the `Notifier` interface
2. Add configuration to `NotifierConfig`
3. Update factory in `NewNotifier()`
4. Add tests with httptest mocking
5. Update this documentation

## References

- [Slack Incoming Webhooks](https://api.slack.com/messaging/webhooks)
- [Telegram Bot API](https://core.telegram.org/bots/api)
- [Discord Webhooks](https://discord.com/developers/docs/resources/webhook)
- [Microsoft Teams Incoming Webhooks](https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook)
