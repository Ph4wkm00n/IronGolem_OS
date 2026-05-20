package probes

import (
	"context"
	"fmt"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit"
)

// ConnectorHealthDrift uses v0.3 Step 1's CheckFn (per-connector env
// readiness probe) to detect environment drift. A connector that
// passed CheckFn at boot but fails it now means env vars were unset or
// mutated mid-run — typically a misconfigured restart or container
// secret rotation that didn't propagate.
//
// Distinct from the connector's live-traffic Health() method (which
// tracks "is the bot token still accepted by the upstream service")
// — CheckFn is purely env-level and runs without any network IO,
// making it safe to invoke every audit tick.
type ConnectorHealthDrift struct{}

// NewConnectorHealthDrift returns the probe. The connectors registry
// is package-global so no constructor args are needed.
func NewConnectorHealthDrift() *ConnectorHealthDrift { return &ConnectorHealthDrift{} }

func (ConnectorHealthDrift) ID() string { return "connector_health_drift" }

func (ConnectorHealthDrift) Run(_ context.Context) audit.Finding {
	regs := connectors.List()
	var failing []map[string]any
	var passing []string
	for _, r := range regs {
		// Webhook (and any future config-driven connector) returns true
		// unconditionally from CheckFn. We don't flag those as drift —
		// their readiness is decided at Connect() time, not boot.
		if r.CheckFn() {
			passing = append(passing, string(r.Type))
			continue
		}
		failing = append(failing, map[string]any{
			"type":         string(r.Type),
			"label":        r.Label,
			"required_env": r.RequiredEnv,
			"install_hint": r.InstallHint,
		})
	}

	switch {
	case len(regs) == 0:
		// No connectors registered at all is suspicious — main.go
		// blank-imports them so this means the binary was built without
		// the connector subpackages, which is a deployment defect.
		return audit.Finding{
			ProbeID:  "connector_health_drift",
			Severity: audit.SeverityCritical,
			Reason:   "no connectors registered; check binary build configuration",
		}
	case len(failing) > 0:
		plural := ""
		if len(failing) != 1 {
			plural = "s"
		}
		return audit.Finding{
			ProbeID:  "connector_health_drift",
			Severity: audit.SeverityWarning,
			Reason:   fmt.Sprintf("%d of %d connector%s failed environment readiness", len(failing), len(regs), plural),
			Evidence: map[string]any{
				"failing": failing,
				"passing": passing,
				"total":   len(regs),
			},
		}
	default:
		return audit.Finding{
			ProbeID:  "connector_health_drift",
			Severity: audit.SeverityInfo,
			Reason:   "every registered connector reports ready",
			Evidence: map[string]any{
				"passing": passing,
				"total":   len(regs),
			},
		}
	}
}
