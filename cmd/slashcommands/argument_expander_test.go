package slashcommands

import (
	"testing"
)

func TestExpandArguments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		args     []string
		expected string
	}{
		{
			name:     "single $ARGUMENTS with one argument",
			input:    "This is a test with $ARGUMENTS",
			args:     []string{"arg1"},
			expected: "This is a test with arg1",
		},
		{
			name:     "single $ARGUMENTS with multiple arguments",
			input:    "This is a test with $ARGUMENTS",
			args:     []string{"arg1", "arg2", "arg3"},
			expected: "This is a test with arg1 arg2 arg3",
		},
		{
			name:     "multiple $ARGUMENTS",
			input:    "First: $ARGUMENTS, Second: $ARGUMENTS",
			args:     []string{"arg1", "arg2", "arg3"},
			expected: "First: arg1, Second: arg2 arg3",
		},
		{
			name:     "no $ARGUMENTS",
			input:    "This is a test without arguments",
			args:     []string{"arg1"},
			expected: "This is a test without arguments",
		},
		{
			name:     "empty arguments",
			input:    "This is a test with $ARGUMENTS",
			args:     []string{},
			expected: "This is a test with ",
		},
		{
			name:     "only $ARGUMENTS",
			input:    "$ARGUMENTS",
			args:     []string{"arg1", "arg2"},
			expected: "arg1 arg2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ExpandArguments(tt.input, tt.args)
			if actual != tt.expected {
				t.Errorf("Expected: %q, Got: %q", tt.expected, actual)
			}
		})
	}
}
