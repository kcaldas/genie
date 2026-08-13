// Package telemetry exposes genie's tracer and W3C trace-context helpers.
// Spans come from the global OpenTelemetry provider: they are no-ops unless
// the host process initializes one, so genie itself configures no exporter
// and adds no overhead for uninstrumented hosts.
package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Tracer returns genie's tracer from the global provider.
func Tracer() oteltrace.Tracer {
	return otel.Tracer("genie")
}

// Traceparent serializes ctx's span context to a W3C traceparent value,
// or "" when ctx carries no valid span.
func Traceparent(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	return carrier["traceparent"]
}

// SpanContextFromTraceparent parses a W3C traceparent value; the zero
// SpanContext (IsValid() == false) when tp is empty or malformed.
func SpanContextFromTraceparent(tp string) oteltrace.SpanContext {
	if tp == "" {
		return oteltrace.SpanContext{}
	}
	ctx := propagation.TraceContext{}.Extract(context.Background(), propagation.MapCarrier{"traceparent": tp})
	return oteltrace.SpanContextFromContext(ctx)
}
