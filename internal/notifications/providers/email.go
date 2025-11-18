package providers

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/notifications"
	"github.com/sirupsen/logrus"
)

// EmailProvider implements the Provider interface for email notifications via SMTP
type EmailProvider struct {
	smtpHost string
	smtpPort int
	from     string
	to       []string
	auth     smtp.Auth
	logger   *logrus.Logger
}

// EmailConfig contains configuration for the Email provider
type EmailConfig struct {
	Enabled  bool     `mapstructure:"enabled"`
	SMTPHost string   `mapstructure:"smtp_host"`
	SMTPPort int      `mapstructure:"smtp_port"`
	From     string   `mapstructure:"from"`
	To       []string `mapstructure:"to"`
	Auth     AuthConfig `mapstructure:"auth"`
}

// AuthConfig contains SMTP authentication settings
type AuthConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// NewEmailProvider creates a new email notification provider
func NewEmailProvider(config *EmailConfig, logger *logrus.Logger) (*EmailProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP host is required")
	}

	if config.SMTPPort == 0 {
		config.SMTPPort = 587 // Default SMTP port
	}

	if config.From == "" {
		return nil, fmt.Errorf("from address is required")
	}

	if len(config.To) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}

	if logger == nil {
		logger = logrus.New()
	}

	// Set up authentication
	var auth smtp.Auth
	if config.Auth.Username != "" && config.Auth.Password != "" {
		auth = smtp.PlainAuth("", config.Auth.Username, config.Auth.Password, config.SMTPHost)
	}

	return &EmailProvider{
		smtpHost: config.SMTPHost,
		smtpPort: config.SMTPPort,
		from:     config.From,
		to:       config.To,
		auth:     auth,
		logger:   logger,
	}, nil
}

// Name returns the provider name
func (e *EmailProvider) Name() string {
	return "email"
}

// Send sends an email notification
func (e *EmailProvider) Send(ctx context.Context, msg *notifications.Message) error {
	if msg == nil {
		return fmt.Errorf("message cannot be nil")
	}

	email, err := e.buildEmail(msg)
	if err != nil {
		return fmt.Errorf("failed to build email: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", e.smtpHost, e.smtpPort)

	// Use context timeout if set
	if deadline, ok := ctx.Deadline(); ok {
		timeout := time.Until(deadline)
		if timeout <= 0 {
			return fmt.Errorf("context deadline exceeded")
		}
	}

	err = smtp.SendMail(addr, e.auth, e.from, e.to, email)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// Health checks if the email provider is properly configured
func (e *EmailProvider) Health(ctx context.Context) error {
	if e.smtpHost == "" {
		return fmt.Errorf("SMTP host is not configured")
	}

	if e.from == "" {
		return fmt.Errorf("from address is not configured")
	}

	if len(e.to) == 0 {
		return fmt.Errorf("no recipients configured")
	}

	// Note: We don't test the SMTP connection here to avoid authentication issues
	// Real health check would require connecting to the SMTP server

	return nil
}

// buildEmail constructs the complete email with headers and multipart content
func (e *EmailProvider) buildEmail(msg *notifications.Message) ([]byte, error) {
	var buf bytes.Buffer

	// Email headers
	buf.WriteString(fmt.Sprintf("From: %s\r\n", e.from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(e.to, ", ")))
	buf.WriteString(fmt.Sprintf("Subject: [%s] %s\r\n", strings.ToUpper(string(msg.Severity)), msg.Title))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: multipart/alternative; boundary=\"lumo-boundary\"\r\n")
	buf.WriteString("\r\n")

	// Plain text version
	buf.WriteString("--lumo-boundary\r\n")
	buf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(e.formatPlainText(msg))
	buf.WriteString("\r\n")

	// HTML version
	buf.WriteString("--lumo-boundary\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	buf.WriteString("\r\n")

	html, err := e.formatHTML(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to format HTML: %w", err)
	}

	buf.WriteString(html)
	buf.WriteString("\r\n")

	// End boundary
	buf.WriteString("--lumo-boundary--\r\n")

	return buf.Bytes(), nil
}

// formatPlainText creates a plain text version of the notification
func (e *EmailProvider) formatPlainText(msg *notifications.Message) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("%s %s\n", msg.SeverityEmoji(), msg.Title))
	buf.WriteString(strings.Repeat("=", 60))
	buf.WriteString("\n\n")
	buf.WriteString(msg.Body)
	buf.WriteString("\n\n")

	buf.WriteString(fmt.Sprintf("Severity: %s\n", msg.Severity))
	buf.WriteString(fmt.Sprintf("Source: %s\n", msg.Source))
	buf.WriteString(fmt.Sprintf("Timestamp: %s\n", msg.Timestamp.Format("2006-01-02 15:04:05 MST")))

	if msg.HostInfo != nil {
		buf.WriteString("\nHost Information:\n")
		buf.WriteString(strings.Repeat("-", 60))
		buf.WriteString("\n")

		if msg.HostInfo.Hostname != "" {
			buf.WriteString(fmt.Sprintf("Hostname: %s\n", msg.HostInfo.Hostname))
		}
		if msg.HostInfo.IP != "" {
			buf.WriteString(fmt.Sprintf("IP: %s\n", msg.HostInfo.IP))
		}
		if msg.HostInfo.Platform != "" {
			buf.WriteString(fmt.Sprintf("Platform: %s\n", msg.HostInfo.Platform))
		}
		if msg.HostInfo.OS != "" {
			buf.WriteString(fmt.Sprintf("OS: %s\n", msg.HostInfo.OS))
		}
	}

	if len(msg.Fields) > 0 {
		buf.WriteString("\nAdditional Details:\n")
		buf.WriteString(strings.Repeat("-", 60))
		buf.WriteString("\n")

		for _, field := range msg.Fields {
			buf.WriteString(fmt.Sprintf("%s: %s\n", field.Name, field.Value))
		}
	}

	buf.WriteString("\n")
	buf.WriteString(strings.Repeat("-", 60))
	buf.WriteString("\n")
	buf.WriteString("🤖 Automated alert from Lumo\n")

	return buf.String()
}

// formatHTML creates an HTML version of the notification
func (e *EmailProvider) formatHTML(msg *notifications.Message) (string, error) {
	tmpl := template.Must(template.New("email").Parse(emailTemplate))

	data := struct {
		Message *notifications.Message
		Color   string
	}{
		Message: msg,
		Color:   msg.SeverityColor(),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// emailTemplate is the HTML template for email notifications
const emailTemplate = `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<style>
		body {
			font-family: Arial, sans-serif;
			line-height: 1.6;
			color: #333;
			max-width: 600px;
			margin: 0 auto;
		}
		.header {
			background: {{.Color}};
			color: white;
			padding: 20px;
			border-radius: 5px 5px 0 0;
		}
		.header h1 {
			margin: 0;
			font-size: 24px;
		}
		.content {
			padding: 20px;
			border: 1px solid #ddd;
			border-top: none;
		}
		.field {
			margin: 10px 0;
			padding: 10px;
			background: #f8f9fa;
			border-left: 3px solid {{.Color}};
		}
		.field-name {
			font-weight: bold;
			color: #555;
		}
		.field-value {
			color: #333;
		}
		.severity {
			display: inline-block;
			padding: 5px 10px;
			background: {{.Color}};
			color: white;
			border-radius: 3px;
			font-weight: bold;
		}
		.footer {
			padding: 20px;
			text-align: center;
			color: #666;
			font-size: 12px;
			border: 1px solid #ddd;
			border-top: none;
			border-radius: 0 0 5px 5px;
			background: #f8f9fa;
		}
		table {
			width: 100%;
			border-collapse: collapse;
		}
		td {
			padding: 8px;
			border-bottom: 1px solid #eee;
		}
		td:first-child {
			font-weight: bold;
			width: 30%;
			color: #555;
		}
	</style>
</head>
<body>
	<div class="header">
		<h1>{{.Message.SeverityEmoji}} {{.Message.Title}}</h1>
	</div>
	<div class="content">
		<p>{{.Message.Body}}</p>

		<div class="field">
			<span class="field-name">Severity:</span>
			<span class="severity">{{.Message.Severity}}</span>
		</div>

		{{if .Message.HostInfo}}
		<h3>Host Information</h3>
		<table>
			{{if .Message.HostInfo.Hostname}}
			<tr>
				<td>Hostname</td>
				<td>{{.Message.HostInfo.Hostname}}</td>
			</tr>
			{{end}}
			{{if .Message.HostInfo.IP}}
			<tr>
				<td>IP Address</td>
				<td>{{.Message.HostInfo.IP}}</td>
			</tr>
			{{end}}
			{{if .Message.HostInfo.Platform}}
			<tr>
				<td>Platform</td>
				<td>{{.Message.HostInfo.Platform}}</td>
			</tr>
			{{end}}
			{{if .Message.HostInfo.OS}}
			<tr>
				<td>Operating System</td>
				<td>{{.Message.HostInfo.OS}}</td>
			</tr>
			{{end}}
		</table>
		{{end}}

		{{if .Message.Fields}}
		<h3>Additional Details</h3>
		<table>
			{{range .Message.Fields}}
			<tr>
				<td>{{.Name}}</td>
				<td>{{.Value}}</td>
			</tr>
			{{end}}
		</table>
		{{end}}

		<div class="field">
			<span class="field-name">Source:</span>
			<span class="field-value">{{.Message.Source}}</span>
		</div>

		<div class="field">
			<span class="field-name">Timestamp:</span>
			<span class="field-value">{{.Message.Timestamp.Format "2006-01-02 15:04:05 MST"}}</span>
		</div>
	</div>
	<div class="footer">
		🤖 Automated alert from Lumo
	</div>
</body>
</html>`
