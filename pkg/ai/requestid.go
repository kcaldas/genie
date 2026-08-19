package ai

import "context"

type requestIDContextKey struct{}

// ContextWithRequestID returns a context carrying a chat invocation's request
// ID. Genie attaches it before running a prompt so code executed on behalf of
// the invocation — tool handlers, provider clients — can stamp the events it
// publishes (for example token counts) with the invocation that caused them.
// A blank ID leaves the context unchanged.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// RequestIDFromContext returns the request ID attached by
// ContextWithRequestID, or "" when none is set — for example on token counts
// requested outside any chat invocation.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(requestIDContextKey{}).(string); ok {
		return value
	}
	return ""
}
