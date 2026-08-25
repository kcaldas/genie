package openaicompat

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

func testChatRequest() ChatRequest {
	return ChatRequest{
		Model: "some-model",
		Messages: []ChatMessage{
			{Role: "user", Content: NewMessageContentFromText("hello")},
		},
	}
}

func TestTurn_Step_TextOutcome(t *testing.T) {
	t.Parallel()

	mock := newMockHTTPClient(t, func(call int, req ChatRequest) ChatResponse {
		return ChatResponse{
			Choices: []ChatChoice{{
				Message:      ResponseMessage{Role: "assistant", Content: textResponseContent("Hello there!")},
				FinishReason: "stop",
			}},
			Usage: &Usage{PromptTokens: 8, CompletionTokens: 2, TotalTokens: 10},
		}
	})
	core := newTestCore(mock)
	turn := core.NewTurn(testChatRequest(), nil, TurnOptions{})

	outcome, err := turn.Step(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, "Hello there!", outcome.Text)
	assert.Empty(t, outcome.ToolCalls)
	require.NotNil(t, outcome.Usage)
	assert.Equal(t, int32(10), outcome.Usage.TotalTokens)
}

func TestTurn_Step_EmptyResponseIsNonRetryable(t *testing.T) {
	t.Parallel()

	mock := newMockHTTPClient(t, func(call int, req ChatRequest) ChatResponse {
		return ChatResponse{Choices: []ChatChoice{{
			Message: ResponseMessage{Role: "assistant", Content: textResponseContent("")},
		}}}
	})
	core := newTestCore(mock)
	turn := core.NewTurn(testChatRequest(), nil, TurnOptions{})

	_, err := turn.Step(context.Background(), nil)
	require.ErrorIs(t, err, ErrEmptyResponse)
	assert.False(t, ai.IsRetryable(err))
	// The provider name identifies who failed.
	assert.Contains(t, err.Error(), "testprovider")
}

func TestTurn_Step_ToolCallsAppendAssistantMessageWithoutReasoning(t *testing.T) {
	t.Parallel()

	mock := newMockHTTPClient(t,
		func(call int, req ChatRequest) ChatResponse {
			return ChatResponse{Choices: []ChatChoice{{
				Message: ResponseMessage{
					Role:             "assistant",
					Content:          textResponseContent(""),
					ReasoningContent: json.RawMessage(`"thinking hard"`),
					ToolCalls: []llmshared.ChatToolCall{{
						ID:   "call_1",
						Type: "function",
						Function: llmshared.ChatToolCallFunction{
							Name:      "get_weather",
							Arguments: json.RawMessage(`{"location":"Lisbon"}`),
						},
					}},
				},
				FinishReason: "tool_calls",
			}}}
		},
		func(call int, req ChatRequest) ChatResponse {
			// History now carries the assistant tool-call message and the tool result.
			require.Len(t, req.Messages, 3)
			assert.Equal(t, "assistant", req.Messages[1].Role)
			assert.Equal(t, "tool", req.Messages[2].Role)
			assert.Equal(t, "call_1", req.Messages[2].ToolCallID)
			return ChatResponse{Choices: []ChatChoice{{
				Message: ResponseMessage{Role: "assistant", Content: textResponseContent("Sunny.")},
			}}}
		},
	)

	bus := events.NewEventBus()
	thinking := make(chan events.ThinkingEvent, 1)
	bus.Subscribe(events.ThinkingEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.ThinkingEvent); ok {
			thinking <- event
		}
	})
	core := newTestCoreWithBus(mock, bus)

	handlers := map[string]ai.HandlerFunc{
		"get_weather": func(ctx context.Context, attr map[string]any) (ai.ToolOutput, error) {
			return ai.JSONToolOutput(map[string]any{"temperature": 22}), nil
		},
	}
	turn := core.NewTurn(testChatRequest(), handlers, TurnOptions{})

	outcome, err := turn.Step(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, outcome.ToolCalls, 1)
	assert.Equal(t, "get_weather", outcome.ToolCalls[0].Name)

	select {
	case event := <-thinking:
		assert.Equal(t, "thinking hard", event.Text)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for thinking event")
	}

	// Execute the tool and continue the turn: the echoed assistant
	// message must not contain reasoning payloads.
	output, err := handlers["get_weather"](context.Background(), outcome.ToolCalls[0].Args)
	require.NoError(t, err)
	require.NoError(t, turn.AddToolResults(context.Background(), []llmshared.PreparedToolResult{
		{Call: outcome.ToolCalls[0], Output: output},
	}))

	final, err := turn.Step(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, "Sunny.", final.Text)
	assert.NotContains(t, string(mock.rawBodies[1]), "reasoning")
}

func TestTurn_Step_EmptyAfterToolUseIsAccepted(t *testing.T) {
	t.Parallel()

	mock := newMockHTTPClient(t,
		func(call int, req ChatRequest) ChatResponse {
			return ChatResponse{Choices: []ChatChoice{{
				Message: ResponseMessage{
					Role:    "assistant",
					Content: textResponseContent(""),
					ToolCalls: []llmshared.ChatToolCall{{
						ID: "call_1", Type: "function",
						Function: llmshared.ChatToolCallFunction{Name: "noop", Arguments: json.RawMessage(`{}`)},
					}},
				},
			}}}
		},
		func(call int, req ChatRequest) ChatResponse {
			return ChatResponse{Choices: []ChatChoice{{
				Message: ResponseMessage{Role: "assistant", Content: textResponseContent("")},
			}}}
		},
	)
	core := newTestCore(mock)
	handlers := map[string]ai.HandlerFunc{
		"noop": func(ctx context.Context, attr map[string]any) (ai.ToolOutput, error) {
			return ai.ToolOutput{Content: []ai.ToolContent{ai.TextContent{Text: "ok"}}}, nil
		},
	}
	turn := core.NewTurn(testChatRequest(), handlers, TurnOptions{})

	outcome, err := turn.Step(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, outcome.ToolCalls, 1)

	require.NoError(t, turn.AddToolResults(context.Background(), []llmshared.PreparedToolResult{
		{Call: outcome.ToolCalls[0], Output: ai.ToolOutput{Content: []ai.ToolContent{ai.TextContent{Text: "ok"}}}},
	}))

	final, err := turn.Step(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, final.Text)
}

func TestTurn_Step_ToolCallsWithoutHandlersFail(t *testing.T) {
	t.Parallel()

	mock := newMockHTTPClient(t, func(call int, req ChatRequest) ChatResponse {
		return ChatResponse{Choices: []ChatChoice{{
			Message: ResponseMessage{
				Role:    "assistant",
				Content: textResponseContent(""),
				ToolCalls: []llmshared.ChatToolCall{{
					ID: "call_1", Type: "function",
					Function: llmshared.ChatToolCallFunction{Name: "get_weather", Arguments: json.RawMessage(`{}`)},
				}},
			},
		}}}
	})
	core := newTestCore(mock)
	turn := core.NewTurn(testChatRequest(), nil, TurnOptions{})

	_, err := turn.Step(context.Background(), nil)
	require.ErrorIs(t, err, ErrToolCallNoHandler)
	assert.False(t, ai.IsRetryable(err))
}

// A text-only provider (no blob hooks) reports attachments as text; a
// provider with a BlobMessage hook appends provider-native messages.
func TestTurn_AddToolResults_BlobHandling(t *testing.T) {
	t.Parallel()

	blobOutput := ai.ToolOutput{Content: []ai.ToolContent{
		ai.TextContent{Text: "took a screenshot"},
		ai.BlobContent{Name: "shot.png", MIMEType: "image/png", Data: []byte{1, 2, 3}},
	}}
	call := llmshared.ToolCall{ID: "call_1", Name: "screenshot", Args: map[string]any{}}

	// Text-only: blob is described, no extra message appended.
	textOnly := newTestCore(newMockHTTPClient(t)).NewTurn(testChatRequest(), nil, TurnOptions{})
	baseline := len(textOnly.messages)
	require.NoError(t, textOnly.AddToolResults(context.Background(), []llmshared.PreparedToolResult{{Call: call, Output: blobOutput}}))
	require.Len(t, textOnly.messages, baseline+1)
	toolMessage := textOnly.messages[baseline]
	assert.Equal(t, "tool", toolMessage.Role)

	// With hooks: an image message is appended after the tool result.
	withBlobs := newTestCore(newMockHTTPClient(t)).NewTurn(testChatRequest(), nil, TurnOptions{
		SupportsBlob: llmshared.SupportsImagesOnly,
		BlobMessage: func(blob ai.BlobContent) ChatMessage {
			return ChatMessage{Role: "user", Content: NewMessageContent([]ContentPart{
				{Type: "image_url", ImageURL: &ImageURL{URL: llmshared.BlobDataURL(blob)}},
			})}
		},
	})
	baseline = len(withBlobs.messages)
	require.NoError(t, withBlobs.AddToolResults(context.Background(), []llmshared.PreparedToolResult{{Call: call, Output: blobOutput}}))
	require.Len(t, withBlobs.messages, baseline+2)
	assert.Equal(t, "tool", withBlobs.messages[baseline].Role)
	assert.Equal(t, "user", withBlobs.messages[baseline+1].Role)
}
