package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_RenderString(t *testing.T) {
	engine := NewEngine()

	result, err := engine.RenderString("Hello {{.name}}", map[string]string{
		"name": "World",
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello World", result)
}

func TestEngine_RenderString_WithIndentFunction(t *testing.T) {
	engine := NewEngine()

	result, err := engine.RenderString("Code:\n{{indent 2 .code}}", map[string]string{
		"code": "func main() {\n  println(\"hello\")\n}",
	})

	require.NoError(t, err)
	expected := "Code:\n  func main() {\n    println(\"hello\")\n  }"
	assert.Equal(t, expected, result)
}

func TestEngine_RenderString_Error(t *testing.T) {
	engine := NewEngine()

	// Test with invalid template syntax
	_, err := engine.RenderString("Hello {{.name", map[string]string{
		"name": "World",
	})

	assert.Error(t, err)
}
