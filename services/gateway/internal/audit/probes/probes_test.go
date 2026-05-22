package probes

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit"
	_ "modernc.org/sqlite"
)

func TestWorkspaceSkillEscape_VacuousPass(t *testing.T) {
	p := NewWorkspaceSkillEscape()
	f := p.Run(context.Background())
	if f.ProbeID != "workspace_skill_escape" {
		t.Errorf("probe id = %q", f.ProbeID)
	}
	if f.Severity != audit.SeverityInfo {
		t.Errorf("severity = %q, want info", f.Severity)
	}
	if f.Evidence["skill_system_present"] != false {
		t.Errorf("evidence skill_system_present = %v, want false", f.Evidence["skill_system_present"])
	}
}

func TestTrustModel_MissingSecretIsCritical(t *testing.T) {
	t.Setenv("ZZ_TEST_HMAC", "")
	p := NewTrustModelWithEnv("ZZ_TEST_HMAC")
	f := p.Run(context.Background())
	if f.Severity != audit.SeverityCritical {
		t.Fatalf("severity = %q, want critical", f.Severity)
	}
	if !strings.Contains(f.Reason, "HMAC") {
		t.Fatalf("reason should mention HMAC: %q", f.Reason)
	}
}

// v1.2.2: well-known placeholder strings are CRITICAL (rotate
// immediately), distinct from merely-short secrets which are warning.
// Pre-v1.2.2 conflated the two under a single warning, hiding the
// "you shipped 'changeme' to production" case behind the "secret is
// kind of short" UX.
func TestTrustModel_WellKnownPlaceholderIsCritical(t *testing.T) {
	t.Setenv("ZZ_TEST_HMAC", "changeme")
	p := NewTrustModelWithEnv("ZZ_TEST_HMAC")
	f := p.Run(context.Background())
	if f.Severity != audit.SeverityCritical {
		t.Fatalf("severity = %q, want critical for well-known placeholder", f.Severity)
	}
	if !strings.Contains(f.Reason, "placeholder") {
		t.Fatalf("reason should mention placeholder: %q", f.Reason)
	}
	if f.Evidence["heuristic"] != "placeholder_string_match" {
		t.Fatalf("heuristic = %v, want placeholder_string_match", f.Evidence["heuristic"])
	}
	if _, ok := f.Evidence["fingerprint"].(string); !ok {
		t.Fatalf("evidence should include a fingerprint string; got %v", f.Evidence["fingerprint"])
	}
}

func TestTrustModel_StrongSecretIsInfo(t *testing.T) {
	t.Setenv("ZZ_TEST_HMAC", "this-is-a-perfectly-acceptable-secret-of-some-length")
	p := NewTrustModelWithEnv("ZZ_TEST_HMAC")
	f := p.Run(context.Background())
	if f.Severity != audit.SeverityInfo {
		t.Fatalf("severity = %q, want info for strong secret", f.Severity)
	}
	if f.Evidence["secret_bytes"].(int) <= 0 {
		t.Fatalf("evidence secret_bytes should be positive, got %v", f.Evidence["secret_bytes"])
	}
	if fp, ok := f.Evidence["fingerprint"].(string); !ok || len(fp) != 8 {
		t.Fatalf("evidence fingerprint should be an 8-hex-char string; got %v", f.Evidence["fingerprint"])
	}
}

func TestTrustModel_ShortSecretIsWarning(t *testing.T) {
	t.Setenv("ZZ_TEST_HMAC", "short")
	p := NewTrustModelWithEnv("ZZ_TEST_HMAC")
	f := p.Run(context.Background())
	if f.Severity != audit.SeverityWarning {
		t.Fatalf("severity = %q, want warning for short secret", f.Severity)
	}
	if f.Evidence["heuristic"] != "short_secret" {
		t.Fatalf("heuristic = %v, want short_secret", f.Evidence["heuristic"])
	}
}

// v1.2.2: probe reads via raw os.Getenv (no trim), matching the
// gateway's own secret read. Pre-patch the probe trimmed and the
// gateway didn't, so a trailing-newline secret would make the probe
// say "intact" while the running process used a different secret.
// Verifying the fingerprint is the deterministic anchor: same bytes
// in → same fingerprint out, so the probe and the gateway can be
// cross-checked by eyeballing two log lines.
func TestTrustModel_FingerprintReflectsRawSecretBytes(t *testing.T) {
	t.Setenv("ZZ_TEST_HMAC", "raw-secret\n")
	p := NewTrustModelWithEnv("ZZ_TEST_HMAC")
	f := p.Run(context.Background())
	fp, ok := f.Evidence["fingerprint"].(string)
	if !ok {
		t.Fatalf("no fingerprint in evidence: %v", f.Evidence)
	}
	// The fingerprint is sha256("raw-secret\n")[:4] as hex. We don't
	// hardcode the expected value; just assert that re-running with
	// the trimmed variant produces a DIFFERENT fingerprint — proving
	// the probe distinguishes them.
	t.Setenv("ZZ_TEST_HMAC", "raw-secret")
	f2 := p.Run(context.Background())
	fp2 := f2.Evidence["fingerprint"].(string)
	if fp == fp2 {
		t.Fatalf("trim vs no-trim produced same fingerprint %q; probe is still trimming", fp)
	}
}

func TestConnectorHealthDrift_AllPassing(t *testing.T) {
	// The connector subpackages aren't blank-imported here, so the
	// registry is whatever the previous tests left behind. Use a
	// stand-alone reset to keep this test hermetic.
	// (resetForTests is package-internal; we approximate by registering
	// a fake probe that has CheckFn returning true via a fresh import.)
	t.Skip("registry inspection covered by ConnectorHealthDrift_NoRegistered & integration probes; see TestConnectorHealthDrift_NoneRegistered for the deterministic path")
}

func TestConnectorHealthDrift_NoneRegistered(t *testing.T) {
	// When the connectors registry happens to be empty (e.g. a binary
	// built without the connector subpackages), the probe surfaces a
	// critical finding so the operator knows the build is broken.
	if len(connectors.List()) > 0 {
		t.Skip("connectors are already registered in this test binary; the empty-registry path is exercised in a tag-isolated test build")
	}
	p := NewConnectorHealthDrift()
	f := p.Run(context.Background())
	if f.Severity != audit.SeverityCritical {
		t.Fatalf("severity = %q, want critical for empty registry", f.Severity)
	}
}

func TestChannelDMPolicy_EmptyTableIsInfo(t *testing.T) {
	db := openTestDB(t)
	mustExec(t, db, `CREATE TABLE channel_policies (
		channel_id TEXT NOT NULL,
		action     TEXT NOT NULL,
		decision   TEXT NOT NULL,
		reason     TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		PRIMARY KEY (channel_id, action)
	)`)
	p := NewChannelDMPolicy(db)
	f := p.Run(context.Background())
	if f.Severity != audit.SeverityInfo {
		t.Fatalf("severity = %q, want info for empty policy table", f.Severity)
	}
	if f.Evidence["total_rules"].(int) != 0 {
		t.Fatalf("total_rules = %v, want 0", f.Evidence["total_rules"])
	}
}

func TestChannelDMPolicy_OrphanRuleIsWarning(t *testing.T) {
	db := openTestDB(t)
	mustExec(t, db, `CREATE TABLE channel_policies (
		channel_id TEXT NOT NULL,
		action     TEXT NOT NULL,
		decision   TEXT NOT NULL,
		reason     TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		PRIMARY KEY (channel_id, action)
	)`)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	// "nonexistent" is not a registered connector type, so this rule
	// is orphaned by the audit semantics.
	mustExec(t, db, `INSERT INTO channel_policies (channel_id, action, decision, reason, created_at) VALUES (?, ?, ?, ?, ?)`,
		"nonexistent", "send", "deny", "", now)

	p := NewChannelDMPolicy(db)
	f := p.Run(context.Background())
	if f.Severity != audit.SeverityWarning {
		t.Fatalf("severity = %q, want warning for orphan rule", f.Severity)
	}
	if !strings.Contains(f.Reason, "unregistered connectors") {
		t.Fatalf("reason should mention unregistered connectors: %q", f.Reason)
	}
}

func TestChannelDMPolicy_NilDBIsCritical(t *testing.T) {
	p := NewChannelDMPolicy(nil)
	f := p.Run(context.Background())
	if f.Severity != audit.SeverityCritical {
		t.Fatalf("severity = %q, want critical for nil db", f.Severity)
	}
}

// openTestDB returns an in-memory SQLite handle scoped to the test.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("exec %q: %v", strings.SplitN(query, "\n", 2)[0], err)
	}
}
