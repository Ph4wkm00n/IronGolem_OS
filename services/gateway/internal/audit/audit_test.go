package audit

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// stubProbe is a Probe whose behavior is fully controlled by test code.
type stubProbe struct {
	id       string
	finding  Finding
	panics   bool
	blockFor time.Duration
	calls    int
	mu       sync.Mutex
}

func (s *stubProbe) ID() string { return s.id }

func (s *stubProbe) Run(ctx context.Context) Finding {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.panics {
		panic("stub probe boom")
	}
	if s.blockFor > 0 {
		select {
		case <-time.After(s.blockFor):
		case <-ctx.Done():
		}
	}
	return s.finding
}

// stubStore is a FindingStore that captures inserts in memory.
type stubStore struct {
	mu        sync.Mutex
	inserts   []Finding
	insertErr error
}

func (s *stubStore) Insert(_ context.Context, f Finding) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.insertErr != nil {
		return "", s.insertErr
	}
	s.inserts = append(s.inserts, f)
	return "stub-id", nil
}

func (s *stubStore) List(_ context.Context, _ Severity, _ int) ([]StoredFinding, error) {
	return nil, nil
}

// stubEmitter captures emitted findings.
type stubEmitter struct {
	mu      sync.Mutex
	emitted []StoredFinding
	emitErr error
}

func (s *stubEmitter) EmitAuditFinding(_ context.Context, f StoredFinding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.emitErr != nil {
		return s.emitErr
	}
	s.emitted = append(s.emitted, f)
	return nil
}

func TestSeverity_Valid(t *testing.T) {
	for _, ok := range []Severity{SeverityInfo, SeverityWarning, SeverityCritical} {
		if !ok.Valid() {
			t.Errorf("%q should be valid", ok)
		}
	}
	for _, bad := range []Severity{"", "fatal", "INFO", "warn"} {
		if Severity(bad).Valid() {
			t.Errorf("%q should be invalid", bad)
		}
	}
}

func TestRegistry_RegisterAndList(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(&stubProbe{id: "b", finding: Finding{ProbeID: "b", Severity: SeverityInfo}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(&stubProbe{id: "a", finding: Finding{ProbeID: "a", Severity: SeverityInfo}}); err != nil {
		t.Fatal(err)
	}
	probes := r.List()
	if len(probes) != 2 || probes[0].ID() != "a" || probes[1].ID() != "b" {
		t.Fatalf("List should be sorted: got %v", []string{probes[0].ID(), probes[1].ID()})
	}
}

func TestRegistry_RejectsDuplicate(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&stubProbe{id: "x", finding: Finding{ProbeID: "x", Severity: SeverityInfo}})
	err := r.Register(&stubProbe{id: "x", finding: Finding{ProbeID: "x", Severity: SeverityInfo}})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestRegistry_RejectsEmptyID(t *testing.T) {
	r := NewRegistry()
	err := r.Register(&stubProbe{id: "", finding: Finding{Severity: SeverityInfo}})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestRuntime_TickEmitsNonInfoFindings(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&stubProbe{id: "ok", finding: Finding{ProbeID: "ok", Severity: SeverityInfo, Reason: "all good"}})
	r.MustRegister(&stubProbe{id: "warn", finding: Finding{ProbeID: "warn", Severity: SeverityWarning, Reason: "drift"}})
	r.MustRegister(&stubProbe{id: "crit", finding: Finding{ProbeID: "crit", Severity: SeverityCritical, Reason: "broken"}})

	store := &stubStore{}
	emitter := &stubEmitter{}
	rt := NewRuntime(r, store, emitter, nil, RuntimeConfig{ProbeTimeout: time.Second})

	rt.tick(context.Background())

	// All three findings persisted.
	if len(store.inserts) != 3 {
		t.Fatalf("expected 3 inserts, got %d", len(store.inserts))
	}
	// Only non-info findings emitted to the timeline.
	if len(emitter.emitted) != 2 {
		t.Fatalf("expected 2 emitted findings (warn+crit), got %d", len(emitter.emitted))
	}
	emittedIDs := map[string]bool{}
	for _, e := range emitter.emitted {
		emittedIDs[e.Finding.ProbeID] = true
	}
	if !emittedIDs["warn"] || !emittedIDs["crit"] {
		t.Fatalf("expected warn+crit emitted, got %v", emittedIDs)
	}
	if emittedIDs["ok"] {
		t.Fatal("info findings should not emit timeline events")
	}
}

func TestRuntime_PanicConvertsToCriticalFinding(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&stubProbe{id: "boom", panics: true})

	store := &stubStore{}
	emitter := &stubEmitter{}
	rt := NewRuntime(r, store, emitter, nil, RuntimeConfig{ProbeTimeout: time.Second})

	rt.tick(context.Background())

	if len(store.inserts) != 1 {
		t.Fatalf("expected 1 insert from panicking probe, got %d", len(store.inserts))
	}
	got := store.inserts[0]
	if got.Severity != SeverityCritical {
		t.Fatalf("panic should produce critical finding, got %q", got.Severity)
	}
	if !strings.Contains(got.Reason, "panic") {
		t.Fatalf("reason should mention panic, got %q", got.Reason)
	}
	if len(emitter.emitted) != 1 {
		t.Fatalf("critical finding should emit one event, got %d", len(emitter.emitted))
	}
}

func TestRuntime_InsertErrorDoesNotEmit(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&stubProbe{id: "warn", finding: Finding{ProbeID: "warn", Severity: SeverityWarning, Reason: "x"}})
	store := &stubStore{insertErr: errors.New("disk full")}
	emitter := &stubEmitter{}
	rt := NewRuntime(r, store, emitter, nil, RuntimeConfig{ProbeTimeout: time.Second})
	rt.tick(context.Background())
	if len(emitter.emitted) != 0 {
		t.Fatalf("emit should be skipped when insert fails, got %d emitted", len(emitter.emitted))
	}
}

func TestRuntime_NilEmitterIsAllowed(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&stubProbe{id: "warn", finding: Finding{ProbeID: "warn", Severity: SeverityWarning}})
	store := &stubStore{}
	rt := NewRuntime(r, store, nil, nil, RuntimeConfig{ProbeTimeout: time.Second})
	rt.tick(context.Background()) // must not panic
	if len(store.inserts) != 1 {
		t.Fatalf("expected 1 insert with nil emitter, got %d", len(store.inserts))
	}
}

func TestRuntime_DefaultsApplied(t *testing.T) {
	rt := NewRuntime(NewRegistry(), &stubStore{}, nil, nil, RuntimeConfig{})
	if rt.cfg.Interval != defaultInterval {
		t.Errorf("default interval not applied: %v", rt.cfg.Interval)
	}
	if rt.cfg.ProbeTimeout != defaultProbeTimeout {
		t.Errorf("default probe timeout not applied: %v", rt.cfg.ProbeTimeout)
	}
}

func TestRuntime_InvalidSeverityNormalized(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&stubProbe{id: "bad", finding: Finding{ProbeID: "bad", Severity: Severity("fatal"), Reason: "x"}})
	store := &stubStore{}
	rt := NewRuntime(r, store, nil, nil, RuntimeConfig{ProbeTimeout: time.Second})
	rt.tick(context.Background())
	if len(store.inserts) != 1 {
		t.Fatalf("expected 1 insert, got %d", len(store.inserts))
	}
	if store.inserts[0].Severity != SeverityCritical {
		t.Fatalf("invalid severity should normalize to critical, got %q", store.inserts[0].Severity)
	}
}

func TestParseSeverity(t *testing.T) {
	cases := []struct {
		in   string
		want Severity
		err  bool
	}{
		{"", "", false},
		{"info", SeverityInfo, false},
		{"WARNING", SeverityWarning, false},
		{"  critical  ", SeverityCritical, false},
		{"fatal", "", true},
	}
	for _, c := range cases {
		got, err := ParseSeverity(c.in)
		if c.err && err == nil {
			t.Errorf("%q should error", c.in)
			continue
		}
		if !c.err && err != nil {
			t.Errorf("%q returned unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q -> %q, want %q", c.in, got, c.want)
		}
	}
}
