package ollama

import (
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

// schemaToFormat wraps the response schema in Ollama's structured
// output "format" field.
func schemaToFormat(prompt ai.Prompt) (map[string]any, error) {
	if prompt.ResponseSchema == nil {
		return nil, nil
	}

	mapped := schemaToMap(prompt.ResponseSchema)
	if mapped == nil {
		return nil, fmt.Errorf("response schema is empty")
	}

	return map[string]any{
		"type": "json_schema",
		"value": map[string]any{
			"name":   llmshared.ChooseSchemaName(prompt),
			"schema": mapped,
		},
	}, nil
}

// schemaToMap converts an ai.Schema into Ollama's JSON-schema map form.
func schemaToMap(schema *ai.Schema) map[string]any {
	return llmshared.SchemaToMap(schema, false)
}
