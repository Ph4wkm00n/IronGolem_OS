// Package-level registration shape for IronGolem connectors. v0.3 Step 1
// of Plans/modular-puzzling-blum.md, adopting the PlatformEntry pattern
// from NousResearch/hermes-agent (gateway/platform_registry.py).
//
// Each connector subpackage (telegram, email, webhook, ...) declares its
// metadata once and self-registers in its init() so the doctor binary
// and setup wizard can iterate the registry without hardcoded if/elif
// chains.
package connectors

import (
	"fmt"
	"sort"
	"sync"
)

// Registration is the declarative metadata a connector exposes to the
// gateway. The check / required-env / install-hint fields exist so that
// the doctor binary and the setup wizard can report actionable status
// per connector ("telegram: OK | signal: missing signal-cli binary in
// PATH") without each operator-facing surface re-encoding the same
// preconditions.
type Registration struct {
	// Type is the canonical connector identifier (e.g. "telegram"). Used
	// as the registry key and the source_service tag on gateway events.
	Type ConnectorType

	// Label is a human-readable name shown in doctor output and the
	// setup wizard. Mixed case OK.
	Label string

	// CheckFn returns true when the connector's environment-level
	// preconditions are satisfied — typically: required env vars set,
	// external binaries present, network reachable. Cheap & synchronous;
	// must not perform expensive validation (auth, RPC). Doctor runs
	// this for every registered connector on every invocation.
	CheckFn func() bool

	// RequiredEnv lists the environment variables the connector reads
	// at boot. Doctor surfaces these by name when CheckFn returns false
	// so the operator knows what to set.
	RequiredEnv []string

	// InstallHint is the one-line suggestion shown alongside a failed
	// CheckFn. Use full sentences. Examples:
	//   - "Set IRONGOLEM_TELEGRAM_BOT_TOKEN from @BotFather"
	//   - "Install signal-cli (brew install signal-cli) and add to PATH"
	InstallHint string

	// ValidateConfig is an optional second-stage check that runs
	// against an instantiated config map (the same shape passed to
	// Connector.Connect). Returns the first reason validation fails.
	// Doctor invokes this only when CheckFn already passed.
	ValidateConfig func(config map[string]string) error

	// Source distinguishes built-in connectors (shipped in this repo)
	// from plugin-registered ones (v0.4+, post-WASM-sandbox). Doctor
	// labels plugin entries with their source for trust review.
	Source RegistrationSource
}

// RegistrationSource tags whether a registration came from the
// built-in connectors module or from a v0.4+ plugin.
type RegistrationSource string

const (
	// SourceBuiltin marks connectors shipped in this repository.
	SourceBuiltin RegistrationSource = "builtin"
	// SourcePlugin marks connectors registered by a v0.4+ plugin.
	SourcePlugin RegistrationSource = "plugin"
)

var (
	registryMu sync.RWMutex
	registry   = map[ConnectorType]Registration{}
)

// Register adds a Registration to the package-level registry. Connector
// subpackages call this from init(). Re-registering the same Type
// returns an error so accidental double-imports surface immediately
// instead of silently shadowing a built-in.
func Register(r Registration) error {
	if r.Type == "" {
		return fmt.Errorf("connectors.Register: Type is required")
	}
	if r.CheckFn == nil {
		return fmt.Errorf("connectors.Register: %s missing CheckFn", r.Type)
	}
	if r.Source == "" {
		r.Source = SourceBuiltin
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := registry[r.Type]; dup {
		return fmt.Errorf("connectors.Register: duplicate registration for %s", r.Type)
	}
	registry[r.Type] = r
	return nil
}

// MustRegister is the init()-friendly variant of Register. It panics on
// duplicate registration so the binary fails fast at boot.
func MustRegister(r Registration) {
	if err := Register(r); err != nil {
		panic(err)
	}
}

// Get returns the Registration for a given connector type, or false if
// it isn't registered.
func Get(t ConnectorType) (Registration, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	r, ok := registry[t]
	return r, ok
}

// List returns every registered Registration sorted by Type. Doctor
// uses this so output ordering is stable regardless of init() order.
func List() []Registration {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]Registration, 0, len(registry))
	for _, r := range registry {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// resetForTests clears the registry. Test-only helper; do not call from
// production code. Exported as a lowercase identifier intentionally —
// tests in the connectors package can use it directly without exposing
// it as API surface.
func resetForTests() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = map[ConnectorType]Registration{}
}
