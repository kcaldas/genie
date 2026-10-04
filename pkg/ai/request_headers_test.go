package ai

import (
	"context"
	"net/http"
	"testing"
)

func labelled(host string) context.Context {
	labels := http.Header{}
	labels.Set("X-Turn", "turn-1")
	labels.Set("Authorization", "Bearer caller-supplied")
	return ContextWithRequestHeaders(context.Background(), host, labels)
}

func TestRequestHeadersAreAddedWithoutReplacingTheClients(t *testing.T) {
	r, _ := http.NewRequest("POST", "https://proxy.test/v1/responses", nil)
	r.Header.Set("Authorization", "Bearer client-credential")
	ApplyRequestHeaders(labelled("proxy.test"), r)
	if r.Header.Get("X-Turn") != "turn-1" {
		t.Fatalf("X-Turn = %q", r.Header.Get("X-Turn"))
	}
	if r.Header.Get("Authorization") != "Bearer client-credential" || len(r.Header.Values("Authorization")) != 1 {
		t.Fatalf("credential replaced: %v", r.Header.Values("Authorization"))
	}
}

// A provider called directly is another host: it never sees the labels.
func TestRequestHeadersStayOnTheirHost(t *testing.T) {
	r, _ := http.NewRequest("POST", "https://api.provider.test/v1/responses", nil)
	ApplyRequestHeaders(labelled("proxy.test"), r)
	if r.Header.Get("X-Turn") != "" {
		t.Fatalf("labels sent to another host: %v", r.Header)
	}
}

func TestAttachedHeadersAreACopy(t *testing.T) {
	labels := http.Header{}
	labels.Set("X-Turn", "turn-1")
	ctx := ContextWithRequestHeaders(context.Background(), "proxy.test", labels)
	labels.Set("X-Turn", "mutated after attaching")
	r, _ := http.NewRequest("POST", "https://proxy.test/", nil)
	ApplyRequestHeaders(ctx, r)
	if r.Header.Get("X-Turn") != "turn-1" {
		t.Fatalf("X-Turn = %q", r.Header.Get("X-Turn"))
	}
}

func TestNoRequestHeadersAddNothing(t *testing.T) {
	r, _ := http.NewRequest("POST", "https://proxy.test/", nil)
	ApplyRequestHeaders(context.Background(), r)
	ApplyRequestHeaders(ContextWithRequestHeaders(context.Background(), "proxy.test", nil), r)
	ApplyRequestHeaders(ContextWithRequestHeaders(context.Background(), "", http.Header{"X-Turn": {"t"}}), r)
	if len(r.Header) != 0 {
		t.Fatalf("%v", r.Header)
	}
}

func TestRequestHeadersMiddlewareLabelsTheRequest(t *testing.T) {
	r, _ := http.NewRequestWithContext(labelled("proxy.test"), "POST", "https://proxy.test/v1/responses", nil)
	var sent string
	_, _ = RequestHeadersMiddleware(r, func(r *http.Request) (*http.Response, error) {
		sent = r.Header.Get("X-Turn")
		return nil, nil
	})
	if sent != "turn-1" {
		t.Fatalf("sent %q", sent)
	}
}
