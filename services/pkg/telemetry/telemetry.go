// Package telemetry provides structured logging, span/trace context, and
// OTLP configuration for IronGolem OS services. All services should use
// these helpers to ensure consistent observability.
package telemetry

import (
	"context"
	"log/slog"
	"os"
	"time"
)

// Config holds the telemetry configuration for a service.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string // "development", "staging", "production"
	LogLevel       slog.Level
	OTLPEndpoint   string // e.g. "localhost:4317"
	OTLPInsecure   bool
}

// DefaultConfig returns a development-friendly configuration.
func DefaultConfig(serviceName string) Config {
	return Config{
		ServiceName:    serviceName,
		ServiceVersion: "0.1.0",
		Environment:    "development",
		LogLevel:       slog.LevelDebug,
		OTLPEndpoint:   "localhost:4317",
		OTLPInsecure:   true,
	}
}

// traceIDKey and spanIDKey are context keys for trace propagation.
type ctxKey int

const (
	traceIDKey ctxKey = iota
	spanIDKey
	tenantIDKey
)

// SpanContext carries trace and span identifiers through the call chain.
type SpanContext struct {
	TraceID   string
	SpanID    string
	ParentID  string
	Operation string
	StartTime time.Time
	Attrs     map[string]string
}

// NewSpan creates a new SpanContext and stores it in the context.
func NewSpan(ctx context.Context, operation string) (context.Context, *SpanContext) {
	sc := &SpanContext{
		TraceID:   extractOrGenerate(ctx, traceIDKey),
		SpanID:    generateSpanID(),
		Operation: operation,
		StartTime: time.Now(),
		Attrs:     make(map[string]string),
	}

	// Inherit parent span if present.
	if parentSpan, ok := SpanFromContext(ctx); ok {
		sc.ParentID = parentSpan.SpanID
		sc.TraceID = parentSpan.TraceID
	}

	ctx = context.WithValue(ctx, spanIDKey, sc)
	return ctx, sc
}

// End records the span duration. In a real implementation this would
// export the span to the OTLP collector.
func (sc *SpanContext) End(logger *slog.Logger) {
	elapsed := time.Since(sc.StartTime)
	logger.Info("span.end",
		slog.String("trace_id", sc.TraceID),
		slog.String("span_id", sc.SpanID),
		slog.String("parent_id", sc.ParentID),
		slog.String("operation", sc.Operation),
		slog.Duration("duration", elapsed),
	)
}

// SpanFromContext retrieves the current SpanContext if one exists.
func SpanFromContext(ctx context.Context) (*SpanContext, bool) {
	sc, ok := ctx.Value(spanIDKey).(*SpanContext)
	return sc, ok
}

// WithTenantID attaches a tenant ID to the context for log correlation.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// TenantIDFromContext retrieves the tenant ID from the context.
func TenantIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(tenantIDKey).(string); ok {
		return v
	}
	return ""
}

// SetupLogger creates a structured slog.Logger configured for the service.
// The logger includes service name and environment as default attributes.
func SetupLogger(cfg Config) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level:     cfg.LogLevel,
		AddSource: cfg.Environment != "production",
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)

	logger := slog.New(handler).With(
		slog.String("service", cfg.ServiceName),
		slog.String("version", cfg.ServiceVersion),
		slog.String("env", cfg.Environment),
	)

	return logger
}

// extractOrGenerate returns an existing trace ID from context or generates
// a new one.
func extractOrGenerate(ctx context.Context, key ctxKey) string {
	if v, ok := ctx.Value(key).(string); ok && v != "" {
		return v
	}
	return generateSpanID()
}

// generateSpanID produces a simple unique span/trace identifier.
// In production this would use a proper 128-bit trace ID generator.
func generateSpanID() string {
	return time.Now().UTC().Format("20060102150405.000000000")
}
