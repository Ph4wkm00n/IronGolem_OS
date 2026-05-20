package connectors

import (
	"strings"
	"testing"
)

func TestRegister_RejectsMissingType(t *testing.T) {
	resetForTests()
	err := Register(Registration{CheckFn: func() bool { return true }})
	if err == nil || !strings.Contains(err.Error(), "Type is required") {
		t.Fatalf("expected 'Type is required' error, got %v", err)
	}
}

func TestRegister_RejectsMissingCheckFn(t *testing.T) {
	resetForTests()
	err := Register(Registration{Type: "fake"})
	if err == nil || !strings.Contains(err.Error(), "missing CheckFn") {
		t.Fatalf("expected 'missing CheckFn' error, got %v", err)
	}
}

func TestRegister_DefaultsSourceToBuiltin(t *testing.T) {
	resetForTests()
	if err := Register(Registration{Type: "fake", CheckFn: func() bool { return true }}); err != nil {
		t.Fatalf("register: %v", err)
	}
	r, ok := Get("fake")
	if !ok {
		t.Fatal("get fake: not found")
	}
	if r.Source != SourceBuiltin {
		t.Fatalf("source = %q, want %q", r.Source, SourceBuiltin)
	}
}

func TestRegister_DuplicateFails(t *testing.T) {
	resetForTests()
	r := Registration{Type: "fake", CheckFn: func() bool { return true }}
	if err := Register(r); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := Register(r); err == nil || !strings.Contains(err.Error(), "duplicate registration") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestMustRegister_PanicsOnDuplicate(t *testing.T) {
	resetForTests()
	r := Registration{Type: "fake", CheckFn: func() bool { return true }}
	MustRegister(r)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate MustRegister")
		}
	}()
	MustRegister(r)
}

func TestList_SortedByType(t *testing.T) {
	resetForTests()
	for _, typ := range []ConnectorType{"zulip", "alpha", "matrix"} {
		MustRegister(Registration{Type: typ, CheckFn: func() bool { return true }})
	}
	got := List()
	if len(got) != 3 {
		t.Fatalf("List len = %d, want 3", len(got))
	}
	want := []ConnectorType{"alpha", "matrix", "zulip"}
	for i, r := range got {
		if r.Type != want[i] {
			t.Fatalf("List[%d].Type = %q, want %q", i, r.Type, want[i])
		}
	}
}

func TestGet_NotRegistered(t *testing.T) {
	resetForTests()
	if _, ok := Get("missing"); ok {
		t.Fatal("Get returned ok=true for unregistered type")
	}
}
