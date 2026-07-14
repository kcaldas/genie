package shared

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
)

// ChatToolDefinition is the OpenAI-style tool declaration used by the
// local providers' chat APIs (Ollama, LM Studio).
type ChatToolDefinition struct {
	Type     string                 `json:"type"`
	Function ChatToolDefinitionSpec `json:"function"`
}

// ChatToolDefinitionSpec describes one declared function.
type ChatToolDefinitionSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ChatToolCall is the OpenAI-style tool invocation the model returns.
type ChatToolCall struct {
	ID       string               `json:"id"`
	Type     string               `json:"type"`
	Function ChatToolCallFunction `json:"function"`
}

// ChatToolCallFunction carries the requested function name and its raw
// JSON arguments.
type ChatToolCallFunction struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ArgumentsAsMap decodes the raw arguments, tolerating null, empty and
// double-encoded (string-wrapped) JSON payloads.
func (f ChatToolCallFunction) ArgumentsAsMap() (map[string]any, error) {
	if len(f.Arguments) == 0 {
		return map[string]any{}, nil
	}

	trimmed := TrimJSONSpace(f.Arguments)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return map[string]any{}, nil
	}

	var args map[string]any
	if trimmed[0] == '"' {
		var raw string
		if err := json.Unmarshal(f.Arguments, &raw); err != nil {
			return nil, err
		}
		if strings.TrimSpace(raw) == "" {
			return map[string]any{}, nil
		}
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			return nil, err
		}
		return args, nil
	}

	if err := json.Unmarshal(f.Arguments, &args); err != nil {
		return nil, err
	}
	if args == nil {
		return map[string]any{}, nil
	}
	return args, nil
}

// MapFunctions converts prompt function declarations into wire tool
// definitions using the provider's schema conversion.
func MapFunctions(functions []*ai.FunctionDeclaration, schemaToMap func(*ai.Schema) map[string]any) []ChatToolDefinition {
	if len(functions) == 0 {
		return nil
	}

	tools := make([]ChatToolDefinition, 0, len(functions))
	for _, fn := range functions {
		if fn == nil {
			continue
		}

		definition := ChatToolDefinition{
			Type: "function",
			Function: ChatToolDefinitionSpec{
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

// DedupeChatToolCalls parses wire tool calls into provider-neutral
// ToolCalls and drops exact duplicates (same resolved name and args)
// using the same fingerprint rule RunToolLoop applies, so the assistant
// message a provider records stays consistent with the calls the loop
// executes. resolveName may map wire names onto registered handler
// names (nil for identity). Argument parse failures abort the turn.
func DedupeChatToolCalls(calls []ChatToolCall, resolveName func(string) string) ([]ChatToolCall, []ToolCall, error) {
	keptWire := make([]ChatToolCall, 0, len(calls))
	kept := make([]ToolCall, 0, len(calls))
	seen := make(map[string]bool, len(calls))

	for _, call := range calls {
		args, err := call.Function.ArgumentsAsMap()
		if err != nil {
			return nil, nil, ai.NonRetryable(fmt.Errorf("invalid arguments for function %q: %w", call.Function.Name, err))
		}
		name := call.Function.Name
		if resolveName != nil {
			name = resolveName(name)
		}
		converted := ToolCall{ID: call.ID, Name: name, Args: args}

		fp := fingerprintCall(converted)
		if seen[fp] {
			continue
		}
		seen[fp] = true

		keptWire = append(keptWire, call)
		kept = append(kept, converted)
	}

	return keptWire, kept, nil
}

// TrimJSONSpace trims JSON whitespace from both ends of raw data.
func TrimJSONSpace(data []byte) []byte {
	start := 0
	end := len(data)
	for start < end && (data[start] == ' ' || data[start] == '\n' || data[start] == '\r' || data[start] == '\t') {
		start++
	}
	for end > start && (data[end-1] == ' ' || data[end-1] == '\n' || data[end-1] == '\r' || data[end-1] == '\t') {
		end--
	}
	return data[start:end]
}
