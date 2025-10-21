package anthropic

import (
	"strings"

	anthropic_sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"

	"github.com/kcaldas/genie/pkg/ai"
)

func mapFunctions(functions []*ai.FunctionDeclaration) []*anthropic_sdk.ToolParam {
	if len(functions) == 0 {
		return nil
	}

	tools := make([]*anthropic_sdk.ToolParam, 0, len(functions))
	for _, fn := range functions {
		if fn == nil || strings.TrimSpace(fn.Name) == "" {
			continue
		}

		inputSchema := anthropic_sdk.ToolInputSchemaParam{
			Type: constant.Object(""),
		}

		if schema := schemaToMap(fn.Parameters); schema != nil {
			if props, ok := schema["properties"]; ok {
				inputSchema.Properties = props
				delete(schema, "properties")
			}
			if req, ok := schema["required"].([]string); ok {
				inputSchema.Required = req
				delete(schema, "required")
			} else if reqSlice, ok := schema["required"].([]any); ok {
				for _, r := range reqSlice {
					if s, ok := r.(string); ok {
						inputSchema.Required = append(inputSchema.Required, s)
					}
				}
				delete(schema, "required")
			}
			if len(schema) > 0 {
				inputSchema.ExtraFields = schema
			}
		}

		tool := &anthropic_sdk.ToolParam{
			Name:        fn.Name,
			InputSchema: inputSchema,
			Type:        anthropic_sdk.ToolTypeCustom,
		}
		if desc := strings.TrimSpace(fn.Description); desc != "" {
			tool.Description = anthropic_sdk.String(desc)
		}
		tools = append(tools, tool)
	}

	if len(tools) == 0 {
		return nil
	}
	return tools
}
