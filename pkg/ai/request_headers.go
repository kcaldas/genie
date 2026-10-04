package ai

import (
	"context"
	"net/http"
)

type requestHeadersContextKey struct{}

// ContextWithRequestHeaders returns a context whose provider requests carry
// headers, so a caller can label each model call with what it is for — for
// example the conversation an inference proxy attributes the call to. The
// provider clients add them to every request made with the context; a header
// the client sets itself, credentials included, is never replaced. Empty
// headers leave the context unchanged.
func ContextWithRequestHeaders(ctx context.Context, headers http.Header) context.Context {
	if len(headers) == 0 {
		return ctx
	}
	return context.WithValue(ctx, requestHeadersContextKey{}, headers.Clone())
}

// ApplyRequestHeaders adds the headers attached by ContextWithRequestHeaders
// to h, skipping any h already carries.
func ApplyRequestHeaders(ctx context.Context, h http.Header) {
	if ctx == nil {
		return
	}
	headers, _ := ctx.Value(requestHeadersContextKey{}).(http.Header)
	for name, values := range headers {
		if h.Get(name) != "" {
			continue
		}
		for _, value := range values {
			h.Add(name, value)
		}
	}
}

// RequestHeadersMiddleware adds the request's context headers before sending
// it, for provider SDKs that take an HTTP middleware (OpenAI, Anthropic).
func RequestHeadersMiddleware(r *http.Request, next func(*http.Request) (*http.Response, error)) (*http.Response, error) {
	ApplyRequestHeaders(r.Context(), r.Header)
	return next(r)
}
