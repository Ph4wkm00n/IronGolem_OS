package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/handler"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/middleware"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/persist"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
)

// testInboxSecret is reused across the inbox tests; the exact bytes don't
// matter as long as MintToken + the auth middleware share them.
var testInboxSecret = []byte("inbox-test-secret")

// inboxAuthChain wraps the inbox handler in the same auth + tenant
// middleware stack production uses, so context-derived identity flows
// the way it does end-to-end.
func inboxAuthChain(h http.Handler) http.Handler {
	logger := quietHandlerLogger()
	h = middleware.TenantMiddleware(logger, middleware.ModeSolo)(h)
	h = middleware.HMACAuthMiddleware(middleware.AuthConfig{
		Secret: testInboxSecret,
	}, logger)(h)
	return h
}

// inboxRequest builds an authenticated GET /api/v1/inbox request whose
// token carries the supplied tenant + workspace claims.
func inboxRequest(t *testing.T, tenant, workspace string) *http.Request {
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
	req := httptest.NewRequest("GET", "/api/v1/inbox", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	return req
}

// quietHandlerLogger is local to this file to avoid colliding with
// `quietLog()` declared in the package-internal SQLite store test.
func quietHandlerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestInboxHandler_ListShape proves the wire contract: events that arrive
// via the event store surface as InboxItem-shaped JSON rows on
// GET /api/v1/inbox. Verifies the connector→source mapping, the
// status/risk defaults, and pagination metadata.
func TestInboxHandler_ListShape(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	store := handler.NewSQLiteEventStore(db, quietHandlerLogger())

	// Seed three inbound events across two connectors so source mapping
	// is exercised end-to-end.
	for _, sample := range []struct {
		connector string
		text      string
	}{
		{"telegram-main", "hello from telegram"},
		{"email-inbox", "hello from email"},
		{"webhook-1", "hello from a webhook"},
	} {
		payload, _ := json.Marshal(events.MessagePayload{
			ConnectorID: sample.connector,
			ChannelID:   "chat-1",
			UserID:      "u-1",
			Content:     sample.text,
			Direction:   "inbound",
		})
		evt := events.NewEvent(events.EventKindMessageInbound, "default", "gateway", payload)
		evt.WorkspaceID = "ws-test"
		store.Append(evt)
	}

	// Also append an outbound event — listing must NOT return it.
	outboundPayload, _ := json.Marshal(events.MessagePayload{
		ConnectorID: "telegram-main",
		ChannelID:   "chat-1",
		Content:     "reply text",
		Direction:   "outbound",
	})
	out := events.NewEvent(events.EventKindMessageOutbound, "default", "gateway", outboundPayload)
	out.WorkspaceID = "ws-test"
	store.Append(out)

	h := handler.NewInboxHandler(quietHandlerLogger(), store)
	wrapped := inboxAuthChain(http.HandlerFunc(h.ListInbox))

	req := inboxRequest(t, "default", "ws-test")
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("ListInbox: status %d, body %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Items    []handler.InboxItem `json:"items"`
		Total    int                 `json:"total"`
		Page     int                 `json:"page"`
		PageSize int                 `json:"page_size"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rr.Body.String())
	}

	if resp.Total != 3 {
		t.Fatalf("Total: got %d, want 3 (outbound should be filtered)", resp.Total)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("Items: got %d rows, want 3", len(resp.Items))
	}

	// Source mapping spot check: every row should derive its source from
	// the connector_id prefix.
	sources := map[string]int{}
	for _, item := range resp.Items {
		sources[item.Source]++

		// All items default to status=awaiting, risk=low, routedBy=Inbox triage.
		if item.Status != "awaiting" {
			t.Errorf("item %s status: got %q, want awaiting", item.ID, item.Status)
		}
		if item.Risk != "low" {
			t.Errorf("item %s risk: got %q, want low", item.ID, item.Risk)
		}
		if item.RoutedBy != "Inbox triage" {
			t.Errorf("item %s routedBy: got %q, want Inbox triage", item.ID, item.RoutedBy)
		}
		if !item.Unread {
			t.Errorf("item %s should be unread", item.ID)
		}
		if len(item.Safety.NeedsApproval) == 0 {
			t.Errorf("item %s safety.needsApproval is empty", item.ID)
		}
		if len(item.Audit) == 0 {
			t.Errorf("item %s audit trail is empty", item.ID)
		}
	}
	if sources["telegram"] != 1 || sources["email"] != 1 || sources["webhook"] != 1 {
		t.Fatalf("source mapping wrong: %+v (want one of each)", sources)
	}
}

// TestInboxHandler_FiltersByWorkspace proves tenant + workspace isolation.
// Events stamped with one workspace must NOT appear in another workspace's
// listing.
func TestInboxHandler_FiltersByWorkspace(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	store := handler.NewSQLiteEventStore(db, quietHandlerLogger())

	for _, ws := range []string{"ws-alpha", "ws-beta", "ws-alpha"} {
		payload, _ := json.Marshal(events.MessagePayload{
			ConnectorID: "telegram", ChannelID: "c", UserID: "u",
			Content: "hi", Direction: "inbound",
		})
		evt := events.NewEvent(events.EventKindMessageInbound, "tenant-x", "gateway", payload)
		evt.WorkspaceID = ws
		store.Append(evt)
	}

	h := handler.NewInboxHandler(quietHandlerLogger(), store)
	wrapped := inboxAuthChain(http.HandlerFunc(h.ListInbox))

	for _, tc := range []struct {
		workspace string
		want      int
	}{
		{"ws-alpha", 2},
		{"ws-beta", 1},
		{"ws-missing", 0},
	} {
		req := inboxRequest(t, "tenant-x", tc.workspace)
		rr := httptest.NewRecorder()
		wrapped.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("[%s] status %d", tc.workspace, rr.Code)
		}
		var resp struct {
			Total int `json:"total"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("[%s] decode: %v", tc.workspace, err)
		}
		if resp.Total != tc.want {
			t.Errorf("[%s] total: got %d, want %d", tc.workspace, resp.Total, tc.want)
		}
	}
}

// TestInboxHandler_TruncatesLongContent guards against an audit-event
// summary leaking a multi-kilobyte body into the wire response.
func TestInboxHandler_TruncatesLongContent(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	store := handler.NewSQLiteEventStore(db, quietHandlerLogger())
	long := strings.Repeat("x", 1024)
	payload, _ := json.Marshal(events.MessagePayload{
		ConnectorID: "email", ChannelID: "c", UserID: "u",
		Content: long, Direction: "inbound",
	})
	evt := events.NewEvent(events.EventKindMessageInbound, "tenant-y", "gateway", payload)
	evt.WorkspaceID = "ws-y"
	store.Append(evt)

	h := handler.NewInboxHandler(quietHandlerLogger(), store)
	wrapped := inboxAuthChain(http.HandlerFunc(h.ListInbox))
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, inboxRequest(t, "tenant-y", "ws-y"))

	var resp struct {
		Items []handler.InboxItem `json:"items"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Items) != 1 {
		t.Fatalf("len(items) = %d", len(resp.Items))
	}
	if len(resp.Items[0].Summary) > 200 {
		t.Fatalf("summary not truncated: len=%d", len(resp.Items[0].Summary))
	}
}

// TestInboxItem_TimestampMonotonic catches the regression that would
// surface if MinutesAgo went negative (server clock drift, future-stamped
// events). The wire shape must never expose a negative duration.
func TestInboxItem_TimestampMonotonic(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	store := handler.NewSQLiteEventStore(db, quietHandlerLogger())
	payload, _ := json.Marshal(events.MessagePayload{
		ConnectorID: "telegram", ChannelID: "c", UserID: "u",
		Content: "future", Direction: "inbound",
	})
	evt := events.NewEvent(events.EventKindMessageInbound, "t", "gateway", payload)
	// Stamp into the future (clock skew); ListInbox must clamp.
	evt.Timestamp = time.Now().UTC().Add(10 * time.Minute)
	evt.WorkspaceID = "ws"
	store.Append(evt)

	h := handler.NewInboxHandler(quietHandlerLogger(), store)
	wrapped := inboxAuthChain(http.HandlerFunc(h.ListInbox))
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, inboxRequest(t, "t", "ws"))

	var resp struct {
		Items []handler.InboxItem `json:"items"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Items) != 1 {
		t.Fatalf("len(items) = %d", len(resp.Items))
	}
	if resp.Items[0].MinutesAgo < 0 {
		t.Fatalf("MinutesAgo went negative: %d", resp.Items[0].MinutesAgo)
	}
}
