package policy_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/persist"
	gwpolicy "github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/policy"
	corepolicy "github.com/Ph4wkm00n/IronGolem_OS/services/pkg/policy"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestSQLiteChannelPolicyStore_LookupUpsertDelete covers the round-trip
// of the canonical CRUD path: upsert installs a rule, lookup returns it
// verbatim, a second upsert replaces it idempotently, and delete clears
// it without affecting siblings.
func TestSQLiteChannelPolicyStore_LookupUpsertDelete(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	store := gwpolicy.NewSQLiteChannelPolicyStore(db, quietLogger())
	ctx := context.Background()

	// Empty store: HasRules false; Lookup returns ok=false.
	if has, err := store.HasRules(ctx); err != nil || has {
		t.Fatalf("HasRules empty: has=%v err=%v", has, err)
	}
	if _, ok, err := store.Lookup(ctx, "chan-1", "read"); err != nil || ok {
		t.Fatalf("Lookup empty: ok=%v err=%v", ok, err)
	}

	// Install a deny rule + an allow rule on the same channel.
	for _, r := range []corepolicy.ChannelRule{
		{ChannelID: "chan-1", Action: "execute", Decision: corepolicy.DecisionDeny, Reason: "quiet hours"},
		{ChannelID: "chan-1", Action: "read", Decision: corepolicy.DecisionAllow, Reason: "reads always ok"},
	} {
		if err := store.Upsert(ctx, r); err != nil {
			t.Fatalf("Upsert(%+v): %v", r, err)
		}
	}

	// HasRules flips.
	if has, _ := store.HasRules(ctx); !has {
		t.Fatal("HasRules: expected true after Upsert")
	}

	// Lookup returns the right rule per (channel, action).
	got, ok, err := store.Lookup(ctx, "chan-1", "execute")
	if err != nil || !ok {
		t.Fatalf("Lookup execute: ok=%v err=%v", ok, err)
	}
	if got.Decision != corepolicy.DecisionDeny || got.Reason != "quiet hours" {
		t.Fatalf("execute rule mismatch: %+v", got)
	}

	got, ok, _ = store.Lookup(ctx, "chan-1", "read")
	if !ok || got.Decision != corepolicy.DecisionAllow {
		t.Fatalf("read rule mismatch: %+v", got)
	}

	// Upsert replaces the existing row (idempotent).
	if err := store.Upsert(ctx, corepolicy.ChannelRule{
		ChannelID: "chan-1", Action: "execute",
		Decision: corepolicy.DecisionDeny, Reason: "updated reason",
	}); err != nil {
		t.Fatalf("re-Upsert: %v", err)
	}
	got, _, _ = store.Lookup(ctx, "chan-1", "execute")
	if got.Reason != "updated reason" {
		t.Fatalf("re-Upsert did not replace: %+v", got)
	}

	// Delete clears one row; the other survives.
	if err := store.Delete(ctx, "chan-1", "execute"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok, _ := store.Lookup(ctx, "chan-1", "execute"); ok {
		t.Fatal("Delete did not remove the row")
	}
	if _, ok, _ := store.Lookup(ctx, "chan-1", "read"); !ok {
		t.Fatal("Delete removed an unrelated row")
	}
}

// TestSQLiteChannelPolicyStore_EngineWiring proves the end-to-end
// behavior the v0.2 plan calls for: a rule in the store flips Layer 4
// from "disabled / allow" to "deny by rule".
func TestSQLiteChannelPolicyStore_EngineWiring(t *testing.T) {
	db, err := persist.Open(":memory:")
	if err != nil {
		t.Fatalf("persist.Open: %v", err)
	}
	defer db.Close()

	store := gwpolicy.NewSQLiteChannelPolicyStore(db, quietLogger())
	engine := corepolicy.NewDefaultPolicyEngineWithStore(quietLogger(), store)
	ctx := context.Background()

	// Build a request that would pass layers 1-3 + 5 so any deny we see
	// must come from Layer 4.
	req := corepolicy.EvalRequest{
		TenantID:    "tenant-1",
		WorkspaceID: "ws-1",
		UserID:      "u-1",
		AgentRole:   "executor",
		ChannelID:   "chan-locked",
		Permission:  corepolicy.Permission{Resource: "message.outbound", Action: "write"},
	}

	// Empty store + env unset → allow with the disabled-reason.
	res, err := engine.Evaluate(ctx, req)
	if err != nil {
		t.Fatalf("Evaluate empty: %v", err)
	}
	if res.Decision != corepolicy.DecisionAllow {
		t.Fatalf("empty store: got %q, want allow", res.Decision)
	}

	// Install a deny rule for this (channel, action).
	if err := store.Upsert(ctx, corepolicy.ChannelRule{
		ChannelID: "chan-locked", Action: "write",
		Decision: corepolicy.DecisionDeny, Reason: "channel under review",
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Now the engine denies, and the reason surfaces verbatim.
	res, err = engine.Evaluate(ctx, req)
	if err != nil {
		t.Fatalf("Evaluate post-rule: %v", err)
	}
	if res.Decision != corepolicy.DecisionDeny {
		t.Fatalf("rule installed: got %q, want deny", res.Decision)
	}
	if res.Reason != "channel under review" {
		t.Fatalf("reason mismatch: got %q", res.Reason)
	}
	if res.Layer != corepolicy.LayerPerChannelRestrict {
		t.Fatalf("layer mismatch: got %s", res.Layer)
	}

	// A different (channel, action) on the same engine still passes —
	// Layer 4 is a deny-list in v0.2, not an allow-list.
	otherReq := req
	otherReq.ChannelID = "chan-untouched"
	res, _ = engine.Evaluate(ctx, otherReq)
	if res.Decision != corepolicy.DecisionAllow {
		t.Fatalf("unrelated channel: got %q, want allow", res.Decision)
	}
}
