// Package probes implements the built-in audit probes the gateway runs
// every tick. Each probe is registered with `audit.Registry` from the
// gateway's main() at boot.
//
// v0.3 Step 5 of `Plans/modular-puzzling-blum.md`. Adopts the
// audit-* taxonomy from openclaw/openclaw `src/security/`. v0.3 ships
// four probes; the catalog grows additively in v0.4+.
package probes

import (
	"context"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit"
)

// WorkspaceSkillEscape verifies that no workspace-scoped skill escapes
// its sandbox.
//
// **v0.3 status: vacuous.** IronGolem has no skill system yet — that's
// v0.4+ work tied to WASM sandbox completion (`runtime/sandbox`). The
// probe ships now so:
//
//   - the probe ID + Finding shape are pre-locked-in. When the skill
//     system arrives, the contract is already defined and the v0.3
//     audit-findings UI already renders it. No retroactive plumbing.
//   - the UI's "workspace_skill_escape" tile shows "passing — no skills"
//     instead of "no data", which would be indistinguishable from "the
//     probe never ran."
//
// The Reason text deliberately names the v0.3 status so an operator
// drilling in understands why it's always green.
type WorkspaceSkillEscape struct{}

// NewWorkspaceSkillEscape returns the probe. No dependencies in v0.3 —
// v0.4+ will inject a SkillRegistry handle here.
func NewWorkspaceSkillEscape() *WorkspaceSkillEscape { return &WorkspaceSkillEscape{} }

// ID is the stable wire identifier. Never change once shipped.
func (WorkspaceSkillEscape) ID() string { return "workspace_skill_escape" }

// Run returns the vacuous-pass finding. Kept as `info` severity so it
// stays out of the timeline (no operator action implied).
func (WorkspaceSkillEscape) Run(_ context.Context) audit.Finding {
	return audit.Finding{
		ProbeID:  "workspace_skill_escape",
		Severity: audit.SeverityInfo,
		Reason:   "no skill system in v0.3 — probe passes vacuously until v0.4 wires WASM sandboxed skills",
		Evidence: map[string]any{
			"skill_system_present": false,
			"v03_status":           "placeholder",
		},
	}
}
