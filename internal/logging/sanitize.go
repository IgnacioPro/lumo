package logging

import (
	"regexp"
	"sync"
)

var (
	// API key patterns to redact
	apiKeyPatterns = []struct {
		pattern     *regexp.Regexp
		replacement string
	}{
		// Anthropic API keys: sk-ant-api03-...
		{regexp.MustCompile(`sk-ant-api\d+-[a-zA-Z0-9_-]{95,}`), "sk-ant-***REDACTED***"},
		// Anthropic session keys
		{regexp.MustCompile(`sk-ant-sid\d+-[a-zA-Z0-9_-]+`), "sk-ant-***REDACTED***"},
		// OpenAI API keys: sk-...
		{regexp.MustCompile(`sk-[a-zA-Z0-9]{48,}`), "sk-***REDACTED***"},
		// OpenAI project keys
		{regexp.MustCompile(`sk-proj-[a-zA-Z0-9]{48,}`), "sk-proj-***REDACTED***"},
		// Google/Gemini API keys: AIza...
		{regexp.MustCompile(`AIza[a-zA-Z0-9_-]{35,}`), "AIza***REDACTED***"},
		// Generic Bearer tokens
		{regexp.MustCompile(`Bearer\s+[a-zA-Z0-9_\-\.]{20,}`), "Bearer ***REDACTED***"},
		// OpenRouter keys
		{regexp.MustCompile(`sk-or-v1-[a-zA-Z0-9]{64,}`), "sk-or-***REDACTED***"},
		// Generic API key patterns in URLs or headers
		{regexp.MustCompile(`(?i)(api[_-]?key|apikey|api-key)['"]?\s*[:=]\s*['"]?[a-zA-Z0-9_\-]{20,}`), "$1=***REDACTED***"},
		// Authorization header values
		{regexp.MustCompile(`(?i)authorization:\s*[a-zA-Z0-9_\-\.]{20,}`), "authorization: ***REDACTED***"},
	}

	// Compile patterns once
	patternsOnce sync.Once
)

// SanitizeAPIKeys removes API keys and tokens from a string
// This function is safe to call concurrently
func SanitizeAPIKeys(s string) string {
	if s == "" {
		return s
	}

	result := s
	for _, p := range apiKeyPatterns {
		result = p.pattern.ReplaceAllString(result, p.replacement)
	}

	return result
}

// SanitizeError sanitizes error messages before logging
// Returns the original error if it's nil
func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	return SanitizeAPIKeys(err.Error())
}

// SanitizeMap sanitizes all string values in a map (useful for logging structured data)
func SanitizeMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}

	sanitized := make(map[string]interface{}, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case string:
			sanitized[k] = SanitizeAPIKeys(val)
		case map[string]interface{}:
			sanitized[k] = SanitizeMap(val)
		default:
			sanitized[k] = v
		}
	}

	return sanitized
}
