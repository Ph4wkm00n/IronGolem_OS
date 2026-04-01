// Package handler implements the HTTP handlers for the Gateway service.
//
// Each handler propagates context for tracing and tenant isolation, uses
// structured logging, and produces events for the audit trail.
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/connector"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// Handler holds the dependencies for the gateway HTTP handlers.
type Handler struct {
	logger  *slog.Logger
	connMgr *connector.Manager
}

// New creates a Handler with the given logger and connector manager.
func New(logger *slog.Logger, connMgr *connector.Manager) *Handler {
	return &Handler{
		logger:  logger,
		connMgr: connMgr,
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

	if req.TenantID == "" || req.ConnectorID == "" || req.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "tenant_id, connector_id, and content are required",
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

	// Build the event for the audit trail.
	payload, _ := json.Marshal(events.MessagePayload{
		ChannelID:   req.ChannelID,
		ConnectorID: req.ConnectorID,
		UserID:      req.UserID,
		Content:     req.Content,
		Direction:   "inbound",
	})
	evt := events.NewEvent(events.EventKindMessageInbound, req.TenantID, "gateway", payload)
	evt.WorkspaceID = req.WorkspaceID

	h.logger.InfoContext(ctx, "message received",
		slog.String("event_id", evt.ID),
		slog.String("connector_id", req.ConnectorID),
		slog.String("tenant_id", req.TenantID),
		slog.String("direction", "inbound"),
	)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"event_id": evt.ID,
		"status":   "accepted",
	})
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
