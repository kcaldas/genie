package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCombineAtPosition(t *testing.T) {
	tests := []struct {
		name           string
		currentContent string
		position       int
		newContent     string
		expected       string
	}{
		{
			name:           "empty content",
			currentContent: "",
			position:       0,
			newContent:     "Hello",
			expected:       "Hello",
		},
		{
			name:           "insert at beginning",
			currentContent: "world",
			position:       0,
			newContent:     "Hello ",
			expected:       "Hello world",
		},
		{
			name:           "insert at end",
			currentContent: "Hello",
			position:       5,
			newContent:     " world",
			expected:       "Hello world",
		},
		{
			name:           "insert in middle",
			currentContent: "Hello world",
			position:       6,
			newContent:     "beautiful ",
			expected:       "Hello beautiful world",
		},
		{
			name:           "position beyond content",
			currentContent: "Short",
			position:       10,
			newContent:     " text",
			expected:       "Short text",
		},
		{
			name:           "multiline paste at beginning",
			currentContent: "existing",
			position:       0,
			newContent:     "Line1\nLine2\n",
			expected:       "Line1\nLine2\nexisting",
		},
		{
			name:           "multiline paste in middle",
			currentContent: "Start End",
			position:       6,
			newContent:     "Line1\nLine2\nLine3",
			expected:       "Start Line1\nLine2\nLine3End",
		},
		{
			name:           "negative position",
			currentContent: "Test",
			position:       -5,
			newContent:     "Before",
			expected:       "BeforeTest",
		},
		{
			name:           "multiline content with newlines",
			currentContent: "Line1\nLine2\nLine3",
			position:       6, // After "Line1\n"
			newContent:     "Insert\n",
			expected:       "Line1\nInsert\nLine2\nLine3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := combineAtPosition(tt.currentContent, tt.position, tt.newContent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCursorToStringPosition(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		cursorX  int
		cursorY  int
		expected int
	}{
		{
			name:     "empty content",
			content:  "",
			cursorX:  0,
			cursorY:  0,
			expected: 0,
		},
		{
			name:     "single line beginning",
			content:  "Hello world",
			cursorX:  0,
			cursorY:  0,
			expected: 0,
		},
		{
			name:     "single line middle",
			content:  "Hello world",
			cursorX:  6,
			cursorY:  0,
			expected: 6,
		},
		{
			name:     "single line end",
			content:  "Hello world",
			cursorX:  11,
			cursorY:  0,
			expected: 11,
		},
		{
			name:     "multiline second line beginning",
			content:  "Line1\nLine2\nLine3",
			cursorX:  0,
			cursorY:  1,
			expected: 6, // After "Line1\n"
		},
		{
			name:     "multiline second line middle",
			content:  "Line1\nLine2\nLine3",
			cursorX:  3,
			cursorY:  1,
			expected: 9, // After "Line1\nLin"
		},
		{
			name:     "cursor beyond line",
			content:  "Short\nLonger line",
			cursorX:  20,
			cursorY:  0,
			expected: 5, // Clamped to end of "Short"
		},
		{
			name:     "cursor beyond content",
			content:  "Line1\nLine2",
			cursorX:  0,
			cursorY:  5,
			expected: 11, // Length of entire content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cursorToStringPosition(tt.content, tt.cursorX, tt.cursorY)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStringPositionToCursor(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		position    int
		expectedX   int
		expectedY   int
	}{
		{
			name:        "empty content",
			content:     "",
			position:    0,
			expectedX:   0,
			expectedY:   0,
		},
		{
			name:        "single line beginning",
			content:     "Hello world",
			position:    0,
			expectedX:   0,
			expectedY:   0,
		},
		{
			name:        "single line middle",
			content:     "Hello world",
			position:    6,
			expectedX:   6,
			expectedY:   0,
		},
		{
			name:        "single line end",
			content:     "Hello world",
			position:    11,
			expectedX:   11,
			expectedY:   0,
		},
		{
			name:        "multiline second line beginning",
			content:     "Line1\nLine2\nLine3",
			position:    6, // After "Line1\n"
			expectedX:   0,
			expectedY:   1,
		},
		{
			name:        "multiline second line middle",
			content:     "Line1\nLine2\nLine3",
			position:    9, // After "Line1\nLin"
			expectedX:   3,
			expectedY:   1,
		},
		{
			name:        "position beyond content",
			content:     "Short",
			position:    10,
			expectedX:   5,
			expectedY:   0,
		},
		{
			name:        "negative position",
			content:     "Test",
			position:    -5,
			expectedX:   0,
			expectedY:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y := stringPositionToCursor(tt.content, tt.position)
			assert.Equal(t, tt.expectedX, x, "X coordinate mismatch")
			assert.Equal(t, tt.expectedY, y, "Y coordinate mismatch")
		})
	}
}