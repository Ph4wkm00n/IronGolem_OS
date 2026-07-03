package probes

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit"
)

// ExposureComposition ports openclaw's highest-leverage audit idea
// (`security.exposure.open_channels_with_exec`, 2026-07 scan): audit the
// PRODUCT of channel openness × action privilege, not each axis alone.
// A channel policy that allows "execute" is a bridge from untrusted
// inbound traffic to code execution — exactly the composition the
// five-layer model exists to prevent — and neither the channel list nor
// the tool policy looks dangerous when reviewed in isolation.
//
// Data source: the Layer-4 channel_policies store (v0.2 Step 3).
type ExposureComposition struct {
	db *sql.DB
}

// NewExposureComposition wires the Layer-4 policy store handle.
func NewExposureComposition(db *sql.DB) *ExposureComposition {
	return &ExposureComposition{db: db}
}

func (ExposureComposition) ID() string { return "exposure_composition" }

func (p *ExposureComposition) Run(ctx context.Context) audit.Finding {
	if p.db == nil {
		return audit.Finding{
			ProbeID:  "exposure_composition",
			Severity: audit.SeverityCritical,
			Reason:   "no database handle wired to probe",
		}
	}

	execChannels, err := p.channelsAllowing(ctx, "execute")
	if err != nil {
		return audit.Finding{
			ProbeID:  "exposure_composition",
			Severity: audit.SeverityWarning,
			Reason:   fmt.Sprintf("channel policy query failed: %v", err),
		}
	}
	writeChannels, err := p.channelsAllowing(ctx, "write")
	if err != nil {
		return audit.Finding{
			ProbeID:  "exposure_composition",
			Severity: audit.SeverityWarning,
			Reason:   fmt.Sprintf("channel policy query failed: %v", err),
		}
	}

	switch {
	case len(execChannels) > 0:
		return audit.Finding{
			ProbeID:  "exposure_composition",
			Severity: audit.SeverityCritical,
			Reason: fmt.Sprintf(
				"%d channel(s) allow the execute action — untrusted inbound traffic can reach code execution",
				len(execChannels)),
			Evidence: map[string]any{
				"execute_channels": execChannels,
				"remediation":      "deny execute at the channel layer; route execution through approval-gated recipes instead",
			},
		}
	case len(writeChannels) > 0:
		return audit.Finding{
			ProbeID:  "exposure_composition",
			Severity: audit.SeverityWarning,
			Reason: fmt.Sprintf(
				"%d channel(s) allow the write action — inbound traffic can mutate state without an approval gate",
				len(writeChannels)),
			Evidence: map[string]any{"write_channels": writeChannels},
		}
	default:
		return audit.Finding{
			ProbeID:  "exposure_composition",
			Severity: audit.SeverityInfo,
			Reason:   "no channel grants execute or write; exposure composition is clean",
		}
	}
}

func (p *ExposureComposition) channelsAllowing(ctx context.Context, action string) ([]string, error) {
	rows, err := p.db.QueryContext(ctx,
		`SELECT channel_id FROM channel_policies WHERE action = ? AND decision = 'allow' ORDER BY channel_id`,
		action,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
