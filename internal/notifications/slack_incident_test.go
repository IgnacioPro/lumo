package notifications

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestSlackBlockKitHelpers(t *testing.T) {
	t.Run("slackBlockMrkdwn", func(t *testing.T) {
		result := slackBlockMrkdwn("*bold* text")
		if result["type"] != "mrkdwn" {
			t.Errorf("expected type 'mrkdwn', got '%v'", result["type"])
		}
		if result["text"] != "*bold* text" {
			t.Errorf("expected text '*bold* text', got '%v'", result["text"])
		}
	})

	t.Run("slackBlockPlainText", func(t *testing.T) {
		result := slackBlockPlainText("plain text", true)
		if result["type"] != "plain_text" {
			t.Errorf("expected type 'plain_text', got '%v'", result["type"])
		}
		if result["text"] != "plain text" {
			t.Errorf("expected text 'plain text', got '%v'", result["text"])
		}
		if result["emoji"] != true {
			t.Errorf("expected emoji true, got '%v'", result["emoji"])
		}
	})

	t.Run("slackBlockButton", func(t *testing.T) {
		result := slackBlockButton("Click me", "action_1", "val1", "primary")
		if result["type"] != "button" {
			t.Errorf("expected type 'button', got '%v'", result["type"])
		}
		if result["action_id"] != "action_1" {
			t.Errorf("expected action_id 'action_1', got '%v'", result["action_id"])
		}
		if result["value"] != "val1" {
			t.Errorf("expected value 'val1', got '%v'", result["value"])
		}
		if result["style"] != "primary" {
			t.Errorf("expected style 'primary', got '%v'", result["style"])
		}
	})

	t.Run("slackBlockButton_no_style", func(t *testing.T) {
		result := slackBlockButton("Click me", "action_1", "val1", "")
		if _, ok := result["style"]; ok {
			t.Error("expected no style key when empty string passed")
		}
	})

	t.Run("slackBlockURLButton", func(t *testing.T) {
		result := slackBlockURLButton("Open Link", "https://example.com", "primary")
		if result["type"] != "button" {
			t.Errorf("expected type 'button', got '%v'", result["type"])
		}
		if result["url"] != "https://example.com" {
			t.Errorf("expected url 'https://example.com', got '%v'", result["url"])
		}
	})

	t.Run("slackBlockOverflow", func(t *testing.T) {
		options := []map[string]interface{}{
			slackBlockOverflowOption("Option 1", "opt1"),
			slackBlockOverflowOption("Option 2", "opt2"),
		}
		result := slackBlockOverflow("overflow_action", options)
		if result["type"] != "overflow" {
			t.Errorf("expected type 'overflow', got '%v'", result["type"])
		}
		if result["action_id"] != "overflow_action" {
			t.Errorf("expected action_id 'overflow_action', got '%v'", result["action_id"])
		}
		opts := result["options"].([]map[string]interface{})
		if len(opts) != 2 {
			t.Errorf("expected 2 options, got %d", len(opts))
		}
	})

	t.Run("slackBlockHeader", func(t *testing.T) {
		result := slackBlockHeader("My Header")
		if result["type"] != "header" {
			t.Errorf("expected type 'header', got '%v'", result["type"])
		}
		text := result["text"].(map[string]interface{})
		if text["text"] != "My Header" {
			t.Errorf("expected header text 'My Header', got '%v'", text["text"])
		}
	})

	t.Run("slackBlockSection", func(t *testing.T) {
		result := slackBlockSection("Section text")
		if result["type"] != "section" {
			t.Errorf("expected type 'section', got '%v'", result["type"])
		}
	})

	t.Run("slackBlockDivider", func(t *testing.T) {
		result := slackBlockDivider()
		if result["type"] != "divider" {
			t.Errorf("expected type 'divider', got '%v'", result["type"])
		}
	})

	t.Run("slackBlockActions", func(t *testing.T) {
		elements := []map[string]interface{}{
			slackBlockButton("Btn1", "a1", "v1", ""),
		}
		result := slackBlockActions(elements)
		if result["type"] != "actions" {
			t.Errorf("expected type 'actions', got '%v'", result["type"])
		}
	})

	t.Run("slackBlockContext", func(t *testing.T) {
		elements := []map[string]interface{}{
			slackBlockMrkdwn("Context text"),
		}
		result := slackBlockContext(elements)
		if result["type"] != "context" {
			t.Errorf("expected type 'context', got '%v'", result["type"])
		}
	})
}

func TestIsIncidentNotification(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "test-slack",
		Type:       NotifierSlack,
		Enabled:    true,
		WebhookURL: "https://hooks.slack.com/test",
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	tests := []struct {
		name     string
		fields   map[string]string
		expected bool
	}{
		{
			name:     "with incident_id",
			fields:   map[string]string{"incident_id": "inc-123"},
			expected: true,
		},
		{
			name:     "with fix_hash",
			fields:   map[string]string{"fix_hash": "abc123"},
			expected: true,
		},
		{
			name:     "with both incident_id and fix_hash",
			fields:   map[string]string{"incident_id": "inc-123", "fix_hash": "abc123"},
			expected: true,
		},
		{
			name:     "without incident fields",
			fields:   map[string]string{"event_id": "evt-456"},
			expected: false,
		},
		{
			name:     "nil fields",
			fields:   nil,
			expected: false,
		},
		{
			name:     "empty fields",
			fields:   map[string]string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &Notification{Fields: tt.fields}
			result := notifier.isIncidentNotification(n)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetFieldSafe(t *testing.T) {
	tests := []struct {
		name     string
		fields   map[string]string
		key      string
		expected string
	}{
		{
			name:     "existing key",
			fields:   map[string]string{"key1": "value1"},
			key:      "key1",
			expected: "value1",
		},
		{
			name:     "missing key",
			fields:   map[string]string{"key1": "value1"},
			key:      "key2",
			expected: "",
		},
		{
			name:     "nil fields",
			fields:   nil,
			key:      "key1",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getFieldSafe(tt.fields, tt.key)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestBuildIncidentBlocks_Structure(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "test-slack",
		Type:       NotifierSlack,
		Enabled:    true,
		WebhookURL: "https://hooks.slack.com/test",
		APIBaseURL: "https://lumo.example.com",
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	notification := NewNotification(
		"Memory Leak Detected in service-x",
		"Service-x pods are experiencing OOMKilled events",
		LevelCritical,
	).
		WithField("incident_id", "inc-12345678").
		WithField("fix_hash", "fix-abc123").
		WithField("hypothesis", "Memory leak in database connection pool").
		WithField("confidence", "85%").
		WithField("recent_change", "Deployed v2.3.4 at 14:30 UTC").
		WithField("signals", "• OOMKilled 5x in 10m\n• Memory usage 98%").
		WithField("proposed_fix", "kubectl rollout undo deployment/service-x").
		WithField("rollback", "kubectl rollout undo deployment/service-x --to-revision=3").
		WithField("impact", "10% of users affected, API latency +200ms").
		WithField("_internal_key", "should not appear")

	blocks := notifier.buildIncidentBlocks(notification)

	// Should have at least: header, summary, context, impact, hypothesis, signals, fix, actions, footer
	if len(blocks) < 5 {
		t.Errorf("expected at least 5 blocks, got %d", len(blocks))
	}

	// Verify header block
	if blocks[0]["type"] != "header" {
		t.Errorf("first block should be header, got %v", blocks[0]["type"])
	}

	// Check for actions block with buttons
	hasActions := false
	for _, block := range blocks {
		if block["type"] == "actions" {
			hasActions = true
			elements := block["elements"].([]map[string]interface{})
			if len(elements) < 3 {
				t.Errorf("expected at least 3 action elements (approve, snooze overflow, decline), got %d", len(elements))
			}

			// Check for approve button
			hasApprove := false
			hasDecline := false
			hasSnooze := false
			for _, elem := range elements {
				if elem["action_id"] == "lumo_approve_fix" {
					hasApprove = true
					if elem["style"] != "primary" {
						t.Error("approve button should have primary style")
					}
				}
				if elem["action_id"] == "lumo_decline" {
					hasDecline = true
					if elem["style"] != "danger" {
						t.Error("decline button should have danger style")
					}
				}
				if elem["action_id"] == "lumo_snooze" {
					hasSnooze = true
					if elem["type"] != "overflow" {
						t.Error("snooze should be overflow menu")
					}
				}
			}
			if !hasApprove {
				t.Error("expected approve button in actions")
			}
			if !hasDecline {
				t.Error("expected decline button in actions")
			}
			if !hasSnooze {
				t.Error("expected snooze overflow in actions")
			}
		}
	}

	if !hasActions {
		t.Error("expected actions block in incident notification")
	}
}

func TestBuildIncidentBlocks_ActionPayloads(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "test-slack",
		Type:       NotifierSlack,
		Enabled:    true,
		WebhookURL: "https://hooks.slack.com/test",
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	notification := NewNotification(
		"Test Incident",
		"Test message",
		LevelCritical,
	).
		WithField("incident_id", "inc-12345").
		WithField("fix_hash", "fix-67890").
		WithField("proposed_fix", "kubectl apply -f fix.yaml")

	blocks := notifier.buildIncidentBlocks(notification)

	// Find actions block and verify payload values
	for _, block := range blocks {
		if block["type"] == "actions" {
			elements := block["elements"].([]map[string]interface{})
			for _, elem := range elements {
				switch elem["action_id"] {
				case "lumo_approve_fix":
					value := elem["value"].(string)
					if !strings.Contains(value, "inc-12345") || !strings.Contains(value, "fix-67890") {
						t.Errorf("approve button value should contain incident_id and fix_hash, got '%s'", value)
					}
				case "lumo_decline":
					value := elem["value"].(string)
					if !strings.Contains(value, "inc-12345") || !strings.Contains(value, "fix-67890") {
						t.Errorf("decline button value should contain incident_id and fix_hash, got '%s'", value)
					}
				case "lumo_snooze":
					options := elem["options"].([]map[string]interface{})
					for _, opt := range options {
						value := opt["value"].(string)
						if !strings.Contains(value, "snooze_") {
							t.Errorf("snooze option value should contain snooze_, got '%s'", value)
						}
						if !strings.Contains(value, "inc-12345") {
							t.Errorf("snooze option value should contain incident_id, got '%s'", value)
						}
					}
				}
			}
		}
	}
}

func TestBuildIncidentBlocks_SnoozeOptions(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "test-slack",
		Type:       NotifierSlack,
		Enabled:    true,
		WebhookURL: "https://hooks.slack.com/test",
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	notification := NewNotification("Test", "Test", LevelCritical).
		WithField("incident_id", "inc-123").
		WithField("fix_hash", "fix-456").
		WithField("proposed_fix", "test fix")

	blocks := notifier.buildIncidentBlocks(notification)

	// Find snooze overflow and verify options
	for _, block := range blocks {
		if block["type"] == "actions" {
			elements := block["elements"].([]map[string]interface{})
			for _, elem := range elements {
				if elem["action_id"] == "lumo_snooze" {
					options := elem["options"].([]map[string]interface{})

					// Should have 3 snooze options: 15m, 30m, 60m
					if len(options) != 3 {
						t.Errorf("expected 3 snooze options, got %d", len(options))
					}

					expectedDurations := []string{"snooze_15", "snooze_30", "snooze_60"}
					for i, opt := range options {
						value := opt["value"].(string)
						if !strings.Contains(value, expectedDurations[i]) {
							t.Errorf("snooze option %d should contain %s, got %s", i, expectedDurations[i], value)
						}
					}
				}
			}
		}
	}
}

func TestBuildIncidentBlocks_InternalFieldsSkipped(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "test-slack",
		Type:       NotifierSlack,
		Enabled:    true,
		WebhookURL: "https://hooks.slack.com/test",
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	notification := NewNotification("Test", "Test", LevelCritical).
		WithField("incident_id", "inc-123").
		WithField("_internal_key", "should not appear").
		WithField("_another_internal", "also hidden").
		WithField("visible_field", "should appear")

	blocks := notifier.buildIncidentBlocks(notification)

	// Serialize to JSON and check for internal keys
	jsonBytes, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("failed to marshal blocks: %v", err)
	}
	jsonStr := string(jsonBytes)

	if strings.Contains(jsonStr, "_internal_key") {
		t.Error("internal field _internal_key should not appear in blocks")
	}
	if strings.Contains(jsonStr, "_another_internal") {
		t.Error("internal field _another_internal should not appear in blocks")
	}
}

func TestBuildIncidentBlocks_WithRunbookURL(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "test-slack",
		Type:       NotifierSlack,
		Enabled:    true,
		WebhookURL: "https://hooks.slack.com/test",
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	notification := NewNotification("Test", "Test", LevelCritical).
		WithField("incident_id", "inc-123").
		WithField("runbook_url", "https://runbooks.example.com/memory-leak")

	blocks := notifier.buildIncidentBlocks(notification)

	// Find runbook button in actions
	hasRunbook := false
	for _, block := range blocks {
		if block["type"] == "actions" {
			elements := block["elements"].([]map[string]interface{})
			for _, elem := range elements {
				if elem["type"] == "button" {
					if url, ok := elem["url"].(string); ok {
						if strings.Contains(url, "runbooks.example.com") {
							hasRunbook = true
						}
					}
				}
			}
		}
	}

	if !hasRunbook {
		t.Error("expected runbook URL button in actions")
	}
}

func TestBuildBlocks_RoutesToIncidentLayout(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "test-slack",
		Type:       NotifierSlack,
		Enabled:    true,
		WebhookURL: "https://hooks.slack.com/test",
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	t.Run("routes to incident layout", func(t *testing.T) {
		notification := NewNotification("Incident", "Message", LevelCritical).
			WithField("incident_id", "inc-123").
			WithField("fix_hash", "fix-456").
			WithField("proposed_fix", "fix command")

		blocks := notifier.buildBlocks(notification)

		// Should have actions block (incident layout)
		hasActions := false
		for _, block := range blocks {
			if block["type"] == "actions" {
				hasActions = true
				break
			}
		}
		if !hasActions {
			t.Error("incident notification should use incident layout with actions")
		}
	})

	t.Run("routes to standard layout", func(t *testing.T) {
		notification := NewNotification("Regular Alert", "Message", LevelWarning).
			WithField("event_id", "evt-123")

		blocks := notifier.buildBlocks(notification)

		// Should have standard layout (no lumo_approve_fix action)
		for _, block := range blocks {
			if block["type"] == "actions" {
				elements := block["elements"].([]map[string]interface{})
				for _, elem := range elements {
					if elem["action_id"] == "lumo_approve_fix" {
						t.Error("standard notification should not have approve fix action")
					}
				}
			}
		}
	})
}

func TestBuildIncidentBlocks_MinimalFields(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "test-slack",
		Type:       NotifierSlack,
		Enabled:    true,
		WebhookURL: "https://hooks.slack.com/test",
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	// Test with only incident_id (minimum required)
	notification := NewNotification("Basic Incident", "Something happened", LevelWarning).
		WithField("incident_id", "inc-minimal")

	blocks := notifier.buildIncidentBlocks(notification)

	// Should still produce valid blocks
	if len(blocks) < 3 {
		t.Errorf("expected at least 3 blocks (header, summary, footer), got %d", len(blocks))
	}

	// First should be header
	if blocks[0]["type"] != "header" {
		t.Errorf("expected header block first, got %v", blocks[0]["type"])
	}
}

func TestBuildIncidentBlocks_TimeWindow(t *testing.T) {
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:       "test-slack",
		Type:       NotifierSlack,
		Enabled:    true,
		WebhookURL: "https://hooks.slack.com/test",
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}

	notification := NewNotification("Test", "Test", LevelCritical).
		WithField("incident_id", "inc-123").
		WithField("time_window", "Last 5 minutes")

	notification.Timestamp = time.Now()

	blocks := notifier.buildIncidentBlocks(notification)

	// Find context block and verify time_window is used
	jsonBytes, _ := json.Marshal(blocks)
	jsonStr := string(jsonBytes)

	if !strings.Contains(jsonStr, "Last 5 minutes") {
		t.Error("expected time_window to appear in context")
	}
}
