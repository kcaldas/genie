package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/logging"
	"github.com/spf13/cobra"
)

func TestLoggingFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectDebug bool
		expectInfo  bool
		expectError bool
		description string
	}{
		{
			name:        "default logging",
			args:        []string{},
			expectDebug: false,
			expectInfo:  true,
			expectError: true,
			description: "Default should show INFO and ERROR, but not DEBUG",
		},
		{
			name:        "verbose logging",
			args:        []string{"--verbose"},
			expectDebug: true,
			expectInfo:  true,
			expectError: true,
			description: "Verbose should show all levels including DEBUG",
		},
		{
			name:        "verbose short flag",
			args:        []string{"-v"},
			expectDebug: true,
			expectInfo:  true,
			expectError: true,
			description: "Short verbose flag should work the same",
		},
		{
			name:        "quiet logging",
			args:        []string{"--quiet"},
			expectDebug: false,
			expectInfo:  false,
			expectError: true,
			description: "Quiet should only show ERROR level",
		},
		{
			name:        "quiet short flag",
			args:        []string{"-q"},
			expectDebug: false,
			expectInfo:  false,
			expectError: true,
			description: "Short quiet flag should work the same",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new command instance for testing
			verbose := false
			quiet := false

			testCmd := &cobra.Command{
				Use: "test",
				PersistentPreRun: func(cmd *cobra.Command, args []string) {
					// Configure logger based on flags (same logic as main)
					var logger logging.Logger
					if quiet {
						logger = logging.NewQuietLogger()
					} else if verbose {
						logger = logging.NewVerboseLogger()
					} else {
						logger = logging.NewDefaultLogger()
					}
					logging.SetGlobalLogger(logger)
				},
				Run: func(cmd *cobra.Command, args []string) {
					// Test logging at different levels
					logging.Debug("debug message")
					logging.Info("info message")
					logging.Error("error message")
				},
			}

			testCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
			testCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet output")

			// Capture stderr output
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Set command args and execute
			testCmd.SetArgs(tt.args)
			err := testCmd.Execute()
			if err != nil {
				t.Fatalf("Command execution failed: %v", err)
			}

			// Close writer and read output
			w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			// Check debug level
			debugFound := strings.Contains(output, "debug message")
			if debugFound != tt.expectDebug {
				t.Errorf("%s: Debug message visibility = %v, want %v. Output: %s",
					tt.description, debugFound, tt.expectDebug, output)
			}

			// Check info level
			infoFound := strings.Contains(output, "info message")
			if infoFound != tt.expectInfo {
				t.Errorf("%s: Info message visibility = %v, want %v. Output: %s",
					tt.description, infoFound, tt.expectInfo, output)
			}

			// Check error level
			errorFound := strings.Contains(output, "error message")
			if errorFound != tt.expectError {
				t.Errorf("%s: Error message visibility = %v, want %v. Output: %s",
					tt.description, errorFound, tt.expectError, output)
			}
		})
	}
}

func TestConflictingFlags(t *testing.T) {
	// Test what happens when both --verbose and --quiet are provided
	// The current implementation will respect whichever is checked first (quiet takes precedence)

	verbose := false
	quiet := false

	testCmd := &cobra.Command{
		Use: "test",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			var logger logging.Logger
			if quiet {
				logger = logging.NewQuietLogger()
			} else if verbose {
				logger = logging.NewVerboseLogger()
			} else {
				logger = logging.NewDefaultLogger()
			}
			logging.SetGlobalLogger(logger)
		},
		Run: func(cmd *cobra.Command, args []string) {
			logging.Debug("debug message")
			logging.Info("info message")
			logging.Error("error message")
		},
	}

	testCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	testCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "quiet output")

	// Capture stderr output
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Set both flags
	testCmd.SetArgs([]string{"--verbose", "--quiet"})
	err := testCmd.Execute()
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Quiet should take precedence (based on current implementation)
	debugFound := strings.Contains(output, "debug message")
	infoFound := strings.Contains(output, "info message")
	errorFound := strings.Contains(output, "error message")

	if debugFound {
		t.Error("When both flags are set, debug should not be shown (quiet takes precedence)")
	}
	if infoFound {
		t.Error("When both flags are set, info should not be shown (quiet takes precedence)")
	}
	if !errorFound {
		t.Error("When both flags are set, error should still be shown")
	}
}

func TestMainCommandSetup(t *testing.T) {
	// Test that the main command has the correct flags set up
	if rootCmd.PersistentFlags().Lookup("verbose") == nil {
		t.Error("Root command should have --verbose flag")
	}

	if rootCmd.PersistentFlags().Lookup("quiet") == nil {
		t.Error("Root command should have --quiet flag")
	}

	// Test short flags
	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	if verboseFlag.Shorthand != "v" {
		t.Error("Verbose flag should have shorthand 'v'")
	}

	quietFlag := rootCmd.PersistentFlags().Lookup("quiet")
	if quietFlag.Shorthand != "q" {
		t.Error("Quiet flag should have shorthand 'q'")
	}
}

// Example integration test that could be used for more complex scenarios
func TestLoggingIntegrationExample(t *testing.T) {
	// This is an example of how you might test logging in actual command execution
	// For now, we'll skip this test since we don't have actual commands implemented yet
	t.Skip("Skipping integration test - no actual commands implemented yet")

	// In the future, you could test something like:
	// 1. Execute a command that uses chain execution
	// 2. Verify that the chain logging appears with correct format
	// 3. Test that verbose mode shows debug info for API calls
	// etc.
}
