package logging

import (
	"bytes"
	"context"
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
		name       string
		loggerType string
		debugShown bool
		infoShown  bool
		warnShown  bool
		errorShown bool
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
			logger := tt.create()

			// We can't directly access the level, so we test behavior
			// by exercising debug logging
			if tt.expected == slog.LevelDebug {
				// Verbose should show debug. Since we can't capture output from
				// default loggers easily, this mainly ensures no panics occur.
				logger.Debug("debug test")
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
			name:     "NewPromptLogger",
			create:   func() Logger { return NewPromptLogger("testprompt") },
			expected: "prompt=testprompt",
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
	LogError(context.Background(), logger, "test error occurred", testErr, "extra", "value")

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
	LogErrorWithOperation(context.Background(), logger, "file_read", "failed to read file", testErr)

	output := buf.String()
	if !strings.Contains(output, "operation=file_read") {
		t.Errorf("LogErrorWithOperation() should contain operation field, got: %s", output)
	}
}

// Direct slog calls (slog.Info etc.) must follow the global logger's
// destination — stray stdlib logging must never leak to the terminal
// when the host routed genie logs elsewhere (e.g. the TUI's log file).
func TestSetGlobalLoggerRoutesSlogDefault(t *testing.T) {
	prevGlobal := GetGlobalLogger()
	prevSlog := slog.Default()
	t.Cleanup(func() {
		SetGlobalLogger(prevGlobal)
		slog.SetDefault(prevSlog)
	})

	var buf bytes.Buffer
	SetGlobalLogger(NewLogger(Config{Level: slog.LevelInfo, Format: FormatText, Output: &buf}))

	slog.Info("via default slog")

	if !strings.Contains(buf.String(), "via default slog") {
		t.Fatalf("slog default output not routed to global logger, got: %q", buf.String())
	}
}

func TestSetGlobalLoggerSlogDefaultRespectsLevel(t *testing.T) {
	prevGlobal := GetGlobalLogger()
	prevSlog := slog.Default()
	t.Cleanup(func() {
		SetGlobalLogger(prevGlobal)
		slog.SetDefault(prevSlog)
	})

	var buf bytes.Buffer
	SetGlobalLogger(NewLogger(Config{Level: slog.LevelInfo, Format: FormatText, Output: &buf}))

	slog.Debug("filtered debug line")

	if strings.Contains(buf.String(), "filtered debug line") {
		t.Fatalf("debug line should be filtered at info level, got: %q", buf.String())
	}
}

func TestSetLevelKeepsSlogDefaultInSync(t *testing.T) {
	prevGlobal := GetGlobalLogger()
	prevSlog := slog.Default()
	t.Cleanup(func() {
		SetGlobalLogger(prevGlobal)
		slog.SetDefault(prevSlog)
	})

	var buf bytes.Buffer
	logger := NewLogger(Config{Level: slog.LevelInfo, Format: FormatText, Output: &buf})
	SetGlobalLogger(logger)

	logger.SetLevel(slog.LevelDebug)
	slog.Debug("debug after level change")

	if !strings.Contains(buf.String(), "debug after level change") {
		t.Fatalf("slog default went stale after SetLevel, got: %q", buf.String())
	}
}

// captureHandler records what it handles, including the context — the
// contract an OTel bridge handler relies on for trace correlation.
type captureHandler struct {
	records []slog.Record
	ctxs    []context.Context
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *captureHandler) Handle(ctx context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	h.ctxs = append(h.ctxs, ctx)
	return nil
}
func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(_ string) slog.Handler      { return h }

type testCtxKey struct{}

// Hosts (e.g. Mutiro with an OTel log bridge) inject their own handler;
// genie must route records through it instead of building its own.
func TestNewLoggerWithInjectedHandler(t *testing.T) {
	capture := &captureHandler{}
	logger := NewLogger(Config{Handler: capture})

	logger.Info("through the host handler")

	if len(capture.records) != 1 || capture.records[0].Message != "through the host handler" {
		t.Fatalf("record did not reach injected handler: %+v", capture.records)
	}
}

// Context-aware calls must hand the caller's context to the handler —
// that is where an OTel bridge reads the active span for correlation.
func TestContextMethodsCarryContextToHandler(t *testing.T) {
	capture := &captureHandler{}
	logger := NewLogger(Config{Handler: capture})

	ctx := context.WithValue(context.Background(), testCtxKey{}, "trace-me")
	logger.InfoContext(ctx, "correlated line")

	if len(capture.ctxs) != 1 {
		t.Fatalf("expected one handled record, got %d", len(capture.ctxs))
	}
	if capture.ctxs[0].Value(testCtxKey{}) != "trace-me" {
		t.Fatal("handler did not receive the caller's context")
	}
}

func TestSetGlobalLoggerBridgesInjectedHandler(t *testing.T) {
	prevGlobal := GetGlobalLogger()
	prevSlog := slog.Default()
	t.Cleanup(func() {
		SetGlobalLogger(prevGlobal)
		slog.SetDefault(prevSlog)
	})

	capture := &captureHandler{}
	SetGlobalLogger(NewLogger(Config{Handler: capture}))

	slog.Info("stray slog line")

	if len(capture.records) != 1 || capture.records[0].Message != "stray slog line" {
		t.Fatalf("stray slog call did not reach injected handler: %+v", capture.records)
	}
}

// SetLevel must not discard a host-injected handler: the host owns
// filtering, so the handler stays and keeps receiving records.
func TestSetLevelKeepsInjectedHandler(t *testing.T) {
	capture := &captureHandler{}
	logger := NewLogger(Config{Handler: capture})

	logger.SetLevel(slog.LevelDebug)
	logger.Debug("still through the host handler")

	if len(capture.records) != 1 {
		t.Fatalf("injected handler lost after SetLevel: %+v", capture.records)
	}
}
