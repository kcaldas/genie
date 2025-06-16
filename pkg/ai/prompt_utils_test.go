package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_To_Schema(t *testing.T) {
	type Forecat struct {
		Location        string `json:"location" schema:"description=The Location,maxLength=100"`
		Temperature     int    `json:"temperature" schema:"description=The Temperature"`
		TemperatureUnit string `json:"temperature_unit" schema:"description=The Temperature Unit,maxLength=1"`
		Umidity         int    `json:"humidity"`
		Wind            string `json:"wind" schema:"maxLength=20"`
	}
	schema, err := ToSchema(&Forecat{})
	require.NoError(t, err)

	// Check that the schema was created correctly
	expectedSchema := &Schema{
		Type: TypeObject,
		Properties: map[string]*Schema{
			"location": {
				Type:        TypeString,
				Description: "The Location",
				MaxLength:   100,
			},
			"temperature": {
				Type:        TypeInteger,
				Description: "The Temperature",
			},
			"temperature_unit": {
				Type:        TypeString,
				Description: "The Temperature Unit",
				MaxLength:   1,
			},
			"humidity": {
				Type: TypeInteger,
			},
			"wind": {
				Type:      TypeString,
				MaxLength: 20,
			},
		},
	}
	assert.Equal(t, expectedSchema, schema)
}

func Test_removeSurroundingMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no markdown",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "markdown at start only",
			input:    "```python\nprint('hello')\n",
			expected: "print('hello')",
		},
		{
			name:     "markdown at end only",
			input:    "print('hello')\n```",
			expected: "print('hello')",
		},
		{
			name:     "markdown at start and end",
			input:    "```python\nprint('hello')\n```",
			expected: "print('hello')",
		},
		{
			name:     "multiple lines with markdown",
			input:    "```python\nprint('hello')\nprint('world')\n```",
			expected: "print('hello')\nprint('world')",
		},
		{
			name:     "empty content",
			input:    "",
			expected: "",
		},
		{
			name:     "just markdown tags",
			input:    "```\n```",
			expected: "",
		},
		{
			name:     "markdown in the middle",
			input:    "print('hello')\n```\nprint('world')",
			expected: "print('hello')\n```\nprint('world')",
		},
		{
			name:     "markdown with language and no content",
			input:    "```go\n```",
			expected: "",
		},
		{
			name:     "markdown at start with language specification",
			input:    "```javascript\nconsole.log('hello');\n",
			expected: "console.log('hello');",
		},
		{
			name:     "markdown with multiple backticks",
			input:    "````\nprint('hello')\n````",
			expected: "print('hello')",
		},
		{
			name:     "single line with markdown - current implementation limitation",
			input:    "```console.log('hello');```",
			expected: "",
		},
		{
			name:     "markdown with space and additional line at the end",
			input:    "```python\nprint('hello')\n ```\n",
			expected: "print('hello')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeSurroundingMarkdown(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
