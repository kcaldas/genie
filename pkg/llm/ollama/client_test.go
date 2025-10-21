package ollama

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
		return chatResponse{
			Model: "llama3",
			Message: responseMessage{
				Role: "assistant",
				Content: responseContent{
					parts: []messagePart{{Type: "text", Text: "Hello there!"}},
				},
			},
			PromptEvalCount: 12,
			EvalCount:       4,
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
		ModelName:   "llama3",
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "Hello there!", resp)

	require.Len(t, mockHTTP.requests, 1)
	request := mockHTTP.requests[0]
	assert.Equal(t, "llama3", request.Model)
	require.Len(t, request.Messages, 2)
	assert.Equal(t, "system", request.Messages[0].Role)
	assert.Equal(t, "You are a helpful assistant.", request.Messages[0].Content.Parts[0].Text)
	assert.Equal(t, "user", request.Messages[1].Role)
	assert.Equal(t, "Say hello.", request.Messages[1].Content.Parts[0].Text)
	assert.False(t, request.Stream)
}

func TestClient_GenerateContent_WithToolCall(t *testing.T) {
	t.Parallel()

	mockHTTP := newMockHTTPClient(t,
		func(call int, req chatRequest) chatResponse {
			require.Equal(t, 0, call)
			require.Len(t, req.Messages, 1)
			return chatResponse{
				Model: "llama3",
				Message: responseMessage{
					Role: "assistant",
					Content: responseContent{
						parts: []messagePart{{Type: "text", Text: ""}},
					},
					ToolCalls: []toolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: toolCallFunction{
								Name:      "get_weather",
								Arguments: json.RawMessage(`{"location":"Lisbon"}`),
							},
						},
					},
				},
				PromptEvalCount: 20,
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
				Model: "llama3",
				Message: responseMessage{
					Role: "assistant",
					Content: responseContent{
						parts: []messagePart{{Type: "text", Text: "It is sunny and 22°C."}},
					},
				},
				PromptEvalCount: 24,
				EvalCount:       10,
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
		ModelName: "llama3",
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

func TestClient_CountTokens(t *testing.T) {
	t.Parallel()

	mockHTTP := newMockHTTPClient(t, func(call int, req chatRequest) chatResponse {
		require.Equal(t, 0, call)
		if assert.NotNil(t, req.Options) {
			value, ok := req.Options["num_predict"]
			require.True(t, ok)
			switch num := value.(type) {
			case float64:
				assert.Equal(t, float64(tokenCountPredict), num)
			case int:
				assert.Equal(t, tokenCountPredict, num)
			default:
				t.Fatalf("unexpected type for num_predict: %T", value)
			}
		}

		return chatResponse{
			Model: "llama3",
			Message: responseMessage{
				Role: "assistant",
				Content: responseContent{
					parts: []messagePart{{Type: "text", Text: ""}},
				},
			},
			PromptEvalCount: 42,
			EvalCount:       0,
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
		Text:      "Count these tokens",
		ModelName: "llama3",
	}

	count, err := client.CountTokens(context.Background(), prompt, false)
	require.NoError(t, err)
	require.NotNil(t, count)
	assert.Equal(t, int32(42), count.TotalTokens)
	assert.Equal(t, int32(42), count.InputTokens)
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
