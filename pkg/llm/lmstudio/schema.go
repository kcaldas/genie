package lmstudio

import (
	"encoding/json"
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

// schemaToResponseFormat wraps the response schema in the OpenAI-style
// response_format field LM Studio expects.
func schemaToResponseFormat(prompt ai.Prompt) (*responseFormat, error) {
	if prompt.ResponseSchema == nil {
		return nil, nil
	}

	mapped := schemaToMap(prompt.ResponseSchema)
	if mapped == nil {
		return nil, fmt.Errorf("response schema is empty")
	}

	return &responseFormat{
		Type: "json_schema",
		JSONSchema: &responseFormatSchema{
			Name:   llmshared.ChooseSchemaName(prompt),
			Schema: mapped,
			Strict: true,
		},
	}, nil
}

func schemaToJSON(schema *ai.Schema) (string, error) {
	if schema == nil {
		return "", nil
	}

	mapped := schemaToMap(schema)
	bytes, err := json.MarshalIndent(mapped, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal schema: %w", err)
	}
	return string(bytes), nil
}

// schemaToMap converts an ai.Schema into LM Studio's JSON-schema map
// form. LM Studio requires an explicit properties object for object
// tool schemas.
func schemaToMap(schema *ai.Schema) map[string]any {
	return llmshared.SchemaToMap(schema, true)
}
