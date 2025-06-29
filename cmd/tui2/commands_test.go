package tui2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseYankArgument(t *testing.T) {
	// Create a minimal app instance for testing
	app := &App{}

	tests := []struct {
		name              string
		input             string
		expectedCount     int
		expectedDirection string
		description       string
	}{
		{
			name:              "empty string",
			input:             "",
			expectedCount:     1,
			expectedDirection: "",
			description:       "Empty input should default to count 1, no direction",
		},
		{
			name:              "single digit",
			input:             "3",
			expectedCount:     3,
			expectedDirection: "",
			description:       "Number only should parse count, no direction",
		},
		{
			name:              "multi digit",
			input:             "25",
			expectedCount:     25,
			expectedDirection: "",
			description:       "Multi-digit number should parse correctly",
		},
		{
			name:              "direction only k",
			input:             "k",
			expectedCount:     1,
			expectedDirection: "k",
			description:       "Direction only should default count to 1",
		},
		{
			name:              "direction only j",
			input:             "j",
			expectedCount:     1,
			expectedDirection: "j",
			description:       "Direction only should default count to 1",
		},
		{
			name:              "count with k direction",
			input:             "2k",
			expectedCount:     2,
			expectedDirection: "k",
			description:       "Count + k direction should parse both",
		},
		{
			name:              "count with j direction",
			input:             "5j",
			expectedCount:     5,
			expectedDirection: "j",
			description:       "Count + j direction should parse both",
		},
		{
			name:              "large count with direction",
			input:             "100k",
			expectedCount:     100,
			expectedDirection: "k",
			description:       "Large count should parse correctly",
		},
		{
			name:              "zero at start",
			input:             "0k",
			expectedCount:     1,
			expectedDirection: "k",
			description:       "Zero count should default to 1",
		},
		{
			name:              "invalid direction",
			input:             "3x",
			expectedCount:     3,
			expectedDirection: "x",
			description:       "Invalid direction should still be captured",
		},
		{
			name:              "complex valid pattern",
			input:             "42k",
			expectedCount:     42,
			expectedDirection: "k",
			description:       "Complex valid pattern should parse correctly",
		},
		{
			name:              "relative positioning -1",
			input:             "-1",
			expectedCount:     1,
			expectedDirection: "-",
			description:       "Relative positioning -1 should parse correctly",
		},
		{
			name:              "relative positioning -5",
			input:             "-5",
			expectedCount:     5,
			expectedDirection: "-",
			description:       "Relative positioning -5 should parse correctly",
		},
		{
			name:              "relative positioning -10",
			input:             "-10",
			expectedCount:     10,
			expectedDirection: "-",
			description:       "Multi-digit relative positioning should work",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, direction := app.parseYankArgument(tt.input)
			assert.Equal(t, tt.expectedCount, count, "Count mismatch: %s", tt.description)
			assert.Equal(t, tt.expectedDirection, direction, "Direction mismatch: %s", tt.description)
		})
	}
}

func TestParseYankArgumentEdgeCases(t *testing.T) {
	app := &App{}

	t.Run("leading zeros", func(t *testing.T) {
		count, direction := app.parseYankArgument("007k")
		assert.Equal(t, 7, count, "Leading zeros should be handled correctly")
		assert.Equal(t, "k", direction, "Direction should be parsed correctly with leading zeros")
	})

	t.Run("multiple characters after number", func(t *testing.T) {
		count, direction := app.parseYankArgument("5kj")
		assert.Equal(t, 5, count, "Count should be parsed correctly")
		assert.Equal(t, "k", direction, "Only first character after number should be direction")
	})

	t.Run("non-numeric start", func(t *testing.T) {
		count, direction := app.parseYankArgument("abc")
		assert.Equal(t, 1, count, "Non-numeric start should default count to 1")
		assert.Equal(t, "a", direction, "First character should be treated as direction")
	})

	t.Run("mixed valid pattern", func(t *testing.T) {
		count, direction := app.parseYankArgument("123j456")
		assert.Equal(t, 123, count, "Should parse consecutive digits as count")
		assert.Equal(t, "j", direction, "Should take first non-digit as direction")
	})

	t.Run("relative positioning edge cases", func(t *testing.T) {
		// Test just dash
		count, direction := app.parseYankArgument("-")
		assert.Equal(t, 1, count, "Just dash should default count to 1")
		assert.Equal(t, "-", direction, "Should indicate relative mode")

		// Test dash with no number
		count, direction = app.parseYankArgument("-k")
		assert.Equal(t, 1, count, "Dash with no number should default count to 1")
		assert.Equal(t, "-", direction, "Should indicate relative mode (ignoring k)")

		// Test leading zeros in relative
		count, direction = app.parseYankArgument("-007")
		assert.Equal(t, 7, count, "Leading zeros in relative should work")
		assert.Equal(t, "-", direction, "Should indicate relative mode")
	})
}

func TestYankCommandValidation(t *testing.T) {
	// Test the validation logic that would be used in the actual command
	testCases := []struct {
		name        string
		arg         string
		expectValid bool
		description string
	}{
		{
			name:        "valid k direction",
			arg:         "5k",
			expectValid: true,
			description: "k direction should be valid",
		},
		{
			name:        "valid j direction", 
			arg:         "3j",
			expectValid: true,
			description: "j direction should be valid",
		},
		{
			name:        "valid count only",
			arg:         "10",
			expectValid: true,
			description: "Count only should be valid (defaults to k)",
		},
		{
			name:        "invalid direction",
			arg:         "5x",
			expectValid: false,
			description: "Invalid direction should be rejected",
		},
		{
			name:        "empty valid",
			arg:         "",
			expectValid: true,
			description: "Empty should be valid (defaults)",
		},
		{
			name:        "relative positioning -1",
			arg:         "-1",
			expectValid: true,
			description: "Relative positioning should be valid",
		},
		{
			name:        "relative positioning -5",
			arg:         "-5",
			expectValid: true,
			description: "Multi-digit relative positioning should be valid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := &App{}
			count, direction := app.parseYankArgument(tc.arg)
			
			// Simulate the validation logic from cmdYank
			isValid := true
			if direction != "" && direction != "k" && direction != "j" && direction != "-" {
				isValid = false
			}
			
			assert.Equal(t, tc.expectValid, isValid, tc.description)
			
			// Additional validation: count should always be positive
			assert.Greater(t, count, 0, "Count should always be positive")
		})
	}
}

// Benchmark the parsing function to ensure it's efficient
func BenchmarkParseYankArgument(b *testing.B) {
	app := &App{}
	testArgs := []string{"", "1", "5", "10k", "25j", "100", "999k"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		arg := testArgs[i%len(testArgs)]
		app.parseYankArgument(arg)
	}
}