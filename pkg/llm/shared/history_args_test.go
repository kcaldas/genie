package shared

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Activity args are recorded as a `key="value"` one-liner. Replaying
// them as a tool call needs a JSON object, so the one-liner is parsed
// back; strings unquote, everything else stays as written.
func TestParseActionArgs_KeyValuePairs(t *testing.T) {
	assert.Equal(t, map[string]any{"command": "go test ./...", "timeout": float64(30)}, ParseActionArgs(`command="go test ./..." timeout=30`))
	assert.Equal(t, map[string]any{"path": `a "quoted" name`}, ParseActionArgs(`path="a \"quoted\" name"`))
	assert.Equal(t, map[string]any{"path": "a.go", "content": "x\ny"}, ParseActionArgs(`content="x\ny" path="a.go"`))
	assert.Equal(t, map[string]any{"flags": "[a b]", "recursive": true, "limit": nil}, ParseActionArgs(`flags=[a b] limit=<nil> recursive=true`))
	assert.Equal(t, map[string]any{"dry-run": false, "max.depth": float64(2)}, ParseActionArgs(`dry-run=false max.depth=2`))
	assert.Equal(t, map[string]any{"path": `a\`, "b": float64(1)}, ParseActionArgs(`path="a\\" b=1`), "an escaped backslash does not escape the closing quote")
}

// A bare value holding its own '=' cannot be split from the next pair
// reliably; the line stays raw rather than guessing.
func TestParseActionArgs_EqualsInsideBareValueFallsBack(t *testing.T) {
	raw := `env=[FOO=bar BAZ=qux] path="x"`
	assert.Equal(t, map[string]any{"args": raw}, ParseActionArgs(raw))
}

func TestParseActionArgs_MalformedFallsBack(t *testing.T) {
	assert.Equal(t, map[string]any{"args": `a="x"  b="y"`}, ParseActionArgs(`a="x"  b="y"`), "pairs are separated by exactly one space")
	assert.Equal(t, map[string]any{"args": `a="x" =1`}, ParseActionArgs(`a="x" =1`))
}

func TestParseActionArgs_Empty(t *testing.T) {
	assert.Equal(t, map[string]any{}, ParseActionArgs(""))
	assert.Equal(t, map[string]any{}, ParseActionArgs("   "))
}

// A truncated one-liner (the digest caps every field) cannot be parsed
// back. The raw text still reaches the model, under a single key.
func TestParseActionArgs_TruncatedFallsBackToRaw(t *testing.T) {
	for _, raw := range []string{
		`command="go test ./... && go vet ./..` + "…", // cut inside a string
		`path="a.go" timeout=3` + "…",                 // cut inside a number
		`path="a.go" recur` + "…",                     // cut inside a key
	} {
		assert.Equal(t, map[string]any{"args": raw}, ParseActionArgs(raw), raw)
	}
	assert.Equal(t, map[string]any{"args": "not pairs at all"}, ParseActionArgs("not pairs at all"))
}

func TestActionArguments_IsCompactJSON(t *testing.T) {
	assert.JSONEq(t, `{"command":"go test","path":"."}`, string(ActionArguments(`command="go test" path="."`)))
	assert.Equal(t, `{}`, string(ActionArguments("")))
}

func TestStringEncodedArguments(t *testing.T) {
	calls := StringEncodedArguments([]ChatToolCall{
		{Function: ChatToolCallFunction{Arguments: json.RawMessage(`{"a":1}`)}},
		{Function: ChatToolCallFunction{Arguments: json.RawMessage(`"{\"b\":2}"`)}},
		{Function: ChatToolCallFunction{}},
	})

	assert.Equal(t, `"{\"a\":1}"`, string(calls[0].Function.Arguments))
	assert.Equal(t, `"{\"b\":2}"`, string(calls[1].Function.Arguments), "already a string: untouched")
	assert.Empty(t, calls[2].Function.Arguments)
	assert.Equal(t, "null", string(StringEncodedArguments([]ChatToolCall{{Function: ChatToolCallFunction{Arguments: json.RawMessage(`null`)}}})[0].Function.Arguments), "null is absent, not the string \"null\"")
}
