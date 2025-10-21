package ollama

import (
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
)

func mapFunctions(functions []*ai.FunctionDeclaration) []toolDefinition {
	if len(functions) == 0 {
		return nil
	}

	tools := make([]toolDefinition, 0, len(functions))
	for _, fn := range functions {
		if fn == nil {
			continue
		}

		definition := toolDefinition{
			Type: "function",
			Function: toolDefinitionDetails{
				Name: fn.Name,
			},
		}
		if strings.TrimSpace(fn.Description) != "" {
			definition.Function.Description = fn.Description
		}
		if schema := schemaToMap(fn.Parameters); schema != nil {
			definition.Function.Parameters = schema
		}
		tools = append(tools, definition)
	}

	if len(tools) == 0 {
		return nil
	}

	return tools
}
