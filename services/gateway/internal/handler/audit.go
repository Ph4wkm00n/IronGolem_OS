// Package handler implements audit export HTTP handlers for the Gateway service.
package handler

import (
	"net/http"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/audit"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// AuditHandler holds the dependencies for audit export endpoints.
type AuditHandler struct {
	store  *audit.InMemoryStore
	logger interface{ Info(string, ...any) }
}

// NewAuditHandler creates an AuditHandler with the given store.
func NewAuditHandler(store *audit.InMemoryStore) *AuditHandler {
	return &AuditHandler{
		store: store,
	}
}

// ExportAudit handles GET /api/v1/audit/export.
// Query parameters:
//   - format: "json" (default) or "csv"
//   - from: RFC3339 start time
//   - to: RFC3339 end time
//   - workspace_id: filter by workspace
//   - user_id: filter by actor
//   - severity: minimum risk level
func (h *AuditHandler) ExportAudit(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.NewSpan(r.Context(), "gateway.audit_export")
	defer span.End(nil)

	query := r.URL.Query()

	formatStr := query.Get("format")
	if formatStr == "" {
		formatStr = "json"
	}

	var format audit.ExportFormat
	switch formatStr {
	case "json":
		format = audit.FormatJSON
	case "csv":
		format = audit.FormatCSV
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "unsupported format; use 'json' or 'csv'",
		})
		return
	}

	filter := audit.AuditFilter{}

	if fromStr := query.Get("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid 'from' time; use RFC3339 format",
			})
			return
		}
		filter.From = &t
	}

	if toStr := query.Get("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid 'to' time; use RFC3339 format",
			})
			return
		}
		filter.To = &t
	}

	if wsID := query.Get("workspace_id"); wsID != "" {
		filter.WorkspaceIDs = []string{wsID}
	}

	if userID := query.Get("user_id"); userID != "" {
		filter.UserIDs = []string{userID}
	}

	if sev := query.Get("severity"); sev != "" {
		filter.Severity = audit.RiskLevel(sev)
	}

	data, err := h.store.Export(filter, format)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	switch format {
	case audit.FormatCSV:
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=audit-export.csv")
	default:
		w.Header().Set("Content-Type", "application/json")
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ComplianceReport handles GET /api/v1/audit/compliance.
// Query parameters:
//   - from: RFC3339 start time (defaults to 30 days ago)
//   - to: RFC3339 end time (defaults to now)
func (h *AuditHandler) ComplianceReport(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.NewSpan(r.Context(), "gateway.audit_compliance")
	defer span.End(nil)

	query := r.URL.Query()

	to := time.Now().UTC()
	from := to.Add(-30 * 24 * time.Hour)

	if fromStr := query.Get("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid 'from' time; use RFC3339 format",
			})
			return
		}
		from = t
	}

	if toStr := query.Get("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid 'to' time; use RFC3339 format",
			})
			return
		}
		to = t
	}

	report := h.store.ComplianceSummary(from, to)
	writeJSON(w, http.StatusOK, report)
}
