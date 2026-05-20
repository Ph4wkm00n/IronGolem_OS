// audit_findings.go — v0.3 Step 5 of Plans/modular-puzzling-blum.md.
//
// HTTP surface for the audit probe subsystem
// (services/gateway/internal/audit). Exposes the most recent findings
// list to the v2 Audit page (which lands in Step 7). Read-only — the
// runtime ticker is the only writer.
//
// Distinct from this file's older sibling `audit.go` which exports the
// COMPLIANCE audit log (services/pkg/audit, the InMemoryStore). The
// two endpoints share a `/api/.../audit/...` URL prefix but serve
// orthogonal concerns:
//   - GET /api/v1/audit/export   — historical event log, CSV/JSON dump
//   - GET /api/v2/audit/findings — live probe findings, JSON list
//
// Naming the file `audit_findings.go` keeps grep-by-feature simple.

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
)

// AuditFindingsHandler serves GET /api/v2/audit/findings.
type AuditFindingsHandler struct {
	store  audit.FindingStore
	logger *slog.Logger
}

// NewAuditFindingsHandler wires the store + logger.
func NewAuditFindingsHandler(store audit.FindingStore, logger *slog.Logger) *AuditFindingsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuditFindingsHandler{
		store:  store,
		logger: logger.With(slog.String("component", "audit_findings_handler")),
	}
}

// ListFindings handles GET /api/v2/audit/findings.
// Query parameters:
//   - severity: "" | "info" | "warning" | "critical" — minimum
//     severity to return. "" includes everything.
//   - limit:    integer cap; 1..1000 valid; default 100.
func (h *AuditFindingsHandler) ListFindings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	sev, err := audit.ParseSeverity(q.Get("severity"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":  "invalid_severity",
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
				"detail": "limit must be an integer between 1 and 1000",
			})
			return
		}
		limit = parsed
	}

	findings, err := h.store.List(r.Context(), sev, limit)
	if err != nil {
		h.logger.Warn("audit findings list failed", slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":  "internal_error",
			"detail": err.Error(),
		})
		return
	}

	// findings may be nil when the table is empty; coerce to [] so the
	// frontend never has to special-case `null`.
	if findings == nil {
		findings = []audit.StoredFinding{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":    findings,
		"count":    len(findings),
		"severity": string(sev),
		"limit":    limit,
	})
}

// AuditFindingEmitter adapts the gateway's EventStore to the
// `audit.EventEmitter` interface. Lives here (alongside the handler
// that consumes the persisted findings) so the audit package stays
// independent of the events package — no import-cycle worries when
// Step 7's frontend lands.
type AuditFindingEmitter struct {
	store EventStore
}

// NewAuditFindingEmitter constructs the adapter.
func NewAuditFindingEmitter(store EventStore) *AuditFindingEmitter {
	return &AuditFindingEmitter{store: store}
}

// auditFindingPayload mirrors `AuditFindingPayload` on the TS side
// exactly (camelCase, optional evidence). Any change here MUST be made
// symmetrically in `packages/schema/src/events.ts` or the wire shape drifts.
type auditFindingPayload struct {
	FindingID string         `json:"findingId"`
	ProbeID   string         `json:"probeId"`
	Severity  string         `json:"severity"`
	Reason    string         `json:"reason"`
	Evidence  map[string]any `json:"evidence,omitempty"`
}

// EmitAuditFinding wraps the finding into an `events.Event` and appends
// it to the event store. Errors are returned (rather than logged
// silently) so the audit runtime can decide whether to surface them.
func (e *AuditFindingEmitter) EmitAuditFinding(_ context.Context, f audit.StoredFinding) error {
	if e.store == nil {
		return fmt.Errorf("audit emitter: nil event store")
	}
	payload := auditFindingPayload{
		FindingID: f.ID,
		ProbeID:   f.Finding.ProbeID,
		Severity:  string(f.Finding.Severity),
		Reason:    f.Finding.Reason,
		Evidence:  f.Finding.Evidence,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("audit emitter: marshal payload: %w", err)
	}
	// Tenant scoping for cross-tenant findings is a v0.4 concern;
	// today's probes are gateway-wide so we use the empty tenant_id +
	// "gateway-audit" source so downstream filters can identify them.
	evt := events.NewEvent(events.EventKindAuditFinding, "", "gateway-audit", raw)
	evt.Metadata = map[string]string{
		"probe_id": f.Finding.ProbeID,
		"severity": string(f.Finding.Severity),
	}
	e.store.Append(evt)
	return nil
}

// Compile-time check: the emitter satisfies audit.EventEmitter.
var _ audit.EventEmitter = (*AuditFindingEmitter)(nil)

