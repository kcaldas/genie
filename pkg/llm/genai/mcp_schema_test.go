package genai

import (
	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMCPJSONSchemaPreserved(t *testing.T) {
	raw := map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"type": []string{"string", "null"}}}, "additionalProperties": false}
	functions := []*ai.FunctionDeclaration{{Name: "remote", Parameters: &ai.Schema{JSONSchema: raw}, Response: &ai.Schema{JSONSchema: raw}}}
	got := (&Client{}).mapFunctions(&functions)
	require.Len(t, got, 1)
	require.Nil(t, got[0].Parameters)
	require.Equal(t, raw, got[0].ParametersJsonSchema)
	require.Equal(t, raw, got[0].ResponseJsonSchema)
}
