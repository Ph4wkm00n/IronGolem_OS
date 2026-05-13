// Package handler implements the HTTP handlers for the Gateway service.
//
// Each handler propagates context for tracing and tenant isolation, uses
// structured logging, and produces events for the audit trail.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/connector"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/middleware"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/planner"
	gwruntime "github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/runtime"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	ipc "github.com/Ph4wkm00n/IronGolem_OS/services/pkg/runtime"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// errNotFound is a sentinel error returned when a resource is not found.
var errNotFound = errors.New("not found")

// RuntimeExecutor is the subset of the gateway's runtime client that the
// handler needs. Pulled out as an interface so tests can inject a fake.
type RuntimeExecutor interface {
	Execute(ctx context.Context, workspaceID string, plan ipc.Plan) (*gwruntime.ExecuteResult, error)
}

// DefaultInboundTimeout bounds an inbound round-trip (synth → execute →
// terminal response). v0.1 uses a synchronous HTTP path; cap is intentionally
// short so misbehaving providers don't pile up gateway goroutines.
const DefaultInboundTimeout = 30 * time.Second

// defaultWorkspaceID is the placeholder UUID used when an inbound message
// arrives without an explicit workspace. The Rust runtime parses this field
// strictly as a UUID, so we use the nil UUID rather than the empty string.
const defaultWorkspaceID = "00000000-0000-0000-0000-000000000000"

// Handler holds the dependencies for the gateway HTTP handlers.
type Handler struct {
	logger     *slog.Logger
	connMgr    *connector.Manager
	runtime    RuntimeExecutor
	eventStore EventStore
	// inboundTimeout caps the synth → execute round-trip; defaults to
	// DefaultInboundTimeout when unset.
	inboundTimeout time.Duration
}

// Options carries the optional dependencies the handler needs once Step 5
// is wired (runtime client + event store for the inbound flow). Passing
// the zero value preserves the v0.1 Step 1-4 behavior.
type Options struct {
	Runtime        RuntimeExecutor
	EventStore     EventStore
	InboundTimeout time.Duration
}

// EventStore is the persistence interface for the gateway audit trail.
// Both InMemoryEventStore and SQLiteEventStore satisfy it — recipes,
// approvals, squads, and timeline handlers all consume the interface,
// so Step 6's mock-to-real flip is purely a wiring change in main.go.
type EventStore interface {
	Append(evt events.Event)
	List(page, pageSize int, workspaceFilter, kindFilter string) ([]events.Event, int)
	Get(id string) (events.Event, bool)
}

// New creates a Handler with the given logger and connector manager.
// For the Step 5 inbound flow, callers should use NewWithOptions to wire
// in the runtime executor + event store.
func New(logger *slog.Logger, connMgr *connector.Manager) *Handler {
	return NewWithOptions(logger, connMgr, Options{})
}

// NewWithOptions creates a Handler with the runtime client + event store
// hooked up. Both are optional: when Runtime is nil, MessageInbound falls
// back to the v0.1 Step 1-4 "accept and log" behavior. When EventStore is
// nil, audit events are dropped silently. Production wiring sets both.
func NewWithOptions(logger *slog.Logger, connMgr *connector.Manager, opts Options) *Handler {
	timeout := opts.InboundTimeout
	if timeout == 0 {
		timeout = DefaultInboundTimeout
	}
	return &Handler{
		logger:         logger,
		connMgr:        connMgr,
		runtime:        opts.Runtime,
		eventStore:     opts.EventStore,
		inboundTimeout: timeout,
	}
}

// HealthCheck responds with the service health status.
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "gateway",
		"time":    time.Now().UTC(),
	})
}

// InboundMessageRequest is the request body for message ingress.
type InboundMessageRequest struct {
	ConnectorID string `json:"connector_id"`
	ChannelID   string `json:"channel_id"`
	UserID      string `json:"user_id,omitempty"`
	Content     string `json:"content"`
	TenantID    string `json:"tenant_id"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// MessageInbound handles incoming messages from external connectors.
// When a runtime executor is wired in (Step 5+), the handler synthesizes a
// plan, executes it via runtimed, and returns the LLM response synchronously.
// When no runtime is wired, the handler falls back to the v0.1 Step 1-4
// "accept and log" behavior so the legacy ingress path keeps working.
func (h *Handler) MessageInbound(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.message_inbound")
	defer span.End(h.logger)

	var req InboundMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WarnContext(ctx, "invalid inbound message body",
			slog.String("error", err.Error()),
		)
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	if req.ConnectorID == "" || req.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "connector_id and content are required",
		})
		return
	}

	// Verify the connector is healthy.
	status, exists := h.connMgr.Status(req.ConnectorID)
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "connector not found",
		})
		return
	}
	if status.Health == connector.HealthDisconnected {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "connector is disconnected",
		})
		return
	}

	// Identity always comes from the HMAC token (auth middleware put it
	// in the request context). The request body's tenant_id / workspace_id
	// are NOT consulted — caller-supplied identity fields would let any
	// authenticated client impersonate any tenant or workspace.
	tenantID := middleware.TenantIDFromContext(ctx)
	if tenantID == "" {
		tenantID = req.TenantID // pre-Step-5 fallback (in-memory tests)
	}
	workspaceID := middleware.WorkspaceIDFromContext(ctx)

	msg := planner.InboundMessage{
		ConnectorID: req.ConnectorID,
		ChannelID:   req.ChannelID,
		UserID:      req.UserID,
		Content:     req.Content,
		TenantID:    tenantID,
		WorkspaceID: workspaceID,
	}

	if h.runtime == nil {
		// Legacy fall-through used by pre-Step-5 integration tests. Records
		// the inbound event and ACKs; no plan execution.
		evtID := h.recordInbound(msg)
		h.logger.InfoContext(ctx, "message received (runtime not wired)",
			slog.String("event_id", evtID),
			slog.String("connector_id", req.ConnectorID),
			slog.String("tenant_id", msg.TenantID),
		)
		writeJSON(w, http.StatusAccepted, map[string]any{
			"event_id": evtID,
			"status":   "accepted",
		})
		return
	}

	result, err := h.HandleInbound(ctx, msg)
	if err != nil {
		h.logger.ErrorContext(ctx, "inbound plan failed",
			slog.String("error", err.Error()),
			slog.String("connector_id", req.ConnectorID),
		)
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"event_id": result.InboundEventID,
		"reply":    result.Reply,
		"status":   "completed",
	})
}

// InboundResult is the outcome of HandleInbound: the audit-trail event id
// for the inbound message plus the LLM-synthesized reply.
type InboundResult struct {
	// InboundEventID is the gateway audit event id for the inbound side.
	InboundEventID string
	// Reply is the LLM text the runtime returned. Empty if the plan
	// completed but produced no text node.
	Reply string
}

// HandleInbound is the internal entry point shared by the HTTP MessageInbound
// handler and the connector receive pump. It records an inbound audit event,
// synthesizes a 1-node plan via the planner, executes it through the runtime
// client, and returns the LLM reply. The caller is responsible for delivering
// the reply (e.g. via connector Send for the pump path; via HTTP response for
// the synchronous path).
func (h *Handler) HandleInbound(ctx context.Context, msg planner.InboundMessage) (*InboundResult, error) {
	if h.runtime == nil {
		return nil, errors.New("runtime client not configured")
	}

	// Stamp a default workspace id when the inbound message doesn't carry
	// one. The Rust side parses workspace_id as a UUID strictly — empty
	// string trips a deserialize failure that surfaces as a generic
	// "parse error" with a nil request_id (and a hung caller). In solo
	// mode there is exactly one workspace, so the all-zero nil UUID is
	// the canonical placeholder. Team mode plumbs a real workspace via
	// the HMAC token claims in a future iteration.
	if msg.WorkspaceID == "" {
		msg.WorkspaceID = defaultWorkspaceID
	}

	inboundEventID := h.recordInbound(msg)

	h.logger.InfoContext(ctx, "synthesizing plan",
		slog.String("event_id", inboundEventID),
		slog.String("connector_id", msg.ConnectorID),
		slog.String("tenant_id", msg.TenantID),
	)

	plan := planner.SynthesizePlan(msg, planner.Options{})

	// Bound the round-trip so a hung provider doesn't pin a goroutine.
	execCtx, cancel := context.WithTimeout(ctx, h.inboundTimeout)
	defer cancel()

	res, err := h.runtime.Execute(execCtx, msg.WorkspaceID, plan)
	if err != nil {
		return nil, fmt.Errorf("runtime execute: %w", err)
	}

	// Drain events into the logger but don't store them yet — Step 6
	// (persistent stores) folds them into the event timeline. For now we
	// just keep the channel from blocking the runtime reader.
	go drainEvents(ctx, h.logger, res.Events)

	select {
	case terminal, ok := <-res.Done:
		if !ok {
			return nil, errors.New("runtime closed before terminal response")
		}
		if terminal.Err != nil {
			return nil, terminal.Err
		}
		if terminal.Response == nil {
			return nil, errors.New("runtime returned empty response")
		}
		if terminal.Response.Status != ipc.StatusCompleted {
			return nil, fmt.Errorf("plan failed: %s", terminal.Response.Error)
		}
		reply := extractLlmText(terminal.Response.Output)
		h.recordOutbound(msg, reply)
		return &InboundResult{InboundEventID: inboundEventID, Reply: reply}, nil
	case <-execCtx.Done():
		return nil, execCtx.Err()
	}
}

// recordInbound writes an inbound audit event and returns its id. Silent
// best-effort: the inbound flow shouldn't fail if the store is offline.
func (h *Handler) recordInbound(msg planner.InboundMessage) string {
	payload, _ := json.Marshal(events.MessagePayload{
		ChannelID:   msg.ChannelID,
		ConnectorID: msg.ConnectorID,
		UserID:      msg.UserID,
		Content:     msg.Content,
		Direction:   "inbound",
	})
	evt := events.NewEvent(events.EventKindMessageInbound, msg.TenantID, "gateway", payload)
	evt.WorkspaceID = msg.WorkspaceID
	if h.eventStore != nil {
		h.eventStore.Append(evt)
	}
	return evt.ID
}

// recordOutbound writes an outbound audit event for the synthesized reply.
func (h *Handler) recordOutbound(msg planner.InboundMessage, reply string) {
	if reply == "" || h.eventStore == nil {
		return
	}
	payload, _ := json.Marshal(events.MessagePayload{
		ChannelID:   msg.ChannelID,
		ConnectorID: msg.ConnectorID,
		Content:     reply,
		Direction:   "outbound",
	})
	evt := events.NewEvent(events.EventKindMessageOutbound, msg.TenantID, "gateway", payload)
	evt.WorkspaceID = msg.WorkspaceID
	h.eventStore.Append(evt)
}

// extractLlmText pulls the "text" field out of the runtime's LlmCall output.
// The runtime emits {"text": "..."} for LlmCall nodes (runtime/runtimed/src/
// executor.rs); anything else returns empty string and the caller logs.
func extractLlmText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var shape struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		return ""
	}
	return shape.Text
}

// drainEvents reads streamed plan-execution events until the channel
// closes, logging each at debug. Step 6 will route them to the persistent
// event store and the timeline subscriber.
func drainEvents(ctx context.Context, logger *slog.Logger, ch <-chan ipc.EventNotification) {
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			logger.DebugContext(ctx, "runtime event",
				slog.String("request_id", evt.RequestID),
			)
		case <-ctx.Done():
			return
		}
	}
}

// OutboundMessageRequest is the request body for message egress.
type OutboundMessageRequest struct {
	ConnectorID string `json:"connector_id"`
	ChannelID   string `json:"channel_id"`
	RecipientID string `json:"recipient_id"`
	Content     string `json:"content"`
	TenantID    string `json:"tenant_id"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// MessageOutbound handles outgoing messages to external connectors.
func (h *Handler) MessageOutbound(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.message_outbound")
	defer span.End(h.logger)

	var req OutboundMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WarnContext(ctx, "invalid outbound message body",
			slog.String("error", err.Error()),
		)
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
		return
	}

	if req.TenantID == "" || req.ConnectorID == "" || req.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "tenant_id, connector_id, and content are required",
		})
		return
	}

	status, exists := h.connMgr.Status(req.ConnectorID)
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "connector not found",
		})
		return
	}
	if status.Health == connector.HealthDisconnected {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "connector is disconnected",
		})
		return
	}

	payload, _ := json.Marshal(events.MessagePayload{
		ChannelID:   req.ChannelID,
		ConnectorID: req.ConnectorID,
		Content:     req.Content,
		Direction:   "outbound",
	})
	evt := events.NewEvent(events.EventKindMessageOutbound, req.TenantID, "gateway", payload)
	evt.WorkspaceID = req.WorkspaceID

	h.logger.InfoContext(ctx, "message dispatched",
		slog.String("event_id", evt.ID),
		slog.String("connector_id", req.ConnectorID),
		slog.String("tenant_id", req.TenantID),
		slog.String("direction", "outbound"),
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"event_id": evt.ID,
		"status":   "dispatched",
	})
}

// ConnectorStatus returns the current health status of a connector.
func (h *Handler) ConnectorStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "connector id is required",
		})
		return
	}

	status, exists := h.connMgr.Status(id)
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "connector not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// ConnectorConnect registers or reconnects a connector.
func (h *Handler) ConnectorConnect(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.connector_connect")
	defer span.End(h.logger)

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "connector id is required",
		})
		return
	}

	h.connMgr.Connect(id)

	h.logger.InfoContext(ctx, "connector connected",
		slog.String("connector_id", id),
	)

	writeJSON(w, http.StatusOK, map[string]string{
		"connector_id": id,
		"status":       "connected",
	})
}

// ConnectorDisconnect gracefully disconnects a connector.
func (h *Handler) ConnectorDisconnect(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.connector_disconnect")
	defer span.End(h.logger)

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "connector id is required",
		})
		return
	}

	if err := h.connMgr.Disconnect(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
		return
	}

	h.logger.InfoContext(ctx, "connector disconnected",
		slog.String("connector_id", id),
	)

	writeJSON(w, http.StatusOK, map[string]string{
		"connector_id": id,
		"status":       "disconnected",
	})
}

// ConnectorHeartbeat processes a heartbeat from a connector.
func (h *Handler) ConnectorHeartbeat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "connector id is required",
		})
		return
	}

	if err := h.connMgr.RecordHeartbeat(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// writeJSON encodes v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
