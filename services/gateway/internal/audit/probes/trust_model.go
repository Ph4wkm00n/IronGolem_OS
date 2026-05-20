package probes

import (
	"context"
	"os"
	"strings"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit"
)

// TrustModel verifies the gateway's authentication+policy foundation
// is intact. v0.1 made HMAC auth fail-closed at boot, but a future
// regression (env unset, middleware chain reordered) would weaken the
// guarantees without crashing. This probe is the continuous check.
//
// What it asserts (each becomes a critical finding when violated):
//   1. IRONGOLEM_HMAC_SECRET is set and non-empty. The gateway refuses
//      to boot without it, but a hot-reload or restart with the env
//      stripped would leave the running process in a vulnerable state
//      that the probe surfaces.
//   2. (Future) auth + policy middleware are in the request chain.
//      Today the chain is statically wired in main.go; the probe will
//      gain runtime middleware-introspection when v0.4 makes the chain
//      configurable.
type TrustModel struct {
	// hmacEnv is the env var name the gateway reads for the HMAC
	// secret. Carried in the struct so tests can override.
	hmacEnv string
}

// NewTrustModel constructs the probe with default env names.
func NewTrustModel() *TrustModel {
	return &TrustModel{hmacEnv: "IRONGOLEM_HMAC_SECRET"}
}

// NewTrustModelWithEnv lets tests inject a custom env-var name.
func NewTrustModelWithEnv(hmacEnv string) *TrustModel {
	return &TrustModel{hmacEnv: hmacEnv}
}

func (TrustModel) ID() string { return "trust_model" }

func (p *TrustModel) Run(_ context.Context) audit.Finding {
	secret := strings.TrimSpace(os.Getenv(p.hmacEnv))
	if secret == "" {
		return audit.Finding{
			ProbeID:  "trust_model",
			Severity: audit.SeverityCritical,
			Reason:   "HMAC secret env var is unset; gateway auth is unsigned",
			Evidence: map[string]any{
				"env_var": p.hmacEnv,
				"status":  "unset_or_empty",
			},
		}
	}
	// Weak-secret heuristic: if the secret looks like a placeholder
	// (e.g. "secret", "changeme", "default"), warn. Not critical
	// because we can't tell a real-but-short secret from a placeholder.
	if isLikelyPlaceholderSecret(secret) {
		return audit.Finding{
			ProbeID:  "trust_model",
			Severity: audit.SeverityWarning,
			Reason:   "HMAC secret looks like a placeholder; rotate before production",
			Evidence: map[string]any{
				"env_var":     p.hmacEnv,
				"length":      len(secret),
				"heuristic":   "placeholder_pattern_matched",
			},
		}
	}
	return audit.Finding{
		ProbeID:  "trust_model",
		Severity: audit.SeverityInfo,
		Reason:   "HMAC secret loaded, trust foundation intact",
		Evidence: map[string]any{
			"env_var":      p.hmacEnv,
			"secret_bytes": len(secret),
		},
	}
}

// isLikelyPlaceholderSecret matches a handful of well-known placeholder
// strings. False positives are tolerated — this is a hint, not an
// enforcement gate.
func isLikelyPlaceholderSecret(s string) bool {
	lower := strings.ToLower(s)
	for _, bad := range []string{"changeme", "default", "secret", "password", "test", "dev"} {
		if lower == bad {
			return true
		}
	}
	return len(s) < 16
}
