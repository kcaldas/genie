package genai

import (
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
