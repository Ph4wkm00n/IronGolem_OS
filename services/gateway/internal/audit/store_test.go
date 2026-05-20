package audit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openStoreTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// Inline the audit-findings schema from persist/db.go so this test
	// stays self-contained. Schema drift between the two will surface
	// when both files are edited; the integration tests in services/
	// catch full-stack mismatches.
	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE gateway_audit_findings (
			id          TEXT PRIMARY KEY NOT NULL,
			probe_id    TEXT NOT NULL,
			severity    TEXT NOT NULL,
			reason      TEXT NOT NULL DEFAULT '',
			evidence    TEXT NOT NULL DEFAULT '{}',
			ts          TEXT NOT NULL,
			stored_at   TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestSQLiteFindingStore_RoundTrip(t *testing.T) {
	db := openStoreTestDB(t)
	s := NewSQLiteFindingStore(db)

	ctx := context.Background()
	id, err := s.Insert(ctx, Finding{
		ProbeID:  "trust_model",
		Severity: SeverityCritical,
		Reason:   "HMAC secret missing",
		Evidence: map[string]any{"env_var": "FOO", "extra": []string{"a", "b"}},
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id == "" {
		t.Fatal("insert returned empty id")
	}

	list, err := s.List(ctx, "", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	got := list[0]
	if got.ID != id {
		t.Errorf("id = %q, want %q", got.ID, id)
	}
	if got.Finding.ProbeID != "trust_model" {
		t.Errorf("probe_id = %q", got.Finding.ProbeID)
	}
	if got.Finding.Severity != SeverityCritical {
		t.Errorf("severity = %q", got.Finding.Severity)
	}
	if got.Finding.Evidence["env_var"] != "FOO" {
		t.Errorf("evidence env_var = %v", got.Finding.Evidence["env_var"])
	}
}

func TestSQLiteFindingStore_SeverityFilter(t *testing.T) {
	db := openStoreTestDB(t)
	s := NewSQLiteFindingStore(db)
	ctx := context.Background()

	seed := []Finding{
		{ProbeID: "a", Severity: SeverityInfo, Reason: "ok"},
		{ProbeID: "b", Severity: SeverityWarning, Reason: "warn"},
		{ProbeID: "c", Severity: SeverityCritical, Reason: "boom"},
	}
	for _, f := range seed {
		if _, err := s.Insert(ctx, f); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		filter Severity
		want   int
	}{
		{"", 3},
		{SeverityInfo, 3},
		{SeverityWarning, 2},
		{SeverityCritical, 1},
	}
	for _, c := range cases {
		got, err := s.List(ctx, c.filter, 10)
		if err != nil {
			t.Fatalf("list(%q): %v", c.filter, err)
		}
		if len(got) != c.want {
			t.Errorf("filter=%q: got %d, want %d", c.filter, len(got), c.want)
		}
	}
}

func TestSQLiteFindingStore_InvalidSeverityRejected(t *testing.T) {
	db := openStoreTestDB(t)
	s := NewSQLiteFindingStore(db)
	_, err := s.Insert(context.Background(), Finding{
		ProbeID:  "x",
		Severity: Severity("fatal"),
	})
	if err == nil {
		t.Fatal("expected error for invalid severity")
	}
}

func TestSQLiteFindingStore_OrderingNewestFirst(t *testing.T) {
	db := openStoreTestDB(t)
	s := NewSQLiteFindingStore(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := s.Insert(ctx, Finding{
			ProbeID:   "p",
			Severity:  SeverityInfo,
			Reason:    "ok",
			Timestamp: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	list, err := s.List(ctx, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("len = %d, want 3", len(list))
	}
	for i := 1; i < len(list); i++ {
		if !list[i-1].StoredAt.After(list[i].StoredAt) && !list[i-1].StoredAt.Equal(list[i].StoredAt) {
			t.Errorf("ordering broken: %v < %v", list[i-1].StoredAt, list[i].StoredAt)
		}
	}
}
