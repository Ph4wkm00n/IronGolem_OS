// Package policy owns the gateway's persistent policy stores. v0.2 Step 4
// ships the channel-policy store (Layer 4); subsequent steps add rate-
// limit + per-tool policy storage as they land.
package policy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	corepolicy "github.com/Ph4wkm00n/IronGolem_OS/services/pkg/policy"
)

// SQLiteChannelPolicyStore implements corepolicy.ChannelPolicyStore on
// the gateway's shared *sql.DB. Schema lives in
// services/gateway/internal/persist/db.go::Migrate so a single grep
// finds every table the gateway owns.
type SQLiteChannelPolicyStore struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewSQLiteChannelPolicyStore wraps the shared *sql.DB. The caller owns
// connection lifetime; this store does not Close the handle.
func NewSQLiteChannelPolicyStore(db *sql.DB, logger *slog.Logger) *SQLiteChannelPolicyStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &SQLiteChannelPolicyStore{
		db:     db,
		logger: logger.With(slog.String("component", "channel_policy_store")),
	}
}

// Lookup returns the rule for (channelID, action). Returns ok=false when
// no row matches; the policy layer decides the fallback.
func (s *SQLiteChannelPolicyStore) Lookup(ctx context.Context, channelID, action string) (corepolicy.ChannelRule, bool, error) {
	var (
		decision string
		reason   string
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT decision, reason FROM channel_policies WHERE channel_id = ? AND action = ?`,
		channelID, action,
	).Scan(&decision, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return corepolicy.ChannelRule{}, false, nil
	}
	if err != nil {
		return corepolicy.ChannelRule{}, false, fmt.Errorf("channel policy lookup: %w", err)
	}
	return corepolicy.ChannelRule{
		ChannelID: channelID,
		Action:    action,
		Decision:  corepolicy.Decision(decision),
		Reason:    reason,
	}, true, nil
}

// HasRules reports whether the store has any rows at all. v0.2 keeps
// this as a single COUNT — at the row counts v0.2 expects (<1k rules)
// the read is sub-millisecond. v0.3 caches it behind an atomic flag if
// the workload changes.
func (s *SQLiteChannelPolicyStore) HasRules(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM channel_policies`).Scan(&count); err != nil {
		return false, fmt.Errorf("channel policy count: %w", err)
	}
	return count > 0, nil
}

// Upsert installs or replaces a rule. Provided as the canonical write
// path for tests + future provisioning tooling (CLI / admin API in v0.3).
func (s *SQLiteChannelPolicyStore) Upsert(ctx context.Context, rule corepolicy.ChannelRule) error {
	if rule.ChannelID == "" || rule.Action == "" {
		return errors.New("channel policy: channel_id + action required")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO channel_policies (channel_id, action, decision, reason, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(channel_id, action) DO UPDATE SET decision = excluded.decision, reason = excluded.reason`,
		rule.ChannelID, rule.Action, string(rule.Decision), rule.Reason, time.Now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("channel policy upsert: %w", err)
	}
	return nil
}

// Delete removes the rule for (channelID, action). No-op when no row matches.
func (s *SQLiteChannelPolicyStore) Delete(ctx context.Context, channelID, action string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM channel_policies WHERE channel_id = ? AND action = ?`,
		channelID, action,
	)
	if err != nil {
		return fmt.Errorf("channel policy delete: %w", err)
	}
	return nil
}
