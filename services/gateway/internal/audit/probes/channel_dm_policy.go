package probes

import (
	"context"
	"database/sql"
	"fmt"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit"
)

// ChannelDMPolicy walks the Layer-4 channel_policies table and flags:
//
//   - Orphan rules: rules whose channel_id no longer matches any known
//     connector type (drift after a connector was renamed/removed).
//   - Self-conflicting rules: the same (channel_id, action) row pair
//     somehow holds both `allow` and `deny` — only possible if the
//     primary-key constraint was bypassed (e.g. raw SQL backfill).
//
// Reads the gateway's *sql.DB directly because the policy store API
// only exposes lookup-by-key; an enumeration query is what the probe
// needs and what tests want to seed.
type ChannelDMPolicy struct {
	db *sql.DB
}

// NewChannelDMPolicy wraps a shared *sql.DB. The caller owns lifetime.
func NewChannelDMPolicy(db *sql.DB) *ChannelDMPolicy { return &ChannelDMPolicy{db: db} }

func (ChannelDMPolicy) ID() string { return "channel_dm_policy" }

func (p *ChannelDMPolicy) Run(ctx context.Context) audit.Finding {
	if p.db == nil {
		return audit.Finding{
			ProbeID:  "channel_dm_policy",
			Severity: audit.SeverityCritical,
			Reason:   "no database handle wired to probe",
		}
	}

	// 1. Enumerate every (channel_id, action, decision) tuple.
	rows, err := p.db.QueryContext(ctx, `
		SELECT channel_id, action, decision FROM channel_policies
	`)
	if err != nil {
		return audit.Finding{
			ProbeID:  "channel_dm_policy",
			Severity: audit.SeverityCritical,
			Reason:   fmt.Sprintf("policy enumeration failed: %v", err),
		}
	}
	defer rows.Close()

	known := knownConnectorTypes()
	type ruleKey struct{ ChannelID, Action string }
	decisions := map[ruleKey]map[string]struct{}{}
	var orphans []map[string]string
	total := 0
	for rows.Next() {
		var channelID, action, decision string
		if err := rows.Scan(&channelID, &action, &decision); err != nil {
			return audit.Finding{
				ProbeID:  "channel_dm_policy",
				Severity: audit.SeverityCritical,
				Reason:   fmt.Sprintf("policy row scan failed: %v", err),
			}
		}
		total++
		// Orphan detection: every channel_id should resolve to a
		// registered connector type. Connectors register their type
		// (e.g. "telegram") via connectors.MustRegister(); channel
		// policies key by connector identity. A rule for "discord"
		// while no Discord connector is registered = orphan.
		if _, ok := known[channelID]; !ok {
			orphans = append(orphans, map[string]string{
				"channel_id": channelID,
				"action":     action,
			})
		}
		key := ruleKey{ChannelID: channelID, Action: action}
		if decisions[key] == nil {
			decisions[key] = map[string]struct{}{}
		}
		decisions[key][decision] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return audit.Finding{
			ProbeID:  "channel_dm_policy",
			Severity: audit.SeverityCritical,
			Reason:   fmt.Sprintf("policy rows iter failed: %v", err),
		}
	}

	// 2. Conflict detection — the primary key should make this
	// impossible, but a manual backfill via INSERT OR REPLACE that
	// raced two callers could in theory leave the row in a weird state.
	// The check is cheap and catches the failure mode that the schema
	// alone can't.
	var conflicts []map[string]any
	for key, set := range decisions {
		if len(set) > 1 {
			variants := make([]string, 0, len(set))
			for d := range set {
				variants = append(variants, d)
			}
			conflicts = append(conflicts, map[string]any{
				"channel_id":        key.ChannelID,
				"action":            key.Action,
				"decision_variants": variants,
			})
		}
	}

	switch {
	case len(conflicts) > 0:
		return audit.Finding{
			ProbeID:  "channel_dm_policy",
			Severity: audit.SeverityCritical,
			Reason:   fmt.Sprintf("%d channel rule(s) hold conflicting decisions", len(conflicts)),
			Evidence: map[string]any{
				"total_rules":  total,
				"conflicts":    conflicts,
				"orphan_count": len(orphans),
			},
		}
	case len(orphans) > 0:
		return audit.Finding{
			ProbeID:  "channel_dm_policy",
			Severity: audit.SeverityWarning,
			Reason:   fmt.Sprintf("%d channel rule(s) reference unregistered connectors", len(orphans)),
			Evidence: map[string]any{
				"total_rules": total,
				"orphans":     orphans,
			},
		}
	default:
		return audit.Finding{
			ProbeID:  "channel_dm_policy",
			Severity: audit.SeverityInfo,
			Reason:   "all channel policy rules resolve to registered connectors with consistent decisions",
			Evidence: map[string]any{
				"total_rules":      total,
				"known_connectors": connectorTypeStringList(known),
			},
		}
	}
}

// knownConnectorTypes converts the live connector registry into a set
// for orphan lookups. Called once per probe run — fine at the row
// counts v0.3 expects (<1k rules).
func knownConnectorTypes() map[string]struct{} {
	out := map[string]struct{}{}
	for _, r := range connectors.List() {
		out[string(r.Type)] = struct{}{}
	}
	return out
}

func connectorTypeStringList(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
