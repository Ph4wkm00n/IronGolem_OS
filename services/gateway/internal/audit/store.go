package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// FindingStore persists Findings and lets the handler page through them.
type FindingStore interface {
	// Insert writes a single finding, generating its ID + timestamp if
	// the caller didn't supply them. Returns the persisted ID.
	Insert(ctx context.Context, f Finding) (string, error)
	// List returns findings ordered newest-first. severity filters by
	// minimum urgency ("" returns all). limit caps the row count.
	List(ctx context.Context, severity Severity, limit int) ([]StoredFinding, error)
}

// StoredFinding is a Finding plus its database identity (ID, persisted
// timestamp). Handlers consume this shape so the UI can drill down by ID.
type StoredFinding struct {
	ID       string `json:"id"`
	Finding  `json:",inline"`
	StoredAt time.Time `json:"stored_at"`
}

// MarshalJSON inlines the Finding fields at the top level instead of
// nesting them under "Finding" (which Go's default tag-based encoder
// would do because of the embedded field). The UI shape stays flat —
// matching how the handler used to serialize unstored Findings.
func (s StoredFinding) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ID        string         `json:"id"`
		ProbeID   string         `json:"probe_id"`
		Severity  Severity       `json:"severity"`
		Reason    string         `json:"reason"`
		Evidence  map[string]any `json:"evidence,omitempty"`
		Timestamp time.Time      `json:"timestamp"`
		StoredAt  time.Time      `json:"stored_at"`
	}{
		ID:        s.ID,
		ProbeID:   s.Finding.ProbeID,
		Severity:  s.Finding.Severity,
		Reason:    s.Finding.Reason,
		Evidence:  s.Finding.Evidence,
		Timestamp: s.Finding.Timestamp,
		StoredAt:  s.StoredAt,
	})
}

// SQLiteFindingStore persists findings to the gateway's shared *sql.DB.
// Schema lives in `services/gateway/internal/persist/db.go` so a single
// grep finds every gateway table.
type SQLiteFindingStore struct {
	db *sql.DB
}

// NewSQLiteFindingStore wraps a shared *sql.DB. The caller owns lifetime.
func NewSQLiteFindingStore(db *sql.DB) *SQLiteFindingStore {
	return &SQLiteFindingStore{db: db}
}

func (s *SQLiteFindingStore) Insert(ctx context.Context, f Finding) (string, error) {
	if f.ProbeID == "" {
		return "", fmt.Errorf("audit insert: probe_id required")
	}
	if !f.Severity.Valid() {
		return "", fmt.Errorf("audit insert: invalid severity %q", f.Severity)
	}
	if f.Timestamp.IsZero() {
		f.Timestamp = time.Now().UTC()
	}

	// v1.2.2: writes still emit "{}" for empty evidence so existing
	// SQLite files migrated from v1.2.0 / v1.2.1 (where the column is
	// NOT NULL DEFAULT '{}') keep accepting writes without an ALTER
	// TABLE. The reader side, however, now uses sql.NullString and
	// treats both NULL and the empty-marker string as "no evidence"
	// — that's the half that catches accidental writer-reader drift
	// when a future schema change relaxes the column.
	evidenceJSON := "{}"
	if len(f.Evidence) > 0 {
		raw, err := json.Marshal(f.Evidence)
		if err != nil {
			return "", fmt.Errorf("audit insert: marshal evidence: %w", err)
		}
		evidenceJSON = string(raw)
	}

	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO gateway_audit_findings (id, probe_id, severity, reason, evidence, ts, stored_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, id, f.ProbeID, string(f.Severity), f.Reason, evidenceJSON, f.Timestamp.UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return "", fmt.Errorf("audit insert: %w", err)
	}
	return id, nil
}

func (s *SQLiteFindingStore) List(ctx context.Context, severity Severity, limit int) ([]StoredFinding, error) {
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT id, probe_id, severity, reason, evidence, ts, stored_at FROM gateway_audit_findings`
	args := []any{}
	switch severity {
	case "":
		// no filter
	case SeverityWarning:
		q += ` WHERE severity IN (?, ?)`
		args = append(args, string(SeverityWarning), string(SeverityCritical))
	case SeverityCritical:
		q += ` WHERE severity = ?`
		args = append(args, string(SeverityCritical))
	case SeverityInfo:
		// info-and-above is "everything" — no filter needed
	default:
		return nil, fmt.Errorf("audit list: invalid severity filter %q", severity)
	}
	q += ` ORDER BY stored_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("audit list: %w", err)
	}
	defer rows.Close()

	var out []StoredFinding
	for rows.Next() {
		var (
			f        StoredFinding
			evidence sql.NullString
			ts       string
			storedAt string
			severity string
		)
		if err := rows.Scan(&f.ID, &f.Finding.ProbeID, &severity, &f.Finding.Reason, &evidence, &ts, &storedAt); err != nil {
			return nil, fmt.Errorf("audit list scan: %w", err)
		}
		f.Finding.Severity = Severity(severity)
		// v1.2.2: sql.NullString tolerates both wire conventions —
		// NULL (a future schema relaxation) and the legacy "{}"
		// sentinel that the v0.3 writer emits to satisfy NOT NULL.
		// Either reads as "no evidence" with no error. Pre-v1.2.2
		// the reader scanned into a plain string, which would panic
		// on NULL the day someone migrated the column.
		if evidence.Valid && evidence.String != "" && evidence.String != "{}" {
			if err := json.Unmarshal([]byte(evidence.String), &f.Finding.Evidence); err != nil {
				// Evidence is best-effort; surface the parse error to the
				// caller via the Evidence map instead of failing the
				// whole list.
				f.Finding.Evidence = map[string]any{"_parse_error": err.Error(), "_raw": evidence.String}
			}
		}
		f.Finding.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		f.StoredAt, _ = time.Parse(time.RFC3339Nano, storedAt)
		out = append(out, f)
	}
	return out, rows.Err()
}

// ParseSeverity converts a query-string value to a Severity, returning
// "" when the input is empty (i.e. "no filter").
func ParseSeverity(raw string) (Severity, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "", nil
	}
	s := Severity(raw)
	if !s.Valid() {
		return "", fmt.Errorf("invalid severity %q (want info|warning|critical)", raw)
	}
	return s, nil
}
