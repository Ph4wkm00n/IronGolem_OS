// Command mint-token prints a single HMAC bearer token suitable for
// authenticating against the gateway during local development and smoke
// tests. Step 7 of the v0.1 plan replaced header-trust auth with HMAC
// tokens; this binary is the canonical way to produce one without
// hand-rolling the HMAC.
//
// Usage:
//
//	IRONGOLEM_HMAC_SECRET=... mint-token \
//	    [--tenant default] [--user smoke] [--role executor] \
//	    [--channel smoke] [--ttl 1h]
//
// Reads the secret from $IRONGOLEM_HMAC_SECRET (required) so the secret
// never appears on argv. Writes the token to stdout with no trailing
// newline so callers can pipe it directly into an Authorization header.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/middleware"
)

func main() {
	tenant := flag.String("tenant", "default", "tenant id claim")
	user := flag.String("user", "smoke", "user id claim")
	role := flag.String("role", "executor", "agent role claim (must match a known policy role)")
	channel := flag.String("channel", "smoke", "channel id claim")
	ttl := flag.Duration("ttl", time.Hour, "token lifetime")
	flag.Parse()

	secret := []byte(os.Getenv("IRONGOLEM_HMAC_SECRET"))
	if len(secret) == 0 {
		fmt.Fprintln(os.Stderr, "IRONGOLEM_HMAC_SECRET is required")
		os.Exit(1)
	}

	tok, err := middleware.MintToken(middleware.TokenClaims{
		TenantID:  *tenant,
		UserID:    *user,
		AgentRole: *role,
		ChannelID: *channel,
		ExpiresAt: time.Now().Add(*ttl),
	}, secret)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mint-token:", err)
		os.Exit(1)
	}
	fmt.Print(tok)
}
