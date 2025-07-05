package ai

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/kcaldas/genie/pkg/template"
)

// RenderPrompt takes a base prompt and renders it with the given data.
func RenderPrompt(base Prompt, data map[string]string) (Prompt, error) {
	renderedText, err := RenderTemplateString(base.Text, data)
	if err != nil {
		return Prompt{}, err
	}
	newPrompt := base
	newPrompt.Text = renderedText

	renderedInstruction, err := RenderTemplateString(base.Instruction, data)
	if err != nil {
		return Prompt{}, err
	}
	renderedInstruction = replaceGoTemplatePlaceholders(renderedInstruction)

	newPrompt.Instruction = renderedInstruction

	return newPrompt, nil
}

// replaceGoTemplatePlaceholders finds all occurrences of <%(...)%> and replaces them
// with {{...}}, preserving the content inside.
// This is necessary when we want to have instructions on how to render gotemplate on the prompt.
//
// Example usage in prompts:
//   Instead of: {{if .chat}}...{{end}}  (would be interpreted as template)
//   Use: <%if .chat%>...<%end%>         (will display as {{if .chat}}...{{end}})
//
// This is particularly useful for personas that generate other prompts or show
// template examples in their output (e.g., the persona_creator persona).
func replaceGoTemplatePlaceholders(input string) string {
	re := regexp.MustCompile(`(?s)<%(.*?)%>`)
	return re.ReplaceAllString(input, `{{$1}}`)
}

func RenderTemplateString(tpl string, data map[string]string) (string, error) {
	engine := template.NewEngine()
	return engine.RenderString(tpl, data)
}

// map string slice to Attr slice
func StringsToAttr(attrs []string) []Attr {
	if len(attrs)%2 != 0 {
		panic("attrs must have an even number of elements")
	}
	var result []Attr
	for i := 0; i < len(attrs); i += 2 {
		result = append(result, Attr{attrs[i], attrs[i+1]})
	}
	return result
}

func MapToAttr(attrs map[string]string) []Attr {
	var result []Attr
	for k, v := range attrs {
		result = append(result, Attr{k, v})
	}
	return result
}

// ToSchema is the top-level function that takes an object (either a value
// or pointer) and returns a *Schema describing it.
func ToSchema(input interface{}) (*Schema, error) {
	t := reflect.TypeOf(input)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return typeToSchema(t), nil
}

// typeToSchema walks a reflect.Type and produces the corresponding *Schema.
func typeToSchema(t reflect.Type) *Schema {
	switch t.Kind() {
	case reflect.Struct:
		// Build an object schema
		schema := &Schema{
			Type:       TypeObject, // integer constant for object
			Properties: map[string]*Schema{},
		}

		// Look at each exported field
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Skip unexported fields (PkgPath != "" means unexported)
			if field.PkgPath != "" {
				continue
			}

			// Derive the property name from the json tag, if present
			jsonTag := field.Tag.Get("json")
			propName := field.Name
			if jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if len(parts[0]) > 0 {
					propName = parts[0]
				}
			}

			fieldSchema := typeToSchema(field.Type)

			// Parse custom `schema:` tag to fill in description, etc.
			schemaTag := field.Tag.Get("schema")
			parseSchemaTag(schemaTag, fieldSchema)

			schema.Properties[propName] = fieldSchema
		}
		return schema

	case reflect.Slice:
		// Build an array schema
		return &Schema{
			Type:     TypeArray,
			Items:    typeToSchema(t.Elem()),
			MinItems: 0,
			MaxItems: 0, // 0 => no explicit max
		}

	case reflect.Map:
		// Optionally treat a map as an object
		return &Schema{
			Type:       TypeObject,
			Properties: map[string]*Schema{},
		}

	case reflect.String:
		// Build a string schema
		return &Schema{
			Type:      TypeString,
			MinLength: 0,
			MaxLength: 255, // Example default
		}

	case reflect.Bool:
		return &Schema{
			Type: TypeBoolean,
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		// Build an integer schema
		return &Schema{
			Type: TypeInteger,
		}

	case reflect.Float32, reflect.Float64:
		// Build a number schema
		return &Schema{
			Type: TypeNumber,
		}

	default:
		// Fallback if it's a func, chan, complex, interface, etc.
		return &Schema{
			Type: TypeString,
		}
	}
}

// parseSchemaTag looks at the custom `schema:` tag (e.g. "description=...,min=...,max=..., pattern=...")
// and fills the relevant fields of the *Schema object.
func parseSchemaTag(tag string, s *Schema) {
	if tag == "" {
		return
	}
	// Split by commas: "description=The person's full name,minLength=1"
	segments := strings.Split(tag, ",")
	for _, seg := range segments {
		// Each segment is "key=value"
		kv := strings.SplitN(seg, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])

		switch key {
		case "description":
			s.Description = val
		case "minLength":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				s.MinLength = n
			}
		case "maxLength":
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				s.MaxLength = n
			}
		case "minimum":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				s.Minimum = f
			}
		case "maximum":
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				s.Maximum = f
			}
		}
	}
}

// RemoveSurroundingMarkdown removes first and last lines if they start with ``` and removes empty lines at the beginning and end
func RemoveSurroundingMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	// Remove leading empty or whitespace lines
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	// Check for the starting backticks after removing empty lines
	if start < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[start]), "```") {
		start++
	}
	// Remove trailing empty/blank lines
	end := len(lines) - 1
	for end >= start && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	// Check for the ending backticks after removing empty lines
	if end >= start && strings.HasPrefix(strings.TrimSpace(lines[end]), "```") {
		end--
	}
	// If end < start, this implies no content between detected boundaries
	if end < start {
		return ""
	}
	return strings.Join(lines[start:end+1], "\n")
}
