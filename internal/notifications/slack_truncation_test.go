package notifications

import (
	"io"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		wantLen  int  // expected max length (not including ellipsis indicator)
		wantFull bool // should be unchanged (not truncated)
	}{
		{
			name:     "short text unchanged",
			input:    "Hello world",
			maxLen:   100,
			wantFull: true,
		},
		{
			name:     "exact length unchanged",
			input:    "Hello",
			maxLen:   5,
			wantFull: true,
		},
		{
			name:     "long text truncated",
			input:    "This is a very long text that should be truncated at some point",
			maxLen:   30,
			wantLen:  30,
			wantFull: false,
		},
		{
			name:     "truncate at word boundary",
			input:    "Hello wonderful world",
			maxLen:   15,
			wantFull: false,
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   100,
			wantFull: true,
		},
		{
			name:     "whitespace trimmed",
			input:    "  Hello world  ",
			maxLen:   100,
			wantFull: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateText(tt.input, tt.maxLen)

			if tt.wantFull {
				expected := strings.TrimSpace(tt.input)
				if result != expected {
					t.Errorf("expected unchanged text '%s', got '%s'", expected, result)
				}
			} else {
				// Should be truncated with ellipsis
				if !strings.HasSuffix(result, "…") {
					t.Errorf("expected truncated text to end with ellipsis, got '%s'", result)
				}
				// Length should be reasonable (around maxLen)
				if len(result) > tt.maxLen+10 { // +10 for ellipsis and word boundary tolerance
					t.Errorf("truncated text too long: %d chars, expected around %d", len(result), tt.maxLen)
				}
			}
		})
	}
}

func TestExtractAISnippet(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantMaxLines int
		wantMaxChars int
		wantContinue bool // should have continuation indicator
	}{
		{
			name:         "short analysis unchanged",
			input:        "Simple analysis",
			wantMaxLines: 1,
			wantContinue: false,
		},
		{
			name:         "bullet list extracts first 3",
			input:        "• Issue 1\n• Issue 2\n• Issue 3\n• Issue 4\n• Issue 5",
			wantMaxLines: 3,
			wantContinue: true,
		},
		{
			name:         "long single line uses truncateText",
			input:        strings.Repeat("This is a long analysis line. ", 20),
			wantMaxChars: maxAISnippetLength + 10, // truncateText adds ellipsis
			wantContinue: false,                   // truncateText uses … not the continuation message
		},
		{
			name:         "empty lines skipped adds continuation",
			input:        "Line 1\n\n\nLine 2\n\nLine 3",
			wantMaxLines: 3,
			wantContinue: true, // continuation added because snippet != original (empty lines removed)
		},
		{
			name:         "empty string",
			input:        "",
			wantMaxLines: 0,
			wantContinue: false,
		},
		{
			name:         "exactly 3 lines no extra",
			input:        "Line 1\nLine 2\nLine 3",
			wantMaxLines: 3,
			wantContinue: false, // no continuation because snippet == original
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAISnippet(tt.input)

			// Check continuation indicator
			hasContinue := strings.Contains(result, "…more in thread/analysis page")
			if tt.wantContinue && !hasContinue {
				t.Errorf("expected continuation indicator in result: '%s'", result)
			}
			if !tt.wantContinue && hasContinue {
				t.Errorf("unexpected continuation indicator in result: '%s'", result)
			}

			// Check line count (excluding continuation line)
			lines := strings.Split(result, "\n")
			nonEmptyLines := 0
			for _, line := range lines {
				if strings.TrimSpace(line) != "" && !strings.Contains(line, "…more") {
					nonEmptyLines++
				}
			}
			if tt.wantMaxLines > 0 && nonEmptyLines > tt.wantMaxLines {
				t.Errorf("expected max %d lines, got %d in: '%s'", tt.wantMaxLines, nonEmptyLines, result)
			}

			// Check char count
			if tt.wantMaxChars > 0 && len(result) > tt.wantMaxChars {
				t.Errorf("expected max %d chars, got %d", tt.wantMaxChars, len(result))
			}
		})
	}
}

func TestParseMessageSections_Truncation(t *testing.T) {
	notifier := &SlackNotifier{}

	tests := []struct {
		name               string
		message            string
		wantDetailsMaxLen  int
		wantSnippetPresent bool
		wantFullAIPresent  bool
	}{
		{
			name:               "simple message no AI",
			message:            "This is a simple message",
			wantDetailsMaxLen:  maxDetailsLength,
			wantSnippetPresent: false,
			wantFullAIPresent:  false,
		},
		{
			name:               "message with AI analysis",
			message:            "Event details here\n*AI Analysis:*\n• Root cause identified\n• Impact: High\n• Recommendation: Fix it",
			wantDetailsMaxLen:  maxDetailsLength,
			wantSnippetPresent: true,
			wantFullAIPresent:  true,
		},
		{
			name:               "long details truncated",
			message:            strings.Repeat("Long details content. ", 50),
			wantDetailsMaxLen:  maxDetailsLength + 10, // some tolerance
			wantSnippetPresent: false,
			wantFullAIPresent:  false,
		},
		{
			name:               "long AI analysis creates snippet",
			message:            "Short details\n*AI Analysis:*\n" + strings.Repeat("• Analysis point number one is very important. ", 20),
			wantDetailsMaxLen:  maxDetailsLength,
			wantSnippetPresent: true,
			wantFullAIPresent:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sections := notifier.parseMessageSections(tt.message)

			// Check details truncation
			if len(sections["details"]) > tt.wantDetailsMaxLen {
				t.Errorf("details too long: %d chars, expected max %d", len(sections["details"]), tt.wantDetailsMaxLen)
			}

			// Check snippet presence
			hasSnippet := sections["ai_snippet"] != ""
			if tt.wantSnippetPresent && !hasSnippet {
				t.Error("expected ai_snippet to be present")
			}
			if !tt.wantSnippetPresent && hasSnippet {
				t.Errorf("unexpected ai_snippet: '%s'", sections["ai_snippet"])
			}

			// Check full AI presence
			hasFullAI := sections["ai_analysis"] != ""
			if tt.wantFullAIPresent && !hasFullAI {
				t.Error("expected ai_analysis to be present")
			}
			if !tt.wantFullAIPresent && hasFullAI {
				t.Errorf("unexpected ai_analysis: '%s'", sections["ai_analysis"])
			}

			// Snippet should be shorter than full analysis
			if hasSnippet && hasFullAI {
				if len(sections["ai_snippet"]) > len(sections["ai_analysis"]) {
					t.Error("snippet should not be longer than full analysis")
				}
			}
		})
	}
}

func TestBuildBlocks_FieldTruncation(t *testing.T) {
	log := setupTestLogger()
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

	// Create notification with long field value
	longValue := strings.Repeat("Very long field value content here. ", 10)
	notification := NewNotification("Test Alert", "Test message", LevelWarning).
		WithField("LongField", longValue).
		WithField("ShortField", "Short value")

	blocks := notifier.buildBlocks(notification)

	// Find the fields section
	for _, block := range blocks {
		if blockType, ok := block["type"].(string); ok && blockType == "section" {
			if fields, ok := block["fields"].([]map[string]interface{}); ok {
				for _, field := range fields {
					if text, ok := field["text"].(string); ok {
						// Field text includes "*FieldName*\nvalue" format
						// Check that value part is truncated
						if strings.Contains(text, "LongField") {
							// The value should be truncated
							lines := strings.SplitN(text, "\n", 2)
							if len(lines) > 1 {
								value := lines[1]
								if len(value) > maxFieldValueLength+10 { // tolerance for ellipsis
									t.Errorf("field value not truncated: %d chars", len(value))
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestBuildBlocks_InternalFieldsHidden(t *testing.T) {
	log := setupTestLogger()
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

	// Create notification with internal field (prefixed with _)
	notification := NewNotification("Test Alert", "Test message", LevelWarning).
		WithField("_event_url", "/api/v1/events/123/analysis").
		WithField("VisibleField", "Visible value")

	blocks := notifier.buildBlocks(notification)

	// Check that internal field is not in the fields section
	for _, block := range blocks {
		if blockType, ok := block["type"].(string); ok && blockType == "section" {
			if fields, ok := block["fields"].([]map[string]interface{}); ok {
				for _, field := range fields {
					if text, ok := field["text"].(string); ok {
						if strings.Contains(text, "_event_url") {
							t.Error("internal field _event_url should not be displayed in fields")
						}
					}
				}
			}
		}
	}
}

func TestBuildBlocks_AISnippetInMainCard(t *testing.T) {
	log := setupTestLogger()
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

	// Create notification with AI analysis in message
	longAI := strings.Repeat("• Analysis point with detailed explanation. ", 20)
	message := "Event occurred\n*AI Analysis:*\n" + longAI

	notification := NewNotification("Test Alert", message, LevelWarning)

	blocks := notifier.buildBlocks(notification)

	// Find AI section and verify it contains "Preview" (indicating snippet)
	foundAIPreview := false
	for _, block := range blocks {
		if blockType, ok := block["type"].(string); ok && blockType == "section" {
			if textMap, ok := block["text"].(map[string]interface{}); ok {
				if text, ok := textMap["text"].(string); ok {
					if strings.Contains(text, "AI Analysis Preview") {
						foundAIPreview = true
						// Should not contain the full analysis
						if len(text) > maxAISnippetLength+200 { // tolerance for header and continuation
							t.Errorf("AI section in main card too long: %d chars", len(text))
						}
					}
				}
			}
		}
	}

	if !foundAIPreview {
		t.Error("expected AI Analysis Preview section in blocks")
	}
}

func setupTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}
