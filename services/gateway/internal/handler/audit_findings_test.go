package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit"
)

// stubFindingStore implements audit.FindingStore for handler tests.
type stubFindingStore struct {
	items    []audit.StoredFinding
	listErr  error
	lastSev  audit.Severity
	lastLim  int
}

func (s *stubFindingStore) Insert(_ context.Context, _ audit.Finding) (string, error) {
	return "stub", nil
}

func (s *stubFindingStore) List(_ context.Context, severity audit.Severity, limit int) ([]audit.StoredFinding, error) {
	s.lastSev = severity
	s.lastLim = limit
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.items, nil
}

func TestAuditFindings_OK(t *testing.T) {
	store := &stubFindingStore{
		items: []audit.StoredFinding{
			{
				ID: "f1",
				Finding: audit.Finding{
					ProbeID:  "trust_model",
					Severity: audit.SeverityCritical,
					Reason:   "missing HMAC",
					Timestamp: time.Now(),
				},
				StoredAt: time.Now(),
			},
		},
	}
	h := NewAuditFindingsHandler(store, slog.Default())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/audit/findings", nil)
	h.ListFindings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if int(body["count"].(float64)) != 1 {
		t.Fatalf("count = %v, want 1", body["count"])
	}
	if body["severity"] != "" {
		t.Fatalf("severity = %q, want '' (no filter)", body["severity"])
	}
	if int(body["limit"].(float64)) != 100 {
		t.Fatalf("limit = %v, want 100 (default)", body["limit"])
	}
}

func TestAuditFindings_EmptyReturnsArrayNotNull(t *testing.T) {
	store := &stubFindingStore{items: nil}
	h := NewAuditFindingsHandler(store, slog.Default())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/audit/findings", nil)
	h.ListFindings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	// "items":[]  — not  "items":null. The frontend mocks expect [].
	if !contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("empty result should serialize as []: %s", rec.Body.String())
	}
}

func TestAuditFindings_SeverityFilterPropagated(t *testing.T) {
	store := &stubFindingStore{}
	h := NewAuditFindingsHandler(store, slog.Default())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/audit/findings?severity=warning&limit=25", nil)
	h.ListFindings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if store.lastSev != audit.SeverityWarning {
		t.Errorf("severity passed = %q, want warning", store.lastSev)
	}
	if store.lastLim != 25 {
		t.Errorf("limit passed = %d, want 25", store.lastLim)
	}
}

func TestAuditFindings_InvalidSeverity400(t *testing.T) {
	h := NewAuditFindingsHandler(&stubFindingStore{}, slog.Default())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/audit/findings?severity=fatal", nil)
	h.ListFindings(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAuditFindings_InvalidLimit400(t *testing.T) {
	cases := []string{"0", "-1", "1001", "abc"}
	for _, lim := range cases {
		h := NewAuditFindingsHandler(&stubFindingStore{}, slog.Default())
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v2/audit/findings?limit="+lim, nil)
		h.ListFindings(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("limit=%q: status = %d, want 400", lim, rec.Code)
		}
	}
}

func TestAuditFindings_StoreError500(t *testing.T) {
	h := NewAuditFindingsHandler(&stubFindingStore{listErr: errors.New("disk full")}, slog.Default())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/audit/findings", nil)
	h.ListFindings(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
