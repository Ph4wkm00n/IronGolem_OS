package handler_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/connector"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/handler"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/planner"
	gwruntime "github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/runtime"
)

// findRuntimedBinary walks up from this test file until it finds the cargo
// target/debug/runtimed produced by `cargo build`. Skips the test if absent.
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
	t.Skip("runtimed binary not found; run `cargo build` first")
	return ""
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestHandleInbound_EndToEnd is the Step 5 smoke test: it boots the real
// runtimed binary, hooks the runtime client into the handler, and verifies
// that a synthesized 1-node plan completes with the mock provider's reply.
func TestHandleInbound_EndToEnd(t *testing.T) {
	bin := findRuntimedBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	logger := quietLogger()
	connMgr := connector.NewManager(logger)
	t.Cleanup(connMgr.DisconnectAll)

	client, err := gwruntime.New(ctx, gwruntime.Config{
		BinaryPath: bin,
		Env:        []string{"IRONGOLEM_LLM_PROVIDER=mock", "IRONGOLEM_LLM_MOCK_RESPONSE=hello from mock"},
	}, logger)
	if err != nil {
		t.Fatalf("runtime.New: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer closeCancel()
		_ = client.Close(closeCtx)
	})

	store := handler.NewInMemoryEventStore()
	h := handler.NewWithOptions(logger, connMgr, handler.Options{
		Runtime:    client,
		EventStore: store,
		// Bound the test tightly — mock provider should respond in ms.
		InboundTimeout: 5 * time.Second,
	})

	res, err := h.HandleInbound(ctx, planner.InboundMessage{
		ConnectorID: "telegram-test",
		ChannelID:   "chat-1",
		UserID:      "u-1",
		Content:     "ping",
		TenantID:    "default",
		WorkspaceID: "11111111-1111-1111-1111-111111111111",
	})
	if err != nil {
		t.Fatalf("HandleInbound: %v", err)
	}

	if res.Reply == "" {
		t.Fatalf("Reply was empty")
	}
	// MockProvider returns the IRONGOLEM_MOCK_RESPONSE value when set.
	if res.Reply != "hello from mock" {
		t.Fatalf("Reply: got %q, want %q", res.Reply, "hello from mock")
	}
	if res.InboundEventID == "" {
		t.Fatalf("InboundEventID should be set")
	}
}
