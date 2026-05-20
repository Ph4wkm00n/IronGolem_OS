// commitments.go — v0.3 Step 4 of Plans/modular-puzzling-blum.md.
//
// HTTP surface for the commitments subsystem
// (services/gateway/internal/commitments). The handler stays thin —
// validation + routing only; business logic lives in the package
// proper.
//
// Routes (registered in services/gateway/cmd/main.go):
//
//   GET    /api/v2/commitments
//   POST   /api/v2/commitments/{id}/dismiss
//   POST   /api/v2/commitments/{id}/snooze
//   DELETE /api/v2/commitments/{id}                (admin only)

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/commitments"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/connector"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
)

// CommitmentsHandler serves the /api/v2/commitments family.
type CommitmentsHandler struct {
	store  commitments.Store
	logger *slog.Logger
}

// NewCommitmentsHandler wires the store + logger.
func NewCommitmentsHandler(store commitments.Store, logger *slog.Logger) *CommitmentsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &CommitmentsHandler{
		store:  store,
		logger: logger.With(slog.String("component", "commitments_handler")),
	}
}

// ListCommitments handles GET /api/v2/commitments.
// Query params:
//   - workspace_id: scope filter (defaults to all when empty)
//   - status:       pending | sent | dismissed | snoozed | expired | "" (all)
//   - limit:        1..1000, default 100
func (h *CommitmentsHandler) ListCommitments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status, err := commitments.ParseStatus(q.Get("status"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":  "invalid_status",
			"detail": err.Error(),
		})
		return
	}
	limit := 100
	if raw := q.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 1000 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":  "invalid_limit",
				"detail": "limit must be 1..1000",
			})
			return
		}
		limit = parsed
	}
	filter := commitments.ListFilter{
		WorkspaceID: q.Get("workspace_id"),
		Status:      status,
		Limit:       limit,
	}
	items, err := h.store.List(r.Context(), filter)
	if err != nil {
		h.logger.Warn("commitments list failed", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":  "internal_error",
			"detail": err.Error(),
		})
		return
	}
	if items == nil {
		items = []commitments.Commitment{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"count":  len(items),
		"status": string(status),
		"limit":  limit,
	})
}

// DismissCommitment handles POST /api/v2/commitments/{id}/dismiss.
func (h *CommitmentsHandler) DismissCommitment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id_required"})
		return
	}
	if err := h.store.MarkDismissed(r.Context(), id); err != nil {
		h.respondMarkError(w, "dismiss", id, err)
		return
	}
	h.respondMarkOK(w, r.Context(), id)
}

// SnoozeCommitment handles POST /api/v2/commitments/{id}/snooze.
// Body: { "until_ms": int64 }
func (h *CommitmentsHandler) SnoozeCommitment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id_required"})
		return
	}
	var body struct {
		UntilMs int64 `json:"until_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":  "invalid_body",
			"detail": err.Error(),
		})
		return
	}
	if body.UntilMs <= time.Now().UTC().UnixMilli() {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":  "invalid_until_ms",
			"detail": "until_ms must be in the future",
		})
		return
	}
	if err := h.store.MarkSnoozed(r.Context(), id, body.UntilMs); err != nil {
		h.respondMarkError(w, "snooze", id, err)
		return
	}
	h.respondMarkOK(w, r.Context(), id)
}

// DeleteCommitment handles DELETE /api/v2/commitments/{id}. Admin only —
// the policy middleware gates the route.
func (h *CommitmentsHandler) DeleteCommitment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id_required"})
		return
	}
	if err := h.store.Delete(r.Context(), id); err != nil {
		h.respondMarkError(w, "delete", id, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

func (h *CommitmentsHandler) respondMarkError(w http.ResponseWriter, op, id string, err error) {
	if errors.Is(err, commitments.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error":  "not_found",
			"detail": fmt.Sprintf("no commitment with id %s", id),
		})
		return
	}
	h.logger.Warn("commitment "+op+" failed", slog.String("id", id), slog.String("error", err.Error()))
	writeJSON(w, http.StatusInternalServerError, map[string]string{
		"error":  "internal_error",
		"detail": err.Error(),
	})
}

func (h *CommitmentsHandler) respondMarkOK(w http.ResponseWriter, ctx context.Context, id string) {
	c, err := h.store.Get(ctx, id)
	if err != nil {
		// Body succeeded but the read-back failed — still 200, with a
		// minimal payload. The frontend re-fetches the list on success.
		writeJSON(w, http.StatusOK, map[string]string{"id": id})
		return
	}
	writeJSON(w, http.StatusOK, c)
}

// CommitmentEventEmitter adapts the event store to
// `commitments.EventEmitter`. Mirrors the audit emitter pattern from
// audit_findings.go so the commitments package stays event-store-free.
type CommitmentEventEmitter struct {
	store EventStore
}

// NewCommitmentEventEmitter constructs the adapter.
func NewCommitmentEventEmitter(store EventStore) *CommitmentEventEmitter {
	return &CommitmentEventEmitter{store: store}
}

// commitmentEventPayload is the wire shape emitted alongside lifecycle
// events. Mirrors the v2 frontend's expected payload shape — keep in
// sync with the schema-side type when it lands in Step 7.
type commitmentEventPayload struct {
	CommitmentID  string                              `json:"commitmentId"`
	Kind          commitments.CommitmentKind          `json:"kind"`
	Sensitivity   commitments.CommitmentSensitivity   `json:"sensitivity"`
	Status        commitments.CommitmentStatus        `json:"status"`
	Reason        string                              `json:"reason"`
	SuggestedText string                              `json:"suggestedText"`
	EarliestMs    int64                               `json:"earliestMs"`
	LatestMs      int64                               `json:"latestMs"`
}

func (e *CommitmentEventEmitter) emit(kind events.EventKind, c commitments.Commitment) error {
	if e.store == nil {
		return fmt.Errorf("commitment emitter: nil event store")
	}
	payload := commitmentEventPayload{
		CommitmentID:  c.ID,
		Kind:          c.Kind,
		Sensitivity:   c.Sensitivity,
		Status:        c.Status,
		Reason:        c.Reason,
		SuggestedText: c.SuggestedText,
		EarliestMs:    c.DueWindow.EarliestMs,
		LatestMs:      c.DueWindow.LatestMs,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("commitment emit: marshal: %w", err)
	}
	evt := events.NewEvent(kind, c.TenantID, "gateway-commitments", raw)
	evt.WorkspaceID = c.WorkspaceID
	evt.Metadata = map[string]string{"commitment_id": c.ID, "kind": string(c.Kind)}
	e.store.Append(evt)
	return nil
}

func (e *CommitmentEventEmitter) EmitCommitmentExtracted(_ context.Context, c commitments.Commitment) error {
	return e.emit(events.EventKindCommitmentExtracted, c)
}
func (e *CommitmentEventEmitter) EmitCommitmentFired(_ context.Context, c commitments.Commitment) error {
	return e.emit(events.EventKindCommitmentFired, c)
}
func (e *CommitmentEventEmitter) EmitCommitmentDismissed(_ context.Context, c commitments.Commitment) error {
	return e.emit(events.EventKindCommitmentDismissed, c)
}
func (e *CommitmentEventEmitter) EmitCommitmentSnoozed(_ context.Context, c commitments.Commitment) error {
	return e.emit(events.EventKindCommitmentSnoozed, c)
}
func (e *CommitmentEventEmitter) EmitCommitmentExpired(_ context.Context, c commitments.Commitment) error {
	return e.emit(events.EventKindCommitmentExpired, c)
}

var _ commitments.EventEmitter = (*CommitmentEventEmitter)(nil)

// CommitmentDispatcher delivers a fired commitment by sending an
// outbound message through the gateway's connector manager.
type CommitmentDispatcher struct {
	connMgr *connector.Manager
	logger  *slog.Logger
}

// NewCommitmentDispatcher constructs the adapter.
func NewCommitmentDispatcher(connMgr *connector.Manager, logger *slog.Logger) *CommitmentDispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &CommitmentDispatcher{connMgr: connMgr, logger: logger}
}

// Dispatch sends the commitment's suggested text via the originating
// connector. If the connector is no longer registered, the dispatch
// returns an error so the runtime retries (or eventually expires).
func (d *CommitmentDispatcher) Dispatch(ctx context.Context, c commitments.Commitment) error {
	if c.ConnectorID == "" || c.ChannelID == "" {
		// Without a routing target, we can't deliver. Surface as a
		// terminal failure so the row expires rather than spinning.
		return fmt.Errorf("commitment %s missing connector/channel routing", c.ID)
	}
	text := c.SuggestedText
	if text == "" {
		text = c.Reason
	}
	if err := d.connMgr.SendOutbound(ctx, c.ConnectorID, c.ChannelID, text); err != nil {
		return fmt.Errorf("connector send: %w", err)
	}
	return nil
}

var _ commitments.Dispatcher = (*CommitmentDispatcher)(nil)
