package handler_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/connector"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/handler"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/middleware"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/persist"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
)

// homeAuthChain mirrors inboxAuthChain: route through the real auth +
// tenant middleware so context-derived identity flows production-style.
func homeAuthChain(h http.Handler) http.Handler {
	logger := slog.New(slog.NewTextHandler(disposingWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	h = middleware.TenantMiddleware(logger, middleware.ModeSolo)(h)
	h = middleware.HMACAuthMiddleware(middleware.AuthConfig{
		Secret: testInboxSecret, // reuse the inbox test secret — same package
	}, logger)(h)
	return h
}

// disposingWriter mimics io.Discard without importing io here twice.
type disposingWriter struct{}

func (disposingWriter) Write(p []byte) (int, error) { return len(p), nil }

func homeRequest(t *testing.T, tenant, workspace string) *http.Request {
	t.Helper()
	tok, err := middleware.MintToken(middleware.TokenClaims{
		TenantID:    tenant,
		WorkspaceID: workspace,
		UserID:      "test-user",
		AgentRole:   "executor",
		ChannelID:   "test",
		ExpiresAt:   time.Now().Add(15 * time.Minute),
	}, testInboxSecret)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	req := httptest.NewRequest("GET", "/api/v1/home", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	return req
}

// TestHomeHandler_FullShape proves the wire contract. Asserts every
// top-level field is present + populated so the v2 Home page never sees
// undefined when it swaps from mock to real.
func TestHomeHandler_FullShape(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	store := handler.NewSQLiteEventStore(db, quietHandlerLogger())
	connMgr := connector.NewManager(quietHandlerLogger())
	t.Cleanup(connMgr.DisconnectAll)

	// Seed two events so the timeline isn't empty.
	for _, content := range []string{"first message", "second message"} {
		payload, _ := json.Marshal(events.MessagePayload{
			ConnectorID: "telegram-test", ChannelID: "c-1", UserID: "u-1",
			Content: content, Direction: "inbound",
		})
		evt := events.NewEvent(events.EventKindMessageInbound, "default", "gateway", payload)
		evt.WorkspaceID = "ws-test"
		store.Append(evt)
	}

	h := handler.NewHomeHandler(quietHandlerLogger(), store, connMgr, db)
	wrapped := homeAuthChain(http.HandlerFunc(h.ListHome))

	req := homeRequest(t, "default", "ws-test")
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d body=%s", rr.Code, rr.Body.String())
	}

	var resp handler.HomeResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rr.Body.String())
	}

	// Workspace populated from probeWorkspace.
	if resp.Workspace.Name == "" {
		t.Errorf("workspace.name empty")
	}
	if resp.Workspace.UptimeHours < 0 {
		t.Errorf("workspace.uptimeHours negative: %d", resp.Workspace.UptimeHours)
	}

	// Heartbeat: with no connectors registered we expect gateway + sqlite = 2 systems.
	if resp.Heartbeat.SystemsTotal < 2 {
		t.Errorf("heartbeat.systemsTotal: got %d, want ≥2", resp.Heartbeat.SystemsTotal)
	}
	if resp.Heartbeat.Status == "" {
		t.Errorf("heartbeat.status empty")
	}

	// Stubs must still be structurally present.
	if len(resp.Teams) == 0 {
		t.Errorf("teams: empty (page renders against this)")
	}
	if resp.TrustHistory == nil || len(resp.TrustHistory) == 0 {
		t.Errorf("trustHistory: nil/empty")
	}
	if len(resp.Safety.Layers) != 5 {
		t.Errorf("safety.layers: got %d, want 5", len(resp.Safety.Layers))
	}

	// Events: both seeded inbound events surface.
	if len(resp.Events) != 2 {
		t.Errorf("events: got %d, want 2 (newest inbound)", len(resp.Events))
	}
	for _, evt := range resp.Events {
		if evt.Status != "proposed" {
			t.Errorf("inbound event status: got %q, want proposed", evt.Status)
		}
		if evt.MinutesAgo < 0 {
			t.Errorf("minutesAgo negative: %d", evt.MinutesAgo)
		}
	}
}

// TestHomeHandler_FiltersEventsByWorkspace proves workspace isolation
// flows through the home endpoint just like it does in the inbox.
func TestHomeHandler_FiltersEventsByWorkspace(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	store := handler.NewSQLiteEventStore(db, quietHandlerLogger())
	connMgr := connector.NewManager(quietHandlerLogger())
	t.Cleanup(connMgr.DisconnectAll)

	for _, ws := range []string{"ws-A", "ws-B", "ws-A"} {
		payload, _ := json.Marshal(events.MessagePayload{
			ConnectorID: "t", ChannelID: "c", UserID: "u",
			Content: "x", Direction: "inbound",
		})
		evt := events.NewEvent(events.EventKindMessageInbound, "tenant", "gw", payload)
		evt.WorkspaceID = ws
		store.Append(evt)
	}

	h := handler.NewHomeHandler(quietHandlerLogger(), store, connMgr, db)
	wrapped := homeAuthChain(http.HandlerFunc(h.ListHome))

	for _, tc := range []struct {
		ws   string
		want int
	}{
		{"ws-A", 2},
		{"ws-B", 1},
		{"ws-missing", 0},
	} {
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, homeRequest(t, "tenant", tc.ws))
		if rr.Code != http.StatusOK {
			t.Fatalf("[%s] status %d", tc.ws, rr.Code)
		}
		var resp handler.HomeResponse
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if len(resp.Events) != tc.want {
			t.Errorf("[%s] events: got %d, want %d", tc.ws, len(resp.Events), tc.want)
		}
	}
}
