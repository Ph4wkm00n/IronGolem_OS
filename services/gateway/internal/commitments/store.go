package commitments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Store is the persistence interface the rest of the package consumes.
// Surface kept narrow on purpose — the runtime + handler should never
// reach past these methods into raw SQL.
type Store interface {
	// Insert persists a new pending commitment, returning its assigned ID.
	// Dedup: returns the existing row's ID (without insert) when a pending
	// row with the same workspace+dedupe_key already exists.
	Insert(ctx context.Context, c Commitment) (string, error)
	// Get returns one commitment by ID. Returns ErrNotFound when missing.
	Get(ctx context.Context, id string) (Commitment, error)
	// List returns commitments matching the filter, newest-first.
	List(ctx context.Context, filter ListFilter) ([]Commitment, error)
	// DueNow returns pending commitments whose window is currently open.
	// nowMs is the comparison instant — passed in so tests stay determinstic.
	DueNow(ctx context.Context, nowMs int64, limit int) ([]Commitment, error)
	// MarkSent flips status to sent and stamps sent_at_ms.
	MarkSent(ctx context.Context, id string, sentAtMs int64) error
	// MarkDismissed flips status to dismissed.
	MarkDismissed(ctx context.Context, id string) error
	// MarkSnoozed sets status=snoozed + snoozed_until_ms.
	MarkSnoozed(ctx context.Context, id string, untilMs int64) error
	// MarkExpired sets status=expired.
	MarkExpired(ctx context.Context, id string) error
	// ExpireOverdue flips every pending row whose latest_ms < cutoffMs
	// to expired in a single UPDATE. Returns the IDs that transitioned
	// so the caller can emit lifecycle events. limit caps the batch so
	// a giant backlog can't wedge a single tick.
	ExpireOverdue(ctx context.Context, cutoffMs int64, limit int) ([]Commitment, error)
	// Delete hard-removes a row. Admin-only.
	Delete(ctx context.Context, id string) error
}

// ListFilter narrows a List query.
type ListFilter struct {
	WorkspaceID string
	Status      CommitmentStatus // empty = all
	Limit       int              // default 100, capped at 1000
}

// ErrNotFound is returned by Get / mark methods when no row matches.
var ErrNotFound = errors.New("commitment not found")

// ErrDeduped is returned by Insert when a pending row with the same
// (workspace_id, dedupe_key) already exists. The returned id refers to
// the EXISTING row, not a newly-inserted one. Callers that care about
// emitting "extracted" lifecycle events MUST check `errors.Is(err,
// ErrDeduped)` and skip the emission — otherwise duplicate extraction
// of the same turn spams the timeline with phantom "extracted" events
// for a commitment that was already there.
//
// Backward compatible: callers that only want the id can ignore the
// error (the id field is always valid on dedupe-hit). The error exists
// purely so emission-side callers can distinguish the two outcomes.
var ErrDeduped = errors.New("commitment deduped against existing pending row")

// SQLiteStore is the persistent Store impl backed by the gateway's
// shared *sql.DB.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore wraps a shared *sql.DB. The caller owns lifetime;
// schema is provisioned by `services/gateway/internal/persist/db.go`.
func NewSQLiteStore(db *sql.DB) *SQLiteStore { return &SQLiteStore{db: db} }

const insertSQL = `
INSERT INTO gateway_commitments (
	id, workspace_id, tenant_id, kind, sensitivity, status, reason,
	suggested_text, dedupe_key, confidence,
	earliest_ms, latest_ms, timezone,
	connector_id, channel_id, user_id, source_event_id,
	created_at_ms, updated_at_ms, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`

const selectColumns = `
id, workspace_id, tenant_id, kind, sensitivity, status, reason,
suggested_text, dedupe_key, confidence,
earliest_ms, latest_ms, timezone,
connector_id, channel_id, user_id, source_event_id,
created_at_ms, updated_at_ms,
COALESCE(sent_at_ms, 0), COALESCE(dismissed_at_ms, 0),
COALESCE(snoozed_until_ms, 0), COALESCE(expired_at_ms, 0),
attempts
`

func (s *SQLiteStore) Insert(ctx context.Context, c Commitment) (string, error) {
	if !c.Kind.Valid() {
		return "", fmt.Errorf("commitments insert: invalid kind %q", c.Kind)
	}
	if !c.Sensitivity.Valid() {
		return "", fmt.Errorf("commitments insert: invalid sensitivity %q", c.Sensitivity)
	}
	if c.WorkspaceID == "" {
		return "", fmt.Errorf("commitments insert: workspace_id required")
	}
	if c.DedupeKey == "" {
		return "", fmt.Errorf("commitments insert: dedupe_key required")
	}

	// Dedup: if a pending row with the same (workspace_id, dedupe_key)
	// exists, return its id paired with ErrDeduped so the caller can
	// skip the lifecycle-event emission. Without the sentinel, an
	// extractor that re-fires over the same turn would silently spawn a
	// phantom "extracted" timeline event per repeat — the row stays
	// unique but the event log lies. See ErrDeduped doc for callers'
	// contract.
	var existing string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM gateway_commitments
		 WHERE workspace_id = ? AND dedupe_key = ? AND status = ?`,
		c.WorkspaceID, c.DedupeKey, string(StatusPending),
	).Scan(&existing)
	if err == nil {
		return existing, ErrDeduped
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("commitments dedup: %w", err)
	}

	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	if c.Status == "" {
		c.Status = StatusPending
	}
	now := time.Now().UTC().UnixMilli()
	if c.CreatedAtMs == 0 {
		c.CreatedAtMs = now
	}
	if c.UpdatedAtMs == 0 {
		c.UpdatedAtMs = now
	}

	_, err = s.db.ExecContext(ctx, insertSQL,
		c.ID, c.WorkspaceID, c.TenantID, string(c.Kind), string(c.Sensitivity), string(c.Status),
		c.Reason, c.SuggestedText, c.DedupeKey, c.Confidence,
		c.DueWindow.EarliestMs, c.DueWindow.LatestMs, c.DueWindow.Timezone,
		c.ConnectorID, c.ChannelID, c.UserID, c.SourceEventID,
		c.CreatedAtMs, c.UpdatedAtMs, c.Attempts,
	)
	if err != nil {
		return "", fmt.Errorf("commitments insert: %w", err)
	}
	return c.ID, nil
}

func (s *SQLiteStore) Get(ctx context.Context, id string) (Commitment, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+selectColumns+` FROM gateway_commitments WHERE id = ?`, id)
	c, err := scanRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Commitment{}, ErrNotFound
	}
	return c, err
}

func (s *SQLiteStore) List(ctx context.Context, filter ListFilter) ([]Commitment, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	q := `SELECT ` + selectColumns + ` FROM gateway_commitments WHERE 1=1`
	var args []any
	if filter.WorkspaceID != "" {
		q += ` AND workspace_id = ?`
		args = append(args, filter.WorkspaceID)
	}
	if filter.Status != "" {
		q += ` AND status = ?`
		args = append(args, string(filter.Status))
	}
	q += ` ORDER BY created_at_ms DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("commitments list: %w", err)
	}
	defer rows.Close()
	var out []Commitment
	for rows.Next() {
		c, err := scanRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) DueNow(ctx context.Context, nowMs int64, limit int) ([]Commitment, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT ` + selectColumns + `
		FROM gateway_commitments
		WHERE status = ? AND earliest_ms <= ? AND latest_ms >= ?
		ORDER BY earliest_ms ASC
		LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, string(StatusPending), nowMs, nowMs, limit)
	if err != nil {
		return nil, fmt.Errorf("commitments due_now: %w", err)
	}
	defer rows.Close()
	var out []Commitment
	for rows.Next() {
		c, err := scanRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) MarkSent(ctx context.Context, id string, sentAtMs int64) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE gateway_commitments
		 SET status = ?, sent_at_ms = ?, updated_at_ms = ?, attempts = attempts + 1
		 WHERE id = ? AND status = ?`,
		string(StatusSent), sentAtMs, sentAtMs, id, string(StatusPending),
	)
	return rowsAffectedErr(res, err, id)
}

func (s *SQLiteStore) MarkDismissed(ctx context.Context, id string) error {
	now := time.Now().UTC().UnixMilli()
	res, err := s.db.ExecContext(ctx,
		`UPDATE gateway_commitments
		 SET status = ?, dismissed_at_ms = ?, updated_at_ms = ?
		 WHERE id = ?`,
		string(StatusDismissed), now, now, id,
	)
	return rowsAffectedErr(res, err, id)
}

func (s *SQLiteStore) MarkSnoozed(ctx context.Context, id string, untilMs int64) error {
	now := time.Now().UTC().UnixMilli()
	// Snooze pushes the earliest_ms to untilMs so DueNow re-finds it
	// when the snooze window opens. Status flips back to pending; the
	// snoozed_until_ms field records when this happened.
	res, err := s.db.ExecContext(ctx,
		`UPDATE gateway_commitments
		 SET status = ?, earliest_ms = ?, snoozed_until_ms = ?, updated_at_ms = ?
		 WHERE id = ?`,
		string(StatusPending), untilMs, untilMs, now, id,
	)
	return rowsAffectedErr(res, err, id)
}

func (s *SQLiteStore) MarkExpired(ctx context.Context, id string) error {
	now := time.Now().UTC().UnixMilli()
	res, err := s.db.ExecContext(ctx,
		`UPDATE gateway_commitments
		 SET status = ?, expired_at_ms = ?, updated_at_ms = ?
		 WHERE id = ?`,
		string(StatusExpired), now, now, id,
	)
	return rowsAffectedErr(res, err, id)
}

// ExpireOverdue is the batched expiry path used by the runtime ticker.
// Replaces the previous "List(pending) → iterate → MarkExpired per row"
// flow with one bounded SELECT + one UPDATE: N round-trips collapse to
// 2 regardless of the batch size. The SELECT happens first so we can
// emit per-id lifecycle events with full row context after the UPDATE.
func (s *SQLiteStore) ExpireOverdue(ctx context.Context, cutoffMs int64, limit int) ([]Commitment, error) {
	if limit <= 0 {
		limit = 200
	}
	// 1. Snapshot the rows we're about to expire — we need their full
	//    body so the runtime can emit `commitment.expired` events with
	//    proper provenance, not just the ids.
	q := `SELECT ` + selectColumns + `
		FROM gateway_commitments
		WHERE status = ? AND latest_ms > 0 AND latest_ms < ?
		ORDER BY latest_ms ASC
		LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, string(StatusPending), cutoffMs, limit)
	if err != nil {
		return nil, fmt.Errorf("commitments expire_overdue select: %w", err)
	}
	var snapshot []Commitment
	for rows.Next() {
		c, err := scanRows(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		snapshot = append(snapshot, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("commitments expire_overdue iter: %w", err)
	}
	if len(snapshot) == 0 {
		return nil, nil
	}

	// 2. Single UPDATE covering every snapshotted id. The IN-clause is
	//    bounded by `limit` (default 200) so SQLite's expression-tree
	//    limit (default 1000) is never reached.
	placeholders := make([]string, len(snapshot))
	args := make([]any, 0, len(snapshot)+3)
	now := time.Now().UTC().UnixMilli()
	args = append(args, string(StatusExpired), now, now)
	for i, c := range snapshot {
		placeholders[i] = "?"
		args = append(args, c.ID)
	}
	args = append(args, string(StatusPending)) // status guard
	upd := `UPDATE gateway_commitments
		SET status = ?, expired_at_ms = ?, updated_at_ms = ?
		WHERE id IN (` + strings.Join(placeholders, ",") + `) AND status = ?`
	res, err := s.db.ExecContext(ctx, upd, args...)
	if err != nil {
		return nil, fmt.Errorf("commitments expire_overdue update: %w", err)
	}
	n, _ := res.RowsAffected()
	// If a row's status changed between snapshot and update (e.g.
	// MarkSent landed in the gap), we still emit for the survivors. Cut
	// the snapshot down to the affected count to keep emit-count
	// honest, dropping from the tail (oldest first stay; newest may
	// have raced).
	if int(n) < len(snapshot) {
		snapshot = snapshot[:n]
	}
	for i := range snapshot {
		snapshot[i].Status = StatusExpired
		snapshot[i].ExpiredAtMs = now
		snapshot[i].UpdatedAtMs = now
	}
	return snapshot, nil
}

func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM gateway_commitments WHERE id = ?`, id)
	return rowsAffectedErr(res, err, id)
}

// rowsAffectedErr wraps an UPDATE/DELETE result and surfaces "not found"
// when no row was touched.
func rowsAffectedErr(res sql.Result, err error, id string) error {
	if err != nil {
		return fmt.Errorf("commitments update %s: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// rowScanner is the common surface for QueryRow + Rows so scanRow and
// scanRows can share field bindings.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(r rowScanner) (Commitment, error) { return scan(r) }
func scanRows(r rowScanner) (Commitment, error) { return scan(r) }

func scan(r rowScanner) (Commitment, error) {
	var (
		c                 Commitment
		kind, sens, stat  string
		timezone          sql.NullString
	)
	err := r.Scan(
		&c.ID, &c.WorkspaceID, &c.TenantID, &kind, &sens, &stat,
		&c.Reason, &c.SuggestedText, &c.DedupeKey, &c.Confidence,
		&c.DueWindow.EarliestMs, &c.DueWindow.LatestMs, &timezone,
		&c.ConnectorID, &c.ChannelID, &c.UserID, &c.SourceEventID,
		&c.CreatedAtMs, &c.UpdatedAtMs,
		&c.SentAtMs, &c.DismissedAtMs, &c.SnoozedUntilMs, &c.ExpiredAtMs,
		&c.Attempts,
	)
	if err != nil {
		return Commitment{}, err
	}
	c.Kind = CommitmentKind(kind)
	c.Sensitivity = CommitmentSensitivity(sens)
	c.Status = CommitmentStatus(stat)
	if timezone.Valid {
		c.DueWindow.Timezone = timezone.String
	}
	return c, nil
}

// ParseStatus is the query-string parser used by the handler. Empty
// input means "no filter".
func ParseStatus(raw string) (CommitmentStatus, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "", nil
	}
	s := CommitmentStatus(raw)
	if !s.Valid() {
		return "", fmt.Errorf("invalid status %q", raw)
	}
	return s, nil
}
