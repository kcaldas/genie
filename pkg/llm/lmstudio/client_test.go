package lmstudio

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/logging"
)

func TestClient_GenerateContent_SimpleResponse(t *testing.T) {
	t.Parallel()

	mockHTTP := newMockHTTPClient(t, func(call int, req chatRequest) chatResponse {
		require.Equal(t, 0, call)
		assert.False(t, req.Stream)
		assert.Equal(t, "local-model", req.Model)
		require.Len(t, req.Messages, 2)
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "You are a helpful assistant.", req.Messages[0].Content.Parts[0].Text)
		assert.Equal(t, "user", req.Messages[1].Role)
		assert.Equal(t, "Say hello.", req.Messages[1].Content.Parts[0].Text)

		return chatResponse{
			Model: "local-model",
			Choices: []chatChoice{{
				Index: 0,
				Message: responseMessage{
					Role: "assistant",
					Content: responseContent{
						parts: []contentPart{{Type: "text", Text: "Hello there!"}},
					},
				},
				FinishReason: "stop",
			}},
			Usage: &usage{
				PromptTokens:     8,
				CompletionTokens: 2,
				TotalTokens:      10,
			},
		}
	})

	rawClient, err := NewClient(
		&events.NoOpEventBus{},
		WithBaseURL("http://test.local"),
		WithHTTPClient(mockHTTP),
		WithLogger(logging.NewDisabledLogger()),
	)
	require.NoError(t, err)
	client := rawClient.(*Client)

	prompt := ai.Prompt{
		Name:        "greeting",
		Instruction: "You are a helpful assistant.",
		Text:        "Say hello.",
		ModelName:   "local-model",
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "Hello there!", resp)

	require.Len(t, mockHTTP.requests, 1)
}

func TestClient_GenerateContent_WithToolCall(t *testing.T) {
	t.Parallel()

	mockHTTP := newMockHTTPClient(t,
		func(call int, req chatRequest) chatResponse {
			require.Equal(t, 0, call)
			return chatResponse{
				Model: "local-model",
				Choices: []chatChoice{{
					Index: 0,
					Message: responseMessage{
						Role: "assistant",
						Content: responseContent{
							parts: []contentPart{{Type: "text", Text: ""}},
						},
						ToolCalls: []toolCall{{
							ID:   "call_1",
							Type: "function",
							Function: toolCallFunction{
								Name:      "get_weather",
								Arguments: json.RawMessage(`{"location":"Lisbon"}`),
							},
						}},
					},
					FinishReason: "tool_calls",
				}},
			}
		},
		func(call int, req chatRequest) chatResponse {
			require.Equal(t, 1, call)
			require.Len(t, req.Messages, 3)
			toolMsg := req.Messages[2]
			assert.Equal(t, "tool", toolMsg.Role)
			assert.Equal(t, "call_1", toolMsg.ToolCallID)
			assert.JSONEq(t, `{"temperature":22}`, toolMsg.Content.Parts[0].Text)

			return chatResponse{
				Model: "local-model",
				Choices: []chatChoice{{
					Index: 0,
					Message: responseMessage{
						Role: "assistant",
						Content: responseContent{
							parts: []contentPart{{Type: "text", Text: "It is sunny and 22°C."}},
						},
					},
					FinishReason: "stop",
				}},
				Usage: &usage{PromptTokens: 20, CompletionTokens: 6, TotalTokens: 26},
			}
		},
	)

	rawClient, err := NewClient(
		&events.NoOpEventBus{},
		WithBaseURL("http://test.local"),
		WithHTTPClient(mockHTTP),
		WithLogger(logging.NewDisabledLogger()),
	)
	require.NoError(t, err)
	client := rawClient.(*Client)

	handlerInvoked := false
	prompt := ai.Prompt{
		Name:      "weather",
		Text:      "What's the weather?",
		ModelName: "local-model",
		Functions: []*ai.FunctionDeclaration{
			{
				Name: "get_weather",
				Parameters: &ai.Schema{
					Type: ai.TypeObject,
				},
			},
		},
		Handlers: map[string]ai.HandlerFunc{
			"get_weather": func(ctx context.Context, attr map[string]any) (map[string]any, error) {
				handlerInvoked = true
				assert.Equal(t, map[string]any{"location": "Lisbon"}, attr)
				return map[string]any{"temperature": 22}, nil
			},
		},
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "It is sunny and 22°C.", resp)
	assert.True(t, handlerInvoked)
	assert.Equal(t, 2, mockHTTP.callCount)
}

func TestClient_CountTokens_UsesUsage(t *testing.T) {
	t.Parallel()

	mockHTTP := newMockHTTPClient(t, func(call int, req chatRequest) chatResponse {
		require.Equal(t, 0, call)
		if assert.NotNil(t, req.MaxTokens) {
			assert.Equal(t, int32(0), *req.MaxTokens)
		}

		return chatResponse{
			Model: "local-model",
			Usage: &usage{
				PromptTokens:     30,
				CompletionTokens: 0,
				TotalTokens:      30,
			},
		}
	})

	rawClient, err := NewClient(
		&events.NoOpEventBus{},
		WithBaseURL("http://test.local"),
		WithHTTPClient(mockHTTP),
		WithLogger(logging.NewDisabledLogger()),
	)
	require.NoError(t, err)
	client := rawClient.(*Client)

	prompt := ai.Prompt{
		Name:      "count",
		Text:      "Count tokens",
		ModelName: "local-model",
	}

	count, err := client.CountTokens(context.Background(), prompt, false)
	require.NoError(t, err)
	require.NotNil(t, count)
	assert.Equal(t, int32(30), count.TotalTokens)
	assert.Equal(t, int32(30), count.InputTokens)
	assert.Equal(t, int32(0), count.OutputTokens)
}

type mockHTTPClient struct {
	t         *testing.T
	mu        sync.Mutex
	handlers  []func(call int, req chatRequest) chatResponse
	requests  []chatRequest
	callCount int
}

func newMockHTTPClient(t *testing.T, handlers ...func(call int, req chatRequest) chatResponse) *mockHTTPClient {
	return &mockHTTPClient{
		t:        t,
		handlers: handlers,
	}
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	require.Equal(m.t, http.MethodPost, req.Method)

	body, err := io.ReadAll(req.Body)
	require.NoError(m.t, err)
	_ = req.Body.Close()

	var parsed chatRequest
	require.NoError(m.t, json.Unmarshal(body, &parsed))
	m.requests = append(m.requests, parsed)

	if m.callCount >= len(m.handlers) {
		require.FailNow(m.t, "mock HTTP client received more calls than handlers configured")
	}

	handler := m.handlers[m.callCount]
	response := handler(m.callCount, parsed)
	m.callCount++

	payload, err := json.Marshal(response)
	require.NoError(m.t, err)

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(payload)),
		Header:     make(http.Header),
	}, nil
}
