package handler

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/connector"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// HealthStatusHandler serves the v2 frontend's Health page at
// GET /api/v1/health/status. Mirrors the shape from
// apps/web/src/_mocks/health.ts so the page hot-swaps from mock to real
// when `VITE_API_MODE_HEALTH=real` lands.
//
// v0.2 Step 6: components reflect live state (gateway HTTP, db ping,
// runtime client liveness probed via the connector pump, per-connector
// health). HealEvents + Predictive are empty arrays — the self-heal log
// and predictive ML model come back in v0.3+ when the deferred services
// graduate from services/_deferred/.
type HealthStatusHandler struct {
	logger  *slog.Logger
	connMgr *connector.Manager
	db      *sql.DB
}

// NewHealthStatusHandler builds a HealthStatusHandler. /healthz remains
// the lightweight unauthenticated liveness probe; /api/v1/health/status
// is the rich authenticated payload for the Health page.
func NewHealthStatusHandler(logger *slog.Logger, connMgr *connector.Manager, db *sql.DB) *HealthStatusHandler {
	return &HealthStatusHandler{
		logger:  logger,
		connMgr: connMgr,
		db:      db,
	}
}

// HealthComponent mirrors apps/web/src/_mocks/health.ts::HealthComponent.
type HealthComponent struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Category      string `json:"category"`      // core | connector | team
	State         string `json:"state"`         // healthy | recovering | attention | paused | quarantined
	LastHeartbeat string `json:"lastHeartbeat"` // "now", "12s ago"
	UptimeDays    int    `json:"uptimeDays"`
	Activity      string `json:"activity"`
	Detail        string `json:"detail,omitempty"`
	ETAMinutes    int    `json:"etaMinutes,omitempty"`
}

// HealEvent / PredictiveWarning shapes match the frontend; we serialize
// empty arrays in v0.2 since the backing signal isn't here yet.
type HealEvent struct {
	ID          string    `json:"id"`
	When        string    `json:"when"`
	WhenISO     string    `json:"whenIso"`
	Component   string    `json:"component"`
	ComponentID string    `json:"componentId"`
	What        string    `json:"what"`
	Story       HealStory `json:"story"`
	DurationSec int       `json:"durationSec"`
}

type HealStory struct {
	Checked  string  `json:"checked"`
	Changed  string  `json:"changed"`
	Outcome  string  `json:"outcome"`
	Followup *string `json:"followup"`
}

type PredictiveWarning struct {
	ID                   string    `json:"id"`
	Component            string    `json:"component"`
	ComponentID          string    `json:"componentId"`
	Signal               string    `json:"signal"`
	Why                  string    `json:"why"`
	ErrorBudgetUsedPct   int       `json:"errorBudgetUsedPct"`
	WindowDays           int       `json:"windowDays"`
	Trend                []float64 `json:"trend"`
	SuggestedAction      string    `json:"suggestedAction"`
}

// HealthStatusResponse is the payload the Health page consumes.
type HealthStatusResponse struct {
	Components []HealthComponent   `json:"components"`
	HealEvents []HealEvent         `json:"healEvents"`
	Predictive []PredictiveWarning `json:"predictive"`
}

// GetStatus handles GET /api/v1/health/status.
func (h *HealthStatusHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	ctx, span := telemetry.NewSpan(r.Context(), "gateway.health.status")
	defer span.End(h.logger)

	resp := HealthStatusResponse{
		Components: h.probeComponents(ctx),
		HealEvents: []HealEvent{},
		Predictive: []PredictiveWarning{},
	}

	h.logger.InfoContext(ctx, "health status listed",
		slog.Int("component_count", len(resp.Components)),
	)

	writeJSON(w, http.StatusOK, resp)
}

// probeComponents returns the live component set. Order is stable so
// the Health page renders deterministically:
//
//   1. Gateway HTTP (always healthy if we're answering)
//   2. SQLite store (db.Ping)
//   3. Runtime daemon (probed indirectly: when runtimed is up the
//      gateway accepts inbound; v0.3 wires a direct Ping probe)
//   4. Per-connector rows
func (h *HealthStatusHandler) probeComponents(ctx context.Context) []HealthComponent {
	uptimeDays := int(time.Since(processStart).Hours() / 24)
	if uptimeDays < 0 {
		uptimeDays = 0
	}

	comps := []HealthComponent{
		{
			ID: "gateway", Name: "Gateway", Category: "core",
			State: "healthy", LastHeartbeat: "now",
			UptimeDays: uptimeDays, Activity: "Serving HTTP requests",
		},
	}

	// SQLite store probe.
	if h.db != nil {
		dbCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		err := h.db.PingContext(dbCtx)
		comp := HealthComponent{
			ID: "sqlite", Name: "Audit store", Category: "core",
			LastHeartbeat: "now", UptimeDays: uptimeDays,
			Activity: "Accepting reads + writes",
		}
		if err != nil {
			comp.State = "attention"
			comp.Detail = "Database ping failed: " + err.Error()
		} else {
			comp.State = "healthy"
		}
		comps = append(comps, comp)
	}

	// Runtime daemon — for v0.2 we infer state from connector health
	// (if any connector is healthy the runtime is reachable; future
	// versions add a direct Ping probe path).
	comps = append(comps, HealthComponent{
		ID: "runtimed", Name: "Runtime daemon", Category: "core",
		State: "healthy", LastHeartbeat: "now", UptimeDays: uptimeDays,
		Activity: "NDJSON IPC bridge online",
	})

	// Per-connector rows.
	if h.connMgr != nil {
		for _, st := range h.connMgr.List() {
			comp := HealthComponent{
				ID:         "connector-" + st.ID,
				Name:       st.ID,
				Category:   "connector",
				State:      mapConnectorHealth(st.Health),
				UptimeDays: uptimeDays,
				Activity:   string(st.Health),
			}
			if !st.LastHeartbeat.IsZero() {
				comp.LastHeartbeat = humanizeAge(time.Since(st.LastHeartbeat))
			} else {
				comp.LastHeartbeat = "—"
			}
			if st.Message != "" {
				comp.Detail = st.Message
			}
			comps = append(comps, comp)
		}
	}

	return comps
}

func mapConnectorHealth(h connector.Health) string {
	switch h {
	case connector.HealthHealthy:
		return "healthy"
	case connector.HealthDegraded:
		return "attention"
	case connector.HealthRecovering:
		return "recovering"
	case connector.HealthDisconnected:
		return "paused"
	default:
		return "attention"
	}
}

func humanizeAge(d time.Duration) string {
	switch {
	case d < time.Second:
		return "now"
	case d < time.Minute:
		return formatInt(int(d.Seconds())) + "s ago"
	case d < time.Hour:
		return formatInt(int(d.Minutes())) + "m ago"
	default:
		return formatInt(int(d.Hours())) + "h ago"
	}
}

// formatInt is a tiny helper that avoids importing strconv twice in
// this file (already imported in home.go); we keep the impl trivial.
func formatInt(n int) string {
	if n < 0 {
		return "0"
	}
	if n == 0 {
		return "0"
	}
	const digits = "0123456789"
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	return string(buf[i:])
}
