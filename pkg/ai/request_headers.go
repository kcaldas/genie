package ai

import (
	"context"
	"net/http"
)

type requestHeadersContextKey struct{}

type requestHeaders struct {
	host    string
	headers http.Header
}

// ContextWithRequestHeaders returns a context whose provider requests to host
// carry headers, so a caller can label each model call with what it is for —
// for example the conversation an inference proxy attributes the call to.
// Requests to any other host, such as a provider called directly, never see
// them. The provider clients add them to every matching request made with
// the context; a header the client sets itself, credentials included, is
// never replaced. host is a URL host ("proxy.example.com" or
// "127.0.0.1:8080"); an empty host or empty headers leave the context
// unchanged.
func ContextWithRequestHeaders(ctx context.Context, host string, headers http.Header) context.Context {
	if host == "" || len(headers) == 0 {
		return ctx
	}
	return context.WithValue(ctx, requestHeadersContextKey{}, requestHeaders{host: host, headers: headers.Clone()})
}

// ApplyRequestHeaders adds the headers attached by ContextWithRequestHeaders
// to r when r is addressed to their host, skipping any r already carries.
func ApplyRequestHeaders(ctx context.Context, r *http.Request) {
	if ctx == nil || r == nil || r.URL == nil {
		return
	}
	labels, ok := ctx.Value(requestHeadersContextKey{}).(requestHeaders)
	if !ok || r.URL.Host != labels.host {
		return
	}
	for name, values := range labels.headers {
		if r.Header.Get(name) != "" {
			continue
		}
		for _, value := range values {
			r.Header.Add(name, value)
		}
	}
}

// RequestHeadersTransport adds the request's context headers on each hop
// addressed to their host, on a copy of the request. The client copies a
// request's headers onto every redirect it follows, so labels added before
// the client sends would reach whatever host it is redirected to; added here,
// a redirect to another host never carries them. A nil base is
// http.DefaultTransport.
func RequestHeadersTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return requestHeadersTransport{base: base}
}

type requestHeadersTransport struct{ base http.RoundTripper }

func (t requestHeadersTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if labels, ok := r.Context().Value(requestHeadersContextKey{}).(requestHeaders); ok && r.URL != nil && r.URL.Host == labels.host {
		r = r.Clone(r.Context())
		ApplyRequestHeaders(r.Context(), r)
	}
	return t.base.RoundTrip(r)
}
