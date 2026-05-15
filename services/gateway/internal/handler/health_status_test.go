package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/connector"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/handler"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/persist"
)

// TestHealthStatusHandler_BasicShape proves the wire contract: the
// endpoint always returns at least gateway + sqlite components plus
// (empty) healEvents / predictive arrays. The page's row renderer
// expects all three keys present.
func TestHealthStatusHandler_BasicShape(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	connMgr := connector.NewManager(quietHandlerLogger())
	t.Cleanup(connMgr.DisconnectAll)

	h := handler.NewHealthStatusHandler(quietHandlerLogger(), connMgr, db)
	wrapped := homeAuthChain(http.HandlerFunc(h.GetStatus))

	rr := httptest.NewRecorder()
	req := homeRequest(t, "tenant", "ws")
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}

	var resp handler.HealthStatusResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Components) < 3 {
		t.Fatalf("components: got %d, want ≥3 (gateway + sqlite + runtimed)", len(resp.Components))
	}
	// healEvents / predictive ship as empty arrays (NOT null) so the
	// frontend's `.map` calls don't blow up.
	if resp.HealEvents == nil {
		t.Errorf("healEvents: must be empty array, not null")
	}
	if resp.Predictive == nil {
		t.Errorf("predictive: must be empty array, not null")
	}

	// Each component carries the keys the page needs.
	for _, c := range resp.Components {
		if c.ID == "" || c.Name == "" || c.Category == "" || c.State == "" {
			t.Errorf("component missing required field: %+v", c)
		}
	}
}

// TestHealthStatusHandler_IncludesRegisteredConnectors proves connector
// rows appear in the response after RegisterSource / Connect happen.
func TestHealthStatusHandler_IncludesRegisteredConnectors(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	connMgr := connector.NewManager(quietHandlerLogger())
	t.Cleanup(connMgr.DisconnectAll)
	connMgr.Connect("telegram-test")

	h := handler.NewHealthStatusHandler(quietHandlerLogger(), connMgr, db)
	wrapped := homeAuthChain(http.HandlerFunc(h.GetStatus))

	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, homeRequest(t, "tenant", "ws"))
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d", rr.Code)
	}

	var resp handler.HealthStatusResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)

	found := false
	for _, c := range resp.Components {
		if c.Category == "connector" && c.ID == "connector-telegram-test" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("connector row missing from response: %+v", resp.Components)
	}
}
