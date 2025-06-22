package mcp

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/manojparvathaneni/go-mcp-sdk/mcp/types"
)


// LogLevelString returns the string representation of the log level
func LogLevelString(l types.LogLevel) string {
	switch l {
	case types.LogLevelDebug:
		return "DEBUG"
	case types.LogLevelInfo:
		return "INFO"
	case types.LogLevelWarning:
		return "WARN"
	case types.LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger is the interface for logging in the MCP SDK
type Logger interface {
	// Debug logs a debug message
	Debug(msg string, fields ...Field)
	// Info logs an informational message
	Info(msg string, fields ...Field)
	// Warn logs a warning message
	Warn(msg string, fields ...Field)
	// Error logs an error message
	Error(msg string, fields ...Field)
	// WithContext returns a logger with context
	WithContext(ctx context.Context) Logger
	// WithFields returns a logger with additional fields
	WithFields(fields ...Field) Logger
}

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// String creates a string field
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an integer field
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a boolean field
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Error creates an error field
func Error(err error) Field {
	return Field{Key: "error", Value: err}
}

// Any creates a field with any value
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// StdLogger is the default logger implementation using the standard library
type StdLogger struct {
	logger   *log.Logger
	level    LogLevel
	fields   []Field
	prefix   string
}

// NewStdLogger creates a new standard logger
func NewStdLogger(level LogLevel) *StdLogger {
	return &StdLogger{
		logger: log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds),
		level:  level,
	}
}

// NewStdLoggerWithOptions creates a new standard logger with options
func NewStdLoggerWithOptions(logger *log.Logger, level LogLevel, prefix string) *StdLogger {
	return &StdLogger{
		logger: logger,
		level:  level,
		prefix: prefix,
	}
}

// Debug logs a debug message
func (l *StdLogger) Debug(msg string, fields ...Field) {
	l.log(LogLevelDebug, msg, fields...)
}

// Info logs an informational message
func (l *StdLogger) Info(msg string, fields ...Field) {
	l.log(LogLevelInfo, msg, fields...)
}

// Warn logs a warning message
func (l *StdLogger) Warn(msg string, fields ...Field) {
	l.log(types.LogLevelWarning, msg, fields...)
}

// Error logs an error message
func (l *StdLogger) Error(msg string, fields ...Field) {
	l.log(LogLevelError, msg, fields...)
}

// WithContext returns a logger with context
func (l *StdLogger) WithContext(ctx context.Context) Logger {
	// Extract request ID or trace ID from context if available
	fields := make([]Field, len(l.fields))
	copy(fields, l.fields)
	
	if requestID := ctx.Value("request_id"); requestID != nil {
		fields = append(fields, String("request_id", fmt.Sprint(requestID)))
	}
	
	return &StdLogger{
		logger: l.logger,
		level:  l.level,
		fields: fields,
		prefix: l.prefix,
	}
}

// WithFields returns a logger with additional fields
func (l *StdLogger) WithFields(fields ...Field) Logger {
	newFields := make([]Field, len(l.fields)+len(fields))
	copy(newFields, l.fields)
	copy(newFields[len(l.fields):], fields)
	
	return &StdLogger{
		logger: l.logger,
		level:  l.level,
		fields: newFields,
		prefix: l.prefix,
	}
}

func (l *StdLogger) log(level LogLevel, msg string, fields ...Field) {
	if level < l.level {
		return
	}
	
	// Build the log message
	prefix := l.prefix
	if prefix != "" {
		prefix += " "
	}
	
	logMsg := fmt.Sprintf("%s[%s] %s", prefix, LogLevelString(level), msg)
	
	// Add fields
	allFields := append(l.fields, fields...)
	if len(allFields) > 0 {
		logMsg += " {"
		for i, field := range allFields {
			if i > 0 {
				logMsg += ", "
			}
			logMsg += fmt.Sprintf("%s=%v", field.Key, field.Value)
		}
		logMsg += "}"
	}
	
	l.logger.Println(logMsg)
}

// NoOpLogger is a logger that does nothing
type NoOpLogger struct{}

// Debug does nothing
func (l *NoOpLogger) Debug(msg string, fields ...Field) {}

// Info does nothing
func (l *NoOpLogger) Info(msg string, fields ...Field) {}

// Warn does nothing
func (l *NoOpLogger) Warn(msg string, fields ...Field) {}

// Error does nothing
func (l *NoOpLogger) Error(msg string, fields ...Field) {}

// WithContext returns the same logger
func (l *NoOpLogger) WithContext(ctx context.Context) Logger {
	return l
}

// WithFields returns the same logger
func (l *NoOpLogger) WithFields(fields ...Field) Logger {
	return l
}

// DefaultLogger is the default logger used by the SDK
var DefaultLogger Logger = NewStdLogger(LogLevelInfo)