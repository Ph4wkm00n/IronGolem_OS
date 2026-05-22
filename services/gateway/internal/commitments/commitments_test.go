package commitments

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

const tableSQL = `
CREATE TABLE gateway_commitments (
	id                TEXT PRIMARY KEY NOT NULL,
	workspace_id      TEXT NOT NULL,
	tenant_id         TEXT NOT NULL DEFAULT '',
	kind              TEXT NOT NULL,
	sensitivity       TEXT NOT NULL,
	status            TEXT NOT NULL,
	reason            TEXT NOT NULL DEFAULT '',
	suggested_text    TEXT NOT NULL DEFAULT '',
	dedupe_key        TEXT NOT NULL,
	confidence        REAL NOT NULL DEFAULT 0,
	earliest_ms       INTEGER NOT NULL,
	latest_ms         INTEGER NOT NULL,
	timezone          TEXT NOT NULL DEFAULT '',
	connector_id      TEXT NOT NULL DEFAULT '',
	channel_id        TEXT NOT NULL DEFAULT '',
	user_id           TEXT NOT NULL DEFAULT '',
	source_event_id   TEXT NOT NULL DEFAULT '',
	created_at_ms     INTEGER NOT NULL,
	updated_at_ms     INTEGER NOT NULL,
	sent_at_ms        INTEGER,
	dismissed_at_ms   INTEGER,
	snoozed_until_ms  INTEGER,
	expired_at_ms     INTEGER,
	attempts          INTEGER NOT NULL DEFAULT 0
)
`

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(tableSQL); err != nil {
		t.Fatalf("schema: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func sample(workspaceID, dedupeKey string, earliestMs, latestMs int64) Commitment {
	return Commitment{
		WorkspaceID:   workspaceID,
		TenantID:      "default",
		Kind:          KindEventCheckIn,
		Sensitivity:   SensitivityRoutine,
		Reason:        "test reason",
		SuggestedText: "test suggestion",
		DedupeKey:     dedupeKey,
		Confidence:    0.8,
		DueWindow:     DueWindow{EarliestMs: earliestMs, LatestMs: latestMs},
		ConnectorID:   "telegram",
		ChannelID:     "c1",
		UserID:        "u1",
	}
}

func TestKind_Valid(t *testing.T) {
	if !KindCareCheckIn.Valid() {
		t.Error("CareCheckIn should be valid")
	}
	if CommitmentKind("invalid").Valid() {
		t.Error("invalid kind should not be Valid")
	}
}

func TestStore_InsertAndGet(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	c := sample("w1", "dk1", 0, 0)
	id, err := s.Insert(context.Background(), c)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id == "" {
		t.Fatal("empty id")
	}
	got, err := s.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Reason != "test reason" || got.Kind != KindEventCheckIn {
		t.Fatalf("got = %+v", got)
	}
	if got.Status != StatusPending {
		t.Errorf("status = %q, want pending", got.Status)
	}
}

func TestStore_InsertDedupesByKey(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	c := sample("w1", "shared-key", 0, 0)
	first, err := s.Insert(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	// v1.2.2: dedup returns the existing id paired with ErrDeduped so
	// callers can distinguish "new row" from "you hit an existing one"
	// without scanning the table again.
	second, err := s.Insert(context.Background(), c)
	if !errors.Is(err, ErrDeduped) {
		t.Fatalf("dedup err = %v, want ErrDeduped", err)
	}
	if first != second {
		t.Fatalf("dedup failed: first=%q second=%q", first, second)
	}
}

func TestStore_RejectsInvalidKind(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	c := sample("w1", "k", 0, 0)
	c.Kind = CommitmentKind("bogus")
	if _, err := s.Insert(context.Background(), c); err == nil {
		t.Fatal("expected error for invalid kind")
	}
}

func TestStore_DueNow(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	now := time.Now().UTC().UnixMilli()
	// past, currently-due, future
	_, _ = s.Insert(context.Background(), sample("w1", "past", now-10_000, now-5_000))
	_, _ = s.Insert(context.Background(), sample("w1", "due", now-5_000, now+5_000))
	_, _ = s.Insert(context.Background(), sample("w1", "future", now+10_000, now+20_000))

	due, err := s.DueNow(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("DueNow: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("len = %d, want 1; got %+v", len(due), due)
	}
	if due[0].DedupeKey != "due" {
		t.Errorf("dedupe_key = %q, want 'due'", due[0].DedupeKey)
	}
}

func TestStore_MarkSentDismissedSnoozedExpired(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	ctx := context.Background()
	id, _ := s.Insert(ctx, sample("w1", "k1", 0, time.Now().Add(time.Hour).UnixMilli()))

	now := time.Now().UTC().UnixMilli()
	if err := s.MarkSent(ctx, id, now); err != nil {
		t.Fatalf("MarkSent: %v", err)
	}
	got, _ := s.Get(ctx, id)
	if got.Status != StatusSent || got.SentAtMs == 0 {
		t.Errorf("after MarkSent: status=%q sent_at_ms=%d", got.Status, got.SentAtMs)
	}
	if got.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", got.Attempts)
	}

	id2, _ := s.Insert(ctx, sample("w1", "k2", 0, time.Now().Add(time.Hour).UnixMilli()))
	if err := s.MarkDismissed(ctx, id2); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Get(ctx, id2)
	if got.Status != StatusDismissed {
		t.Errorf("dismissed status = %q", got.Status)
	}

	id3, _ := s.Insert(ctx, sample("w1", "k3", 0, time.Now().Add(time.Hour).UnixMilli()))
	future := time.Now().Add(time.Hour).UnixMilli()
	if err := s.MarkSnoozed(ctx, id3, future); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Get(ctx, id3)
	if got.Status != StatusPending || got.DueWindow.EarliestMs != future {
		t.Errorf("snooze status=%q earliest=%d want=%d", got.Status, got.DueWindow.EarliestMs, future)
	}

	id4, _ := s.Insert(ctx, sample("w1", "k4", 0, time.Now().Add(time.Hour).UnixMilli()))
	if err := s.MarkExpired(ctx, id4); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Get(ctx, id4)
	if got.Status != StatusExpired {
		t.Errorf("expired status = %q", got.Status)
	}
}

func TestStore_MarkSentNoOpAfterDismissed(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	ctx := context.Background()
	id, _ := s.Insert(ctx, sample("w1", "k1", 0, time.Now().Add(time.Hour).UnixMilli()))
	if err := s.MarkDismissed(ctx, id); err != nil {
		t.Fatal(err)
	}
	// MarkSent only flips pending → sent; the dismissed row should
	// surface as not-found (zero rows affected).
	err := s.MarkSent(ctx, id, time.Now().UnixMilli())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("MarkSent on dismissed should return ErrNotFound, got %v", err)
	}
}

func TestStore_GetMissing(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	_, err := s.Get(context.Background(), "no-such-id")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestParseStatus(t *testing.T) {
	cases := []struct {
		in   string
		want CommitmentStatus
		err  bool
	}{
		{"", "", false},
		{"PENDING", StatusPending, false},
		{"sent", StatusSent, false},
		{"  expired  ", StatusExpired, false},
		{"bogus", "", true},
	}
	for _, c := range cases {
		got, err := ParseStatus(c.in)
		if (err != nil) != c.err {
			t.Errorf("%q: err=%v want_err=%v", c.in, err, c.err)
			continue
		}
		if got != c.want {
			t.Errorf("%q: got %q want %q", c.in, got, c.want)
		}
	}
}

func TestExtractor_RelativeTime(t *testing.T) {
	e := NewHeuristicExtractor()
	cands, err := e.Extract(context.Background(), Turn{
		UserText:      "Remind me in 2 hours to call mom",
		AssistantText: "Got it, I'll remind you in 2 hours.",
		WorkspaceID:   "w1",
		SourceEventID: "evt1",
		NowMs:         time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC).UnixMilli(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) == 0 {
		t.Fatal("expected at least one candidate")
	}
	var found bool
	for _, c := range cands {
		if c.Kind == KindEventCheckIn && c.Confidence >= MinConfidence {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected an event_check_in candidate above threshold; got %+v", cands)
	}
}

func TestExtractor_OpenLoop(t *testing.T) {
	e := NewHeuristicExtractor()
	cands, _ := e.Extract(context.Background(), Turn{
		UserText:      "Thoughts on the contract?",
		AssistantText: "I'll follow up with the legal team on this.",
		WorkspaceID:   "w1",
	})
	if len(cands) == 0 {
		t.Fatal("expected open-loop candidate")
	}
	if cands[0].Kind != KindOpenLoop && cands[0].Kind != KindCareCheckIn {
		t.Errorf("got kind %q, want open_loop or care_check_in", cands[0].Kind)
	}
}

func TestExtractor_CareSensitivity(t *testing.T) {
	e := NewHeuristicExtractor()
	cands, _ := e.Extract(context.Background(), Turn{
		AssistantText: "I'll check on you tomorrow.",
		WorkspaceID:   "w1",
	})
	if len(cands) == 0 {
		t.Fatal("expected at least one candidate")
	}
	if cands[0].Sensitivity != SensitivityCare {
		t.Errorf("sensitivity = %q, want care", cands[0].Sensitivity)
	}
}

func TestExtractor_DedupeKeyStableForSameTurn(t *testing.T) {
	e := NewHeuristicExtractor()
	in := Turn{
		AssistantText: "I'll follow up.",
		WorkspaceID:   "w1",
		SourceEventID: "evt1",
	}
	first, _ := e.Extract(context.Background(), in)
	second, _ := e.Extract(context.Background(), in)
	if len(first) == 0 || len(second) == 0 {
		t.Fatal("no candidates")
	}
	if first[0].DedupeKey != second[0].DedupeKey {
		t.Errorf("dedupe key drifted across calls: %q vs %q", first[0].DedupeKey, second[0].DedupeKey)
	}
}

// ----- Runtime tests --------------------------------------------------

type stubDispatcher struct {
	mu       sync.Mutex
	calls    []Commitment
	failWith error
}

func (s *stubDispatcher) Dispatch(_ context.Context, c Commitment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failWith != nil {
		return s.failWith
	}
	s.calls = append(s.calls, c)
	return nil
}

type stubEmitter struct {
	mu        sync.Mutex
	fired     []string
	expired   []string
	extracted []string
}

func (s *stubEmitter) EmitCommitmentExtracted(_ context.Context, c Commitment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extracted = append(s.extracted, c.ID)
	return nil
}
func (s *stubEmitter) EmitCommitmentFired(_ context.Context, c Commitment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fired = append(s.fired, c.ID)
	return nil
}
func (s *stubEmitter) EmitCommitmentDismissed(_ context.Context, _ Commitment) error { return nil }
func (s *stubEmitter) EmitCommitmentSnoozed(_ context.Context, _ Commitment) error   { return nil }
func (s *stubEmitter) EmitCommitmentExpired(_ context.Context, c Commitment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expired = append(s.expired, c.ID)
	return nil
}

func TestRuntime_FireDueCallsDispatcherAndEmits(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	ctx := context.Background()
	now := time.Now().UTC().UnixMilli()
	id, _ := s.Insert(ctx, sample("w1", "k1", now-5_000, now+5_000))

	disp := &stubDispatcher{}
	emit := &stubEmitter{}
	rt := NewRuntime(s, disp, emit, nil, RuntimeConfig{})
	rt.tick(ctx)

	if len(disp.calls) != 1 || disp.calls[0].ID != id {
		t.Fatalf("dispatcher calls: %+v", disp.calls)
	}
	got, _ := s.Get(ctx, id)
	if got.Status != StatusSent {
		t.Fatalf("post-tick status: %q", got.Status)
	}
	if len(emit.fired) != 1 || emit.fired[0] != id {
		t.Fatalf("emitter fired: %+v", emit.fired)
	}
}

func TestRuntime_DispatchFailureLeavesPending(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	ctx := context.Background()
	now := time.Now().UTC().UnixMilli()
	id, _ := s.Insert(ctx, sample("w1", "k1", now-1_000, now+5_000))

	disp := &stubDispatcher{failWith: errors.New("network down")}
	rt := NewRuntime(s, disp, &stubEmitter{}, nil, RuntimeConfig{})
	rt.tick(ctx)

	got, _ := s.Get(ctx, id)
	if got.Status != StatusPending {
		t.Fatalf("status after failed dispatch: %q, want pending", got.Status)
	}
}

func TestRuntime_ExpireStale(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	ctx := context.Background()
	now := time.Now().UTC().UnixMilli()
	// Latest 30min in the past — outside the grace.
	id, _ := s.Insert(ctx, sample("w1", "k1", now-3600_000, now-1800_000))

	emit := &stubEmitter{}
	rt := NewRuntime(s, nil, emit, nil, RuntimeConfig{ExpireGrace: time.Minute})
	rt.tick(ctx)
	got, _ := s.Get(ctx, id)
	if got.Status != StatusExpired {
		t.Fatalf("status after expire tick: %q", got.Status)
	}
	if len(emit.expired) != 1 {
		t.Fatalf("expired emit: %+v", emit.expired)
	}
}

func TestRuntime_EnqueueAppliesThreshold(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	rt := NewRuntime(s, nil, &stubEmitter{}, nil, RuntimeConfig{})

	extractor := stubExtractor{
		candidates: []Candidate{
			{Kind: KindOpenLoop, Sensitivity: SensitivityRoutine, Reason: "low conf", DedupeKey: "low", Confidence: 0.4, DueWindow: DueWindow{EarliestMs: 1, LatestMs: 2}},
			{Kind: KindOpenLoop, Sensitivity: SensitivityRoutine, Reason: "high conf", DedupeKey: "high", Confidence: 0.9, DueWindow: DueWindow{EarliestMs: 1, LatestMs: 2}},
		},
	}
	rt.Enqueue(context.Background(), extractor, Turn{WorkspaceID: "w1", SourceEventID: "evt1"})
	all, err := s.List(context.Background(), ListFilter{WorkspaceID: "w1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("expected only the high-confidence candidate to land; got %d", len(all))
	}
	if !strings.Contains(all[0].Reason, "high conf") {
		t.Errorf("wrong candidate persisted: %q", all[0].Reason)
	}
}

// v1.2.2: regression. Re-running the same extractor over the same Turn
// must dedupe at the store layer AND must NOT emit a second
// `commitment.extracted` event for the existing row. Pre-patch the
// runtime treated ErrNoRows-free Insert as "new" and emitted on every
// repeat extraction.
func TestRuntime_EnqueueDedupSkipsExtractedEmission(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	emit := &stubEmitter{}
	rt := NewRuntime(s, nil, emit, nil, RuntimeConfig{})

	cands := []Candidate{
		{Kind: KindOpenLoop, Sensitivity: SensitivityRoutine, Reason: "dup", DedupeKey: "same-key", Confidence: 0.9, DueWindow: DueWindow{EarliestMs: 1, LatestMs: 2}},
	}
	in := Turn{WorkspaceID: "w1", SourceEventID: "evt1"}

	rt.Enqueue(context.Background(), stubExtractor{candidates: cands}, in)
	rt.Enqueue(context.Background(), stubExtractor{candidates: cands}, in)
	rt.Enqueue(context.Background(), stubExtractor{candidates: cands}, in)

	all, err := s.List(context.Background(), ListFilter{WorkspaceID: "w1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("store should hold one row after 3 dedup hits; got %d", len(all))
	}
	if len(emit.extracted) != 1 {
		t.Fatalf("extracted emission count = %d, want 1 (one per genuinely new row)", len(emit.extracted))
	}
}

// v1.2.2: ExpireOverdue batches the per-id MarkExpired path into a
// single SELECT + UPDATE. Verify the snapshot returned matches the
// row(s) that flipped to expired and that already-non-pending rows
// don't trip the IN-clause guard.
func TestStore_ExpireOverdueBatch(t *testing.T) {
	db := openTestDB(t)
	s := NewSQLiteStore(db)
	ctx := context.Background()
	now := time.Now().UTC().UnixMilli()

	// Two stale, one fresh, one already-sent. Only the two stale ones
	// should expire.
	stale1, _ := s.Insert(ctx, sample("w1", "stale1", now-3600_000, now-1800_000))
	stale2, _ := s.Insert(ctx, sample("w1", "stale2", now-7200_000, now-3600_000))
	fresh, _ := s.Insert(ctx, sample("w1", "fresh", now+10_000, now+20_000))
	sent, _ := s.Insert(ctx, sample("w1", "sent", now-3600_000, now-1800_000))
	if err := s.MarkSent(ctx, sent, now); err != nil {
		t.Fatal(err)
	}

	expired, err := s.ExpireOverdue(ctx, now, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 2 {
		t.Fatalf("expired count = %d, want 2 (stale1+stale2); got ids=%v", len(expired), idsOf(expired))
	}
	gotIDs := map[string]bool{}
	for _, c := range expired {
		gotIDs[c.ID] = true
		if c.Status != StatusExpired {
			t.Errorf("snapshot status %q, want expired", c.Status)
		}
		if c.ExpiredAtMs == 0 {
			t.Errorf("snapshot expired_at_ms still zero")
		}
	}
	if !gotIDs[stale1] || !gotIDs[stale2] {
		t.Errorf("missing expected ids: stale1=%v stale2=%v gotIDs=%v", gotIDs[stale1], gotIDs[stale2], gotIDs)
	}

	// Fresh row stays pending; sent row stays sent.
	freshRow, _ := s.Get(ctx, fresh)
	if freshRow.Status != StatusPending {
		t.Errorf("fresh row flipped to %q", freshRow.Status)
	}
	sentRow, _ := s.Get(ctx, sent)
	if sentRow.Status != StatusSent {
		t.Errorf("sent row flipped to %q", sentRow.Status)
	}
}

func idsOf(cs []Commitment) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

type stubExtractor struct {
	candidates []Candidate
	err        error
}

func (s stubExtractor) Extract(_ context.Context, _ Turn) ([]Candidate, error) {
	return s.candidates, s.err
}
