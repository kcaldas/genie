package cli

import (
	"testing"
)

func TestConstructMessage(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "args only",
			args:     []string{"hello", "world"},
			expected: "hello world",
		},
		{
			name:     "single arg",
			args:     []string{"test"},
			expected: "test",
		},
		{
			name:     "empty args",
			args:     []string{},
			expected: "", // Will fail without stdin
		},
		{
			name:     "args with spaces",
			args:     []string{"how", "are", "you", "today?"},
			expected: "how are you today?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test only covers args without stdin
			// For stdin testing, we'd need to mock os.Stdin
			if len(tt.args) == 0 {
				_, err := constructMessage(tt.args)
				if err == nil {
					t.Error("Expected error for empty args without stdin")
				}
				return
			}

			result, err := constructMessage(tt.args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTruncateContent(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		maxLength int
		expected  string
	}{
		{
			name:      "no truncation needed",
			content:   "short",
			maxLength: 10,
			expected:  "short",
		},
		{
			name:      "exact length",
			content:   "exactly10c",
			maxLength: 10,
			expected:  "exactly10c",
		},
		{
			name:      "truncation needed",
			content:   "this is a very long string that needs truncation",
			maxLength: 20,
			expected:  "this is a very long ...",
		},
		{
			name:      "empty string",
			content:   "",
			maxLength: 5,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateContent(tt.content, tt.maxLength)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestValidateAskArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		expectErr bool
	}{
		{
			name:      "with args",
			args:      []string{"hello"},
			expectErr: false,
		},
		{
			name:      "multiple args",
			args:      []string{"hello", "world"},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAskArgs(nil, tt.args)
			
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}

	// Test empty args separately since stdin behavior varies in test environment
	t.Run("empty args behavior", func(t *testing.T) {
		err := validateAskArgs(nil, []string{})
		// In a test environment, this might or might not error depending on stdin
		// We just verify the function doesn't panic
		t.Logf("Empty args validation result: %v", err)
	})
}

// Mock tests for stdin scenarios would require more complex setup
// involving pipes and process manipulation, which is beyond basic unit tests
// Integration tests would be better suited for those scenarios