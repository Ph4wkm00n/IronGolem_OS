package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/handler"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/persist"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
)

func quiet() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestSQLitePersistence_RestartSurvival proves Step 6's whole point: an
// event written before "shutdown" survives a fresh process boot against
// the same DB file.
func TestSQLitePersistence_RestartSurvival(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "gateway-test.db")

	// First "boot": write one event, then close the DB.
	{
		db, err := persist.Open(dbPath)
		if err != nil {
			t.Fatalf("first open: %v", err)
		}
		store := handler.NewSQLiteEventStore(db, quiet())
		evt := events.NewEvent(events.EventKindMessageInbound, "t", "gw", json.RawMessage(`{"q":"hello"}`))
		evt.WorkspaceID = "ws-persist"
		store.Append(evt)
		if err := db.Close(); err != nil {
			t.Fatalf("first close: %v", err)
		}
	}

	// Second "boot": reopen the same file and confirm the event is there.
	db, err := persist.Open(dbPath)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer db.Close()
	store := handler.NewSQLiteEventStore(db, quiet())

	page, total := store.List(1, 10, "ws-persist", "")
	if total != 1 || len(page) != 1 {
		t.Fatalf("event did not survive restart: total=%d len=%d", total, len(page))
	}
	if string(page[0].Payload) != `{"q":"hello"}` {
		t.Fatalf("payload mismatch after restart: got %q", page[0].Payload)
	}
}
