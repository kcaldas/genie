package shared

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type headerRecordingHTTPClient struct {
	lastRequest *http.Request
}

func (h *headerRecordingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	h.lastRequest = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Header:     make(http.Header),
	}, nil
}

// A configured auth token must travel as a Bearer Authorization header on
// every request (cloud OpenAI-compatible providers such as DeepSeek).
func TestPostJSONSendsBearerAuthToken(t *testing.T) {
	recorder := &headerRecordingHTTPClient{}
	core := NewLocalClientCore("deepseek", &events.NoOpEventBus{})
	core.HTTPClient = recorder
	core.AuthToken = "sk-test-123"

	resp, err := core.PostJSON(context.Background(), "http://test.local/chat/completions", []byte(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.NotNil(t, recorder.lastRequest)
	assert.Equal(t, "Bearer sk-test-123", recorder.lastRequest.Header.Get("Authorization"))
}

// Local servers keep working unauthenticated: no token, no header.
func TestPostJSONWithoutAuthTokenOmitsAuthorization(t *testing.T) {
	recorder := &headerRecordingHTTPClient{}
	core := NewLocalClientCore("lmstudio", &events.NoOpEventBus{})
	core.HTTPClient = recorder

	resp, err := core.PostJSON(context.Background(), "http://test.local/chat/completions", []byte(`{}`))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.NotNil(t, recorder.lastRequest)
	assert.Empty(t, recorder.lastRequest.Header.Get("Authorization"))
}

// Token events published by the local providers (Ollama, LM Studio) must
// carry the chat request ID from the context so hosts can attribute usage
// per invocation.
func TestPublishTokenCountCarriesRequestIDFromContext(t *testing.T) {
	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})

	core := NewLocalClientCore("ollama", bus)
	ctx := ai.ContextWithRequestID(context.Background(), "req-9")
	core.PublishTokenCount(ctx, &ai.TokenCount{InputTokens: 10, OutputTokens: 3, TotalTokens: 13})

	select {
	case event := <-received:
		assert.Equal(t, "req-9", event.RequestID)
		assert.Equal(t, "ollama", event.Provider)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}
}

// A token count requested outside any chat invocation carries no request ID.
func TestPublishTokenCountWithoutRequestID(t *testing.T) {
	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})

	core := NewLocalClientCore("lmstudio", bus)
	core.PublishTokenCount(context.Background(), &ai.TokenCount{TotalTokens: 5})

	select {
	case event := <-received:
		require.Empty(t, event.RequestID)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}
}
