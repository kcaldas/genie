package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// newTransport delegates wire framing, multiplexing, cancellation and protocol
// negotiation to the SDK. A command's lifetime is the session's, not the short
// connection context's. Its stderr goes to the null device, never the TUI.
func newTransport(server string, config ServerConfig, options Options, maxBlobBytes int) (sdk.Transport, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if config.GetTransportType() == TransportStdio {
		command := exec.Command(config.Command, config.Args...)
		command.Env = os.Environ()
		for k, v := range config.Env {
			command.Env = append(command.Env, k+"="+v)
		}
		return &sdk.CommandTransport{Command: command}, nil
	}
	base := http.DefaultClient
	if options.HTTPClient != nil {
		base = options.HTTPClient
	}
	client := *base
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	endpoint, err := url.Parse(config.URL)
	if err != nil {
		return nil, err
	}
	client.Transport = &headerTransport{base: transport, endpoint: endpoint, headers: config.Headers}
	redirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if !sameOrigin(endpoint, req.URL) {
			return fmt.Errorf("MCP cross-origin redirect refused")
		}
		if redirect != nil {
			return redirect(req, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("too many MCP redirects")
		}
		return nil
	}
	maxEventSize := -1
	if maxBlobBytes >= 0 {
		maxEventSize = maxBlobBytes*2 + (1 << 20)
	}
	if config.GetTransportType() == TransportSSE {
		return &legacySSETransport{SSEClientTransport: &sdk.SSEClientTransport{Endpoint: config.URL, HTTPClient: &client, MaxEventSize: maxEventSize}}, nil
	}
	stream := &sdk.StreamableClientTransport{Endpoint: config.URL, HTTPClient: &client, MaxRetries: -1, MaxEventSize: maxEventSize}
	if options.OAuthHandler != nil {
		stream.OAuthHandler = options.OAuthHandler(server)
	}
	return stream, nil
}

type headerTransport struct {
	base     http.RoundTripper
	endpoint *url.URL
	headers  map[string]string
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) &&
		strings.EqualFold(a.Hostname(), b.Hostname()) && originPort(a) == originPort(b)
}

func originPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !sameOrigin(t.endpoint, req.URL) {
		return nil, fmt.Errorf("MCP cross-origin request refused")
	}
	clone := req.Clone(req.Context())
	for name, value := range t.headers {
		clone.Header.Set(name, value)
	}
	return t.base.RoundTrip(clone)
}

// The legacy SDK transport binds its long-lived GET to the Connect context.
// Keep cancellation during connection establishment, then let session.Close
// own that stream rather than the short server-discovery deadline.
type legacySSETransport struct{ *sdk.SSEClientTransport }

func (t *legacySSETransport) Connect(ctx context.Context) (sdk.Connection, error) {
	streamCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	stop := context.AfterFunc(ctx, cancel)
	conn, err := t.SSEClientTransport.Connect(streamCtx)
	stop()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		cancel()
		_ = conn.Close()
		return nil, err
	}
	return &legacySSEConnection{Connection: conn, cancel: cancel}, nil
}

type legacySSEConnection struct {
	sdk.Connection
	cancel context.CancelFunc
}

func (c *legacySSEConnection) Close() error {
	c.cancel()
	return c.Connection.Close()
}
