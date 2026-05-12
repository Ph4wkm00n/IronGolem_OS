package runtime

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"

	ipc "github.com/Ph4wkm00n/IronGolem_OS/services/pkg/runtime"
)

// findRuntimedBinary walks up from this test file until it finds the cargo
// target/debug/runtimed produced by `cargo build`. The test is skipped if
// the binary isn't present so contributors who haven't built the Rust
// workspace can still run `go test ./...`.
func findRuntimedBinary(t *testing.T) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("runtime.Caller unavailable")
	}
	dir := filepath.Dir(here)
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "target", "debug", "runtimed")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("runtimed binary not found; run `cargo build -p irongolem-runtimed` first")
	return ""
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestClient_PingRoundTrip spawns the real runtimed binary, issues a
// Ping, and asserts the response arrives. This is the Step 4 smoke test.
func TestClient_PingRoundTrip(t *testing.T) {
	bin := findRuntimedBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := New(ctx, Config{
		BinaryPath:  bin,
		Env:         []string{"IRONGOLEM_LLM_PROVIDER=mock"},
		PingTimeout: 5 * time.Second,
	}, newTestLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer closeCancel()
		_ = client.Close(closeCtx)
	})

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

// TestClient_PingAfterClose proves the client refuses work once closed.
func TestClient_PingAfterClose(t *testing.T) {
	bin := findRuntimedBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := New(ctx, Config{
		BinaryPath: bin,
		Env:        []string{"IRONGOLEM_LLM_PROVIDER=mock"},
	}, newTestLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer closeCancel()
	if err := client.Close(closeCtx); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := client.Ping(ctx); err == nil {
		t.Fatal("expected error pinging after Close, got nil")
	}
}

// TestClient_ExecuteEcho is the Step 8 Gate 2 linchpin test. It spawns the
// real runtimed binary, sends an ExecutePlanRequest with a single ToolCall
// node invoking the `echo` built-in, and verifies:
//
//  1. The terminal response's output equals the echo input verbatim.
//  2. The streamed events include PlanCreated → PlanStepStarted →
//     PlanStepCompleted → PlanCompleted, in that order.
//
// This proves the contract (IPC types), the binary (runtimed), the registry
// (sandbox tool dispatch), and the executor wiring all work without going
// near Telegram or any LLM provider — exactly the layering the v0.1 plan
// asks for. Failure modes here are unambiguous: bad NDJSON framing, bad
// event ordering, bad output decoding all surface as named failures rather
// than a single opaque "didn't work".
func TestClient_ExecuteEcho(t *testing.T) {
	bin := findRuntimedBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := New(ctx, Config{
		BinaryPath: bin,
		Env:        []string{"IRONGOLEM_LLM_PROVIDER=mock"},
	}, newTestLogger())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer closeCancel()
		_ = client.Close(closeCtx)
	})

	input := json.RawMessage(`{"hello":"world"}`)
	now := time.Now().UTC()
	plan := ipc.Plan{
		ID:          uuid.NewString(),
		Description: "step-8 gate-2 echo plan",
		AgentID:     uuid.NewString(),
		Status:      ipc.PlanStatusPending,
		Risk:        ipc.DefaultRisk(),
		CreatedAt:   now,
		UpdatedAt:   now,
		Nodes: []ipc.PlanNode{{
			ID:           uuid.NewString(),
			Description:  "echo back the input",
			Status:       ipc.NodeStatusPending,
			Dependencies: []string{},
			Risk:         ipc.DefaultRisk(),
			Kind: ipc.PlanNodeKind{
				Type:     ipc.NodeKindToolCall,
				ToolName: "echo",
				Input:    input,
			},
		}},
	}

	res, err := client.Execute(ctx, uuid.NewString(), plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Drain events into a slice — the engine emits them after plan
	// completion, so they arrive before or near the terminal response.
	var events []string
	done := false
	for !done {
		select {
		case evt, ok := <-res.Events:
			if !ok {
				done = true
				continue
			}
			var k struct {
				Kind struct{ Type string } `json:"kind"`
			}
			if err := json.Unmarshal(evt.Event, &k); err != nil {
				t.Fatalf("bad event payload: %v (%s)", err, evt.Event)
			}
			events = append(events, k.Kind.Type)
		case terminal, ok := <-res.Done:
			if !ok {
				t.Fatal("Done channel closed before terminal response")
			}
			if terminal.Err != nil {
				t.Fatalf("terminal error: %v", terminal.Err)
			}
			if terminal.Response == nil {
				t.Fatal("nil terminal response")
			}
			if terminal.Response.Status != ipc.StatusCompleted {
				t.Fatalf("plan status: got %q error=%q", terminal.Response.Status, terminal.Response.Error)
			}
			// runtimed packs the last node's output as the plan's output.
			// The echo tool returns its input verbatim.
			if !jsonEqual(terminal.Response.Output, input) {
				t.Fatalf("output mismatch: got %s want %s", terminal.Response.Output, input)
			}
			// Drain any remaining events queued before the terminal.
			for evt := range res.Events {
				var k struct {
					Kind struct{ Type string } `json:"kind"`
				}
				if err := json.Unmarshal(evt.Event, &k); err == nil {
					events = append(events, k.Kind.Type)
				}
			}
			done = true
			_ = ok
		case <-ctx.Done():
			t.Fatalf("test timed out: %v", ctx.Err())
		}
	}

	// Assert the four canonical events appear in order. The runtime may
	// emit extras (Checkpoint, etc.); we only require the prescribed
	// ordering of these four.
	required := []string{"PlanCreated", "PlanStepStarted", "PlanStepCompleted", "PlanCompleted"}
	pos := 0
	for _, e := range events {
		if pos < len(required) && e == required[pos] {
			pos++
		}
	}
	if pos != len(required) {
		t.Fatalf("event ordering: matched %d/%d of %v in stream %v",
			pos, len(required), required, events)
	}
}

// jsonEqual compares two JSON payloads structurally so cosmetic whitespace
// differences don't fail the test.
func jsonEqual(a, b json.RawMessage) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	ab, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return string(ab) == string(bb)
}
