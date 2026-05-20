// Package audit owns the gateway's continuous-security probe subsystem.
//
// v0.3 Step 5 of `Plans/modular-puzzling-blum.md`. Adopts the
// audit-* taxonomy from openclaw/openclaw `src/security/` (80+ runtime
// probes — we start with four). The five-layer policy engine handles
// *enforcement* at request time; this package handles *probing* between
// requests: assertions that the runtime invariants still hold even when
// no traffic is flowing.
//
// Distinct from `services/pkg/audit/` (audit-log export for compliance)
// — these probes evaluate live state, not historical events. The two
// packages co-exist; their concerns don't overlap.
//
// Probes are registered at boot; the runtime ticker (`runtime.go`)
// invokes them on a 5-minute cadence, persists findings to
// `gateway_audit_findings`, and emits an `audit-finding` event for each
// non-info result so the timeline + UI surface them.
package audit

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Severity classifies a finding's urgency.
//
// info     — probe ran, system is in expected state. Logged for
//             completeness; UI defaults filter these out.
// warning  — drift or misconfiguration detected; not load-bearing yet.
// critical — invariant violation that could compromise the trust model.
//             Surfaced prominently in the UI; emit page-able telemetry.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Valid reports whether s is one of the canonical severity values.
func (s Severity) Valid() bool {
	switch s {
	case SeverityInfo, SeverityWarning, SeverityCritical:
		return true
	}
	return false
}

// Finding is the structured output of a single Probe.Run invocation.
//
// Probes that detect nothing wrong still produce an `info` Finding so
// the UI can distinguish "this probe ran and passed" from "this probe
// has never run." Skipping silently is the v0.2 silent-fail antipattern
// the v0.3 plan is trying to retire.
type Finding struct {
	// ProbeID is the stable identifier for the probe that produced this.
	// Used as the primary grouping key in the UI; never change once a
	// probe ships.
	ProbeID string `json:"probe_id"`
	// Severity classifies urgency. See Severity for semantics.
	Severity Severity `json:"severity"`
	// Reason is the one-line human-readable explanation. Keep terse;
	// detail goes in Evidence.
	Reason string `json:"reason"`
	// Evidence is the structured payload the UI renders as a key/value
	// table when the operator drills in. Probes populate this with
	// whatever artifact convinced them of the finding (row counts,
	// missing env names, conflicting policy rules, etc.).
	Evidence map[string]any `json:"evidence,omitempty"`
	// Timestamp is when the probe ran. Filled in by the runtime, not
	// the probe — keeps the probe contract pure.
	Timestamp time.Time `json:"timestamp"`
}

// Probe is the interface every audit probe implements. Probes are
// stateless across runs; any persistent context (db, connector
// registry) is captured at construction time. Run is invoked from the
// runtime ticker; the supplied context carries a 15s timeout so a hung
// probe doesn't block the next tick.
type Probe interface {
	// ID is the stable identifier (e.g. "trust_model"). Returned per-
	// call rather than a struct field so the interface stays the only
	// surface implementers have to satisfy.
	ID() string
	// Run executes the probe and returns 0+ findings. Returning an
	// empty slice means "I had nothing to report"; an `info` finding
	// means "I ran and confirmed the invariant holds." Convention: each
	// probe emits exactly ONE Finding per Run so the UI's "latest
	// finding per probe" query is trivially stable.
	Run(ctx context.Context) Finding
}

// Registry is the package-level set of probes the runtime walks every
// tick. Built-in probes self-register in their init(); plugin probes
// (v0.4+) call Register() from their loader.
type Registry struct {
	mu     sync.RWMutex
	probes map[string]Probe
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{probes: map[string]Probe{}} }

// Register adds a probe. Duplicate IDs return an error so accidental
// double-imports surface immediately.
func (r *Registry) Register(p Probe) error {
	if p == nil {
		return fmt.Errorf("audit.Register: nil probe")
	}
	if p.ID() == "" {
		return fmt.Errorf("audit.Register: probe has empty ID")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.probes[p.ID()]; dup {
		return fmt.Errorf("audit.Register: duplicate probe id %q", p.ID())
	}
	r.probes[p.ID()] = p
	return nil
}

// MustRegister is the init()-friendly Register that panics on error.
func (r *Registry) MustRegister(p Probe) {
	if err := r.Register(p); err != nil {
		panic(err)
	}
}

// List returns every registered probe sorted by ID. Order is stable
// across ticks so the UI's per-probe drilldown doesn't jump.
func (r *Registry) List() []Probe {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Probe, 0, len(r.probes))
	for _, p := range r.probes {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// Get returns a probe by ID, or false if not registered.
func (r *Registry) Get(id string) (Probe, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.probes[id]
	return p, ok
}
