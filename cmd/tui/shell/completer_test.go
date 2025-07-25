package shell

import (
	"testing"
)


func TestCompleter_Suggest(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		suggestion string // Expected suggestion (full text), empty if no suggestion
	}{
		// Basic typing scenarios
		{"empty", "", ""},
		{"colon", ":", ""},
		{"colon-w", ":w", ":write"},
		{"colon-wr", ":wr", ":write"},
		{"colon-wri", ":wri", ":write"},
		{"colon-writ", ":writ", ":write"},
		{"colon-write", ":write", ""}, // Exact match, no suggestion
		
		// Yank command
		{"colon-y", ":y", ":yank"},
		{"colon-ya", ":ya", ":yank"},
		{"colon-yan", ":yan", ":yank"},
		{"colon-yank", ":yank", ""}, // Exact match
		
		// Edge cases
		{"no-colon", "w", ""},
		{"text", "hello", ""},
		{"unknown", ":unknown", ""},
		{"with-space", ":write ", ""}, // Space disables suggestions
	}
	
	registry := CreateTestCommandRegistry()
	completer := NewCompleter()
	completer.RegisterSuggester(NewCommandSuggester(registry))
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := completer.Suggest(tt.input)
			if got != tt.suggestion {
				t.Errorf("Suggest(%q) = %q, want %q", tt.input, got, tt.suggestion)
			}
		})
	}
}

func TestCompleter_MultipleSuggesters(t *testing.T) {
	completer := NewCompleter()
	
	// Register multiple suggesters - first one should win
	registry := CreateTestCommandRegistry()
	completer.RegisterSuggester(NewCommandSuggester(registry))
	// Could add another suggester here in the future
	
	// Test that we get the first suggester's result
	suggestion := completer.Suggest(":w")
	if suggestion != ":write" {
		t.Errorf("Expected ':write' from first suggester, got %q", suggestion)
	}
}

func TestCompleter_NoSuggesters(t *testing.T) {
	completer := NewCompleter()
	
	// No suggesters registered
	suggestion := completer.Suggest(":w")
	if suggestion != "" {
		t.Errorf("Expected empty suggestion with no suggesters, got %q", suggestion)
	}
}