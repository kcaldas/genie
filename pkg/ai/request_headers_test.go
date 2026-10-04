package ai

import (
	"context"
	"net/http"
	"testing"
)

func TestRequestHeadersAreAddedWithoutReplacingTheClients(t *testing.T) {
	labels := http.Header{}
	labels.Set("X-Turn", "turn-1")
	labels.Set("Authorization", "Bearer caller-supplied")
	ctx := ContextWithRequestHeaders(context.Background(), labels)
	labels.Set("X-Turn", "mutated after attaching")

	h := http.Header{}
	h.Set("Authorization", "Bearer client-credential")
	ApplyRequestHeaders(ctx, h)
	if h.Get("X-Turn") != "turn-1" {
		t.Fatalf("X-Turn = %q", h.Get("X-Turn"))
	}
	if h.Get("Authorization") != "Bearer client-credential" || len(h.Values("Authorization")) != 1 {
		t.Fatalf("credential replaced: %v", h.Values("Authorization"))
	}
}

func TestNoRequestHeadersAddNothing(t *testing.T) {
	h := http.Header{}
	ApplyRequestHeaders(context.Background(), h)
	ApplyRequestHeaders(ContextWithRequestHeaders(context.Background(), nil), h)
	if len(h) != 0 {
		t.Fatalf("%v", h)
	}
}

func TestRequestHeadersMiddlewareLabelsTheRequest(t *testing.T) {
	labels := http.Header{}
	labels.Set("X-Turn", "turn-1")
	r, _ := http.NewRequestWithContext(ContextWithRequestHeaders(context.Background(), labels), "POST", "https://example.test", nil)
	var sent string
	_, _ = RequestHeadersMiddleware(r, func(r *http.Request) (*http.Response, error) {
		sent = r.Header.Get("X-Turn")
		return nil, nil
	})
	if sent != "turn-1" {
		t.Fatalf("sent %q", sent)
	}
}
