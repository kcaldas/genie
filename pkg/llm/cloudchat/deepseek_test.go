package cloudchat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/logging"
)

func newDeepSeekClient(t *testing.T, mockHTTP *mockHTTPClient, values map[string]string, bus events.EventBus) *Client {
	t.Helper()
	if values == nil {
		values = map[string]string{"DEEPSEEK_API_KEY": "test-key"}
	}
	raw, err := NewClient(DeepSeek, bus,
		WithConfigManager(&stubConfig{values: values}),
		WithHTTPClient(mockHTTP),
		WithLogger(logging.NewDisabledLogger()),
	)
	require.NoError(t, err)
	return raw.(*Client)
}

func answer(text string) chatResponse {
	return chatResponse{Choices: []chatChoice{{
		Message:      responseMessage{Role: "assistant", Content: responseContent{Parts: []contentPart{{Type: "text", Text: text}}}},
		FinishReason: "stop",
	}}}
}

func TestDeepSeek_SimpleResponse(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, func(_ int, req chatRequest) chatResponse {
		assert.False(t, req.Stream)
		assert.Equal(t, "deepseek-chat", req.Model)
		require.Len(t, req.Messages, 2)
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "You are a helpful assistant.", req.Messages[0].Content.Parts[0].Text)
		assert.Equal(t, "Say hello.", req.Messages[1].Content.Parts[0].Text)
		return answer("Hello there!")
	})
	client := newDeepSeekClient(t, mockHTTP, nil, &events.NoOpEventBus{})

	resp, err := client.GenerateContent(context.Background(), ai.Prompt{
		Instruction: "You are a helpful assistant.", Text: "Say hello.", ModelName: "deepseek-chat",
	}, false)
	require.NoError(t, err)
	assert.Equal(t, "Hello there!", resp)
	assert.Equal(t, "Bearer test-key", mockHTTP.headers[0].Get("Authorization"))
	assert.Equal(t, "https://api.deepseek.com/chat/completions", mockHTTP.urls[0])
}

func TestDeepSeek_MissingAPIKey(t *testing.T) {
	client := newDeepSeekClient(t, newMockHTTPClient(t), map[string]string{}, &events.NoOpEventBus{})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi"}, false)
	require.ErrorIs(t, err, errMissingAPIKey)
	assert.Contains(t, err.Error(), "DEEPSEEK_API_KEY")
}

// GENIE_DEEPSEEK_* names work as well as the vendor's own.
func TestDeepSeek_GenieKeyAndBaseURL(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, func(int, chatRequest) chatResponse { return answer("ok") })
	client := newDeepSeekClient(t, mockHTTP, map[string]string{
		"GENIE_DEEPSEEK_API_KEY":  "genie-key",
		"GENIE_DEEPSEEK_BASE_URL": "https://proxy.example.com/mutiro/deepseek/v1/",
	}, &events.NoOpEventBus{})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "deepseek-chat"}, false)
	require.NoError(t, err)
	assert.Equal(t, "https://proxy.example.com/mutiro/deepseek/v1/chat/completions", mockHTTP.urls[0])
	assert.Equal(t, "Bearer genie-key", mockHTTP.headers[0].Get("Authorization"))
}

func TestDeepSeek_DefaultModel(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, func(int, chatRequest) chatResponse { return answer("ok") })
	client := newDeepSeekClient(t, mockHTTP, nil, &events.NoOpEventBus{})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi"}, false)
	require.NoError(t, err)
	assert.Equal(t, "deepseek-flash", mockHTTP.requests[0].Model)
}

func TestDeepSeek_WithToolCall(t *testing.T) {
	mockHTTP := newMockHTTPClient(t,
		func(_ int, _ chatRequest) chatResponse {
			return chatResponse{Choices: []chatChoice{{
				Message: responseMessage{
					Role:    "assistant",
					Content: responseContent{Parts: []contentPart{{Type: "text", Text: ""}}},
					ToolCalls: []toolCall{{ID: "call_1", Type: "function",
						Function: toolCallFunction{Name: "get_weather", Arguments: json.RawMessage(`{"location":"Lisbon"}`)}}},
				},
				FinishReason: "tool_calls",
			}}}
		},
		func(_ int, req chatRequest) chatResponse {
			require.Len(t, req.Messages, 3)
			toolMsg := req.Messages[2]
			assert.Equal(t, "tool", toolMsg.Role)
			assert.Equal(t, "call_1", toolMsg.ToolCallID)
			assert.JSONEq(t, `{"temperature":22}`, toolMsg.Content.Parts[0].Text)
			return answer("It is sunny and 22°C.")
		},
	)
	client := newDeepSeekClient(t, mockHTTP, nil, &events.NoOpEventBus{})

	invoked := false
	resp, err := client.GenerateContent(context.Background(), ai.Prompt{
		Text: "What's the weather?", ModelName: "deepseek-chat",
		Functions: []*ai.FunctionDeclaration{{Name: "get_weather", Parameters: &ai.Schema{Type: ai.TypeObject}}},
		Handlers: map[string]ai.HandlerFunc{"get_weather": func(_ context.Context, attr map[string]any) (ai.ToolOutput, error) {
			invoked = true
			assert.Equal(t, map[string]any{"location": "Lisbon"}, attr)
			return ai.JSONToolOutput(map[string]any{"temperature": 22}), nil
		}},
	}, false)
	require.NoError(t, err)
	assert.Equal(t, "It is sunny and 22°C.", resp)
	assert.True(t, invoked)
	assert.Equal(t, 2, mockHTTP.callCount)
}

// Reasoning is published as a thinking event and never sent back:
// DeepSeek rejects reasoning_content in requests.
func TestDeepSeek_ReasoningPublishedNotEchoed(t *testing.T) {
	mockHTTP := newMockHTTPClient(t,
		func(int, chatRequest) chatResponse {
			return chatResponse{Choices: []chatChoice{{
				Message: responseMessage{
					Role:             "assistant",
					Content:          responseContent{Parts: []contentPart{{Type: "text", Text: ""}}},
					ReasoningContent: json.RawMessage(`"thinking about the weather"`),
					ToolCalls: []toolCall{{ID: "call_1", Type: "function",
						Function: toolCallFunction{Name: "get_weather", Arguments: json.RawMessage(`{}`)}}},
				},
				FinishReason: "tool_calls",
			}}}
		},
		func(int, chatRequest) chatResponse { return answer("Done.") },
	)
	bus := events.NewEventBus()
	thinking := make(chan events.ThinkingEvent, 2)
	bus.Subscribe(events.ThinkingEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.ThinkingEvent); ok {
			thinking <- event
		}
	})
	client := newDeepSeekClient(t, mockHTTP, nil, bus)

	resp, err := client.GenerateContent(context.Background(), ai.Prompt{
		Text: "What's the weather?", ModelName: "deepseek-reasoner",
		Functions: []*ai.FunctionDeclaration{{Name: "get_weather", Parameters: &ai.Schema{Type: ai.TypeObject}}},
		Handlers: map[string]ai.HandlerFunc{"get_weather": func(context.Context, map[string]any) (ai.ToolOutput, error) {
			return ai.JSONToolOutput(map[string]any{"ok": true}), nil
		}},
	}, false)
	require.NoError(t, err)
	assert.Equal(t, "Done.", resp)
	select {
	case event := <-thinking:
		assert.Equal(t, "thinking about the weather", event.Text)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for thinking event")
	}
	require.Len(t, mockHTTP.rawBodies, 2)
	assert.NotContains(t, string(mockHTTP.rawBodies[1]), "reasoning_content")
}

func TestDeepSeek_ResponseSchemaUsesJSONObjectMode(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, func(_ int, req chatRequest) chatResponse {
		require.NotNil(t, req.ResponseFormat)
		assert.Equal(t, "json_object", req.ResponseFormat.Type)
		systemText := req.Messages[0].Content.Parts[0].Text
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Contains(t, systemText, "Existing instruction.")
		assert.Contains(t, systemText, "JSON matching this schema")
		assert.Contains(t, systemText, "answer")
		return answer(`{"answer":"42"}`)
	})
	client := newDeepSeekClient(t, mockHTTP, nil, &events.NoOpEventBus{})

	resp, err := client.GenerateContent(context.Background(), ai.Prompt{
		Instruction: "Existing instruction.", Text: "Answer the question.", ModelName: "deepseek-chat",
		ResponseSchema: &ai.Schema{Type: ai.TypeObject, Properties: map[string]*ai.Schema{"answer": {Type: ai.TypeString}}},
	}, false)
	require.NoError(t, err)
	assert.JSONEq(t, `{"answer":"42"}`, resp)
}

// DeepSeek's native cache fields split the prompt.
func TestDeepSeek_CacheAwareUsage(t *testing.T) {
	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})
	mockHTTP := newMockHTTPClient(t, func(int, chatRequest) chatResponse {
		r := answer("ok")
		r.Usage = &usage{PromptTokens: 100, CompletionTokens: 20, TotalTokens: 120, PromptCacheHitTokens: 60, PromptCacheMissTokens: 40}
		return r
	})
	client := newDeepSeekClient(t, mockHTTP, nil, bus)

	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "deepseek-chat"}, false)
	require.NoError(t, err)
	select {
	case event := <-received:
		assert.Equal(t, "deepseek", event.Provider)
		assert.Equal(t, "deepseek-chat", event.Model)
		assert.Equal(t, int32(40), event.InputTokens)
		assert.Equal(t, int32(20), event.OutputTokens)
		assert.Equal(t, int32(60), event.CachedTokens)
		assert.Equal(t, int32(120), event.TotalTokens)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}
}

func TestDeepSeek_CountTokensMakesNoRequest(t *testing.T) {
	mockHTTP := newMockHTTPClient(t) // any request fails the test
	client := newDeepSeekClient(t, mockHTTP, nil, &events.NoOpEventBus{})

	count, err := client.CountTokens(context.Background(), ai.Prompt{
		Instruction: "You are a helpful assistant.", Text: "Count tokens for this text.", ModelName: "deepseek-chat",
	}, false)
	require.NoError(t, err)
	assert.Positive(t, count.TotalTokens)
	assert.Equal(t, count.TotalTokens, count.InputTokens)
	assert.Equal(t, 0, mockHTTP.callCount)
}

func TestDeepSeek_VisionModelSendsImageParts(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, func(_ int, req chatRequest) chatResponse {
		require.Len(t, req.Messages, 1)
		parts := req.Messages[0].Content.Parts
		require.Len(t, parts, 2)
		assert.Equal(t, "text", parts[0].Type)
		assert.Equal(t, "Describe this.", parts[0].Text)
		assert.Equal(t, "image_url", parts[1].Type)
		require.NotNil(t, parts[1].ImageURL)
		assert.True(t, strings.HasPrefix(parts[1].ImageURL.URL, "data:image/png;base64,"))
		return answer("A cat.")
	})
	client := newDeepSeekClient(t, mockHTTP, nil, &events.NoOpEventBus{})

	resp, err := client.GenerateContent(context.Background(), ai.Prompt{
		Text: "Describe this.", ModelName: "deepseek-v4-flash-vision-exp",
		Images: []*ai.Image{{Type: "image/png", Data: []byte{1, 2, 3}}},
	}, false)
	require.NoError(t, err)
	assert.Equal(t, "A cat.", resp)
}

func TestDeepSeek_TextModelDescribesImages(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, func(_ int, req chatRequest) chatResponse {
		parts := req.Messages[0].Content.Parts
		require.Len(t, parts, 1)
		assert.Contains(t, parts[0].Text, "attached image")
		assert.NotContains(t, parts[0].Text, "base64")
		return answer("ok")
	})
	client := newDeepSeekClient(t, mockHTTP, nil, &events.NoOpEventBus{})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{
		Text: "Describe this.", ModelName: "deepseek-v4-flash",
		Images: []*ai.Image{{Type: "image/png", Data: []byte{1, 2, 3}}},
	}, false)
	require.NoError(t, err)
}

// Tool-result images reach vision models as data-URL user messages and
// stay textual descriptions on text models.
func TestDeepSeek_ToolResultBlobHandling(t *testing.T) {
	blobOutput := ai.ToolOutput{Content: []ai.ToolContent{
		ai.TextContent{Text: "took a screenshot"},
		ai.BlobContent{Name: "shot.png", MIMEType: "image/png", Data: []byte{9, 9, 9}},
	}}
	makeMock := func(t *testing.T, assertSecond func(*testing.T, chatRequest)) *mockHTTPClient {
		return newMockHTTPClient(t,
			func(int, chatRequest) chatResponse {
				return chatResponse{Choices: []chatChoice{{
					Message: responseMessage{
						Role:    "assistant",
						Content: responseContent{Parts: []contentPart{{Type: "text", Text: ""}}},
						ToolCalls: []toolCall{{ID: "call_1", Type: "function",
							Function: toolCallFunction{Name: "screenshot", Arguments: json.RawMessage(`{}`)}}},
					},
					FinishReason: "tool_calls",
				}}}
			},
			func(_ int, req chatRequest) chatResponse {
				assertSecond(t, req)
				return answer("done")
			},
		)
	}
	run := func(t *testing.T, mockHTTP *mockHTTPClient, model string) {
		client := newDeepSeekClient(t, mockHTTP, nil, &events.NoOpEventBus{})
		_, err := client.GenerateContent(context.Background(), ai.Prompt{
			Text: "screenshot please", ModelName: model,
			Functions: []*ai.FunctionDeclaration{{Name: "screenshot", Parameters: &ai.Schema{Type: ai.TypeObject}}},
			Handlers: map[string]ai.HandlerFunc{"screenshot": func(context.Context, map[string]any) (ai.ToolOutput, error) {
				return blobOutput, nil
			}},
		}, false)
		require.NoError(t, err)
	}

	t.Run("vision model appends image message", func(t *testing.T) {
		run(t, makeMock(t, func(t *testing.T, req chatRequest) {
			require.Len(t, req.Messages, 4) // user, assistant, tool, image
			assert.Equal(t, "tool", req.Messages[2].Role)
			imageMsg := req.Messages[3]
			assert.Equal(t, "user", imageMsg.Role)
			require.Len(t, imageMsg.Content.Parts, 2)
			assert.Equal(t, "image_url", imageMsg.Content.Parts[1].Type)
			require.NotNil(t, imageMsg.Content.Parts[1].ImageURL)
			assert.True(t, strings.HasPrefix(imageMsg.Content.Parts[1].ImageURL.URL, "data:image/png;base64,"))
		}), "deepseek-v4-flash-vision-exp")
	})
	t.Run("text model describes the blob", func(t *testing.T) {
		run(t, makeMock(t, func(t *testing.T, req chatRequest) {
			require.Len(t, req.Messages, 3) // user, assistant, tool
			toolText := req.Messages[2].Content.Parts[0].Text
			assert.Contains(t, toolText, "shot.png")
			assert.Contains(t, toolText, "image/png")
		}), "deepseek-v4-flash")
	})
}

func TestDeepSeek_NativeHistoryLayout(t *testing.T) {
	client := newDeepSeekClient(t, newMockHTTPClient(t), nil, &events.NoOpEventBus{})
	messages, err := client.buildMessages(ai.Prompt{
		Instruction: "be kind",
		Context:     ai.TurnContext{Project: "# AGENTS.md", Files: "File: a.md", Host: "[Memory]"},
		History:     []ai.HistoryTurn{{User: "q1", Assistant: "a1"}},
		Text:        "q2",
	}, "deepseek-chat")
	require.NoError(t, err)
	require.Len(t, messages, 4)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "be kind\n\n# AGENTS.md", messages[0].Content.Parts[0].Text)
	assert.Equal(t, "user", messages[1].Role)
	assert.Equal(t, "assistant", messages[2].Role)
	assert.Equal(t, "File: a.md\n\n[Memory]\n\nq2", messages[3].Content.Parts[0].Text)
}

func TestDeepSeek_SchemaOnlyPromptGetsASystemMessage(t *testing.T) {
	client := newDeepSeekClient(t, newMockHTTPClient(t), nil, &events.NoOpEventBus{})
	schema := &ai.Schema{Type: ai.TypeObject, Properties: map[string]*ai.Schema{"ok": {Type: ai.TypeBoolean}}}
	messages, err := client.buildMessages(ai.Prompt{Text: "hi", ResponseSchema: schema}, "deepseek-chat")
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0].Role)
	assert.Contains(t, messages[0].Content.Parts[0].Text, "You must respond with JSON matching this schema")
	assert.Contains(t, messages[0].Content.Parts[0].Text, `"ok"`)
	assert.Equal(t, "user", messages[1].Role)
}
