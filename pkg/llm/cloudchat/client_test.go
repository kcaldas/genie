package cloudchat

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

func TestMaritaca_DefaultEndpoint(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, textAnswer("Olá!"))
	client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "owner-key"})

	resp, err := client.GenerateContent(context.Background(), ai.Prompt{
		Instruction: "Você é um assistente.",
		Text:        "Diga olá.",
		ModelName:   "sabia-4",
	}, false)
	require.NoError(t, err)
	assert.Equal(t, "Olá!", resp)

	require.Len(t, mockHTTP.urls, 1)
	assert.Equal(t, "https://chat.maritaca.ai/api/chat/completions", mockHTTP.urls[0])
	assert.Equal(t, "Bearer owner-key", mockHTTP.headers[0].Get("Authorization"))
	assert.Equal(t, "sabia-4", mockHTTP.requests[0].Model)
	require.Len(t, mockHTTP.requests[0].Messages, 2)
	assert.Equal(t, "system", mockHTTP.requests[0].Messages[0].Role)
}

// A base URL override (the Mutiro LLM proxy) replaces the default; the key
// is whatever the environment carries, the agent key behind a proxy.
func TestMaritaca_BaseURLOverride(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, textAnswer("ok"))
	client := newTestClient(t, mockHTTP, map[string]string{
		"MARITACA_API_KEY":  "agent-key",
		"MARITACA_BASE_URL": "https://llm.example.com/mutiro/maritaca/",
	})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "sabiazinho-4"}, false)
	require.NoError(t, err)
	assert.Equal(t, "https://llm.example.com/mutiro/maritaca/chat/completions", mockHTTP.urls[0])
	assert.Equal(t, "Bearer agent-key", mockHTTP.headers[0].Get("Authorization"))
}

func TestMaritaca_MissingKey(t *testing.T) {
	client := newTestClient(t, newMockHTTPClient(t), map[string]string{})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "sabia-4"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MARITACA_API_KEY")

	status := client.GetStatus()
	assert.False(t, status.Connected)
	assert.Equal(t, "maritaca", status.Backend)
}

// Without vision, an image becomes a note instead of a part the endpoint
// would reject.
func TestMaritaca_ImagesBecomeNotes(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, textAnswer("ok"))
	client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{
		Text:      "O que é isto?",
		ModelName: "sabia-4",
		Images:    []*ai.Image{{Type: "image/png", Data: []byte{1, 2, 3}}},
	}, false)
	require.NoError(t, err)

	user := mockHTTP.requests[0].Messages[len(mockHTTP.requests[0].Messages)-1]
	require.Len(t, user.Content.Parts, 1)
	assert.Contains(t, user.Content.Parts[0].Text, "accepts text input only")
}

// Usage events carry the provider name and the cache split.
func TestMaritaca_PublishesCachedUsage(t *testing.T) {
	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})
	mockHTTP := newMockHTTPClient(t, func(int, chatRequest) chatResponse {
		answer := textAnswer("ok")(0, chatRequest{})
		answer.Usage = &usage{PromptTokens: 4000, CompletionTokens: 5, TotalTokens: 4005, PromptTokensDetails: &promptTokensDetail{CachedTokens: 3968}}
		return answer
	})
	client := newTestClientWithBus(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"}, bus)

	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi", ModelName: "sabia-4"}, false)
	require.NoError(t, err)

	select {
	case event := <-received:
		assert.Equal(t, "maritaca", event.Provider)
		assert.Equal(t, "sabia-4", event.Model)
		assert.Equal(t, int32(32), event.InputTokens)
		assert.Equal(t, int32(3968), event.CachedTokens)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}
}

// A prompt naming no model uses the configured one, else the provider default.
func TestMaritaca_DefaultModel(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, textAnswer("ok"), textAnswer("ok"))
	client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"})
	_, err := client.GenerateContent(context.Background(), ai.Prompt{Text: "hi"}, false)
	require.NoError(t, err)
	assert.Equal(t, "sabia-4", mockHTTP.requests[0].Model)

	configured := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k", "GENIE_MODEL_NAME": "sabiazinho-4"})
	_, err = configured.GenerateContent(context.Background(), ai.Prompt{Text: "hi"}, false)
	require.NoError(t, err)
	assert.Equal(t, "sabiazinho-4", mockHTTP.requests[1].Model)
}

// max_tokens above the model's output limit is clamped: Maritaca rejects
// the request (422) instead of capping it, and Genie's default is higher
// than the Sabiá limit.
func TestMaritaca_MaxTokensClampedToModelLimit(t *testing.T) {
	mockHTTP := newMockHTTPClient(t, textAnswer("ok"))
	client := newTestClient(t, mockHTTP, map[string]string{"MARITACA_API_KEY": "k"})

	_, err := client.GenerateContent(context.Background(), ai.Prompt{
		Text:              "hi",
		ModelName:         "sabia-4",
		MaxTokens:         65535,
		ModelCapabilities: &ai.ModelCapabilities{OutputTokenLimit: 32000},
	}, false)
	require.NoError(t, err)
	require.NotNil(t, mockHTTP.requests[0].MaxTokens)
	assert.Equal(t, int32(32000), *mockHTTP.requests[0].MaxTokens)
}

// --- helpers ---

func textAnswer(text string) func(int, chatRequest) chatResponse {
	return func(int, chatRequest) chatResponse {
		return chatResponse{
			Choices: []chatChoice{{
				Message: responseMessage{
					Role:    "assistant",
					Content: responseContent{Parts: []contentPart{{Type: "text", Text: text}}},
				},
				FinishReason: "stop",
			}},
		}
	}
}

func newTestClient(t *testing.T, mockHTTP *mockHTTPClient, values map[string]string) *Client {
	t.Helper()
	return newTestClientWithBus(t, mockHTTP, values, &events.NoOpEventBus{})
}

func newTestClientWithBus(t *testing.T, mockHTTP *mockHTTPClient, values map[string]string, bus events.EventBus) *Client {
	t.Helper()
	rawClient, err := NewClient(
		Maritaca,
		bus,
		WithConfigManager(&stubConfig{values: values}),
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

func (s *stubConfig) GetString(key string) (string, error) { return s.values[key], nil }

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

func (s *stubConfig) GetModelConfig() config.ModelConfig {
	return config.ModelConfig{ModelName: s.values["GENIE_MODEL_NAME"]}
}

type mockHTTPClient struct {
	t         *testing.T
	mu        sync.Mutex
	handlers  []func(call int, req chatRequest) chatResponse
	requests  []chatRequest
	headers   []http.Header
	urls      []string
	callCount int
}

func newMockHTTPClient(t *testing.T, handlers ...func(call int, req chatRequest) chatResponse) *mockHTTPClient {
	return &mockHTTPClient{t: t, handlers: handlers}
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	body, err := io.ReadAll(req.Body)
	require.NoError(m.t, err)
	_ = req.Body.Close()

	var parsed chatRequest
	require.NoError(m.t, json.Unmarshal(body, &parsed))
	m.requests = append(m.requests, parsed)
	m.headers = append(m.headers, req.Header.Clone())
	m.urls = append(m.urls, req.URL.String())

	if m.callCount >= len(m.handlers) {
		require.FailNow(m.t, "mock HTTP client received more calls than handlers configured")
	}
	response := m.handlers[m.callCount](m.callCount, parsed)
	m.callCount++

	payload, err := json.Marshal(response)
	require.NoError(m.t, err)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header)}, nil
}
