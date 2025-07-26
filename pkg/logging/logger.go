package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Logger interface for dependency injection and testing
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
	WithGroup(name string) Logger
	SetLevel(level slog.Level) // Add method for dynamic level changes
}

// Config holds logger configuration
type Config struct {
	Level   slog.Level
	Format  Format
	Output  io.Writer
	AddTime bool
}

// Format represents the output format
type Format int

const (
	FormatText Format = iota
	FormatJSON
)

// slogLogger wraps slog.Logger to implement our Logger interface
type slogLogger struct {
	logger *slog.Logger
	config Config // Keep config for level updates
}

// NewLogger creates a new logger with the given configuration
func NewLogger(config Config) Logger {
	if config.Output == nil {
		config.Output = os.Stderr
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: config.Level,
	}

	if !config.AddTime {
		opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		}
	}

	switch config.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(config.Output, opts)
	default:
		handler = slog.NewTextHandler(config.Output, opts)
	}

	return &slogLogger{
		logger: slog.New(handler),
		config: config,
	}
}

// NewDefaultLogger creates a logger with sensible defaults for CLI tools
func NewDefaultLogger() Logger {
	return NewLogger(Config{
		Level:   slog.LevelInfo,
		Format:  FormatText,
		Output:  os.Stderr,
		AddTime: false, // CLI tools typically don't need timestamps
	})
}

// NewQuietLogger creates a logger that only shows errors
func NewQuietLogger() Logger {
	return NewLogger(Config{
		Level:   slog.LevelError,
		Format:  FormatText,
		Output:  os.Stderr,
		AddTime: false,
	})
}

// NewVerboseLogger creates a logger that shows debug information
func NewVerboseLogger() Logger {
	return NewLogger(Config{
		Level:   slog.LevelDebug,
		Format:  FormatText,
		Output:  os.Stderr,
		AddTime: false,
	})
}

// NewDisabledLogger creates a logger that discards all output (useful for tests)
func NewDisabledLogger() Logger {
	return NewLogger(Config{
		Level:   slog.Level(1000), // Set to a very high level to disable all logging
		Format:  FormatText,
		Output:  io.Discard,
		AddTime: false,
	})
}

// GetDebugFilePath returns the debug file path from environment variable or default
func GetDebugFilePath(defaultFileName string) string {
	debugFile := os.Getenv("GENIE_DEBUG_FILE")
	if debugFile == "" {
		debugFile = filepath.Join(os.TempDir(), defaultFileName)
	}
	return debugFile
}

// NewFileLoggerFromEnv creates a file-based logger using standard environment variables
// Uses GENIE_DEBUG_FILE for file path (defaults to temp file) and GENIE_DEBUG_LEVEL for level
func NewFileLoggerFromEnv(defaultFileName string) Logger {
	// Get debug file path using the helper
	debugFile := GetDebugFilePath(defaultFileName)
	
	// Get debug level (default to error level - no debug)
	debugLevel := os.Getenv("GENIE_DEBUG_LEVEL")
	var logLevel slog.Level
	switch strings.ToLower(debugLevel) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo  
	case "warn", "warning":
		logLevel = slog.LevelWarn
	default:
		logLevel = slog.LevelError // Default: only errors
	}
	
	// Always write to debug file, but control level
	if file, err := os.OpenFile(debugFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		return NewLogger(Config{
			Level:   logLevel,
			Format:  FormatText,
			Output:  file,
			AddTime: true,
		})
	} else {
		// Fallback to discard if file can't be opened
		return NewLogger(Config{
			Level:   logLevel,
			Format:  FormatText,
			Output:  io.Discard,
			AddTime: false,
		})
	}
}

// Debug logs a debug message
func (l *slogLogger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Info logs an info message
func (l *slogLogger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Warn logs a warning message
func (l *slogLogger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// Error logs an error message
func (l *slogLogger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// With returns a logger with additional attributes
func (l *slogLogger) With(args ...any) Logger {
	return &slogLogger{
		logger: l.logger.With(args...),
		config: l.config,
	}
}

// WithGroup returns a logger with a group name
func (l *slogLogger) WithGroup(name string) Logger {
	return &slogLogger{
		logger: l.logger.WithGroup(name),
		config: l.config,
	}
}

// SetLevel updates the logger's level dynamically
func (l *slogLogger) SetLevel(level slog.Level) {
	// Update the config
	l.config.Level = level
	
	// Create a new handler with the new level
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if !l.config.AddTime {
		opts.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		}
	}

	var handler slog.Handler
	switch l.config.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(l.config.Output, opts)
	default:
		handler = slog.NewTextHandler(l.config.Output, opts)
	}

	// Replace the logger with a new one with the updated level
	l.logger = slog.New(handler)
}

// Global logger instance
var globalLogger Logger = NewDefaultLogger()

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger Logger) {
	globalLogger = logger
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() Logger {
	return globalLogger
}

// Convenience functions that use the global logger
func Debug(msg string, args ...any) {
	globalLogger.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	globalLogger.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	globalLogger.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	globalLogger.Error(msg, args...)
}

// Fatal logs an error message and exits the program
func Fatal(msg string, args ...any) {
	globalLogger.Error(msg, args...)
	os.Exit(1)
}

// Component logger helpers for common use cases
func NewComponentLogger(component string) Logger {
	return globalLogger.With("component", component)
}

// Operation logger for tracking operations with duration
func NewOperationLogger(component, operation string) Logger {
	return globalLogger.With(
		"component", component,
		"operation", operation,
	)
}

// Prompt logger specifically for AI prompt operations
func NewPromptLogger(promptName string) Logger {
	return globalLogger.With(
		"component", "prompt",
		"prompt", promptName,
	)
}

// API logger for HTTP/GRPC requests
func NewAPILogger(service string) Logger {
	return globalLogger.With(
		"component", "api",
		"service", service,
	)
}

// Context helpers for structured error logging
func LogError(ctx context.Context, logger Logger, msg string, err error, args ...any) {
	allArgs := append(args, "error", err)
	logger.Error(msg, allArgs...)
}

func LogErrorWithOperation(ctx context.Context, logger Logger, operation, msg string, err error, args ...any) {
	allArgs := append(args, "operation", operation, "error", err)
	logger.Error(msg, allArgs...)
}
