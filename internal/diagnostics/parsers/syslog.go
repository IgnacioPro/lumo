package parsers

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ignacio/lumo/internal/intelligence/ingestion"
)

type SyslogParser struct {
	// RFC 3164 pattern: <priority>timestamp hostname tag: message
	rfc3164Pattern *regexp.Regexp
	// RFC 5424 pattern: <priority>version timestamp hostname app-name procid msgid structured-data message
	rfc5424Pattern *regexp.Regexp
}

func NewSyslogParser() *SyslogParser {
	return &SyslogParser{
		// Simplified patterns, full implementation would be more robust
		rfc3164Pattern: regexp.MustCompile(`^(\w+\s+\d+\s+\d+:\d+:\d+)\s+(\S+)\s+(\S+?):\s+(.*)$`),
		rfc5424Pattern: regexp.MustCompile(`^<(\d+)>(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(.*)$`),
	}
}

func (p *SyslogParser) Parse(raw string) (*ingestion.LogEntry, error) {
	// Try RFC 3164 first
	if matches := p.rfc3164Pattern.FindStringSubmatch(raw); matches != nil {
		timestamp, _ := time.Parse("Jan 02 15:04:05", matches[1])

		return &ingestion.LogEntry{
			Timestamp: timestamp,
			Source:    matches[2], // hostname
			Level:     p.inferLevel(matches[4]),
			Message:   matches[4],
			Raw:       raw,
			Fields: map[string]interface{}{
				"hostname": matches[2],
				"tag":      matches[3],
			},
		}, nil
	}

	// Try RFC 5424
	if matches := p.rfc5424Pattern.FindStringSubmatch(raw); matches != nil {
		timestamp, _ := time.Parse(time.RFC3339, matches[3])

		return &ingestion.LogEntry{
			Timestamp: timestamp,
			Source:    matches[4], // hostname
			Level:     p.inferLevel(matches[8]),
			Message:   matches[8],
			Raw:       raw,
			Fields: map[string]interface{}{
				"priority": matches[1],
				"version":  matches[2],
				"hostname": matches[4],
				"app_name": matches[5],
				"proc_id":  matches[6],
				"msg_id":   matches[7],
			},
		}, nil
	}

	return nil, fmt.Errorf("failed to parse syslog format")
}

func (p *SyslogParser) ParseMulti(raw string) ([]*ingestion.LogEntry, error) {
	lines := strings.Split(raw, "\n")
	entries := make([]*ingestion.LogEntry, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}
		entry, err := p.Parse(line)
		if err != nil {
			// Skip unparseable lines
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (p *SyslogParser) DetectFormat(raw string) ingestion.LogFormat {
	if p.rfc3164Pattern.MatchString(raw) || p.rfc5424Pattern.MatchString(raw) {
		return ingestion.LogFormatSyslog
	}
	return ingestion.LogFormatUnknown
}

func (p *SyslogParser) inferLevel(message string) string {
	messageLower := strings.ToLower(message)
	if strings.Contains(messageLower, "error") || strings.Contains(messageLower, "fail") {
		return "ERROR"
	}
	if strings.Contains(messageLower, "warn") {
		return "WARN"
	}
	if strings.Contains(messageLower, "critical") || strings.Contains(messageLower, "fatal") {
		return "CRITICAL"
	}
	if strings.Contains(messageLower, "debug") {
		return "DEBUG"
	}
	return "INFO"
}
