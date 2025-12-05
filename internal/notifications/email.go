package notifications

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ignacio/lumo/internal/reliability"
)

// EmailNotifier sends notifications via SMTP email.
type EmailNotifier struct {
	config         *NotifierConfig
	log            *logrus.Logger
	circuitBreaker *reliability.CircuitBreaker
}

// NewEmailNotifier creates a new email notifier.
func NewEmailNotifier(config *NotifierConfig, log *logrus.Logger) (*EmailNotifier, error) {
	if config.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP host is required")
	}
	if config.SMTPPort == 0 {
		config.SMTPPort = 587 // Default to submission port
	}
	if config.From == "" {
		return nil, fmt.Errorf("from address is required")
	}
	if len(config.To) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}

	return &EmailNotifier{
		config:         config,
		log:            log,
		circuitBreaker: reliability.NewCircuitBreaker(fmt.Sprintf("email-%s", config.Name)),
	}, nil
}

// Name returns the notifier name.
func (e *EmailNotifier) Name() string {
	return e.config.Name
}

// Send sends a notification via email.
func (e *EmailNotifier) Send(ctx context.Context, notification *Notification) (string, error) {
	// Wrap execution in circuit breaker
	_, err := e.circuitBreaker.Execute(func() (interface{}, error) {
		return nil, e.sendInternal(ctx, notification)
	})
	return "", err
}

func (e *EmailNotifier) sendInternal(ctx context.Context, notification *Notification) error {
	// Build email message
	msg := e.buildMessage(notification)

	// Connect to SMTP server
	addr := net.JoinHostPort(e.config.SMTPHost, fmt.Sprintf("%d", e.config.SMTPPort))

	// Setup authentication
	var auth smtp.Auth
	if e.config.SMTPUsername != "" && e.config.SMTPPassword != "" {
		auth = smtp.PlainAuth("", e.config.SMTPUsername, e.config.SMTPPassword, e.config.SMTPHost)
	}

	// Send email with or without TLS
	var err error
	if e.config.UseTLS {
		err = e.sendWithTLS(addr, auth, msg)
	} else {
		err = smtp.SendMail(addr, auth, e.config.From, e.config.To, []byte(msg))
	}

	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	e.log.WithFields(logrus.Fields{
		"notifier": e.Name(),
		"level":    notification.Level,
		"title":    notification.Title,
		"to":       e.config.To,
	}).Debug("notification sent successfully")

	return nil
}

// Health checks if the email configuration is valid.
func (e *EmailNotifier) Health(ctx context.Context) error {
	if e.config.SMTPHost == "" {
		return fmt.Errorf("SMTP host not configured")
	}
	if e.config.From == "" {
		return fmt.Errorf("from address not configured")
	}
	if len(e.config.To) == 0 {
		return fmt.Errorf("no recipients configured")
	}
	return nil
}

// sendWithTLS sends email with explicit TLS connection.
func (e *EmailNotifier) sendWithTLS(addr string, auth smtp.Auth, msg string) error {
	// Connect with TLS
	tlsConfig := &tls.Config{
		ServerName: e.config.SMTPHost,
		MinVersion: tls.VersionTLS12,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect with TLS: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	// Create SMTP client
	client, err := smtp.NewClient(conn, e.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer func() {
		_ = client.Close()
	}()

	// Authenticate if credentials provided
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(e.config.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, to := range e.config.To {
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", to, err)
		}
	}

	// Send message
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return client.Quit()
}

// buildMessage builds an email message from a notification.
func (e *EmailNotifier) buildMessage(notification *Notification) string {
	var sb strings.Builder

	// Email headers
	sb.WriteString(fmt.Sprintf("From: %s\r\n", e.config.From))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(e.config.To, ", ")))
	sb.WriteString(fmt.Sprintf("Subject: [%s] %s\r\n", notification.Level.String(), notification.Title))
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	sb.WriteString("Date: " + notification.Timestamp.Format(time.RFC1123Z) + "\r\n")
	sb.WriteString("\r\n")

	// Email body (HTML)
	sb.WriteString("<html><body>\r\n")
	sb.WriteString(fmt.Sprintf("<h2 style='color: %s'>%s %s</h2>\r\n",
		notification.Level.Color(),
		notification.Level.Icon(),
		htmlEscape(notification.Title)))
	sb.WriteString(fmt.Sprintf("<p>%s</p>\r\n", htmlEscape(notification.Message)))

	// Add fields if present
	if len(notification.Fields) > 0 {
		sb.WriteString("<table border='1' cellpadding='5' cellspacing='0' style='border-collapse: collapse; margin-top: 20px;'>\r\n")
		sb.WriteString("<thead><tr><th>Field</th><th>Value</th></tr></thead>\r\n")
		sb.WriteString("<tbody>\r\n")
		for key, value := range notification.Fields {
			sb.WriteString(fmt.Sprintf("<tr><td><strong>%s</strong></td><td>%s</td></tr>\r\n",
				htmlEscape(key), htmlEscape(value)))
		}
		sb.WriteString("</tbody>\r\n")
		sb.WriteString("</table>\r\n")
	}

	// Add footer
	sb.WriteString("<hr style='margin-top: 30px;'>\r\n")
	sb.WriteString(fmt.Sprintf("<p style='color: #888; font-size: 12px;'>Lumo Notification | %s</p>\r\n",
		notification.Timestamp.Format(time.RFC1123)))
	sb.WriteString("</body></html>\r\n")

	return sb.String()
}

// htmlEscape escapes HTML special characters.
func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

// TestSMTPConnection tests the SMTP connection without sending a message.
func TestSMTPConnection(host string, port int, username, password string, useTLS bool) error {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	// Try to connect
	var conn net.Conn
	var err error

	if useTLS {
		tlsConfig := &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		}
		conn, err = tls.Dial("tcp", addr, tlsConfig)
	} else {
		conn, err = net.DialTimeout("tcp", addr, 10*time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	// Create SMTP client
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer func() {
		_ = client.Close()
	}()

	// Test authentication if credentials provided
	if username != "" && password != "" {
		auth := smtp.PlainAuth("", username, password, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}

	return client.Quit()
}
