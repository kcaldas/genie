package deepseek

import (
	"encoding/json"
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

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

// schemaToMap converts an ai.Schema into DeepSeek's JSON-schema map form
// (used for tool parameter declarations and prompt-carried response
// schemas). Object schemas keep an explicit properties object.
func schemaToMap(schema *ai.Schema) map[string]any {
	return llmshared.SchemaToMap(schema, true)
}
