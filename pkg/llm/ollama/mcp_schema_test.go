package ollama

import (
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMCPJSONSchemaPreserved(t *testing.T) {
	raw := map[string]any{"type": "object", "$defs": map[string]any{"value": map[string]any{"type": []string{"string", "null"}}}, "properties": map[string]any{"value": map[string]any{"$ref": "#/$defs/value"}}, "required": []string{"value"}, "additionalProperties": false}
	got := schemaToMap(&ai.Schema{JSONSchema: raw})
	require.Equal(t, raw, got)
	delete(got, "properties")
	require.Contains(t, raw, "properties", "provider conversion must not mutate the shared catalog")
}
