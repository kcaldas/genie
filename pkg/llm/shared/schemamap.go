package shared

import (
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
)

// SchemaToMap converts an ai.Schema into the JSON-schema map form the
// local providers send on the wire. A nil schema becomes a bare object
// schema. When ensureObjectProperties is set, object schemas without
// declared properties get an explicit empty properties map (LM Studio
// requires it for tool schemas).
func SchemaToMap(schema *ai.Schema, ensureObjectProperties bool) map[string]any {
	if schema == nil {
		return map[string]any{
			"type": "object",
		}
	}

	result := map[string]any{
		"type": schemaTypeName(schema.Type),
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
	if len(schema.Enum) > 0 {
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
	if len(schema.Required) > 0 {
		result["required"] = schema.Required
	}
	if schema.Items != nil {
		result["items"] = SchemaToMap(schema.Items, ensureObjectProperties)
	}
	if len(schema.Properties) > 0 {
		props := make(map[string]any, len(schema.Properties))
		for key, value := range schema.Properties {
			props[key] = SchemaToMap(value, ensureObjectProperties)
		}
		result["properties"] = props
	} else if ensureObjectProperties && result["type"] == "object" {
		result["properties"] = map[string]any{}
	}

	return result
}

func schemaTypeName(t ai.Type) string {
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

// ChooseSchemaName picks a stable name for a response schema: the
// schema title, then the prompt name, then "response".
func ChooseSchemaName(prompt ai.Prompt) string {
	if prompt.ResponseSchema != nil && prompt.ResponseSchema.Title != "" {
		return prompt.ResponseSchema.Title
	}
	if strings.TrimSpace(prompt.Name) != "" {
		return prompt.Name
	}
	return "response"
}
