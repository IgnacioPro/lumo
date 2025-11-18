package logging

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeAPIKeys(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		want        string
		contains    string // substring that should be present
		notContains string // substring that should NOT be present
	}{
		{
			name:        "Anthropic API key",
			input:       "Error: unauthorized with key sk-ant-api03-1234567890abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqrstuvwxyz1234567890abcdefghijklmnopqr",
			contains:    "sk-ant-***REDACTED***",
			notContains: "sk-ant-api03-1234567890",
		},
		{
			name:        "OpenAI API key",
			input:       "Failed to call API with key sk-1234567890abcdefghijklmnopqrstuvwxyz12345678901234",
			contains:    "sk-***REDACTED***",
			notContains: "sk-1234567890abcdefgh",
		},
		{
			name:        "OpenAI project key",
			input:       "Authentication failed: sk-proj-abcdefghijklmnopqrstuvwxyz1234567890abcdefgh12345",
			contains:    "sk-proj-***REDACTED***",
			notContains: "sk-proj-abcdefghij",
		},
		{
			name:        "Google/Gemini API key",
			input:       "API error with key AIzaSyAbCdEfGhIjKlMnOpQrStUvWxYz1234567",
			contains:    "AIza***REDACTED***",
			notContains: "AIzaSyAbCdEfGhIjKl",
		},
		{
			name:        "Bearer token",
			input:       "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ",
			contains:    "Bearer ***REDACTED***",
			notContains: "eyJhbGciOiJIUzI1NiIs",
		},
		{
			name:        "OpenRouter key",
			input:       "Using key sk-or-v1-abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			contains:    "sk-or-***REDACTED***",
			notContains: "sk-or-v1-abcdef123",
		},
		{
			name:        "API key in header format",
			input:       "Request failed with api_key: sk_test_1234567890abcdefghij",
			contains:    "api_key=***REDACTED***",
			notContains: "sk_test_1234567890",
		},
		{
			name:        "Multiple keys in same string",
			input:       "Failed with keys: sk-ant-api03-abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567ABC890DEF123GHI456JKL789MNO012PQR345STU678VW and AIzaSyAbCdEfGhIjKlMnOpQrStUvWxYz1234567",
			contains:    "sk-ant-***REDACTED***",
			notContains: "abc123def456",
		},
		{
			name:  "No API keys",
			input: "This is a normal error message without any keys",
			want:  "This is a normal error message without any keys",
		},
		{
			name:  "Empty string",
			input: "",
			want:  "",
		},
		{
			name:        "Authorization header",
			input:       "HTTP request failed: authorization: sk-1234567890abcdefghijklmnopqr",
			contains:    "authorization: ***REDACTED***",
			notContains: "sk-1234567890abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeAPIKeys(tt.input)

			if tt.want != "" && result != tt.want {
				t.Errorf("SanitizeAPIKeys() = %q, want %q", result, tt.want)
			}

			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("SanitizeAPIKeys() result should contain %q, got %q", tt.contains, result)
			}

			if tt.notContains != "" && strings.Contains(result, tt.notContains) {
				t.Errorf("SanitizeAPIKeys() result should NOT contain %q, but it does. Got %q", tt.notContains, result)
			}
		})
	}
}

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		contains    string
		notContains string
	}{
		{
			name:        "error with API key",
			err:         errors.New("authentication failed with key sk-ant-api03-abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567ABC890DEF123GHI456JKL789MNO012PQR345STU678VW"),
			contains:    "sk-ant-***REDACTED***",
			notContains: "abc123def456",
		},
		{
			name:     "nil error",
			err:      nil,
			contains: "",
		},
		{
			name:     "normal error without keys",
			err:      errors.New("connection timeout"),
			contains: "connection timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeError(tt.err)

			if tt.err == nil && result != "" {
				t.Errorf("SanitizeError(nil) should return empty string, got %q", result)
			}

			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("SanitizeError() result should contain %q, got %q", tt.contains, result)
			}

			if tt.notContains != "" && strings.Contains(result, tt.notContains) {
				t.Errorf("SanitizeError() result should NOT contain %q, got %q", tt.notContains, result)
			}
		})
	}
}

func TestSanitizeMap(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		checkKey    string
		contains    string
		notContains string
	}{
		{
			name: "map with API key",
			input: map[string]interface{}{
				"error":   "authentication failed",
				"api_key": "sk-ant-api03-abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567ABC890DEF123GHI456JKL789MNO012PQR345STU678VW",
			},
			checkKey:    "api_key",
			contains:    "sk-ant-***REDACTED***",
			notContains: "abc123def456",
		},
		{
			name: "nested map",
			input: map[string]interface{}{
				"request": map[string]interface{}{
					"headers": map[string]interface{}{
						"Authorization": "Bearer sk-1234567890abcdefghijklmnopqrstuvwxyz",
					},
				},
			},
			checkKey:    "Authorization",
			contains:    "Bearer ***REDACTED***",
			notContains: "sk-1234567890",
		},
		{
			name:  "nil map",
			input: nil,
		},
		{
			name: "map without keys",
			input: map[string]interface{}{
				"status": "ok",
				"count":  42,
			},
			checkKey: "status",
			contains: "ok",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeMap(tt.input)

			if tt.input == nil && result != nil {
				t.Errorf("SanitizeMap(nil) should return nil, got %v", result)
				return
			}

			if tt.checkKey != "" && tt.input != nil {
				var value string
				// Navigate nested maps
				if tt.name == "nested map" {
					request := result["request"].(map[string]interface{})
					headers := request["headers"].(map[string]interface{})
					value = headers[tt.checkKey].(string)
				} else {
					value = result[tt.checkKey].(string)
				}

				if tt.contains != "" && !strings.Contains(value, tt.contains) {
					t.Errorf("SanitizeMap() result[%q] should contain %q, got %q", tt.checkKey, tt.contains, value)
				}

				if tt.notContains != "" && strings.Contains(value, tt.notContains) {
					t.Errorf("SanitizeMap() result[%q] should NOT contain %q, got %q", tt.checkKey, tt.notContains, value)
				}
			}
		})
	}
}

func BenchmarkSanitizeAPIKeys(b *testing.B) {
	input := "Error calling API with key sk-ant-api03-abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567ABC890DEF123GHI456JKL789MNO012PQR345STU678VW and another key AIzaSyAbCdEfGhIjKlMnOpQrStUvWxYz1234567"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SanitizeAPIKeys(input)
	}
}
