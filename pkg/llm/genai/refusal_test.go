package genai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
)

func TestProxyRefusalIsFinal(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Should-Retry", "false")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"message":"plan limit reached (quota daily_spend)","status":"Too Many Requests"}}`))
	}))
	defer s.Close()
	u, _ := url.Parse(s.URL)
	tr := proxyTransport{base: http.DefaultTransport, token: "t", host: u.Host}
	req, _ := http.NewRequest("POST", s.URL+"/v1/models/m:generateContent", strings.NewReader("{}"))
	resp, err := tr.RoundTrip(req)
	if resp != nil {
		t.Fatal("a refusal becomes an error, not a response")
	}
	if err == nil || ai.IsRetryable(err) || !strings.Contains(err.Error(), "plan limit reached") {
		t.Fatalf("%v", err)
	}
}

func TestProxyCarriesTheContextHeadersButNotItsCredential(t *testing.T) {
	var got http.Header
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer s.Close()
	u, _ := url.Parse(s.URL)
	labels := http.Header{}
	labels.Set("X-Turn", "turn-1")
	labels.Set("Authorization", "Bearer caller-supplied")
	req, _ := http.NewRequestWithContext(ai.ContextWithRequestHeaders(context.Background(), u.Host, labels), "POST", s.URL+"/v1/models/m:generateContent", strings.NewReader("{}"))
	resp, err := proxyTransport{base: http.DefaultTransport, token: "t", host: u.Host}.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if got.Get("X-Turn") != "turn-1" || got.Get("Authorization") != "Bearer t" {
		t.Fatalf("headers %v", got)
	}
}
