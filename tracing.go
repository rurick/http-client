package httpclient

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Tracer is a wrapper for OpenTelemetry tracing.
type Tracer struct {
	tracer trace.Tracer
}

// NewTracer creates a new tracer instance.
func NewTracer() *Tracer {
	tracer := otel.Tracer("github.com/rurick/http-client")

	return &Tracer{
		tracer: tracer,
	}
}

// StartSpan starts a new span.
func (t *Tracer) StartSpan(
	ctx context.Context,
	name string,
	opts ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// SpanFromContext returns the span from the context.
func (t *Tracer) SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}
