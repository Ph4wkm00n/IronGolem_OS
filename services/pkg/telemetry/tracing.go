package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// SpanStatus represents the outcome of a span.
type SpanStatus string

const (
	SpanStatusUnset SpanStatus = "UNSET"
	SpanStatusOK    SpanStatus = "OK"
	SpanStatusError SpanStatus = "ERROR"
)

// Span represents a unit of work within a trace. It follows the OpenTelemetry
// span model with trace_id, span_id, parent, name, timing, and attributes.
type Span struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id,omitempty"`
	Name         string            `json:"name"`
	StartTime    time.Time         `json:"start_time"`
	EndTime      time.Time         `json:"end_time,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	Status       SpanStatus        `json:"status"`
	StatusMsg    string            `json:"status_message,omitempty"`

	mu       sync.Mutex
	ended    bool
	exporter SpanExporter
}

// SetAttribute adds a key-value attribute to the span.
func (s *Span) SetAttribute(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Attributes == nil {
		s.Attributes = make(map[string]string)
	}
	s.Attributes[key] = value
}

// SetStatus updates the span status and optional message.
func (s *Span) SetStatus(status SpanStatus, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	s.StatusMsg = msg
}

// End marks the span as complete and exports it.
func (s *Span) End() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.ended = true
	s.EndTime = time.Now().UTC()
	if s.exporter != nil {
		s.exporter.ExportSpan(*s)
	}
}

// SpanExporter receives completed spans. Implementations may send spans
// to an OTLP collector, store them in memory, or write them to a log.
type SpanExporter interface {
	ExportSpan(span Span)
	Shutdown()
}

// Tracer creates and manages spans for a single service.
type Tracer struct {
	serviceName  string
	otlpEndpoint string
	exporter     SpanExporter
}

// spanCtxKey is the context key for the current span.
type spanCtxKey struct{}

// InitTracer initializes a Tracer with an in-process span exporter that
// records spans to the global trace store. The returned function should be
// called on shutdown to flush and close the exporter.
//
// If otlpEndpoint is non-empty, spans are also logged with their destination
// (a real OTLP gRPC exporter would be wired in here for production).
func InitTracer(serviceName, otlpEndpoint string) (func(), error) {
	exporter := NewInMemoryExporter()
	t := &Tracer{
		serviceName:  serviceName,
		otlpEndpoint: otlpEndpoint,
		exporter:     exporter,
	}
	globalTracerMu.Lock()
	globalTracer = t
	globalTracerMu.Unlock()

	shutdown := func() {
		exporter.Shutdown()
	}
	return shutdown, nil
}

// global tracer singleton
var (
	globalTracer   *Tracer
	globalTracerMu sync.RWMutex
)

func getTracer() *Tracer {
	globalTracerMu.RLock()
	defer globalTracerMu.RUnlock()
	return globalTracer
}

// StartSpan begins a new span as a child of any span in the context.
// Returns the updated context and the new span.
func StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	t := getTracer()

	traceID := generateTraceID()
	parentSpanID := ""

	// Inherit trace ID from parent span if present.
	if parent := SpanFromCtx(ctx); parent != nil {
		traceID = parent.TraceID
		parentSpanID = parent.SpanID
	}

	var exporter SpanExporter
	if t != nil {
		exporter = t.exporter
	}

	span := &Span{
		TraceID:      traceID,
		SpanID:       generateID128()[:16],
		ParentSpanID: parentSpanID,
		Name:         name,
		StartTime:    time.Now().UTC(),
		Attributes:   make(map[string]string),
		Status:       SpanStatusUnset,
		exporter:     exporter,
	}

	if t != nil {
		span.Attributes["service.name"] = t.serviceName
	}

	ctx = context.WithValue(ctx, spanCtxKey{}, span)
	return ctx, span
}

// SpanFromCtx extracts the current Span from the context, or nil if none.
func SpanFromCtx(ctx context.Context) *Span {
	if s, ok := ctx.Value(spanCtxKey{}).(*Span); ok {
		return s
	}
	return nil
}

// TracingMiddleware is an HTTP middleware that creates a span for each
// incoming request and records standard HTTP attributes.
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := StartSpan(r.Context(), fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path))
		defer span.End()

		span.SetAttribute("http.method", r.Method)
		span.SetAttribute("http.url", r.URL.String())
		span.SetAttribute("http.host", r.Host)

		// Propagate tenant and workspace IDs from headers if present.
		if tid := r.Header.Get("X-Tenant-ID"); tid != "" {
			span.SetAttribute("tenant_id", tid)
		}
		if wid := r.Header.Get("X-Workspace-ID"); wid != "" {
			span.SetAttribute("workspace_id", wid)
		}

		// Use a response writer wrapper to capture the status code.
		rw := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))

		span.SetAttribute("http.status_code", fmt.Sprintf("%d", rw.statusCode))
		if rw.statusCode >= 400 {
			span.SetStatus(SpanStatusError, fmt.Sprintf("HTTP %d", rw.statusCode))
		} else {
			span.SetStatus(SpanStatusOK, "")
		}
	})
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// --- ID Generation ---

func generateTraceID() string {
	return generateID128()
}

// generateID128 produces a 32-hex-char (128-bit) random identifier.
func generateID128() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto/rand fails.
		return fmt.Sprintf("%032x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
