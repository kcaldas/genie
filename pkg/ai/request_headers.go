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

// RequestHeadersMiddleware adds the request's context headers before sending
// it, for provider SDKs that take an HTTP middleware (OpenAI, Anthropic).
func RequestHeadersMiddleware(r *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	ApplyRequestHeaders(r.Context(), r)
	return next(r)
}
