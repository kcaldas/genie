package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/logging"
)

func newTestCore(mock *mockHTTPClient) *Core {
	return newTestCoreWithBus(mock, &events.NoOpEventBus{})
}

func newTestCoreWithBus(mock *mockHTTPClient, bus events.EventBus) *Core {
	core := NewCore("testprovider", bus)
	core.BaseURL = "http://test.local"
	core.HTTPClient = mock
	core.Logger = logging.NewDisabledLogger()
	return &core
}

func TestSendChat_PostsAndDecodes(t *testing.T) {
	t.Parallel()

	mock := newMockHTTPClient(t, func(call int, req ChatRequest) ChatResponse {
		assert.False(t, req.Stream)
		assert.Nil(t, req.StreamOptions)
		assert.Equal(t, "some-model", req.Model)
		return ChatResponse{
			Choices: []ChatChoice{{
				Message: ResponseMessage{Role: "assistant", Content: textResponseContent("hi")},
			}},
		}
	})
	core := newTestCore(mock)

	resp, err := core.SendChat(context.Background(), ChatRequest{Model: "some-model", Stream: true})
	require.NoError(t, err)
	require.Len(t, resp.Choices, 1)
	assert.Equal(t, "hi", resp.Choices[0].Message.Content.Text())
	assert.Equal(t, "http://test.local/chat/completions", mock.urls[0])
}

func TestSendChat_SurfacesAPIError(t *testing.T) {
	t.Parallel()

	mock := newMockHTTPClient(t, func(call int, req ChatRequest) ChatResponse {
		return ChatResponse{Error: &APIError{Message: "model exploded"}}
	})
	core := newTestCore(mock)

	_, err := core.SendChat(context.Background(), ChatRequest{Model: "m"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model exploded")
	assert.Contains(t, err.Error(), "testprovider")
}

func TestSendChat_SurfacesHTTPStatusError(t *testing.T) {
	t.Parallel()

	mock := newRawMockHTTPClient(t, func(req *http.Request) *http.Response {
		return rawResponse(http.StatusUnauthorized, `{"error":{"message":"bad key"}}`)
	})
	core := newTestCore(mock)

	_, err := core.SendChat(context.Background(), ChatRequest{Model: "m"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad key")
}

func TestSendChatStream_ParsesSSEAndSetsStreamFlags(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"one"}}]}`,
		``,
		`event: ping`,
		`not json`,
		`data: {"choices":[{"delta":{"content":"two"},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
	}, "\n")

	var captured ChatRequest
	mock := newRawMockHTTPClient(t, func(req *http.Request) *http.Response {
		payload, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(payload, &captured))
		return rawResponse(http.StatusOK, body)
	})
	core := newTestCore(mock)
	core.StreamIncludeUsage = true

	var texts []string
	err := core.SendChatStream(context.Background(), ChatRequest{Model: "m"}, func(chunk *ChatStreamResponse) error {
		for _, choice := range chunk.Choices {
			if text := choice.Delta.Text(); text != "" {
				texts = append(texts, text)
			}
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"one", "two"}, texts)

	assert.True(t, captured.Stream)
	require.NotNil(t, captured.StreamOptions)
	assert.True(t, captured.StreamOptions.IncludeUsage)
}

// Providers that opt out (local servers with uneven support) must not
// send stream_options at all.
func TestSendChatStream_OmitsStreamOptionsWhenDisabled(t *testing.T) {
	t.Parallel()

	mock := newRawMockHTTPClient(t, func(req *http.Request) *http.Response {
		payload, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		assert.NotContains(t, string(payload), "stream_options")
		return rawResponse(http.StatusOK, "data: [DONE]\n")
	})
	core := newTestCore(mock)

	err := core.SendChatStream(context.Background(), ChatRequest{Model: "m"}, func(*ChatStreamResponse) error { return nil })
	require.NoError(t, err)
}

func TestPublishUsage_CacheAwareTokenEvent(t *testing.T) {
	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})

	core := newTestCoreWithBus(newMockHTTPClient(t), bus)

	ctx := ai.ContextWithRequestID(context.Background(), "req-9")
	tokenCount := core.PublishUsage(ctx, "some-model", &Usage{
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
		assert.Equal(t, "testprovider", event.Provider)
		assert.Equal(t, "some-model", event.Model)
		assert.Equal(t, int32(40), event.InputTokens)
		assert.Equal(t, int32(20), event.OutputTokens)
		assert.Equal(t, int32(60), event.CachedTokens)
		assert.Equal(t, int32(60), event.CacheReadInputTokens)
		assert.Equal(t, int32(120), event.TotalTokens)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}
}

// Servers that report no cache split (LM Studio, Ollama-compat) publish
// the whole prompt as uncached input.
func TestPublishUsage_WithoutCacheFields(t *testing.T) {
	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})

	core := newTestCoreWithBus(newMockHTTPClient(t), bus)
	core.PublishUsage(context.Background(), "some-model", &Usage{PromptTokens: 10, CompletionTokens: 3, TotalTokens: 13})

	select {
	case event := <-received:
		assert.Equal(t, int32(10), event.InputTokens)
		assert.Equal(t, int32(0), event.CachedTokens)
		assert.Equal(t, int32(13), event.TotalTokens)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}

	assert.Nil(t, core.PublishUsage(context.Background(), "some-model", nil))
}

// --- shared test helpers ---

func textResponseContent(text string) ResponseContent {
	return ResponseContent{Parts: []ContentPart{{Type: "text", Text: text}}}
}

func rawResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

type mockHTTPClient struct {
	t            *testing.T
	mu           sync.Mutex
	handlers     []func(call int, req ChatRequest) ChatResponse
	rawResponder func(req *http.Request) *http.Response
	requests     []ChatRequest
	rawBodies    [][]byte
	headers      []http.Header
	urls         []string
	callCount    int
}

func newMockHTTPClient(t *testing.T, handlers ...func(call int, req ChatRequest) ChatResponse) *mockHTTPClient {
	return &mockHTTPClient{t: t, handlers: handlers}
}

func newRawMockHTTPClient(t *testing.T, responder func(req *http.Request) *http.Response) *mockHTTPClient {
	return &mockHTTPClient{t: t, rawResponder: responder}
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	require.Equal(m.t, http.MethodPost, req.Method)
	m.headers = append(m.headers, req.Header.Clone())
	m.urls = append(m.urls, req.URL.String())

	if m.rawResponder != nil {
		m.callCount++
		return m.rawResponder(req), nil
	}

	body, err := io.ReadAll(req.Body)
	require.NoError(m.t, err)
	_ = req.Body.Close()

	var parsed ChatRequest
	require.NoError(m.t, json.Unmarshal(body, &parsed))
	m.requests = append(m.requests, parsed)
	m.rawBodies = append(m.rawBodies, body)

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
