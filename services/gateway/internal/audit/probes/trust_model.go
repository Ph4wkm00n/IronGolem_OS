package probes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
//  1. IRONGOLEM_HMAC_SECRET is set and non-empty. The gateway refuses
//     to boot without it, but a hot-reload or restart with the env
//     stripped would leave the running process in a vulnerable state
//     that the probe surfaces.
//  2. (Future) auth + policy middleware are in the request chain.
//     Today the chain is statically wired in main.go; the probe will
//     gain runtime middleware-introspection when v0.4 makes the chain
//     configurable.
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
	// v1.2.2: read the secret WITHOUT TrimSpace so the probe sees
	// exactly what gateway/cmd/main.go sees (it calls os.Getenv
	// directly, with no trim). Pre-patch the probe trimmed and the
	// gateway didn't — a secret with trailing whitespace would make
	// the probe say "trust foundation intact" while the running
	// process used a different secret. The probe must observe the same
	// bytes the consumer does.
	secret := os.Getenv(p.hmacEnv)
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

	// v1.2.2: split the "looks like a placeholder string" check from
	// the "secret is short" check. The pre-patch heuristic conflated
	// them under one Reason — operators saw a real 12-char production
	// secret reported as "looks like a placeholder", which is wrong:
	// it's short, not a placeholder. The two findings carry different
	// remediation (rotate-to-a-real-secret vs lengthen-the-secret).
	fp := fingerprint(secret)
	if isWellKnownPlaceholder(secret) {
		return audit.Finding{
			ProbeID:  "trust_model",
			Severity: audit.SeverityCritical,
			Reason:   "HMAC secret matches a well-known placeholder string; rotate immediately",
			Evidence: map[string]any{
				"env_var":     p.hmacEnv,
				"length":      len(secret),
				"fingerprint": fp,
				"heuristic":   "placeholder_string_match",
			},
		}
	}
	if len(secret) < 32 {
		// 32 bytes is the recommended minimum for HMAC-SHA256 inputs
		// (matches the block size). Below that we warn but stay
		// non-blocking; the gateway already started, and a real-but-
		// short secret is workable, just not best-practice.
		return audit.Finding{
			ProbeID:  "trust_model",
			Severity: audit.SeverityWarning,
			Reason:   "HMAC secret is shorter than 32 bytes; consider a longer one for HMAC-SHA256",
			Evidence: map[string]any{
				"env_var":     p.hmacEnv,
				"length":      len(secret),
				"fingerprint": fp,
				"heuristic":   "short_secret",
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
			"fingerprint":  fp,
		},
	}
}

// isWellKnownPlaceholder matches a handful of well-known placeholder
// strings (case-insensitive, whitespace-trimmed for the comparison so
// "  secret  " still fires). Distinct from short-secret detection.
func isWellKnownPlaceholder(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	for _, bad := range []string{"changeme", "default", "secret", "password", "test", "dev", "insecure"} {
		if lower == bad {
			return true
		}
	}
	return false
}

// fingerprint returns the first 8 hex chars of sha256(secret). Lets the
// operator confirm the probe sees the same secret as the gateway by
// eyeballing two log lines, without ever printing the secret itself.
// 8 hex chars = 32 bits — collision probability negligible at the
// single-secret-per-deployment scale this is used at.
func fingerprint(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:4])
}
