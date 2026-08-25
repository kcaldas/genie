package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/logging"
)

func TestClient_GenerateContent_SimpleResponse(t *testing.T) {
	t.Parallel()

	mockHTTP := newMockHTTPClient(t, func(call int, req chatRequest) chatResponse {
		require.Equal(t, 0, call)
		assert.False(t, req.Stream)
		assert.Equal(t, "deepseek-chat", req.Model)
		require.Len(t, req.Messages, 2)
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "You are a helpful assistant.", req.Messages[0].Content.Parts[0].Text)
		assert.Equal(t, "user", req.Messages[1].Role)
		assert.Equal(t, "Say hello.", req.Messages[1].Content.Parts[0].Text)

		return chatResponse{
			Model: "deepseek-chat",
			Choices: []chatChoice{{
				Index: 0,
				Message: responseMessage{
					Role: "assistant",
					Content: responseContent{
						Parts: []contentPart{{Type: "text", Text: "Hello there!"}},
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

	client := newTestClient(t, mockHTTP)

	prompt := ai.Prompt{
		Name:        "greeting",
		Instruction: "You are a helpful assistant.",
		Text:        "Say hello.",
		ModelName:   "deepseek-chat",
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "Hello there!", resp)

	require.Len(t, mockHTTP.requests, 1)
	require.Len(t, mockHTTP.headers, 1)
	assert.Equal(t, "Bearer test-key", mockHTTP.headers[0].Get("Authorization"))
	assert.Equal(t, "https://api.deepseek.com/chat/completions", mockHTTP.urls[0])
}

func TestClient_GenerateContent_MissingAPIKey(t *testing.T) {
	rawClient, err := NewClient(
		&events.NoOpEventBus{},
		WithLogger(logging.NewDisabledLogger()),
	)
	require.NoError(t, err)
	client := rawClient.(*Client)
	client.Config = &stubConfig{values: map[string]string{}}

	_, err = client.GenerateContent(context.Background(), ai.Prompt{Text: "hi"}, false)
	require.ErrorIs(t, err, errMissingAPIKey)
	assert.Contains(t, err.Error(), "DEEPSEEK_API_KEY")
}

func TestClient_GenerateContent_WithToolCall(t *testing.T) {
	t.Parallel()

	mockHTTP := newMockHTTPClient(t,
		func(call int, req chatRequest) chatResponse {
			require.Equal(t, 0, call)
			return chatResponse{
				Model: "deepseek-chat",
				Choices: []chatChoice{{
					Index: 0,
					Message: responseMessage{
						Role: "assistant",
						Content: responseContent{
							Parts: []contentPart{{Type: "text", Text: ""}},
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
				Model: "deepseek-chat",
				Choices: []chatChoice{{
					Index: 0,
					Message: responseMessage{
						Role: "assistant",
						Content: responseContent{
							Parts: []contentPart{{Type: "text", Text: "It is sunny and 22°C."}},
						},
					},
					FinishReason: "stop",
				}},
				Usage: &usage{PromptTokens: 20, CompletionTokens: 6, TotalTokens: 26},
			}
		},
	)

	client := newTestClient(t, mockHTTP)

	handlerInvoked := false
	prompt := ai.Prompt{
		Name:      "weather",
		Text:      "What's the weather?",
		ModelName: "deepseek-chat",
		Functions: []*ai.FunctionDeclaration{
			{
				Name: "get_weather",
				Parameters: &ai.Schema{
					Type: ai.TypeObject,
				},
			},
		},
		Handlers: map[string]ai.HandlerFunc{
			"get_weather": func(ctx context.Context, attr map[string]any) (ai.ToolOutput, error) {
				handlerInvoked = true
				assert.Equal(t, map[string]any{"location": "Lisbon"}, attr)
				return ai.JSONToolOutput(map[string]any{"temperature": 22}), nil
			},
		},
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "It is sunny and 22°C.", resp)
	assert.True(t, handlerInvoked)
	assert.Equal(t, 2, mockHTTP.callCount)
}

func TestClient_GenerateContent_ReasoningPublishedNotEchoed(t *testing.T) {
	t.Parallel()

	mockHTTP := newMockHTTPClient(t,
		func(call int, req chatRequest) chatResponse {
			return chatResponse{
				Model: "deepseek-reasoner",
				Choices: []chatChoice{{
					Message: responseMessage{
						Role:             "assistant",
						Content:          responseContent{Parts: []contentPart{{Type: "text", Text: ""}}},
						ReasoningContent: json.RawMessage(`"thinking about the weather"`),
						ToolCalls: []toolCall{{
							ID:   "call_1",
							Type: "function",
							Function: toolCallFunction{
								Name:      "get_weather",
								Arguments: json.RawMessage(`{}`),
							},
						}},
					},
					FinishReason: "tool_calls",
				}},
			}
		},
		func(call int, req chatRequest) chatResponse {
			return chatResponse{
				Model: "deepseek-reasoner",
				Choices: []chatChoice{{
					Message: responseMessage{
						Role:    "assistant",
						Content: responseContent{Parts: []contentPart{{Type: "text", Text: "Done."}}},
					},
					FinishReason: "stop",
				}},
			}
		},
	)

	bus := events.NewEventBus()
	thinking := make(chan events.ThinkingEvent, 2)
	bus.Subscribe(events.ThinkingEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.ThinkingEvent); ok {
			thinking <- event
		}
	})

	client := newTestClientWithBus(t, mockHTTP, bus)

	prompt := ai.Prompt{
		Name:      "weather",
		Text:      "What's the weather?",
		ModelName: "deepseek-reasoner",
		Functions: []*ai.FunctionDeclaration{{Name: "get_weather", Parameters: &ai.Schema{Type: ai.TypeObject}}},
		Handlers: map[string]ai.HandlerFunc{
			"get_weather": func(ctx context.Context, attr map[string]any) (ai.ToolOutput, error) {
				return ai.JSONToolOutput(map[string]any{"ok": true}), nil
			},
		},
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.Equal(t, "Done.", resp)

	select {
	case event := <-thinking:
		assert.Equal(t, "thinking about the weather", event.Text)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for thinking event")
	}

	// The second request must echo the assistant tool-call message
	// without any reasoning_content key.
	require.Len(t, mockHTTP.rawBodies, 2)
	assert.NotContains(t, string(mockHTTP.rawBodies[1]), "reasoning_content")
}

func TestClient_GenerateContent_ResponseSchemaUsesJSONObjectMode(t *testing.T) {
	t.Parallel()

	mockHTTP := newMockHTTPClient(t, func(call int, req chatRequest) chatResponse {
		require.NotNil(t, req.ResponseFormat)
		assert.Equal(t, "json_object", req.ResponseFormat.Type)
		// The schema travels in the system prompt, even when an
		// instruction is present.
		require.NotEmpty(t, req.Messages)
		assert.Equal(t, "system", req.Messages[0].Role)
		systemText := req.Messages[0].Content.Parts[0].Text
		assert.Contains(t, systemText, "Existing instruction.")
		assert.Contains(t, systemText, "JSON matching this schema")
		assert.Contains(t, systemText, "answer")

		return chatResponse{
			Model: "deepseek-chat",
			Choices: []chatChoice{{
				Message: responseMessage{
					Role:    "assistant",
					Content: responseContent{Parts: []contentPart{{Type: "text", Text: `{"answer":"42"}`}}},
				},
				FinishReason: "stop",
			}},
		}
	})

	client := newTestClient(t, mockHTTP)

	prompt := ai.Prompt{
		Name:        "structured",
		Instruction: "Existing instruction.",
		Text:        "Answer the question.",
		ModelName:   "deepseek-chat",
		ResponseSchema: &ai.Schema{
			Type: ai.TypeObject,
			Properties: map[string]*ai.Schema{
				"answer": {Type: ai.TypeString},
			},
		},
	}

	resp, err := client.GenerateContent(context.Background(), prompt, false)
	require.NoError(t, err)
	assert.JSONEq(t, `{"answer":"42"}`, resp)
}

func TestClient_PublishUsage_CacheAwareTokenEvent(t *testing.T) {
	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})

	rawClient, err := NewClient(bus, WithAPIKey("test-key"), WithLogger(logging.NewDisabledLogger()))
	require.NoError(t, err)
	client := rawClient.(*Client)

	ctx := ai.ContextWithRequestID(context.Background(), "req-9")
	tokenCount := client.PublishUsage(ctx, "deepseek-chat", &usage{
		PromptTokens:          100,
		CompletionTokens:      20,
		TotalTokens:           120,
		PromptCacheHitTokens:  60,
		PromptCacheMissTokens: 40,
	})

	require.NotNil(t, tokenCount)
	assert.Equal(t, int32(120), tokenCount.TotalTokens)

	select {
	case event := <-received:
		assert.Equal(t, "req-9", event.RequestID)
		assert.Equal(t, "deepseek", event.Provider)
		assert.Equal(t, "deepseek-chat", event.Model)
		assert.Equal(t, int32(40), event.InputTokens)
		assert.Equal(t, int32(20), event.OutputTokens)
		assert.Equal(t, int32(60), event.CachedTokens)
		assert.Equal(t, int32(60), event.CacheReadInputTokens)
		assert.Equal(t, int32(120), event.TotalTokens)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}
}

func TestClient_CountTokens_NoAPIRequest(t *testing.T) {
	t.Parallel()

	mockHTTP := newMockHTTPClient(t) // no handlers: any request fails the test

	client := newTestClient(t, mockHTTP)

	prompt := ai.Prompt{
		Name:        "count",
		Instruction: "You are a helpful assistant.",
		Text:        "Count tokens for this text.",
		ModelName:   "deepseek-chat",
	}

	count, err := client.CountTokens(context.Background(), prompt, false)
	require.NoError(t, err)
	require.NotNil(t, count)
	assert.Positive(t, count.TotalTokens)
	assert.Equal(t, count.TotalTokens, count.InputTokens)
	assert.Equal(t, 0, mockHTTP.callCount)
}

func TestClient_BaseURLOverride(t *testing.T) {
	t.Parallel()

	mockHTTP := newMockHTTPClient(t, func(call int, req chatRequest) chatResponse {
		return chatResponse{
			Choices: []chatChoice{{
				Message: responseMessage{
					Role:    "assistant",
					Content: responseContent{Parts: []contentPart{{Type: "text", Text: "ok"}}},
				},
			}},
		}
	})

	rawClient, err := NewClient(
		&events.NoOpEventBus{},
		WithBaseURL("https://proxy.example.com/v1/"),
		WithAPIKey("test-key"),
		WithHTTPClient(mockHTTP),
		WithLogger(logging.NewDisabledLogger()),
	)
	require.NoError(t, err)
	client := rawClient.(*Client)

	_, err = client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "deepseek-chat"}, false)
	require.NoError(t, err)
	require.Len(t, mockHTTP.urls, 1)
	assert.Equal(t, "https://proxy.example.com/v1/chat/completions", mockHTTP.urls[0])
}

// --- helpers ---

func newTestClient(t *testing.T, mockHTTP *mockHTTPClient) *Client {
	t.Helper()
	return newTestClientWithBus(t, mockHTTP, &events.NoOpEventBus{})
}

func newTestClientWithBus(t *testing.T, mockHTTP *mockHTTPClient, bus events.EventBus) *Client {
	t.Helper()
	rawClient, err := NewClient(
		bus,
		WithAPIKey("test-key"),
		WithHTTPClient(mockHTTP),
		WithLogger(logging.NewDisabledLogger()),
	)
	require.NoError(t, err)
	return rawClient.(*Client)
}

// stubConfig returns only the configured values, ignoring process env.
type stubConfig struct {
	values map[string]string
}

func (s *stubConfig) GetString(key string) (string, error) {
	return s.values[key], nil
}

func (s *stubConfig) GetStringWithDefault(key, defaultValue string) string {
	if v, ok := s.values[key]; ok && v != "" {
		return v
	}
	return defaultValue
}

func (s *stubConfig) RequireString(key string) string { return s.values[key] }

func (s *stubConfig) GetInt(key string) (int, error) { return 0, nil }

func (s *stubConfig) GetIntWithDefault(key string, defaultValue int) int { return defaultValue }

func (s *stubConfig) GetBoolWithDefault(key string, defaultValue bool) bool { return defaultValue }

func (s *stubConfig) GetDurationWithDefault(key string, defaultValue time.Duration) time.Duration {
	return defaultValue
}

func (s *stubConfig) GetModelConfig() config.ModelConfig { return config.ModelConfig{} }

type mockHTTPClient struct {
	t         *testing.T
	mu        sync.Mutex
	handlers  []func(call int, req chatRequest) chatResponse
	requests  []chatRequest
	rawBodies [][]byte
	headers   []http.Header
	urls      []string
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
	m.rawBodies = append(m.rawBodies, body)
	m.headers = append(m.headers, req.Header.Clone())
	m.urls = append(m.urls, req.URL.String())

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
