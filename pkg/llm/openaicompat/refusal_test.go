package openaicompat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// The labels reach their host, and a redirect to another host does not
// carry them.
func TestRequestsCarryTheContextHeadersToTheirHostOnly(t *testing.T) {
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Turn") != "" {
			t.Errorf("labels followed the redirect: %v", r.Header)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`))
	}))
	t.Cleanup(elsewhere.Close)
	var got string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Turn")
		http.Redirect(w, r, elsewhere.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(s.Close)
	labels := http.Header{}
	labels.Set("X-Turn", "turn-1")
	u, _ := url.Parse(s.URL)
	core := newRefusalCore(t, s.URL)
	core.HTTPClient = NewCore("deepseek", &events.NoOpEventBus{}).HTTPClient
	_, _ = core.SendChat(ai.ContextWithRequestHeaders(context.Background(), u.Host, labels), ChatRequest{Model: "m"})
	if got != "turn-1" {
		t.Fatalf("X-Turn = %q", got)
	}
}
