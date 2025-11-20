package parsers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/intelligence/ingestion"
)

type JSONParser struct{}

func NewJSONParser() *JSONParser {
	return &JSONParser{}
}

func (p *JSONParser) Parse(raw string) (*ingestion.LogEntry, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("not valid JSON: %w", err)
	}

	entry := &ingestion.LogEntry{
		Raw:    raw,
		Fields: data,
	}

	// Extract common fields
	if ts, ok := data["timestamp"].(string); ok {
		entry.Timestamp, _ = time.Parse(time.RFC3339, ts)
	} else if ts, ok := data["time"].(string); ok {
		entry.Timestamp, _ = time.Parse(time.RFC3339, ts)
	} else {
		entry.Timestamp = time.Now()
	}

	if level, ok := data["level"].(string); ok {
		entry.Level = level
	} else if level, ok := data["severity"].(string); ok {
		entry.Level = level
	}

	if msg, ok := data["message"].(string); ok {
		entry.Message = msg
	} else if msg, ok := data["msg"].(string); ok {
		entry.Message = msg
	}

	if source, ok := data["source"].(string); ok {
		entry.Source = source
	} else if logger, ok := data["logger"].(string); ok {
		entry.Source = logger
	}

	return entry, nil
}

func (p *JSONParser) ParseMulti(raw string) ([]*ingestion.LogEntry, error) {
	// NDJSON (newline-delimited JSON)
	lines := strings.Split(raw, "\n")
	entries := make([]*ingestion.LogEntry, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}
		entry, err := p.Parse(line)
		if err != nil {
			continue // Skip invalid JSON lines
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (p *JSONParser) DetectFormat(raw string) ingestion.LogFormat {
	var data map[string]interface{}
	if json.Unmarshal([]byte(raw), &data) == nil {
		return ingestion.LogFormatJSON
	}
	return ingestion.LogFormatUnknown
}
