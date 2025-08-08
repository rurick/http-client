package httpclient

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Tracer обёртка для OpenTelemetry трассировки
type Tracer struct {
	tracer trace.Tracer
}

// NewTracer создаёт новый экземпляр трассировщика
func NewTracer() *Tracer {
	tracer := otel.Tracer("gitlab.citydrive.tech/back-end/go/pkg/http-client")

	return &Tracer{
		tracer: tracer,
	}
}

// StartSpan начинает новый span
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// SpanFromContext возвращает span из контекста
func (t *Tracer) SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}
