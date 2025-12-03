package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestSlackNotifier_DynamicUpdate(t *testing.T) {
	// Store received requests for verification
	var mu sync.Mutex
	var postCalls []map[string]interface{}
	var updateCalls []map[string]interface{}

	// Mock Slack API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)

		switch r.URL.Path {
		case "/chat.postMessage":
			postCalls = append(postCalls, payload)
			// Return a fake timestamp
			response := map[string]interface{}{
				"ok":      true,
				"ts":      "1234.5678",
				"channel": "C12345",
			}
			_ = json.NewEncoder(w).Encode(response)
		case "/chat.update":
			updateCalls = append(updateCalls, payload)
			response := map[string]interface{}{
				"ok":      true,
				"ts":      "1234.5678",
				"channel": "C12345",
			}
			_ = json.NewEncoder(w).Encode(response)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create notifier
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:          "test-slack-dynamic",
		Type:          NotifierSlack,
		Enabled:       true,
		SlackBotToken: "xoxb-test-token",
		SlackChannel:  "C12345",
		Timeout:       5,
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}
	// Override API URL to mock server
	notifier.slackAPIURL = server.URL

	// 1. Send first notification (should be a POST)
	ctx := context.Background()
	n1 := NewNotification("Incident Started", "Something is wrong", LevelError).
		WithField("event_id", "evt-001")

	if err := notifier.Send(ctx, n1); err != nil {
		t.Fatalf("failed to send first notification: %v", err)
	}

	// Verify POST called
	if len(postCalls) != 1 {
		t.Fatalf("expected 1 POST call, got %d", len(postCalls))
	}
	if len(updateCalls) != 0 {
		t.Fatalf("expected 0 UPDATE calls, got %d", len(updateCalls))
	}

	// 2. Send update notification (should be an UPDATE)
	n2 := NewNotification("Incident Resolved", "It is fixed now", LevelSuccess).
		WithField("event_id", "evt-001")

	if err := notifier.Send(ctx, n2); err != nil {
		t.Fatalf("failed to send update notification: %v", err)
	}

	// Verify UPDATE called
	if len(postCalls) != 1 {
		t.Errorf("expected 1 POST call total, got %d", len(postCalls))
	}
	if len(updateCalls) != 1 {
		t.Fatalf("expected 1 UPDATE call, got %d", len(updateCalls))
	}

	// Verify payload of update contains correct ts
	updatePayload := updateCalls[0]
	if updatePayload["ts"] != "1234.5678" {
		t.Errorf("expected update ts '1234.5678', got '%v'", updatePayload["ts"])
	}
	// Note: Implementation details of how title is stored in blocks might vary,
	// but we can check for presence of string in the serialized payload or map.
	// In this test we just check the call happened.
}

func TestSlackNotifier_NoEventID_NoUpdate(t *testing.T) {
	// Store received requests
	var mu sync.Mutex
	var postCalls []map[string]interface{}

	// Mock Slack API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if r.URL.Path == "/chat.postMessage" {
			body, _ := io.ReadAll(r.Body)
			var payload map[string]interface{}
			_ = json.Unmarshal(body, &payload)
			postCalls = append(postCalls, payload)

			// Return unique timestamp for each
			ts := fmt.Sprintf("1234.%d", len(postCalls))

			response := map[string]interface{}{
				"ok":      true,
				"ts":      ts,
				"channel": "C12345",
			}
			_ = json.NewEncoder(w).Encode(response)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	// Create notifier
	log := logrus.New()
	log.SetOutput(io.Discard)

	config := &NotifierConfig{
		Name:          "test-slack-no-id",
		Type:          NotifierSlack,
		Enabled:       true,
		SlackBotToken: "xoxb-test-token",
		SlackChannel:  "C12345",
		Timeout:       5,
	}

	notifier, err := NewSlackNotifier(config, log)
	if err != nil {
		t.Fatalf("failed to create notifier: %v", err)
	}
	notifier.slackAPIURL = server.URL

	ctx := context.Background()

	// Send two notifications without event_id
	n1 := NewNotification("Alert 1", "Msg 1", LevelError)
	n2 := NewNotification("Alert 2", "Msg 2", LevelInfo)

	_ = notifier.Send(ctx, n1)
	_ = notifier.Send(ctx, n2)

	// Should be 2 POST calls, 0 updates
	if len(postCalls) != 2 {
		t.Errorf("expected 2 POST calls, got %d", len(postCalls))
	}
}
