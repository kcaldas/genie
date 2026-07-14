package shared

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatToolCallFunctionArgumentsAsMap(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want map[string]any
	}{
		{"empty", "", map[string]any{}},
		{"null", "null", map[string]any{}},
		{"object", `{"city":"Lisbon"}`, map[string]any{"city": "Lisbon"}},
		{"string wrapped", `"{\"city\":\"Lisbon\"}"`, map[string]any{"city": "Lisbon"}},
		{"blank string", `"  "`, map[string]any{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := ChatToolCallFunction{Name: "f", Arguments: json.RawMessage(tc.raw)}
			args, err := fn.ArgumentsAsMap()
			require.NoError(t, err)
			assert.Equal(t, tc.want, args)
		})
	}

	t.Run("invalid json errors", func(t *testing.T) {
		fn := ChatToolCallFunction{Name: "f", Arguments: json.RawMessage(`{"city":`)}
		_, err := fn.ArgumentsAsMap()
		require.Error(t, err)
	})
}

func TestMapFunctions(t *testing.T) {
	schemaToMap := func(s *ai.Schema) map[string]any { return SchemaToMap(s, false) }

	assert.Nil(t, MapFunctions(nil, schemaToMap))
	assert.Nil(t, MapFunctions([]*ai.FunctionDeclaration{nil}, schemaToMap))

	tools := MapFunctions([]*ai.FunctionDeclaration{
		{
			Name:        "get_weather",
			Description: "Weather lookup",
			Parameters: &ai.Schema{
				Type: ai.TypeObject,
				Properties: map[string]*ai.Schema{
					"city": {Type: ai.TypeString},
				},
			},
		},
	}, schemaToMap)

	require.Len(t, tools, 1)
	assert.Equal(t, "function", tools[0].Type)
	assert.Equal(t, "get_weather", tools[0].Function.Name)
	assert.Equal(t, "Weather lookup", tools[0].Function.Description)
	require.NotNil(t, tools[0].Function.Parameters)
	assert.Equal(t, "object", tools[0].Function.Parameters["type"])
}

func TestDedupeChatToolCalls(t *testing.T) {
	call := func(id, name, args string) ChatToolCall {
		return ChatToolCall{
			ID:   id,
			Type: "function",
			Function: ChatToolCallFunction{
				Name:      name,
				Arguments: json.RawMessage(args),
			},
		}
	}

	t.Run("drops identical calls keeping the first", func(t *testing.T) {
		wire, converted, err := DedupeChatToolCalls([]ChatToolCall{
			call("1", "a", `{"x":1}`),
			call("2", "a", `{"x":1}`),
			call("3", "a", `{"x":2}`),
		}, nil)
		require.NoError(t, err)
		require.Len(t, wire, 2)
		require.Len(t, converted, 2)
		assert.Equal(t, "1", wire[0].ID)
		assert.Equal(t, "3", wire[1].ID)
		assert.Equal(t, "a", converted[0].Name)
	})

	t.Run("resolves names before fingerprinting", func(t *testing.T) {
		resolve := func(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
		wire, converted, err := DedupeChatToolCalls([]ChatToolCall{
			call("1", "Tool ", `{"x":1}`),
			call("2", "tool", `{"x":1}`),
		}, resolve)
		require.NoError(t, err)
		require.Len(t, wire, 1)
		assert.Equal(t, "tool", converted[0].Name)
	})

	t.Run("invalid arguments abort without retry", func(t *testing.T) {
		_, _, err := DedupeChatToolCalls([]ChatToolCall{call("1", "a", `{"x":`)}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid arguments for function "a"`)
		assert.False(t, ai.IsRetryable(err))
	})
}

func TestSchemaToMapEnsureObjectProperties(t *testing.T) {
	schema := &ai.Schema{
		Type: ai.TypeObject,
		Items: &ai.Schema{
			Type: ai.TypeObject,
		},
	}

	plain := SchemaToMap(schema, false)
	assert.NotContains(t, plain, "properties")

	strict := SchemaToMap(schema, true)
	assert.Equal(t, map[string]any{}, strict["properties"])
	items, ok := strict["items"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{}, items["properties"], "flag must propagate to nested schemas")

	assert.Equal(t, map[string]any{"type": "object"}, SchemaToMap(nil, true))
}

func TestChooseSchemaName(t *testing.T) {
	assert.Equal(t, "titled", ChooseSchemaName(ai.Prompt{ResponseSchema: &ai.Schema{Title: "titled"}}))
	assert.Equal(t, "prompted", ChooseSchemaName(ai.Prompt{Name: "prompted", ResponseSchema: &ai.Schema{}}))
	assert.Equal(t, "response", ChooseSchemaName(ai.Prompt{}))
}

func TestScanStreamLines(t *testing.T) {
	t.Run("skips blanks and trims", func(t *testing.T) {
		var lines []string
		err := ScanStreamLines(strings.NewReader("a\n\n  b  \n"), "test", func(line string) error {
			lines = append(lines, line)
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, lines)
	})

	t.Run("stops cleanly on ErrStopStream", func(t *testing.T) {
		var lines []string
		err := ScanStreamLines(strings.NewReader("a\nb\nc\n"), "test", func(line string) error {
			lines = append(lines, line)
			if line == "b" {
				return ErrStopStream
			}
			return nil
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, lines)
	})

	t.Run("propagates handler errors", func(t *testing.T) {
		boom := errors.New("boom")
		err := ScanStreamLines(strings.NewReader("a\n"), "test", func(string) error { return boom })
		assert.ErrorIs(t, err, boom)
	})
}

func TestEncodeImageDataURL(t *testing.T) {
	assert.Empty(t, EncodeImageDataURL(nil))
	assert.Empty(t, EncodeImageDataURL(&ai.Image{}))

	url := EncodeImageDataURL(&ai.Image{Data: []byte{0x1}, Type: "image/webp"})
	assert.Equal(t, "data:image/webp;base64,AQ==", url)

	defaulted := EncodeImageDataURL(&ai.Image{Data: []byte{0x1}})
	assert.True(t, strings.HasPrefix(defaulted, "data:image/png;base64,"))
}
