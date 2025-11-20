package ai

import (
	"io"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStreamParser struct {
	results []struct {
		content string
		done    bool
		err     error
	}
	callIndex int
}

func (m *mockStreamParser) ParseLine(line string) (string, bool, error) {
	if m.callIndex >= len(m.results) {
		return "", false, nil
	}
	result := m.results[m.callIndex]
	m.callIndex++
	return result.content, result.done, result.err
}

func TestStreamToChannel_Success(t *testing.T) {
	stream := `data: chunk1
data: chunk2
data: chunk3
data: [DONE]`

	parser := &mockStreamParser{
		results: []struct {
			content string
			done    bool
			err     error
		}{
			{content: "chunk1", done: false, err: nil},
			{content: "chunk2", done: false, err: nil},
			{content: "chunk3", done: false, err: nil},
			{content: "", done: true, err: nil},
		},
	}

	ch := make(chan StreamChunk, 10)
	body := io.NopCloser(strings.NewReader(stream))

	go func() {
		defer close(ch)
		err := StreamToChannel(body, parser, ch, logrus.New())
		assert.NoError(t, err)
	}()

	// Collect chunks
	var chunks []StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	// Verify chunks
	require.Len(t, chunks, 4) // 3 content chunks + 1 final chunk

	assert.Equal(t, "chunk1", chunks[0].Content)
	assert.False(t, chunks[0].Done)

	assert.Equal(t, "chunk2", chunks[1].Content)
	assert.False(t, chunks[1].Done)

	assert.Equal(t, "chunk3", chunks[2].Content)
	assert.False(t, chunks[2].Done)

	assert.Equal(t, "chunk1chunk2chunk3", chunks[3].Content)
	assert.True(t, chunks[3].Done)
}

func TestStreamToChannel_EmptyLines(t *testing.T) {
	stream := `
data: chunk1

data: chunk2

`

	parser := &mockStreamParser{
		results: []struct {
			content string
			done    bool
			err     error
		}{
			{content: "chunk1", done: false, err: nil},
			{content: "chunk2", done: false, err: nil},
		},
	}

	ch := make(chan StreamChunk, 10)
	body := io.NopCloser(strings.NewReader(stream))

	go func() {
		defer close(ch)
		err := StreamToChannel(body, parser, ch, logrus.New())
		assert.NoError(t, err)
	}()

	// Collect chunks
	var chunks []StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	// Verify chunks (should ignore empty lines)
	require.Len(t, chunks, 3) // 2 content + 1 final
}

func TestSSEStreamParser_ParseLine_Success(t *testing.T) {
	parser := &SSEStreamParser{
		DoneMarker: "[DONE]",
		ExtractContent: func(data map[string]interface{}) (string, bool, error) {
			if content, ok := data["content"].(string); ok {
				return content, false, nil
			}
			return "", false, nil
		},
	}

	tests := []struct {
		name            string
		line            string
		expectedContent string
		expectedDone    bool
		expectError     bool
	}{
		{
			name:            "valid data line",
			line:            `data: {"content": "hello"}`,
			expectedContent: "hello",
			expectedDone:    false,
			expectError:     false,
		},
		{
			name:            "done marker",
			line:            "data: [DONE]",
			expectedContent: "",
			expectedDone:    true,
			expectError:     false,
		},
		{
			name:            "non-data line",
			line:            "event: message",
			expectedContent: "",
			expectedDone:    false,
			expectError:     false,
		},
		{
			name:            "invalid json",
			line:            "data: {invalid json}",
			expectedContent: "",
			expectedDone:    false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, done, err := parser.ParseLine(tt.line)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedContent, content)
			assert.Equal(t, tt.expectedDone, done)
		})
	}
}

func TestSSEStreamParser_ExtractContent_OpenAI(t *testing.T) {
	// OpenAI-style format
	parser := &SSEStreamParser{
		DoneMarker: "[DONE]",
		ExtractContent: func(data map[string]interface{}) (string, bool, error) {
			// Extract from choices[0].delta.content
			if choices, ok := data["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if delta, ok := choice["delta"].(map[string]interface{}); ok {
						if content, ok := delta["content"].(string); ok {
							return content, false, nil
						}
					}
				}
			}
			return "", false, nil
		},
	}

	line := `data: {"choices": [{"delta": {"content": "Hello"}, "finish_reason": null}]}`
	content, done, err := parser.ParseLine(line)

	assert.NoError(t, err)
	assert.Equal(t, "Hello", content)
	assert.False(t, done)
}

func TestSSEStreamParser_ExtractContent_Anthropic(t *testing.T) {
	// Anthropic-style format
	parser := &SSEStreamParser{
		ExtractContent: func(data map[string]interface{}) (string, bool, error) {
			// Check for content_block_delta
			if eventType, ok := data["type"].(string); ok {
				switch eventType {
				case "content_block_delta":
					if delta, ok := data["delta"].(map[string]interface{}); ok {
						if deltaType, ok := delta["type"].(string); ok && deltaType == "text_delta" {
							if text, ok := delta["text"].(string); ok {
								return text, false, nil
							}
						}
					}
				case "message_stop":
					return "", true, nil
				}
			}
			return "", false, nil
		},
	}

	// Test content chunk
	line := `data: {"type": "content_block_delta", "delta": {"type": "text_delta", "text": "Hello"}}`
	content, done, err := parser.ParseLine(line)

	assert.NoError(t, err)
	assert.Equal(t, "Hello", content)
	assert.False(t, done)

	// Test stop message
	line = `data: {"type": "message_stop"}`
	content, done, err = parser.ParseLine(line)

	assert.NoError(t, err)
	assert.Equal(t, "", content)
	assert.True(t, done)
}

func TestJSONLineStreamParser_ParseLine_Success(t *testing.T) {
	parser := &JSONLineStreamParser{
		ExtractContent: func(data map[string]interface{}) (string, bool, error) {
			if message, ok := data["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok {
					if done, ok := data["done"].(bool); ok {
						return content, done, nil
					}
				}
			}
			return "", false, nil
		},
	}

	tests := []struct {
		name            string
		line            string
		expectedContent string
		expectedDone    bool
		expectError     bool
	}{
		{
			name:            "content line",
			line:            `{"message": {"content": "Hello"}, "done": false}`,
			expectedContent: "Hello",
			expectedDone:    false,
			expectError:     false,
		},
		{
			name:            "final line",
			line:            `{"message": {"content": " world"}, "done": true}`,
			expectedContent: " world",
			expectedDone:    true,
			expectError:     false,
		},
		{
			name:            "invalid json",
			line:            `{invalid}`,
			expectedContent: "",
			expectedDone:    false,
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, done, err := parser.ParseLine(tt.line)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedContent, content)
			assert.Equal(t, tt.expectedDone, done)
		})
	}
}

func TestGetNestedString(t *testing.T) {
	data := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"delta": map[string]interface{}{
					"content": "hello",
				},
			},
		},
		"message": map[string]interface{}{
			"text": "world",
		},
	}

	tests := []struct {
		name     string
		keys     []string
		expected string
	}{
		{
			name:     "nested in array",
			keys:     []string{"choices", "delta", "content"},
			expected: "hello",
		},
		{
			name:     "nested in map",
			keys:     []string{"message", "text"},
			expected: "world",
		},
		{
			name:     "non-existent key",
			keys:     []string{"foo", "bar"},
			expected: "",
		},
		{
			name:     "wrong type",
			keys:     []string{"choices", "delta", "wrong"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetNestedString(data, tt.keys...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetNestedBool(t *testing.T) {
	data := map[string]interface{}{
		"response": map[string]interface{}{
			"done":  true,
			"error": false,
		},
	}

	tests := []struct {
		name     string
		keys     []string
		expected bool
	}{
		{
			name:     "true value",
			keys:     []string{"response", "done"},
			expected: true,
		},
		{
			name:     "false value",
			keys:     []string{"response", "error"},
			expected: false,
		},
		{
			name:     "non-existent key",
			keys:     []string{"foo", "bar"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetNestedBool(data, tt.keys...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetNestedArray(t *testing.T) {
	data := map[string]interface{}{
		"response": map[string]interface{}{
			"items": []interface{}{"a", "b", "c"},
		},
	}

	tests := []struct {
		name     string
		keys     []string
		expected []interface{}
	}{
		{
			name:     "valid array",
			keys:     []string{"response", "items"},
			expected: []interface{}{"a", "b", "c"},
		},
		{
			name:     "non-existent key",
			keys:     []string{"foo", "bar"},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetNestedArray(data, tt.keys...)
			assert.Equal(t, tt.expected, result)
		})
	}
}
