package ingestion

import (
	"time"
)

// LogFormat represents different log formats
type LogFormat int

const (
	LogFormatUnknown LogFormat = iota
	LogFormatSyslog
	LogFormatJSON
	LogFormatApache
	LogFormatNginx
	LogFormatCustom
)

// LogEntry represents a parsed log line
type LogEntry struct {
	Timestamp time.Time
	// Level represents the log severity level, normalized to one of:
	// - "DEBUG": Detailed diagnostic information
	// - "INFO": General informational messages
	// - "WARN" or "WARNING": Warning messages
	// - "ERROR": Error conditions
	// - "CRITICAL" or "FATAL": Critical/fatal error conditions
	// Parsers should normalize various log level formats to these standard values.
	Level     string
	Source    string                 // File path or logger name
	Message   string                 // Primary log message
	Fields    map[string]interface{} // Structured fields (from JSON logs)
	Raw       string                 // Original raw line
}

// LogParser extracts structured data from raw log lines
type LogParser interface {
	Parse(raw string) (*LogEntry, error)
	ParseMulti(raw string) ([]*LogEntry, error) // Parse multiple lines
	DetectFormat(raw string) LogFormat
}
