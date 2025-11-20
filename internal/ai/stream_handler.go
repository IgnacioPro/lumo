package ai

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/sirupsen/logrus"
)

// StreamParser defines how to parse provider-specific streaming events.
// Each AI provider implements this interface to handle their specific stream format.
type StreamParser interface {
	// ParseLine parses a single line from the SSE stream.
	// Returns the extracted content, whether it's done, and any error.
	ParseLine(line string) (content string, done bool, err error)
}

// StreamToChannel reads an SSE stream and sends chunks to a channel.
// It uses the provided StreamParser to handle provider-specific formats.
func StreamToChannel(body io.ReadCloser, parser StreamParser, ch chan<- StreamChunk, log *logrus.Logger) error {
	defer func() {
		_ = body.Close()
	}()

	scanner := bufio.NewScanner(body)
	var fullContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse line using provider-specific parser
		content, done, err := parser.ParseLine(line)
		if err != nil {
			log.WithError(err).Warn("Failed to parse stream line")
			continue
		}

		// Append content if any
		if content != "" {
			fullContent.WriteString(content)
			ch <- StreamChunk{
				Type:    ChunkSummary,
				Content: content,
				Done:    false,
			}
		}

		// Check if streaming is complete
		if done {
			ch <- StreamChunk{
				Type:    ChunkSummary,
				Content: fullContent.String(),
				Done:    true,
			}
			return nil
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read error: %w", err)
	}

	// Send final chunk if not already sent
	if fullContent.Len() > 0 {
		ch <- StreamChunk{
			Type:    ChunkSummary,
			Content: fullContent.String(),
			Done:    true,
		}
	}

	return nil
}

// SSEStreamParser provides common SSE parsing utilities.
// Handles the "data: {json}" format used by most providers.
type SSEStreamParser struct {
	// ExtractContent extracts content from the parsed JSON event.
	ExtractContent func(data map[string]interface{}) (string, bool, error)

	// DoneMarker is the marker string that indicates end of stream (e.g., "[DONE]")
	DoneMarker string
}

// ParseLine implements StreamParser for SSE format.
func (p *SSEStreamParser) ParseLine(line string) (string, bool, error) {
	// Check for SSE "data: " prefix
	if !strings.HasPrefix(line, "data: ") {
		return "", false, nil // Skip non-data lines
	}

	// Extract data payload
	data := strings.TrimPrefix(line, "data: ")

	// Check for done marker
	if p.DoneMarker != "" && data == p.DoneMarker {
		return "", true, nil
	}

	// Parse JSON
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return "", false, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Extract content using provider-specific function
	if p.ExtractContent != nil {
		return p.ExtractContent(event)
	}

	return "", false, nil
}

// JSONLineStreamParser parses JSON-per-line streams (used by Ollama).
// Each line is a complete JSON object.
type JSONLineStreamParser struct {
	// ExtractContent extracts content from the parsed JSON line.
	ExtractContent func(data map[string]interface{}) (string, bool, error)
}

// ParseLine implements StreamParser for JSON-per-line format.
func (p *JSONLineStreamParser) ParseLine(line string) (string, bool, error) {
	// Parse JSON
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return "", false, fmt.Errorf("failed to parse JSON line: %w", err)
	}

	// Extract content using provider-specific function
	if p.ExtractContent != nil {
		return p.ExtractContent(event)
	}

	return "", false, nil
}

// Helper functions for common streaming patterns

// GetNestedString safely extracts a nested string value from a map.
// Example: GetNestedString(data, "choices", "0", "delta", "content")
func GetNestedString(data map[string]interface{}, keys ...string) string {
	current := data
	for i, key := range keys {
		if i == len(keys)-1 {
			// Last key - extract string value
			if val, ok := current[key].(string); ok {
				return val
			}
			return ""
		}

		// Navigate deeper
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else if next, ok := current[key].([]interface{}); ok && len(next) > 0 {
			// Handle array access (e.g., "choices.0.delta")
			if elem, ok := next[0].(map[string]interface{}); ok {
				current = elem
			} else {
				return ""
			}
		} else {
			return ""
		}
	}
	return ""
}

// GetNestedBool safely extracts a nested boolean value from a map.
func GetNestedBool(data map[string]interface{}, keys ...string) bool {
	current := data
	for i, key := range keys {
		if i == len(keys)-1 {
			// Last key - extract bool value
			if val, ok := current[key].(bool); ok {
				return val
			}
			return false
		}

		// Navigate deeper
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return false
		}
	}
	return false
}

// GetNestedArray safely extracts a nested array from a map.
func GetNestedArray(data map[string]interface{}, keys ...string) []interface{} {
	current := data
	for i, key := range keys {
		if i == len(keys)-1 {
			// Last key - extract array
			if val, ok := current[key].([]interface{}); ok {
				return val
			}
			return nil
		}

		// Navigate deeper
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	return nil
}
