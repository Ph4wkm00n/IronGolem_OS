// Package persist owns the single *sql.DB the gateway uses for its
// SQLite-backed stores (events, recipes, approvals, squads). Step 6 of
// the v0.1 plan flipped these stores from in-memory to durable; this
// package centralizes connection setup + schema migration so each store
// only has to consume an already-open handle.
package persist

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	// Pure-Go SQLite driver (no cgo). Registered for side effects.
	_ "modernc.org/sqlite"
)

// DefaultDBPath is the canonical gateway database location per the v0.1
// plan. Override via IRONGOLEM_GATEWAY_DB at boot for tests / multi-instance
// runs.
const DefaultDBPath = "~/.irongolem/gateway.db"

// Open expands `path` (handling `~` and relative paths), ensures the
// parent directory exists, opens a SQLite connection, runs the gateway
// schema, and returns a ready-to-use *sql.DB.
//
// Use `:memory:` for tests — directory creation is skipped automatically.
func Open(path string) (*sql.DB, error) {
	if path == "" {
		path = DefaultDBPath
	}
	resolved, err := expandPath(path)
	if err != nil {
		return nil, fmt.Errorf("persist: resolve path: %w", err)
	}

	if resolved != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
			return nil, fmt.Errorf("persist: mkdir %s: %w", filepath.Dir(resolved), err)
		}
	}

	// `_pragma=` tunes for the gateway's mixed read/write load: WAL keeps
	// writers from blocking readers; foreign_keys gives us trustworthy
	// FK constraints; busy_timeout prevents transient SQLITE_BUSY on
	// concurrent goroutines.
	dsn := resolved + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("persist: sql.Open: %w", err)
	}
	// Cap pool size — SQLite serializes writers, so a deep pool wastes
	// connections. One writer + a handful of readers is plenty for v0.1.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("persist: ping: %w", err)
	}
	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("persist: migrate: %w", err)
	}
	return db, nil
}

// Migrate runs the gateway schema. Idempotent — every CREATE uses
// IF NOT EXISTS. The handler-store files own their per-table operations;
// this file owns the canonical schema so a single grep finds it.
func Migrate(db *sql.DB) error {
	stmts := []string{
		// Events: append-only audit log feeding the timeline view.
		// Payload + metadata are stored as JSON strings so the column set
		// stays stable as events.Event grows new fields.
		`CREATE TABLE IF NOT EXISTS gateway_events (
			id              TEXT PRIMARY KEY NOT NULL,
			kind            TEXT NOT NULL,
			tenant_id       TEXT NOT NULL DEFAULT '',
			workspace_id    TEXT NOT NULL DEFAULT '',
			source_service  TEXT NOT NULL DEFAULT '',
			correlation_id  TEXT NOT NULL DEFAULT '',
			causation_id    TEXT NOT NULL DEFAULT '',
			ts              TEXT NOT NULL,
			payload         TEXT NOT NULL DEFAULT '',
			metadata        TEXT NOT NULL DEFAULT '',
			version         INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE INDEX IF NOT EXISTS idx_gateway_events_ts ON gateway_events(ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_gateway_events_workspace ON gateway_events(workspace_id, ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_gateway_events_kind ON gateway_events(kind, ts DESC)`,

		// Recipes: persisted user/builtin recipe library. We store the
		// full DetailedRecipe as JSON so its rich nested shape doesn't
		// fight the relational model — Step 6's job is durability, not
		// query optimization. Step 7+ can normalize where it helps.
		`CREATE TABLE IF NOT EXISTS gateway_recipes (
			id          TEXT PRIMARY KEY NOT NULL,
			is_active   INTEGER NOT NULL DEFAULT 0,
			created_at  TEXT NOT NULL,
			updated_at  TEXT NOT NULL,
			body        TEXT NOT NULL,
			seq         INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_gateway_recipes_seq ON gateway_recipes(seq)`,

		// Approvals: pending/approved/denied requests.
		`CREATE TABLE IF NOT EXISTS gateway_approvals (
			id            TEXT PRIMARY KEY NOT NULL,
			status        TEXT NOT NULL,
			body          TEXT NOT NULL,
			created_at    TEXT NOT NULL,
			updated_at    TEXT NOT NULL,
			seq           INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_gateway_approvals_status ON gateway_approvals(status, seq)`,

		// Squads: built-in templates persist as rows; users can create
		// custom ones via POST /squads.
		`CREATE TABLE IF NOT EXISTS gateway_squads (
			id          TEXT PRIMARY KEY NOT NULL,
			is_active   INTEGER NOT NULL DEFAULT 0,
			body        TEXT NOT NULL,
			created_at  TEXT NOT NULL,
			updated_at  TEXT NOT NULL,
			seq         INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_gateway_squads_seq ON gateway_squads(seq)`,
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate stmt %q: %w", firstLine(stmt), err)
		}
	}
	return tx.Commit()
}

func expandPath(p string) (string, error) {
	if p == ":memory:" {
		return p, nil
	}
	if len(p) >= 2 && p[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[2:])
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' || r == '\r' {
			return s[:i]
		}
	}
	if len(s) > 60 {
		return s[:60]
	}
	return s
}
