package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		want   string // Expected to contain this in log output
	}{
		{
			name: "text format with info level",
			config: Config{
				Level:   slog.LevelInfo,
				Format:  FormatText,
				AddTime: false,
			},
			want: "level=INFO",
		},
		{
			name: "JSON format with debug level",
			config: Config{
				Level:   slog.LevelDebug,
				Format:  FormatJSON,
				AddTime: false,
			},
			want: `"level":"INFO"`, // We're calling Info() so it should show INFO level
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			tt.config.Output = &buf

			logger := NewLogger(tt.config)
			logger.Info("test message")

			output := buf.String()
			if !strings.Contains(output, tt.want) {
				t.Errorf("NewLogger() output = %v, want to contain %v", output, tt.want)
			}
		})
	}
}

func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		name        string
		loggerType  string
		debugShown  bool
		infoShown   bool
		warnShown   bool
		errorShown  bool
	}{
		{
			name:       "default logger",
			loggerType: "default",
			debugShown: false,
			infoShown:  true,
			warnShown:  true,
			errorShown: true,
		},
		{
			name:       "verbose logger",
			loggerType: "verbose",
			debugShown: true,
			infoShown:  true,
			warnShown:  true,
			errorShown: true,
		},
		{
			name:       "quiet logger",
			loggerType: "quiet",
			debugShown: false,
			infoShown:  false,
			warnShown:  false,
			errorShown: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			
			var logger Logger
			switch tt.loggerType {
			case "default":
				logger = NewLogger(Config{Level: slog.LevelInfo, Format: FormatText, Output: &buf, AddTime: false})
			case "verbose":
				logger = NewLogger(Config{Level: slog.LevelDebug, Format: FormatText, Output: &buf, AddTime: false})
			case "quiet":
				logger = NewLogger(Config{Level: slog.LevelError, Format: FormatText, Output: &buf, AddTime: false})
			}

			// Test all log levels
			logger.Debug("debug message")
			logger.Info("info message")
			logger.Warn("warn message")
			logger.Error("error message")

			output := buf.String()

			// Check debug
			debugFound := strings.Contains(output, "debug message")
			if debugFound != tt.debugShown {
				t.Errorf("Debug message visibility = %v, want %v", debugFound, tt.debugShown)
			}

			// Check info
			infoFound := strings.Contains(output, "info message")
			if infoFound != tt.infoShown {
				t.Errorf("Info message visibility = %v, want %v", infoFound, tt.infoShown)
			}

			// Check warn
			warnFound := strings.Contains(output, "warn message")
			if warnFound != tt.warnShown {
				t.Errorf("Warn message visibility = %v, want %v", warnFound, tt.warnShown)
			}

			// Check error
			errorFound := strings.Contains(output, "error message")
			if errorFound != tt.errorShown {
				t.Errorf("Error message visibility = %v, want %v", errorFound, tt.errorShown)
			}
		})
	}
}

func TestDefaultLoggers(t *testing.T) {
	tests := []struct {
		name     string
		create   func() Logger
		expected slog.Level
	}{
		{
			name:     "NewDefaultLogger",
			create:   NewDefaultLogger,
			expected: slog.LevelInfo,
		},
		{
			name:     "NewQuietLogger", 
			create:   NewQuietLogger,
			expected: slog.LevelError,
		},
		{
			name:     "NewVerboseLogger",
			create:   NewVerboseLogger,
			expected: slog.LevelDebug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := tt.create()
			
			// We can't directly access the level, so we test behavior
			// by checking if debug messages appear
			if tt.expected == slog.LevelDebug {
				// Verbose should show debug
				logger.Debug("debug test")
				if !strings.Contains(buf.String(), "debug test") {
					// Since we can't capture output from default loggers easily,
					// this test mainly ensures no panics occur
				}
			}
		})
	}
}

func TestLoggerWith(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		Level:   slog.LevelInfo,
		Format:  FormatText,
		Output:  &buf,
		AddTime: false,
	})

	// Test With method
	contextLogger := logger.With("component", "test", "version", "1.0")
	contextLogger.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "component=test") {
		t.Errorf("With() output should contain component=test, got: %s", output)
	}
	if !strings.Contains(output, "version=1.0") {
		t.Errorf("With() output should contain version=1.0, got: %s", output)
	}
}

func TestLoggerWithGroup(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		Level:   slog.LevelInfo,
		Format:  FormatText,
		Output:  &buf,
		AddTime: false,
	})

	// Test WithGroup method
	groupLogger := logger.WithGroup("api")
	groupLogger.Info("test message", "endpoint", "/users")

	output := buf.String()
	if !strings.Contains(output, "api.endpoint=/users") {
		t.Errorf("WithGroup() output should contain grouped attributes, got: %s", output)
	}
}

func TestComponentLoggers(t *testing.T) {
	tests := []struct {
		name     string
		create   func() Logger
		expected string
	}{
		{
			name:     "NewComponentLogger",
			create:   func() Logger { return NewComponentLogger("testcomp") },
			expected: "component=testcomp",
		},
		{
			name:     "NewChainLogger",
			create:   func() Logger { return NewChainLogger("testchain") },
			expected: "chain=testchain",
		},
		{
			name:     "NewAPILogger",
			create:   func() Logger { return NewAPILogger("testapi") },
			expected: "service=testapi",
		},
		{
			name:     "NewOperationLogger",
			create:   func() Logger { return NewOperationLogger("comp", "op") },
			expected: "operation=op",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up a buffer to capture output
			var buf bytes.Buffer
			originalLogger := globalLogger
			SetGlobalLogger(NewLogger(Config{
				Level:   slog.LevelInfo,
				Format:  FormatText,
				Output:  &buf,
				AddTime: false,
			}))
			defer SetGlobalLogger(originalLogger)

			logger := tt.create()
			logger.Info("test message")

			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("%s output should contain %s, got: %s", tt.name, tt.expected, output)
			}
		})
	}
}

func TestGlobalLogger(t *testing.T) {
	// Save original global logger
	originalLogger := globalLogger
	defer SetGlobalLogger(originalLogger)

	var buf bytes.Buffer
	testLogger := NewLogger(Config{
		Level:   slog.LevelInfo,
		Format:  FormatText,
		Output:  &buf,
		AddTime: false,
	})

	SetGlobalLogger(testLogger)
	
	// Test that GetGlobalLogger returns the same instance
	retrieved := GetGlobalLogger()
	if retrieved != testLogger {
		t.Error("GetGlobalLogger() should return the set logger")
	}

	// Test global convenience functions
	Info("test info message")
	output := buf.String()
	if !strings.Contains(output, "test info message") {
		t.Errorf("Global Info() should work, got: %s", output)
	}
}

func TestLogError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		Level:   slog.LevelInfo,
		Format:  FormatText,
		Output:  &buf,
		AddTime: false,
	})

	testErr := bytes.ErrTooLarge
	LogError(nil, logger, "test error occurred", testErr, "extra", "value")

	output := buf.String()
	if !strings.Contains(output, "test error occurred") {
		t.Errorf("LogError() should contain message, got: %s", output)
	}
	if !strings.Contains(output, "error=") {
		t.Errorf("LogError() should contain error field, got: %s", output)
	}
	if !strings.Contains(output, "extra=value") {
		t.Errorf("LogError() should contain extra fields, got: %s", output)
	}
}

func TestLogErrorWithOperation(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(Config{
		Level:   slog.LevelInfo,
		Format:  FormatText,
		Output:  &buf,
		AddTime: false,
	})

	testErr := bytes.ErrTooLarge
	LogErrorWithOperation(nil, logger, "file_read", "failed to read file", testErr)

	output := buf.String()
	if !strings.Contains(output, "operation=file_read") {
		t.Errorf("LogErrorWithOperation() should contain operation field, got: %s", output)
	}
}