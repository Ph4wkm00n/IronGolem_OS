package probes

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit"
	_ "modernc.org/sqlite"
)

func openPolicyDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE channel_policies (
		channel_id TEXT NOT NULL, action TEXT NOT NULL, decision TEXT NOT NULL,
		reason TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL,
		PRIMARY KEY (channel_id, action))`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func insertPolicy(t *testing.T, db *sql.DB, channel, action, decision string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO channel_policies (channel_id, action, decision, created_at) VALUES (?, ?, ?, '2026-07-02')`,
		channel, action, decision,
	); err != nil {
		t.Fatalf("insert policy: %v", err)
	}
}

func TestExposureComposition(t *testing.T) {
	tests := []struct {
		name    string
		seed    func(t *testing.T, db *sql.DB)
		wantSev audit.Severity
	}{
		{
			name:    "no permissive policies is info",
			seed:    func(t *testing.T, db *sql.DB) { insertPolicy(t, db, "tg-1", "execute", "deny") },
			wantSev: audit.SeverityInfo,
		},
		{
			name:    "channel allowing write is warning",
			seed:    func(t *testing.T, db *sql.DB) { insertPolicy(t, db, "slack-1", "write", "allow") },
			wantSev: audit.SeverityWarning,
		},
		{
			name: "channel allowing execute is critical even when others are clean",
			seed: func(t *testing.T, db *sql.DB) {
				insertPolicy(t, db, "tg-1", "execute", "deny")
				insertPolicy(t, db, "discord-1", "execute", "allow")
			},
			wantSev: audit.SeverityCritical,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openPolicyDB(t)
			tt.seed(t, db)
			f := NewExposureComposition(db).Run(context.Background())
			if f.Severity != tt.wantSev {
				t.Fatalf("severity = %s, want %s (reason: %s)", f.Severity, tt.wantSev, f.Reason)
			}
		})
	}

	t.Run("nil db is critical", func(t *testing.T) {
		f := NewExposureComposition(nil).Run(context.Background())
		if f.Severity != audit.SeverityCritical {
			t.Fatalf("severity = %s, want critical", f.Severity)
		}
	})
}

func TestFSPermissions(t *testing.T) {
	tests := []struct {
		name     string
		dirMode  os.FileMode
		fileMode os.FileMode
		wantSev  audit.Severity
	}{
		{"tight perms are info", 0o700, 0o600, audit.SeverityInfo},
		{"world-readable file is critical", 0o700, 0o644, audit.SeverityCritical},
		{"world-writable file is critical", 0o700, 0o666, audit.SeverityCritical},
		{"group-writable file is warning", 0o700, 0o620, audit.SeverityWarning},
		{"world-readable dir is warning", 0o755, 0o600, audit.SeverityWarning},
		{"world-writable dir is critical", 0o777, 0o600, audit.SeverityCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "state")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			dbPath := filepath.Join(dir, "gateway.db")
			if err := os.WriteFile(dbPath, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
			// chmod after creation — umask would strip bits otherwise.
			if err := os.Chmod(dbPath, tt.fileMode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dir, tt.dirMode); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

			f := NewFSPermissions(dbPath).Run(context.Background())
			if f.Severity != tt.wantSev {
				t.Fatalf("severity = %s, want %s (reason: %s, evidence: %v)",
					f.Severity, tt.wantSev, f.Reason, f.Evidence)
			}
		})
	}

	t.Run("missing db file is info", func(t *testing.T) {
		f := NewFSPermissions(filepath.Join(t.TempDir(), "nope", "gateway.db")).Run(context.Background())
		if f.Severity != audit.SeverityInfo {
			t.Fatalf("severity = %s, want info", f.Severity)
		}
	})
}

func TestGatewayExposure(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		secret  string
		wantSev audit.Severity
	}{
		{"loopback with secret is info", "127.0.0.1:8080", "a-perfectly-fine-secret", audit.SeverityInfo},
		{"localhost hostname is info", "localhost:8080", "a-perfectly-fine-secret", audit.SeverityInfo},
		{"ipv6 loopback is info", "[::1]:8080", "a-perfectly-fine-secret", audit.SeverityInfo},
		{"all-interfaces default is warning", "", "a-perfectly-fine-secret", audit.SeverityWarning},
		{"explicit 0.0.0.0 is warning", "0.0.0.0:8080", "a-perfectly-fine-secret", audit.SeverityWarning},
		{"empty secret is critical regardless of bind", "127.0.0.1:8080", "", audit.SeverityCritical},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GATEWAY_ADDR", tt.addr)
			t.Setenv("IRONGOLEM_HMAC_SECRET", tt.secret)
			f := NewGatewayExposure().Run(context.Background())
			if f.Severity != tt.wantSev {
				t.Fatalf("severity = %s, want %s (reason: %s)", f.Severity, tt.wantSev, f.Reason)
			}
		})
	}
}
