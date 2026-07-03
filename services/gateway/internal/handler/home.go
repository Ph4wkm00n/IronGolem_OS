package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/connector"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/middleware"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// HomeHandler serves the v2 frontend's dashboard at GET /api/v1/home.
//
// v0.2 Step 6 — the dashboard's data is a blend of real signals (heartbeat
// probes, event timeline from the SQLite store, workspace metadata) and
// structurally-correct stubs for fields whose backend services were
// parked under services/_deferred/ in v0.1 Step 7 (teams, trust history,
// research findings, full safety state). The frontend already renders
// against the mock shape; this endpoint returns the same shape so the
// page hot-swaps from mock to real on `VITE_API_MODE_HOME=real` without
// layout flicker.
//
// As deferred services come back online (v0.3+), the stub fields here
// get replaced with their real signals one at a time.
type HomeHandler struct {
	logger     *slog.Logger
	eventStore EventStore
	connMgr    *connector.Manager
	db         *sql.DB
}

// NewHomeHandler builds a HomeHandler. The runtime + connector handles
// flow in so the heartbeat probe can ask "is the runtime up?" without
// importing the runtime client into this package.
func NewHomeHandler(logger *slog.Logger, eventStore EventStore, connMgr *connector.Manager, db *sql.DB) *HomeHandler {
	return &HomeHandler{
		logger:     logger,
		eventStore: eventStore,
		connMgr:    connMgr,
		db:         db,
	}
}

// HomeWorkspaceInfo mirrors apps/web/src/_mocks/home.ts::WorkspaceInfo.
type HomeWorkspaceInfo struct {
	Name         string `json:"name"`
	Initials     string `json:"initials"`
	Region       string `json:"region"`
	UptimeHours  int    `json:"uptimeHours"`
	UptimeStreak string `json:"uptimeStreak"`
	LastSync     string `json:"lastSync"`
}

// HomeHeartbeat mirrors HeartbeatState.
type HomeHeartbeat struct {
	Status       string          `json:"status"` // healthy | degraded | down
	SystemsGreen int             `json:"systemsGreen"`
	SystemsTotal int             `json:"systemsTotal"`
	OneDegraded  HomeOneDegraded `json:"oneDegraded"`
}

type HomeOneDegraded struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// HomeTeam mirrors Team.
type HomeTeam struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Members     int    `json:"members"`
	Description string `json:"description"`
}

// HomeResearchFinding mirrors ResearchFinding.
type HomeResearchFinding struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
	Freshness  string  `json:"freshness"`
	Summary    string  `json:"summary"`
}

// HomeSafetyLayer mirrors SafetyLayer.
type HomeSafetyLayer struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
	Note  string `json:"note"`
}

// HomeSafety mirrors SafetyShape on the Home page.
type HomeSafety struct {
	Posture       string            `json:"posture"`
	Layers        []HomeSafetyLayer `json:"layers"`
	Can           []string          `json:"can"`
	Cannot        []string          `json:"cannot"`
	NeedsApproval []string          `json:"needsApproval"`
	StopsIf       []string          `json:"stopsIf"`
}

// HomeEventItem mirrors EventItem.
type HomeEventItem struct {
	ID              string `json:"id"`
	Status          string `json:"status"`
	Title           string `json:"title"`
	TeamID          string `json:"teamId"`
	Permission      string `json:"permission"`
	PermissionScope string `json:"permissionScope"`
	Risk            string `json:"risk"`
	MinutesAgo      int    `json:"minutesAgo"`
	Why             string `json:"why"`
	Cause           string `json:"cause,omitempty"`
	Target          string `json:"target,omitempty"`
	Approvals       int    `json:"approvals,omitempty"`
}

// HomeResponse is the full payload the v2 Home page consumes.
type HomeResponse struct {
	Workspace        HomeWorkspaceInfo     `json:"workspace"`
	Heartbeat        HomeHeartbeat         `json:"heartbeat"`
	Teams            []HomeTeam            `json:"teams"`
	TrustHistory     map[string][]int      `json:"trustHistory"`
	Safety           HomeSafety            `json:"safety"`
	ResearchFindings []HomeResearchFinding `json:"researchFindings"`
	Events           []HomeEventItem       `json:"events"`
}

// ListHome handles GET /api/v1/home. Returns the dashboard payload for
// the authenticated tenant + workspace. Real signals: workspace,
// heartbeat probe, events from the SQLite store. Stub signals:
// teams, trust history, research findings, full safety state.
func (h *HomeHandler) ListHome(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.home.list")
	defer span.End(h.logger)

	tenantID := middleware.TenantIDFromContext(ctx)
	workspaceID := middleware.WorkspaceIDFromContext(ctx)

	resp := HomeResponse{
		Workspace:        h.probeWorkspace(ctx),
		Heartbeat:        h.probeHeartbeat(ctx),
		Teams:            stubTeams(),
		TrustHistory:     stubTrustHistory(),
		Safety:           stubSafety(),
		ResearchFindings: stubResearchFindings(),
		Events:           h.recentEvents(tenantID, workspaceID),
	}

	h.logger.InfoContext(ctx, "home listed",
		slog.String("tenant_id", tenantID),
		slog.String("workspace_id", workspaceID),
		slog.Int("event_count", len(resp.Events)),
		slog.Int("systems_green", resp.Heartbeat.SystemsGreen),
		slog.Int("systems_total", resp.Heartbeat.SystemsTotal),
	)

	writeJSON(w, http.StatusOK, resp)
}

// probeWorkspace returns workspace metadata. v0.2: most of it is env-
// or static-derived; v0.3 reads from a workspaces table once provisioning
// tooling exists.
func (h *HomeHandler) probeWorkspace(_ context.Context) HomeWorkspaceInfo {
	return HomeWorkspaceInfo{
		Name:         workspaceLabel(),
		Initials:     "WS",
		Region:       "solo-local",
		UptimeHours:  int(time.Since(processStart).Hours()),
		UptimeStreak: formatUptime(time.Since(processStart)),
		LastSync:     "just now",
	}
}

// probeHeartbeat synthesizes a live status from the gateway's own
// dependencies. Counted as systems: gateway HTTP server (we're answering
// the request → always green), database (Ping), runtime (db reachable
// proxies for now — runtime is exercised via separate Step 4 work).
func (h *HomeHandler) probeHeartbeat(ctx context.Context) HomeHeartbeat {
	systems := []struct {
		name string
		ok   bool
		why  string
	}{
		{"gateway HTTP", true, ""},
	}

	// DB probe.
	if h.db != nil {
		dbCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		err := h.db.PingContext(dbCtx)
		why := ""
		if err != nil {
			why = err.Error()
		}
		systems = append(systems, struct {
			name string
			ok   bool
			why  string
		}{"SQLite store", err == nil, why})
	}

	// Connector count — any registered connector is a "system" we report.
	if h.connMgr != nil {
		for _, st := range h.connMgr.List() {
			systems = append(systems, struct {
				name string
				ok   bool
				why  string
			}{"connector " + st.ID, st.Health != connector.HealthDisconnected, ""})
		}
	}

	total := len(systems)
	green := 0
	var degraded HomeOneDegraded
	for _, s := range systems {
		if s.ok {
			green++
			continue
		}
		if degraded.Name == "" {
			degraded.Name = s.name
			degraded.Reason = s.why
			if degraded.Reason == "" {
				degraded.Reason = "system reported degraded state"
			}
		}
	}
	status := "healthy"
	if green < total {
		status = "degraded"
	}
	if green == 0 {
		status = "down"
	}

	return HomeHeartbeat{
		Status:       status,
		SystemsGreen: green,
		SystemsTotal: total,
		OneDegraded:  degraded,
	}
}

// recentEvents pulls the last N events from the store and maps them to
// HomeEventItem shape. Inbound → "proposed", outbound → "taken",
// PolicyDenied → "blocked".
func (h *HomeHandler) recentEvents(tenantID, workspaceID string) []HomeEventItem {
	if h.eventStore == nil {
		return []HomeEventItem{}
	}
	raw, _ := h.eventStore.List(1, 20, workspaceID, "")
	now := time.Now().UTC()
	items := make([]HomeEventItem, 0, len(raw))
	for _, evt := range raw {
		if tenantID != "" && evt.TenantID != tenantID {
			continue
		}
		items = append(items, mapHomeEvent(evt, now))
	}
	return items
}

func mapHomeEvent(evt events.Event, now time.Time) HomeEventItem {
	var payload inboundEventPayload
	if len(evt.Payload) > 0 {
		_ = json.Unmarshal(evt.Payload, &payload)
	}
	status := "taken"
	switch evt.Kind {
	case events.EventKindMessageInbound:
		status = "proposed"
	case events.EventKindMessageOutbound:
		status = "taken"
	case events.EventKindPolicyDenied:
		status = "blocked"
	}
	minutes := int(now.Sub(evt.Timestamp).Minutes())
	if minutes < 0 {
		minutes = 0
	}

	title := payload.Content
	if title == "" {
		title = string(evt.Kind)
	}
	title = truncate(title, 120)

	return HomeEventItem{
		ID:              evt.ID,
		Status:          status,
		Title:           title,
		TeamID:          "inbox-triage",
		Permission:      string(evt.Kind),
		PermissionScope: "scoped",
		Risk:            "low",
		MinutesAgo:      minutes,
		Why:             "Recorded by the gateway audit trail.",
		Target:          payload.UserID,
	}
}

// processStart is captured at package init so the uptime probe can derive
// "hours since gateway started" without per-request bookkeeping.
var processStart = time.Now()

func workspaceLabel() string {
	if v := os.Getenv("IRONGOLEM_WORKSPACE_LABEL"); v != "" {
		return v
	}
	return "IronGolem Workspace"
}

func formatUptime(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days <= 0 {
		hours := int(d.Hours())
		return strconv.Itoa(hours) + " hours"
	}
	if days == 1 {
		return "1 day"
	}
	return strconv.Itoa(days) + " days"
}

// stubTeams etc. provide structurally-correct placeholders for fields
// whose backing services are parked under services/_deferred/. They
// match the mock shape verbatim so the frontend renders identically on
// the real-API path.
func stubTeams() []HomeTeam {
	return []HomeTeam{
		{ID: "inbox-triage", Name: "Inbox triage", Color: "accent", Members: 1, Description: "Sorts mail, drafts replies, escalates the rest."},
	}
}

func stubTrustHistory() map[string][]int {
	full := []int{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9}
	return map[string][]int{"inbox-triage": full}
}

func stubSafety() HomeSafety {
	return HomeSafety{
		Posture: "active",
		Layers: []HomeSafetyLayer{
			{ID: 1, Name: "Identity", State: "ok", Note: "HMAC token verified"},
			{ID: 2, Name: "Workspace", State: "ok", Note: "Solo mode"},
			{ID: 3, Name: "Team", State: "ok", Note: "Single team"},
			{ID: 4, Name: "Action", State: "ok", Note: "Layer-4 store consulted"},
			{ID: 5, Name: "Outcome", State: "ok", Note: "Audit trail recorded"},
		},
		Can:           []string{"Receive inbound messages", "Synthesize replies through runtimed"},
		Cannot:        []string{"Send replies without your approval", "Forward to unverified channels"},
		NeedsApproval: []string{"Any outbound message", "Any tool call"},
		StopsIf:       []string{"HMAC verification fails", "Runtime client unreachable"},
	}
}

func stubResearchFindings() []HomeResearchFinding {
	return []HomeResearchFinding{}
}
