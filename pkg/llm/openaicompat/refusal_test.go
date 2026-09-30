package openaicompat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

func refusingServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Should-Retry", "false")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"plan limit reached (quota daily_spend)"}}`))
	}))
	t.Cleanup(s.Close)
	return s
}

func TestRefusalIsFinal(t *testing.T) {
	core := newRefusalCore(t, refusingServer(t).URL)
	if _, err := core.SendChat(context.Background(), ChatRequest{Model: "m"}); err == nil || ai.IsRetryable(err) {
		t.Fatalf("chat: %v", err)
	}
	if err := core.SendChatStream(context.Background(), ChatRequest{Model: "m"}, func(*ChatStreamResponse) error { return nil }); err == nil || ai.IsRetryable(err) {
		t.Fatalf("stream: %v", err)
	}
}

func newRefusalCore(t *testing.T, base string) *Core {
	t.Helper()
	core := NewCore("deepseek", &events.NoOpEventBus{})
	core.BaseURL = base
	core.HTTPClient = http.DefaultClient
	return &core
}
