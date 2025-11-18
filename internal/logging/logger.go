package logging

import (
	"github.com/sirupsen/logrus"
)

// SanitizingLogger wraps a logrus.Logger and automatically sanitizes API keys from log messages
type SanitizingLogger struct {
	*logrus.Logger
}

// NewSanitizingLogger creates a new logger that automatically sanitizes sensitive data
func NewSanitizingLogger(base *logrus.Logger) *SanitizingLogger {
	if base == nil {
		base = logrus.New()
	}
	return &SanitizingLogger{Logger: base}
}

// Errorf logs an error message with sanitization
func (l *SanitizingLogger) Errorf(format string, args ...interface{}) {
	sanitizedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			sanitizedArgs[i] = SanitizeError(err)
		} else if str, ok := arg.(string); ok {
			sanitizedArgs[i] = SanitizeAPIKeys(str)
		} else {
			sanitizedArgs[i] = arg
		}
	}
	l.Logger.Errorf(format, sanitizedArgs...)
}

// Error logs an error with sanitization
func (l *SanitizingLogger) Error(args ...interface{}) {
	sanitizedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			sanitizedArgs[i] = SanitizeError(err)
		} else if str, ok := arg.(string); ok {
			sanitizedArgs[i] = SanitizeAPIKeys(str)
		} else {
			sanitizedArgs[i] = arg
		}
	}
	l.Logger.Error(sanitizedArgs...)
}

// Warnf logs a warning message with sanitization
func (l *SanitizingLogger) Warnf(format string, args ...interface{}) {
	sanitizedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			sanitizedArgs[i] = SanitizeError(err)
		} else if str, ok := arg.(string); ok {
			sanitizedArgs[i] = SanitizeAPIKeys(str)
		} else {
			sanitizedArgs[i] = arg
		}
	}
	l.Logger.Warnf(format, sanitizedArgs...)
}

// Warn logs a warning with sanitization
func (l *SanitizingLogger) Warn(args ...interface{}) {
	sanitizedArgs := make([]interface{}, len(args))
	for i, arg := range args {
		if err, ok := arg.(error); ok {
			sanitizedArgs[i] = SanitizeError(err)
		} else if str, ok := arg.(string); ok {
			sanitizedArgs[i] = SanitizeAPIKeys(str)
		} else {
			sanitizedArgs[i] = arg
		}
	}
	l.Logger.Warn(sanitizedArgs...)
}

// WithFields creates an entry with sanitized fields
func (l *SanitizingLogger) WithFields(fields logrus.Fields) *logrus.Entry {
	sanitizedFields := make(logrus.Fields, len(fields))
	for k, v := range fields {
		if err, ok := v.(error); ok {
			sanitizedFields[k] = SanitizeError(err)
		} else if str, ok := v.(string); ok {
			sanitizedFields[k] = SanitizeAPIKeys(str)
		} else if m, ok := v.(map[string]interface{}); ok {
			sanitizedFields[k] = SanitizeMap(m)
		} else {
			sanitizedFields[k] = v
		}
	}
	return l.Logger.WithFields(sanitizedFields)
}

// WithError creates an entry with a sanitized error
func (l *SanitizingLogger) WithError(err error) *logrus.Entry {
	return l.Logger.WithError(err).WithField("error_sanitized", SanitizeError(err))
}
