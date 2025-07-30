package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMultilineHistoryEscaping(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "text with newline",
			input:    "hello\nworld",
			expected: "hello\\nworld",
		},
		{
			name:     "text with multiple newlines",
			input:    "line1\nline2\nline3",
			expected: "line1\\nline2\\nline3",
		},
		{
			name:     "text with backslash",
			input:    "path\\to\\file",
			expected: "path\\\\to\\\\file",
		},
		{
			name:     "text with newline and backslash",
			input:    "path\\to\\file\nnext line",
			expected: "path\\\\to\\\\file\\nnext line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			escaped := escapeForHistory(tt.input)
			if escaped != tt.expected {
				t.Errorf("escapeForHistory() = %q, want %q", escaped, tt.expected)
			}

			// Test round trip
			unescaped := unescapeFromHistory(escaped)
			if unescaped != tt.input {
				t.Errorf("unescapeFromHistory() = %q, want %q", unescaped, tt.input)
			}
		})
	}
}

func TestMultilineHistorySaveLoad(t *testing.T) {
	// Create a temporary directory for test
	tmpDir, err := os.MkdirTemp("", "history_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	historyFile := filepath.Join(tmpDir, "test_history")
	history := NewChatHistory(historyFile, true).(*FileChatHistory)

	// Test multiline commands
	multilineCommands := []string{
		"single line command",
		"first line\nsecond line\nthird line",
		"command with\\backslash",
		"complex\nmultiline\\nwith\\\\escapes",
	}

	// Add commands
	for _, cmd := range multilineCommands {
		history.AddCommand(cmd)
	}

	// Save to file
	if err := history.Save(); err != nil {
		t.Fatalf("Failed to save history: %v", err)
	}

	// Create new history instance and load
	history2 := NewChatHistory(historyFile, true).(*FileChatHistory)
	if err := history2.Load(); err != nil {
		t.Fatalf("Failed to load history: %v", err)
	}

	// Verify commands were preserved
	loaded := history2.GetHistory()
	if len(loaded) != len(multilineCommands) {
		t.Fatalf("Expected %d commands, got %d", len(multilineCommands), len(loaded))
	}

	for i, cmd := range multilineCommands {
		if loaded[i] != cmd {
			t.Errorf("Command %d: expected %q, got %q", i, cmd, loaded[i])
		}
	}

	// Verify file contains escaped versions
	content, err := os.ReadFile(historyFile)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != len(multilineCommands) {
		t.Fatalf("Expected %d lines in file, got %d", len(multilineCommands), len(lines))
	}

	// Check that multiline commands are properly escaped in file
	if !strings.Contains(lines[1], "\\n") {
		t.Error("Multiline command should contain escaped newline in file")
	}
	if !strings.Contains(lines[2], "\\\\") {
		t.Error("Command with backslash should contain escaped backslash in file")
	}
}

func TestHistoryNavigationWithMultiline(t *testing.T) {
	// Create temporary history
	tmpDir, err := os.MkdirTemp("", "history_nav_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	historyFile := filepath.Join(tmpDir, "test_history")
	history := NewChatHistory(historyFile, true).(*FileChatHistory)

	// Add some commands including multiline
	commands := []string{
		"command one",
		"multiline\ncommand\nhere",
		"command three",
	}

	for _, cmd := range commands {
		history.AddCommand(cmd)
	}

	// Test navigation
	// Should get most recent first
	if got := history.NavigatePrev(); got != "command three" {
		t.Errorf("First NavigatePrev() = %q, want %q", got, "command three")
	}

	if got := history.NavigatePrev(); got != "multiline\ncommand\nhere" {
		t.Errorf("Second NavigatePrev() = %q, want multiline command", got)
	}

	if got := history.NavigatePrev(); got != "command one" {
		t.Errorf("Third NavigatePrev() = %q, want %q", got, "command one")
	}

	// Navigate forward
	if got := history.NavigateNext(); got != "multiline\ncommand\nhere" {
		t.Errorf("NavigateNext() = %q, want multiline command", got)
	}
}