package genai

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"google.golang.org/genai"
)

// configureProxy preserves Google's native backend and wire format. Supplying
// an HTTPClient makes authentication explicit and prevents Vertex ADC discovery.
func configureProxy(cfg *genai.ClientConfig, manager config.Manager) error {
	base := manager.GetStringWithDefault("GENIE_GOOGLE_BASE_URL", "")
	if base == "" {
		return nil
	}
	token := manager.GetStringWithDefault("GENIE_GOOGLE_AUTH_TOKEN", "")
	if token == "" {
		return fmt.Errorf("GENIE_GOOGLE_AUTH_TOKEN is required with GENIE_GOOGLE_BASE_URL")
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
		return fmt.Errorf("GENIE_GOOGLE_BASE_URL must be HTTPS (HTTP is allowed on loopback)")
	}
	if cfg.Backend == genai.BackendVertexAI && cfg.Project == "" {
		cfg.Project = "proxy"
	}
	cfg.HTTPOptions.BaseURL = strings.TrimRight(base, "/") + "/"
	cfg.HTTPClient = &http.Client{Transport: proxyTransport{base: http.DefaultTransport, token: token, host: u.Host}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return nil
}

type proxyTransport struct {
	base        http.RoundTripper
	token, host string
}

func (t proxyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != t.host {
		return nil, fmt.Errorf("Google proxy request escaped configured host")
	}
	r = r.Clone(r.Context())
	r.Header.Del("X-Goog-Api-Key")
	r.Header.Del("X-Goog-User-Project")
	r.Header.Set("Authorization", "Bearer "+t.token)
	resp, err := t.base.RoundTrip(r)
	if err != nil || resp.StatusCode < 400 || !ai.RefusesRetry(resp.Header) {
		return resp, err
	}
	// The proxy refused the call for good (a budget refusal): report it as a
	// final error so no layer repeats it. The Google SDK does not retry
	// transport errors on generate calls.
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	return nil, ai.NonRetryable(fmt.Errorf("Google proxy refused the request: status %s: %s", resp.Status, strings.TrimSpace(string(body))))
}
