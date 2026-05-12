package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/persist"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/models"
)

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestSQLiteEventStore_AppendListGet(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	store := NewSQLiteEventStore(db, quietLog())

	evt := events.NewEvent(events.EventKindMessageInbound, "t1", "gateway", json.RawMessage(`{"hello":"world"}`))
	evt.WorkspaceID = "ws-1"
	store.Append(evt)

	page, total := store.List(1, 10, "", "")
	if total != 1 || len(page) != 1 {
		t.Fatalf("List total=%d len=%d, want 1/1", total, len(page))
	}
	if page[0].ID != evt.ID {
		t.Fatalf("List id mismatch: got %q want %q", page[0].ID, evt.ID)
	}

	got, ok := store.Get(evt.ID)
	if !ok {
		t.Fatalf("Get returned !ok")
	}
	if got.WorkspaceID != "ws-1" {
		t.Fatalf("Get WorkspaceID: got %q want ws-1", got.WorkspaceID)
	}
}

func TestSQLiteEventStore_FilterByWorkspaceAndKind(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	store := NewSQLiteEventStore(db, quietLog())

	a := events.NewEvent(events.EventKindMessageInbound, "t", "gw", json.RawMessage(`{}`))
	a.WorkspaceID = "ws-A"
	b := events.NewEvent(events.EventKindMessageOutbound, "t", "gw", json.RawMessage(`{}`))
	b.WorkspaceID = "ws-B"
	store.Append(a)
	store.Append(b)

	got, total := store.List(1, 10, "ws-A", "")
	if total != 1 || got[0].ID != a.ID {
		t.Fatalf("workspace filter: total=%d ids=%v", total, idsOf(got))
	}
	got, total = store.List(1, 10, "", string(events.EventKindMessageOutbound))
	if total != 1 || got[0].ID != b.ID {
		t.Fatalf("kind filter: total=%d ids=%v", total, idsOf(got))
	}
}

func TestSQLiteRecipeStore_SeedsBuiltinsOnce(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	first, err := NewSQLiteRecipeStore(db, quietLog())
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	_, total := first.List(1, 50)
	if total != 4 {
		t.Fatalf("seeded total: got %d, want 4 built-ins", total)
	}

	// Re-open: must NOT double-seed.
	second, err := NewSQLiteRecipeStore(db, quietLog())
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	_, total = second.List(1, 50)
	if total != 4 {
		t.Fatalf("re-seed produced duplicates: got %d, want 4", total)
	}
}

func TestSQLiteRecipeStore_ActivateDeactivate(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()
	store, err := NewSQLiteRecipeStore(db, quietLog())
	if err != nil {
		t.Fatalf("recipe store: %v", err)
	}

	recipes, _ := store.List(1, 50)
	if len(recipes) == 0 {
		t.Fatal("expected seeded recipes")
	}
	id := recipes[0].ID

	got, err := store.Activate(id)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if !got.IsActive {
		t.Fatal("recipe not active after Activate")
	}

	refetched, ok := store.GetByID(id)
	if !ok || !refetched.IsActive {
		t.Fatalf("Activate did not persist: ok=%v active=%v", ok, refetched.IsActive)
	}

	if _, err := store.Deactivate(id); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	refetched, _ = store.GetByID(id)
	if refetched.IsActive {
		t.Fatal("Deactivate did not persist")
	}
}

func TestSQLiteApprovalStore_CreateApproveDeny(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()
	store := NewSQLiteApprovalStore(db, quietLog())

	req := models.ApprovalRequest{
		ID:     "ap-1",
		Status: models.ApprovalStatusPending,
	}
	store.Create(req)

	got, ok := store.Get("ap-1")
	if !ok || got.Status != models.ApprovalStatusPending {
		t.Fatalf("after Create: ok=%v status=%v", ok, got.Status)
	}

	approved, ok := store.Approve("ap-1", "operator")
	if !ok || approved.Status != models.ApprovalStatusApproved {
		t.Fatalf("Approve: ok=%v status=%v", ok, approved.Status)
	}

	// Idempotency: a re-approve on a non-pending request should report
	// (req, false) — does not regress.
	again, ok := store.Approve("ap-1", "operator")
	if ok {
		t.Fatalf("re-Approve unexpectedly succeeded: %+v", again)
	}

	// Filter by status.
	page, total := store.List(1, 10, string(models.ApprovalStatusApproved))
	if total != 1 || page[0].ID != "ap-1" {
		t.Fatalf("filter approved: total=%d ids=%v", total, approvalIDs(page))
	}

	// Deny on a fresh pending request.
	store.Create(models.ApprovalRequest{ID: "ap-2", Status: models.ApprovalStatusPending})
	denied, ok := store.Deny("ap-2", "operator", "out of scope")
	if !ok || denied.Reason != "out of scope" {
		t.Fatalf("Deny: ok=%v reason=%q", ok, denied.Reason)
	}
}

func TestSQLiteSquadStore_SeedsBuiltinsAndCreate(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()
	store, err := NewSQLiteSquadStore(db, quietLog())
	if err != nil {
		t.Fatalf("squad store: %v", err)
	}
	_, total := store.List(1, 50)
	if total < 1 {
		t.Fatalf("squad store seeded %d squads, want > 0", total)
	}

	// Creating a custom squad lifts total by one.
	prev := total
	custom := models.Squad{ID: "sq-custom", IsActive: false}
	if _, err := store.Create(custom); err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, total = store.List(1, 50)
	if total != prev+1 {
		t.Fatalf("Create did not lift total: prev=%d now=%d", prev, total)
	}

	// Activate must lift the flag and persist it.
	got, err := store.Activate("sq-custom")
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if !got.IsActive {
		t.Fatal("squad not active after Activate")
	}
	persisted, _ := store.GetByID("sq-custom")
	if !persisted.IsActive {
		t.Fatal("Activate did not persist")
	}
}

func idsOf(es []events.Event) []string {
	ids := make([]string, 0, len(es))
	for _, e := range es {
		ids = append(ids, e.ID)
	}
	return ids
}

func approvalIDs(as []models.ApprovalRequest) []string {
	ids := make([]string, 0, len(as))
	for _, a := range as {
		ids = append(ids, a.ID)
	}
	return ids
}
