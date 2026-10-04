package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

// A redirect to another host is a new hop: the client copies the request's
// headers onto it, so the labels must be added per hop, never to the request
// the client copies from.
func TestRequestHeadersDoNotFollowRedirectsToAnotherHost(t *testing.T) {
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Turn") != "" {
			t.Errorf("labels followed the redirect: %v", r.Header)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer elsewhere.Close()
	labelledHit := false
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		labelledHit = r.Header.Get("X-Turn") == "turn-1"
		http.Redirect(w, r, elsewhere.URL+"/landed", http.StatusTemporaryRedirect)
	}))
	defer proxy.Close()
	u, _ := url.Parse(proxy.URL)
	client := &http.Client{Transport: RequestHeadersTransport(nil)}
	r, _ := http.NewRequestWithContext(labelled(u.Host), "POST", proxy.URL+"/v1/responses", strings.NewReader("{}"))
	resp, err := client.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if !labelledHit {
		t.Fatal("the labelled host did not receive the labels")
	}
}
