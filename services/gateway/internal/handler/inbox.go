package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/middleware"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// InboxHandler serves the v2 frontend's GET /api/v1/inbox endpoint.
// It reads `message.inbound` events from the audit trail and shapes
// them to the wire contract the v2 Inbox page expects (matching
// apps/web/src/_mocks/inbox.ts::Item).
//
// v0.2 Step 3 of Plans/v0.2-foundation.md: this is the first live
// frontend↔backend integration. Mappings are deliberately deterministic
// so the wire shape is reproducible:
//
//   - status   — always "awaiting" (v0.2 has no draft/held/done lifecycle yet)
//   - source   — derived from event payload `connector_id` prefix
//   - risk     — always "low" (no risk classifier in v0.2)
//   - routedBy — always "Inbox triage" (single ingress path)
//   - draft    — omitted (drafts arrive in v0.3 when Drafting team lands)
//   - safety / audit — minimal defaults so the frontend renders cleanly
//
// The richer mock shape lives in the frontend; v0.2 supplies the subset
// the page needs to render rows. Subsequent steps will progressively fill
// in the gaps as backend behavior grows.
type InboxHandler struct {
	logger     *slog.Logger
	eventStore EventStore
}

// NewInboxHandler creates an InboxHandler.
func NewInboxHandler(logger *slog.Logger, eventStore EventStore) *InboxHandler {
	return &InboxHandler{
		logger:     logger,
		eventStore: eventStore,
	}
}

// InboxItem is the wire shape the v2 Inbox page consumes. Mirrors
// apps/web/src/_mocks/inbox.ts::Item field-for-field with JSON tags
// matching the frontend type. Optional / structured fields use
// json:",omitempty" so the wire stays compact when nothing meaningful
// is populated yet.
type InboxItem struct {
	ID         string          `json:"id"`
	Status     string          `json:"status"`
	Title      string          `json:"title"`
	Source     string          `json:"source"`
	Risk       string          `json:"risk"`
	MinutesAgo int             `json:"minutesAgo"`
	Summary    string          `json:"summary"`
	Cause      string          `json:"cause"`
	RoutedBy   string          `json:"routedBy"`
	Unread     bool            `json:"unread"`
	Safety     inboxSafety     `json:"safety"`
	Audit      []inboxAudit    `json:"audit"`
}

type inboxSafety struct {
	Can            []string `json:"can"`
	Cannot         []string `json:"cannot"`
	NeedsApproval  []string `json:"needsApproval"`
	StopsIf        []string `json:"stopsIf"`
}

type inboxAudit struct {
	At    string `json:"at"`
	Actor string `json:"actor"`
	Note  string `json:"note"`
}

// inboundEventPayload mirrors events.MessagePayload but is local so we
// can decode without coupling to the events package's serialization
// quirks (the wire `direction` field, etc.).
type inboundEventPayload struct {
	ChannelID   string `json:"channel_id"`
	ConnectorID string `json:"connector_id"`
	UserID      string `json:"user_id"`
	Content     string `json:"content"`
	Direction   string `json:"direction"`
}

// ListInbox handles GET /api/v1/inbox. Returns the authenticated
// tenant + workspace's inbound messages newest-first, paginated.
//
// Query parameters:
//
//	page      (default 1)
//	page_size (default 50, max 200)
//
// The endpoint reads tenant + workspace from the HMAC token claims
// (Step 2). It scans the SQLite event store for `message.inbound`
// events scoped to that pair and shapes each into the InboxItem wire
// contract.
func (h *InboxHandler) ListInbox(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.inbox.list")
	defer span.End(h.logger)

	tenantID := middleware.TenantIDFromContext(ctx)
	workspaceID := middleware.WorkspaceIDFromContext(ctx)

	page, pageSize := parsePagination(r)
	if pageSize > 200 {
		pageSize = 200
	}

	// The event store filters by workspace when supplied. We page in raw
	// rows + then filter by tenant + kind in code; v0.2's event volumes
	// are small enough that this is fine, and avoids re-shaping the store
	// API. Step 4 (channel-policy store) will add tenant indexes that
	// open the door to push-down filtering.
	all, _ := h.eventStore.List(1, pageSize*4, workspaceID, string(events.EventKindMessageInbound))

	items := make([]InboxItem, 0, len(all))
	now := time.Now().UTC()
	for _, evt := range all {
		if tenantID != "" && evt.TenantID != tenantID {
			continue
		}
		items = append(items, shapeInboxItem(evt, now))
	}

	// Manual pagination after filter (see note above re: push-down).
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	page = max(page, 1)
	pageItems := items[start:end]

	h.logger.InfoContext(ctx, "inbox listed",
		slog.Int("page", page),
		slog.Int("page_size", pageSize),
		slog.Int("total", total),
		slog.String("tenant_id", tenantID),
		slog.String("workspace_id", workspaceID),
	)

	writeJSON(w, http.StatusOK, map[string]any{
		"items":     pageItems,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// shapeInboxItem maps a stored message.inbound event into the wire shape
// the v2 Inbox page consumes. The mapping is deterministic so the same
// event always produces the same item — important for tests + visual
// baselines.
func shapeInboxItem(evt events.Event, now time.Time) InboxItem {
	var payload inboundEventPayload
	if len(evt.Payload) > 0 {
		_ = json.Unmarshal(evt.Payload, &payload)
	}

	minutesAgo := int(now.Sub(evt.Timestamp).Minutes())
	if minutesAgo < 0 {
		minutesAgo = 0
	}

	return InboxItem{
		ID:         evt.ID,
		Status:     "awaiting",
		Title:      inboxTitle(payload),
		Source:     inboxSource(payload.ConnectorID),
		Risk:       "low",
		MinutesAgo: minutesAgo,
		Summary:    truncate(payload.Content, 160),
		Cause:      "User sent a message via " + inboxSource(payload.ConnectorID) + ".",
		RoutedBy:   "Inbox triage",
		Unread:     true,
		Safety: inboxSafety{
			Can:           []string{"Read and triage the inbound message"},
			Cannot:        []string{"Send a reply without your approval"},
			NeedsApproval: []string{"Any outbound send"},
			StopsIf:       []string{"Sender is on the workspace deny-list"},
		},
		Audit: []inboxAudit{
			{
				At:    evt.Timestamp.UTC().Format(time.RFC3339),
				Actor: "Inbox triage",
				Note:  "Received inbound message.",
			},
		},
	}
}

// inboxSource maps the connector_id stamped on the event payload into one
// of the four Source values the frontend expects. Unknown connectors
// fall back to "webhook" as the generic ingress label.
func inboxSource(connectorID string) string {
	switch {
	case connectorID == "":
		return "webhook"
	case startsWith(connectorID, "telegram"):
		return "telegram"
	case startsWith(connectorID, "email"):
		return "email"
	case startsWith(connectorID, "calendar"):
		return "calendar"
	default:
		return "webhook"
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func inboxTitle(p inboundEventPayload) string {
	if p.UserID != "" {
		return "Inbound from " + p.UserID
	}
	if p.ChannelID != "" {
		return "Inbound on " + p.ChannelID
	}
	return "Inbound message"
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
