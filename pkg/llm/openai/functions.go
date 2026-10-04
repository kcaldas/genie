package openai

import (
	"strings"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/kcaldas/genie/pkg/ai"
)

func mapFunctions(functions []*ai.FunctionDeclaration) []openai.ChatCompletionToolUnionParam {
	if len(functions) == 0 {
		return nil
	}

	tools := make([]openai.ChatCompletionToolUnionParam, 0, len(functions))
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

		tools = append(tools, openai.ChatCompletionFunctionTool(definition))
	}

	if len(tools) == 0 {
		return nil
	}

	return tools
}

func mapResponseFunctions(functions []*ai.FunctionDeclaration) []responses.ToolUnionParam {
	if len(functions) == 0 {
		return nil
	}

	tools := make([]responses.ToolUnionParam, 0, len(functions))
	for _, fn := range functions {
		if fn == nil {
			continue
		}

		definition := responses.FunctionToolParam{
			Name:   fn.Name,
			Strict: openai.Bool(false),
		}
		if strings.TrimSpace(fn.Description) != "" {
			definition.Description = openai.String(fn.Description)
		}
		if schema := schemaToMap(fn.Parameters); schema != nil {
			definition.Parameters = schema
		}

		tools = append(tools, responses.ToolUnionParam{
			OfFunction: &definition,
		})
	}

	if len(tools) == 0 {
		return nil
	}

	return tools
}
