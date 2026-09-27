package genai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kcaldas/genie/pkg/config"
	"google.golang.org/genai"
)

func TestNativeProxyWithoutGoogleCredentials(t *testing.T) {
	for _, backend := range []Backend{BackendGeminiAPI, BackendVertexAI} {
		t.Run(string(backend), func(t *testing.T) {
			for _, key := range []string{"GOOGLE_CLOUD_PROJECT", "GOOGLE_CLOUD_LOCATION", "GOOGLE_API_KEY", "GEMINI_API_KEY"} {
				t.Setenv(key, "")
			}
			t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/does-not-exist")
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Header.Get("Authorization") != "Bearer mutiro-key" || r.Header.Get("X-Goog-Api-Key") != "" {
					t.Errorf("wrong authentication: %v", r.Header)
				}
				if !strings.Contains(r.URL.Path, "/models/gemini-test:generateContent") {
					t.Errorf("wrong native path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]}}]}`)
			}))
			defer server.Close()
			t.Setenv("GENIE_GOOGLE_BASE_URL", server.URL+"/mutiro/"+string(backend))
			t.Setenv("GENIE_GOOGLE_AUTH_TOKEN", "mutiro-key")
			client, actual, err := createClientWithBackend(config.NewConfigManager(), backend)
			if err != nil {
				t.Fatal(err)
			}
			if actual != backend {
				t.Fatalf("changed backend to %s", actual)
			}
			response, err := client.Models.GenerateContent(context.Background(), "gemini-test", genai.Text("hi"), nil)
			if err != nil {
				t.Fatal(err)
			}
			if !called || response.Text() != "hello" {
				t.Fatal("native response missing")
			}
		})
	}
}

func TestProxyRequiresExplicitCredentialAndSafeURL(t *testing.T) {
	for _, tc := range []struct{ url, token string }{
		{"https://example.test", ""},
		{"http://example.test", "key"},
		{"https://user:pass@example.test", "key"},
		{"https://example.test?key=secret", "key"},
	} {
		t.Run(tc.url, func(t *testing.T) {
			t.Setenv("GENIE_GOOGLE_BASE_URL", tc.url)
			t.Setenv("GENIE_GOOGLE_AUTH_TOKEN", tc.token)
			if err := configureProxy(&genai.ClientConfig{}, config.NewConfigManager()); err == nil {
				t.Fatal("unsafe proxy configuration accepted")
			}
		})
	}
}
