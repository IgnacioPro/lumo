package messaging

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNewMessage(t *testing.T) {
	data := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}

	msg := NewMessage(TopicDiagnostics, data)

	if msg.ID == "" {
		t.Error("Message ID should not be empty")
	}

	if msg.Topic != TopicDiagnostics {
		t.Errorf("Expected topic %s, got %s", TopicDiagnostics, msg.Topic)
	}

	if msg.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	if len(msg.Data) != 2 {
		t.Errorf("Expected 2 data fields, got %d", len(msg.Data))
	}

	if msg.Metadata == nil {
		t.Error("Metadata should not be nil")
	}
}

func TestMessageMarshal(t *testing.T) {
	msg := NewMessage(TopicAlerts, map[string]interface{}{
		"severity": "high",
		"message":  "Test alert",
	})
	msg.AgentID = "test-agent-123"
	msg.Hostname = "test-host"

	data, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	if len(data) == 0 {
		t.Error("Marshaled data should not be empty")
	}

	// Verify it's valid JSON
	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Errorf("Marshaled data is not valid JSON: %v", err)
	}
}

func TestMessageUnmarshal(t *testing.T) {
	original := NewMessage(TopicRemediation, map[string]interface{}{
		"action": "restart",
		"target": "service",
	})
	original.AgentID = "agent-456"
	original.Hostname = "hostname-test"

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	unmarshaled, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if unmarshaled.ID != original.ID {
		t.Errorf("Expected ID %s, got %s", original.ID, unmarshaled.ID)
	}

	if unmarshaled.Topic != original.Topic {
		t.Errorf("Expected topic %s, got %s", original.Topic, unmarshaled.Topic)
	}

	if unmarshaled.AgentID != original.AgentID {
		t.Errorf("Expected agent ID %s, got %s", original.AgentID, unmarshaled.AgentID)
	}

	if unmarshaled.Hostname != original.Hostname {
		t.Errorf("Expected hostname %s, got %s", original.Hostname, unmarshaled.Hostname)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Enabled {
		t.Error("Expected messaging to be disabled by default")
	}

	if cfg.Provider != "nats" {
		t.Errorf("Expected default provider 'nats', got '%s'", cfg.Provider)
	}

	if cfg.URL == "" {
		t.Error("Default URL should not be empty")
	}

	if cfg.Timeout <= 0 {
		t.Error("Default timeout should be positive")
	}

	if cfg.Retry.MaxAttempts < 1 {
		t.Error("Default retry max attempts should be at least 1")
	}

	if cfg.Retry.BaseDelay <= 0 {
		t.Error("Default retry base delay should be positive")
	}

	if cfg.DLQ.Topic == "" {
		t.Error("Default DLQ topic should not be empty")
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name: "disabled config is valid",
			cfg: &Config{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "valid nats config",
			cfg: &Config{
				Enabled:  true,
				Provider: "nats",
				URL:      "nats://localhost:4222",
				Timeout:  30 * time.Second,
				Retry: RetryConfig{
					MaxAttempts: 3,
					BaseDelay:   1 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid provider",
			cfg: &Config{
				Enabled:  true,
				Provider: "invalid",
				URL:      "localhost:4222",
				Timeout:  30 * time.Second,
				Retry: RetryConfig{
					MaxAttempts: 3,
					BaseDelay:   1 * time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "empty URL",
			cfg: &Config{
				Enabled:  true,
				Provider: "nats",
				URL:      "",
				Timeout:  30 * time.Second,
				Retry: RetryConfig{
					MaxAttempts: 3,
					BaseDelay:   1 * time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid timeout",
			cfg: &Config{
				Enabled:  true,
				Provider: "kafka",
				URL:      "localhost:9092",
				Timeout:  0,
				Retry: RetryConfig{
					MaxAttempts: 3,
					BaseDelay:   1 * time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid retry attempts",
			cfg: &Config{
				Enabled:  true,
				Provider: "redis",
				URL:      "localhost:6379",
				Timeout:  30 * time.Second,
				Retry: RetryConfig{
					MaxAttempts: 0,
					BaseDelay:   1 * time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "TLS enabled without cert files",
			cfg: &Config{
				Enabled:  true,
				Provider: "nats",
				URL:      "nats://localhost:4222",
				Timeout:  30 * time.Second,
				TLS: TLSConfig{
					Enabled: true,
				},
				Retry: RetryConfig{
					MaxAttempts: 3,
					BaseDelay:   1 * time.Second,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMessageTopics(t *testing.T) {
	topics := []MessageTopic{
		TopicDiagnostics,
		TopicRemediation,
		TopicAlerts,
		TopicLifecycle,
		TopicMetrics,
	}

	// Ensure all topics are unique
	seen := make(map[MessageTopic]bool)
	for _, topic := range topics {
		if seen[topic] {
			t.Errorf("Duplicate topic found: %s", topic)
		}
		seen[topic] = true

		if topic == "" {
			t.Error("Topic should not be empty")
		}
	}

	if len(topics) != 5 {
		t.Errorf("Expected 5 topics, got %d", len(topics))
	}
}

func TestNoOpPublisher(t *testing.T) {
	publisher := &NoOpPublisher{}

	// Should not return errors
	msg := NewMessage(TopicDiagnostics, map[string]interface{}{
		"test": "data",
	})

	ctx := context.Background()
	if err := publisher.Publish(ctx, msg); err != nil {
		t.Errorf("NoOpPublisher.Publish() should not return error, got %v", err)
	}

	if err := publisher.PublishBatch(ctx, []*Message{msg}); err != nil {
		t.Errorf("NoOpPublisher.PublishBatch() should not return error, got %v", err)
	}

	if err := publisher.Close(); err != nil {
		t.Errorf("NoOpPublisher.Close() should not return error, got %v", err)
	}
}

func TestNoOpSubscriber(t *testing.T) {
	subscriber := &NoOpSubscriber{}

	// Should not return errors
	handler := func(ctx context.Context, msg *Message) error {
		return nil
	}

	ctx := context.Background()
	if err := subscriber.Subscribe(ctx, []MessageTopic{TopicDiagnostics}, handler); err != nil {
		t.Errorf("NoOpSubscriber.Subscribe() should not return error, got %v", err)
	}

	if err := subscriber.Unsubscribe([]MessageTopic{TopicDiagnostics}); err != nil {
		t.Errorf("NoOpSubscriber.Unsubscribe() should not return error, got %v", err)
	}

	if err := subscriber.Close(); err != nil {
		t.Errorf("NoOpSubscriber.Close() should not return error, got %v", err)
	}
}
