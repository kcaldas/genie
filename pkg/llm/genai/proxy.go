package genai

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

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
	if err != nil || !validProxyURL(u) {
		return fmt.Errorf("GENIE_GOOGLE_BASE_URL must be HTTPS (HTTP is allowed on loopback)")
	}
	if cfg.Backend == genai.BackendVertexAI && cfg.Project == "" {
		cfg.Project = "proxy"
	}
	cfg.HTTPOptions.BaseURL = strings.TrimRight(base, "/") + "/"
	cfg.HTTPClient = &http.Client{Transport: proxyTransport{base: http.DefaultTransport, token: token, host: u.Host}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return nil
}

// validProxyURL accepts a bare HTTPS origin, or HTTP on loopback.
func validProxyURL(u *url.URL) bool {
	if u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		host := u.Hostname()
		return host == "localhost" || host == "127.0.0.1" || host == "::1"
	}
	return false
}

type proxyTransport struct {
	base        http.RoundTripper
	token, host string
}

func (t proxyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != t.host {
		return nil, fmt.Errorf("proxy request escaped the configured Google proxy host")
	}
	r = r.Clone(r.Context())
	r.Header.Del("X-Goog-Api-Key")
	r.Header.Del("X-Goog-User-Project")
	r.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(r)
}
