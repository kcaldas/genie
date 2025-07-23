package slashcommands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper function to create a dummy command file for testing
func createTestCommandFile(t *testing.T, dir, name, content string) string {
	cmdPath := filepath.Join(dir, name)
	err := os.MkdirAll(filepath.Dir(cmdPath), 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	err = os.WriteFile(cmdPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test command file: %v", err)
	}
	return cmdPath
}

func TestSlashCommandExpansion(t *testing.T) {
	tests := []struct {
		name            string
		cmdFileName     string
		cmdContent      string
		args            []string
		expectedOutput  string
		expectError     bool
	}{
		{
			name:            "single argument expansion",
			cmdFileName:     "fix-issue.md",
			cmdContent:      "echo 'Fix issue #$ARGUMENTS following our coding standards'",
			args:            []string{"123"},
			expectedOutput:  "echo 'Fix issue #123 following our coding standards'",
			expectError:     false,
		},
		{
			name:            "multiple arguments to single last placeholder",
			cmdFileName:     "log-args.md",
			cmdContent:      "log all: $ARGUMENTS",
			args:            []string{"arg1", "arg2", "arg3"},
			expectedOutput:  "log all: arg1 arg2 arg3",
			expectError:     false,
		},
		{
			name:            "multiple placeholders with exact arguments",
			cmdFileName:     "multi-parts.md",
			cmdContent:      "part1: $ARGUMENTS, part2: $ARGUMENTS",
			args:            []string{"value1", "value2"},
			expectedOutput:  "part1: value1, part2: value2",
			expectError:     false,
		},
		{
			name:            "multiple placeholders with more arguments than placeholders (last consumes rest)",
			cmdFileName:     "consume-rest.md",
			cmdContent:      "first: $ARGUMENTS, rest: $ARGUMENTS",
			args:            []string{"one", "two", "three"},
			expectedOutput:  "first: one, rest: two three",
			expectError:     false,
		},
		{
			name:            "multiple placeholders with fewer arguments than placeholders",
			cmdFileName:     "missing-args.md",
			cmdContent:      "first: $ARGUMENTS, second: $ARGUMENTS, third: $ARGUMENTS",
			args:            []string{"alpha", "beta"},
			expectedOutput:  "first: alpha, second: beta, third: ", // third should be empty
			expectError:     false,
		},
		{
			name:            "no arguments placeholder",
			cmdFileName:     "no-args-cmd.md",
			cmdContent:      "just a static command",
			args:            []string{"any", "args"},
			expectedOutput:  "just a static command",
			expectError:     false,
		},
		{
			name:            "empty command content",
			cmdFileName:     "empty-cmd.md",
			cmdContent:      "",
			args:            []string{"some", "data"},
			expectedOutput:  "",
			expectError:     false,
		},
		{
			name:            "empty arguments list",
			cmdFileName:     "empty-args-list.md",
			cmdContent:      "command with $ARGUMENTS",
			args:            []string{},
			expectedOutput:  "command with ",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir, err := os.MkdirTemp("", "genie_test_commands_")
			if err != nil {
				t.Fatalf("Failed to create temporary directory: %v", err)
			}
			defer os.RemoveAll(tempDir) // Clean up after the test

			projectCmdDir := filepath.Join(tempDir, ".claude", "commands")
			createTestCommandFile(t, projectCmdDir, tt.cmdFileName, tt.cmdContent)

			manager := NewManager()
			mockGetUserHomeDir := func() (string, error) {
				return filepath.Join(tempDir, "home"), nil
			}

			err = manager.DiscoverCommands(tempDir, mockGetUserHomeDir)
			if err != nil {
				t.Fatalf("DiscoverCommands failed: %v", err)
			}

			cmdName := strings.TrimSuffix(tt.cmdFileName, ".md")
			cmd, found := manager.commands[cmdName]
			if !found {
				t.Fatalf("Expected command '%s' not found after discovery", cmdName)
			}

			if cmd.Expand == nil {
				t.Fatalf("Expected Expand function to be set for command '%s'", cmdName)
			}

			output, err := cmd.Expand(tt.args)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect an error but got: %v", err)
				}
				if output != tt.expectedOutput {
					t.Errorf("Expected output %q, got %q", tt.expectedOutput, output)
				}
			}
		})
	}
}