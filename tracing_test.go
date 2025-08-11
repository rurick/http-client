package httpclient

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestNewTracer(t *testing.T) {
	tracer := NewTracer()
	if tracer == nil {
		t.Fatal("NewTracer returned nil")
	}

	if tracer.tracer == nil {
		t.Error("NewTracer created tracer with nil internal tracer")
	}
}

func TestTracerStartSpan(t *testing.T) {
	tracer := NewTracer()

	ctx := context.Background()
	spanName := "http-request"

	newCtx, span := tracer.StartSpan(ctx, spanName)

	if newCtx == nil {
		t.Error("StartSpan returned nil context")
	}
	if span == nil {
		t.Error("StartSpan returned nil span")
	}

	// Test that span can be ended without panic
	span.End()
}

func TestTracerStartSpanWithOptions(t *testing.T) {
	tracer := NewTracer()

	ctx := context.Background()
	spanName := "http-request-with-options"

	// Test with span options
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindClient),
	}

	newCtx, span := tracer.StartSpan(ctx, spanName, opts...)

	if newCtx == nil {
		t.Error("StartSpan with options returned nil context")
	}
	if span == nil {
		t.Error("StartSpan with options returned nil span")
	}

	span.End()
}

func TestTracerSpanFromContext(t *testing.T) {
	tracer := NewTracer()

	ctx := context.Background()
	spanName := "test-span"

	// Start a span
	newCtx, span := tracer.StartSpan(ctx, spanName)
	defer span.End()

	// Get span from context
	retrievedSpan := tracer.SpanFromContext(newCtx)
	if retrievedSpan == nil {
		t.Error("SpanFromContext returned nil span")
	}

	// Test that we can retrieve a span (spans may not be directly comparable)
	if retrievedSpan == nil {
		t.Error("SpanFromContext should return a valid span from context with span")
	}
}
