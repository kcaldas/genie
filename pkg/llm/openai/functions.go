package openai

import (
	"strings"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"

	"github.com/kcaldas/genie/pkg/ai"
)

func mapFunctions(functions []*ai.FunctionDeclaration) []openai.ChatCompletionToolParam {
	if len(functions) == 0 {
		return nil
	}

	tools := make([]openai.ChatCompletionToolParam, 0, len(functions))
	for _, fn := range functions {
		if fn == nil {
			continue
		}

		definition := shared.FunctionDefinitionParam{
			Name: fn.Name,
		}
		if strings.TrimSpace(fn.Description) != "" {
			definition.Description = openai.String(fn.Description)
		}
		if schema := schemaToMap(fn.Parameters); schema != nil {
			definition.Parameters = schema
		}

		tools = append(tools, openai.ChatCompletionToolParam{
			Function: definition,
		})
	}

	if len(tools) == 0 {
		return nil
	}

	return tools
}
