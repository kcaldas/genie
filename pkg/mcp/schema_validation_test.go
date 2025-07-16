package mcp

import (
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
)

func TestConvertToolSchemaProperty_HandlesArrays(t *testing.T) {
	// Test array with explicit items
	arrayPropWithItems := ToolSchemaProperty{
		Type:        "array",
		Description: "Array of strings",
		Items: &ToolSchemaProperty{
			Type: "string",
		},
	}

	result := convertToolSchemaProperty(arrayPropWithItems)
	
	if result.Type != ai.TypeArray {
		t.Errorf("Expected TypeArray, got %v", result.Type)
	}
	
	if result.Items == nil {
		t.Fatal("Expected Items to be set for array type")
	}
	
	if result.Items.Type != ai.TypeString {
		t.Errorf("Expected Items.Type to be TypeString, got %v", result.Items.Type)
	}
}

func TestConvertToolSchemaProperty_HandlesArraysWithoutItems(t *testing.T) {
	// Test array without explicit items (should get default)
	arrayPropWithoutItems := ToolSchemaProperty{
		Type:        "array",
		Description: "Array without items specified",
	}

	result := convertToolSchemaProperty(arrayPropWithoutItems)
	
	if result.Type != ai.TypeArray {
		t.Errorf("Expected TypeArray, got %v", result.Type)
	}
	
	if result.Items == nil {
		t.Fatal("Expected Items to be set for array type (default)")
	}
	
	if result.Items.Type != ai.TypeString {
		t.Errorf("Expected default Items.Type to be TypeString, got %v", result.Items.Type)
	}
}

func TestConvertToolSchemaProperty_HandlesNestedObjects(t *testing.T) {
	// Test object with nested properties
	objectProp := ToolSchemaProperty{
		Type:        "object",
		Description: "Object with nested properties",
		Properties: map[string]ToolSchemaProperty{
			"name": {
				Type:        "string",
				Description: "Name field",
			},
			"tags": {
				Type:        "array",
				Description: "Array of tags",
				Items: &ToolSchemaProperty{
					Type: "string",
				},
			},
		},
	}

	result := convertToolSchemaProperty(objectProp)
	
	if result.Type != ai.TypeObject {
		t.Errorf("Expected TypeObject, got %v", result.Type)
	}
	
	if result.Properties == nil {
		t.Fatal("Expected Properties to be set for object type")
	}
	
	// Check name property
	nameProp, exists := result.Properties["name"]
	if !exists {
		t.Fatal("Expected 'name' property to exist")
	}
	if nameProp.Type != ai.TypeString {
		t.Errorf("Expected name property to be TypeString, got %v", nameProp.Type)
	}
	
	// Check tags property (array)
	tagsProp, exists := result.Properties["tags"]
	if !exists {
		t.Fatal("Expected 'tags' property to exist")
	}
	if tagsProp.Type != ai.TypeArray {
		t.Errorf("Expected tags property to be TypeArray, got %v", tagsProp.Type)
	}
	if tagsProp.Items == nil {
		t.Fatal("Expected tags property to have Items")
	}
	if tagsProp.Items.Type != ai.TypeString {
		t.Errorf("Expected tags items to be TypeString, got %v", tagsProp.Items.Type)
	}
}

func TestConvertMCPSchemaToGenieSchema_Integration(t *testing.T) {
	// Test the full schema conversion with complex schema
	mcpSchema := ToolSchema{
		Type: "object",
		Properties: map[string]ToolSchemaProperty{
			"entities": {
				Type:        "array",
				Description: "List of entities",
				// No Items specified - should get default
			},
			"relations": {
				Type:        "array",
				Description: "List of relations",
				Items: &ToolSchemaProperty{
					Type: "object",
					Properties: map[string]ToolSchemaProperty{
						"from": {Type: "string"},
						"to":   {Type: "string"},
						"type": {Type: "string"},
					},
				},
			},
			"metadata": {
				Type:        "object",
				Description: "Metadata object",
				Properties: map[string]ToolSchemaProperty{
					"version": {Type: "string"},
					"created": {Type: "string"},
				},
			},
		},
		Required: []string{"entities"},
	}

	result := convertMCPSchemaToGenieSchema(mcpSchema)
	
	// Check top-level structure
	if result.Type != ai.TypeObject {
		t.Errorf("Expected TypeObject, got %v", result.Type)
	}
	
	if len(result.Required) != 1 || result.Required[0] != "entities" {
		t.Errorf("Expected Required to be ['entities'], got %v", result.Required)
	}
	
	// Check entities array (should have default items)
	entities, exists := result.Properties["entities"]
	if !exists {
		t.Fatal("Expected 'entities' property")
	}
	if entities.Type != ai.TypeArray {
		t.Errorf("Expected entities to be TypeArray, got %v", entities.Type)
	}
	if entities.Items == nil {
		t.Fatal("Expected entities to have default Items")
	}
	if entities.Items.Type != ai.TypeString {
		t.Errorf("Expected entities items to be default TypeString, got %v", entities.Items.Type)
	}
	
	// Check relations array (should have explicit object items)
	relations, exists := result.Properties["relations"]
	if !exists {
		t.Fatal("Expected 'relations' property")
	}
	if relations.Type != ai.TypeArray {
		t.Errorf("Expected relations to be TypeArray, got %v", relations.Type)
	}
	if relations.Items == nil {
		t.Fatal("Expected relations to have Items")
	}
	if relations.Items.Type != ai.TypeObject {
		t.Errorf("Expected relations items to be TypeObject, got %v", relations.Items.Type)
	}
	
	// Check nested object properties in relations items
	if relations.Items.Properties == nil {
		t.Fatal("Expected relations items to have Properties")
	}
	if len(relations.Items.Properties) != 3 {
		t.Errorf("Expected relations items to have 3 properties, got %d", len(relations.Items.Properties))
	}
}