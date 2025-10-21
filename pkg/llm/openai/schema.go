package openai

import (
	"encoding/json"
	"fmt"

	"github.com/kcaldas/genie/pkg/ai"
)

// schemaToJSON returns a formatted JSON representation of the provided schema.
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

func schemaToMap(schema *ai.Schema) map[string]any {
	if schema == nil {
		return map[string]any{
			"type": "object",
		}
	}

	result := map[string]any{
		"type": mapSchemaType(schema.Type),
	}

	if schema.Format != "" {
		result["format"] = schema.Format
	}
	if schema.Title != "" {
		result["title"] = schema.Title
	}
	if schema.Description != "" {
		result["description"] = schema.Description
	}
	if schema.Nullable {
		result["nullable"] = true
	}
	if schema.Enum != nil && len(schema.Enum) > 0 {
		result["enum"] = schema.Enum
	}
	if schema.Minimum != 0 {
		result["minimum"] = schema.Minimum
	}
	if schema.Maximum != 0 {
		result["maximum"] = schema.Maximum
	}
	if schema.MinLength > 0 {
		result["minLength"] = schema.MinLength
	}
	if schema.MaxLength > 0 {
		result["maxLength"] = schema.MaxLength
	}
	if schema.Pattern != "" {
		result["pattern"] = schema.Pattern
	}
	if schema.MinItems > 0 {
		result["minItems"] = schema.MinItems
	}
	if schema.MaxItems > 0 {
		result["maxItems"] = schema.MaxItems
	}
	if schema.MinProperties > 0 {
		result["minProperties"] = schema.MinProperties
	}
	if schema.MaxProperties > 0 {
		result["maxProperties"] = schema.MaxProperties
	}
	if schema.Required != nil && len(schema.Required) > 0 {
		result["required"] = schema.Required
	}
	if schema.Items != nil {
		result["items"] = schemaToMap(schema.Items)
	}
	if schema.Properties != nil && len(schema.Properties) > 0 {
		props := make(map[string]any, len(schema.Properties))
		for key, value := range schema.Properties {
			props[key] = schemaToMap(value)
		}
		result["properties"] = props
	}

	return result
}

func mapSchemaType(t ai.Type) string {
	switch t {
	case ai.TypeString:
		return "string"
	case ai.TypeNumber:
		return "number"
	case ai.TypeInteger:
		return "integer"
	case ai.TypeBoolean:
		return "boolean"
	case ai.TypeArray:
		return "array"
	case ai.TypeObject:
		return "object"
	default:
		return "object"
	}
}
