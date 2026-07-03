package probes

import (
	"context"
	"net"
	"os"
	"strings"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit"
)

// GatewayExposure ports openclaw's gateway.bind_no_auth family
// (2026-07 scan) to IronGolem's single-listener shape. The gateway
// always requires HMAC auth to boot, so the openclaw "no auth at all"
// case can't happen at startup — but two drifts still matter per tick:
//
//   - IRONGOLEM_HMAC_SECRET emptied after boot (secret rotation gone
//     wrong): the running process still holds the old secret while the
//     environment says otherwise — operators debugging auth failures
//     need this surfaced. Critical.
//   - Binding beyond loopback (GATEWAY_ADDR ":8080" binds every
//     interface — the default!): legitimate behind a reverse proxy or
//     firewall, but it widens the attack surface to token brute-forcing
//     and any future authz bug, so it warrants a standing warning the
//     operator explicitly suppresses.
type GatewayExposure struct {
	addrEnv   string
	secretEnv string
}

// NewGatewayExposure uses the same env names cmd/main.go reads.
func NewGatewayExposure() *GatewayExposure {
	return &GatewayExposure{addrEnv: "GATEWAY_ADDR", secretEnv: "IRONGOLEM_HMAC_SECRET"}
}

func (GatewayExposure) ID() string { return "gateway_exposure" }

func (p *GatewayExposure) Run(_ context.Context) audit.Finding {
	if os.Getenv(p.secretEnv) == "" {
		return audit.Finding{
			ProbeID:  "gateway_exposure",
			Severity: audit.SeverityCritical,
			Reason:   p.secretEnv + " is empty in the running environment — auth secret drifted since boot",
			Evidence: map[string]any{
				"remediation": "restore the secret or restart the gateway with the rotated value",
			},
		}
	}

	addr := os.Getenv(p.addrEnv)
	if addr == "" {
		addr = ":8080" // cmd/main.go default
	}

	if bindIsLoopback(addr) {
		return audit.Finding{
			ProbeID:  "gateway_exposure",
			Severity: audit.SeverityInfo,
			Reason:   "gateway bound to loopback; network exposure is minimal",
			Evidence: map[string]any{"addr": addr},
		}
	}
	return audit.Finding{
		ProbeID:  "gateway_exposure",
		Severity: audit.SeverityWarning,
		Reason:   "gateway listens beyond loopback (" + addr + "); acceptable only behind a reverse proxy or firewall",
		Evidence: map[string]any{
			"addr":        addr,
			"remediation": "set GATEWAY_ADDR=127.0.0.1:8080 unless a fronting proxy terminates external traffic",
		},
	}
}

// bindIsLoopback reports whether a listen address is loopback-only.
// ":8080" and "0.0.0.0:8080" bind every interface; "127.0.0.1:8080",
// "[::1]:8080" and "localhost:8080" do not.
func bindIsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port or unparseable — treat bare values conservatively.
		host = strings.Trim(addr, "[]")
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
