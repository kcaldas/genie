package openaicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageContent_MarshalShapes(t *testing.T) {
	t.Parallel()

	empty, err := json.Marshal(MessageContent{})
	require.NoError(t, err)
	assert.Equal(t, `""`, string(empty))

	single, err := json.Marshal(NewMessageContentFromText("hello"))
	require.NoError(t, err)
	assert.Equal(t, `"hello"`, string(single))

	multi, err := json.Marshal(NewMessageContent([]ContentPart{
		{Type: "text", Text: "look:"},
		{Type: "image_url", ImageURL: &ImageURL{URL: "data:image/png;base64,xx"}},
	}))
	require.NoError(t, err)
	assert.JSONEq(t, `[{"type":"text","text":"look:"},{"type":"image_url","image_url":{"url":"data:image/png;base64,xx"}}]`, string(multi))
}

func TestMessageContent_UnmarshalShapes(t *testing.T) {
	t.Parallel()

	var fromString MessageContent
	require.NoError(t, json.Unmarshal([]byte(`"hi"`), &fromString))
	require.Len(t, fromString.Parts, 1)
	assert.Equal(t, "hi", fromString.Parts[0].Text)

	var fromArray MessageContent
	require.NoError(t, json.Unmarshal([]byte(`[{"type":"text","text":"a"},{"type":"text","text":"b"}]`), &fromArray))
	require.Len(t, fromArray.Parts, 2)
}

func TestResponseContent_UnmarshalShapesAndText(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		`"plain"`:                         "plain",
		`{"type":"text","text":"object"}`: "object",
		`[{"type":"text","text":"a"},{"type":"text","text":"b"}]`: "a\nb",
	}
	for input, want := range cases {
		var content ResponseContent
		require.NoError(t, json.Unmarshal([]byte(input), &content), input)
		assert.Equal(t, want, content.Text(), input)
	}

	var null ResponseContent
	require.NoError(t, json.Unmarshal([]byte(`null`), &null))
	assert.Equal(t, "", null.Text())
}

// Reasoning arrives under different keys depending on the server:
// LM Studio streams `reasoning`, DeepSeek uses `reasoning_content`.
// Both must decode.
func TestResponseMessage_ReasoningText_BothKeys(t *testing.T) {
	t.Parallel()

	var viaReasoning ResponseMessage
	require.NoError(t, json.Unmarshal([]byte(`{"role":"assistant","content":"","reasoning":"thinking A"}`), &viaReasoning))
	assert.Equal(t, "thinking A", viaReasoning.ReasoningText())

	var viaReasoningContent ResponseMessage
	require.NoError(t, json.Unmarshal([]byte(`{"role":"assistant","content":"","reasoning_content":"thinking B"}`), &viaReasoningContent))
	assert.Equal(t, "thinking B", viaReasoningContent.ReasoningText())
}

func TestStreamDelta_ReasoningText_BothKeys(t *testing.T) {
	t.Parallel()

	var viaReasoning StreamDelta
	require.NoError(t, json.Unmarshal([]byte(`{"reasoning":"step A"}`), &viaReasoning))
	assert.Equal(t, "step A", viaReasoning.ReasoningText())

	var viaReasoningContent StreamDelta
	require.NoError(t, json.Unmarshal([]byte(`{"reasoning_content":"step B"}`), &viaReasoningContent))
	assert.Equal(t, "step B", viaReasoningContent.ReasoningText())

	var content StreamDelta
	require.NoError(t, json.Unmarshal([]byte(`{"content":"visible"}`), &content))
	assert.Equal(t, "visible", content.Text())
}

// Assistant messages echoed back into the conversation must never carry
// reasoning payloads: DeepSeek rejects reasoning_content in input messages.
func TestResponseMessage_ToChatMessage_DropsReasoning(t *testing.T) {
	t.Parallel()

	var message ResponseMessage
	require.NoError(t, json.Unmarshal([]byte(`{"role":"assistant","content":"done","reasoning_content":"secret thoughts"}`), &message))

	echoed, err := json.Marshal(message.ToChatMessage())
	require.NoError(t, err)
	assert.NotContains(t, string(echoed), "reasoning")
	assert.NotContains(t, string(echoed), "secret thoughts")
	assert.Contains(t, string(echoed), "done")
}

func TestUsage_TokenCount(t *testing.T) {
	t.Parallel()

	count := (&Usage{PromptTokens: 8, CompletionTokens: 2, TotalTokens: 10}).TokenCount()
	require.NotNil(t, count)
	assert.Equal(t, int32(10), count.TotalTokens)
	assert.Equal(t, int32(8), count.InputTokens)
	assert.Equal(t, int32(2), count.OutputTokens)

	// A missing total falls back to input + output.
	fallback := (&Usage{PromptTokens: 3, CompletionTokens: 4}).TokenCount()
	assert.Equal(t, int32(7), fallback.TotalTokens)

	var nilUsage *Usage
	assert.Nil(t, nilUsage.TokenCount())
}
