package runtime

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
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
