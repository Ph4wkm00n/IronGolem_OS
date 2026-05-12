package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HMAC-signed token authentication for v0.1 per Step 7 of the v0.1 plan
// (Plans/create-a-plan-to-glowing-nest.md).
//
// Wire format:
//
//   <tenant>:<user>:<role>:<channel>:<exp_unix_seconds>.<hex_hmac_sha256>
//
// The MAC is `HMAC-SHA256(secret, <payload>)` where <payload> is the entire
// colon-joined string before the trailing `.<mac>`. Fields must not contain
// `:` or `.`. `exp` is a Unix-seconds timestamp; verification rejects expired
// tokens (with a small configurable clock skew).
//
// Mutual TLS is the v0.3 successor; HMAC tokens are the minimum viable
// identity primitive that lets the gateway drop the prior header-trust model
// without waiting on PKI.

// TokenClaims is the parsed shape of a bearer token. Field names match the
// X-* request headers they replace.
type TokenClaims struct {
	TenantID  string
	UserID    string
	AgentRole string
	ChannelID string
	// ExpiresAt is the absolute UTC moment after which the token is invalid.
	ExpiresAt time.Time
}

// ErrInvalidToken is returned by VerifyToken on any malformed / expired /
// MAC-mismatched token. The middleware translates this into 401.
var ErrInvalidToken = errors.New("auth: invalid token")

// MintToken produces a bearer-token string for the supplied claims.
// `exp` is rounded to the nearest second; expirations in the past are
// rejected so callers don't accidentally create unusable tokens.
func MintToken(claims TokenClaims, secret []byte) (string, error) {
	if len(secret) == 0 {
		return "", errors.New("auth: secret required")
	}
	if claims.TenantID == "" {
		return "", errors.New("auth: tenant_id required")
	}
	if !claims.ExpiresAt.After(time.Now()) {
		return "", errors.New("auth: expiry must be in the future")
	}
	for _, v := range []string{claims.TenantID, claims.UserID, claims.AgentRole, claims.ChannelID} {
		if strings.ContainsAny(v, ":.") {
			return "", fmt.Errorf("auth: token field %q must not contain ':' or '.'", v)
		}
	}

	payload := fmt.Sprintf("%s:%s:%s:%s:%d",
		claims.TenantID, claims.UserID, claims.AgentRole, claims.ChannelID,
		claims.ExpiresAt.UTC().Unix(),
	)
	mac := hmacHex(secret, payload)
	return payload + "." + mac, nil
}

// VerifyToken parses and authenticates a bearer-token string. Returns
// ErrInvalidToken on any failure — the precise reason is logged but never
// surfaced to the caller (to avoid leaking signal to attackers).
func VerifyToken(token string, secret []byte, now time.Time, clockSkew time.Duration) (TokenClaims, error) {
	if len(secret) == 0 {
		return TokenClaims{}, errors.New("auth: secret required")
	}

	// Split off the trailing MAC on the LAST dot so payload colons aren't a
	// concern (and so a future MAC encoding can't be confused with a payload).
	dot := strings.LastIndexByte(token, '.')
	if dot < 1 || dot == len(token)-1 {
		return TokenClaims{}, ErrInvalidToken
	}
	payload := token[:dot]
	gotMac := token[dot+1:]
	expectedMac := hmacHex(secret, payload)
	if !hmac.Equal([]byte(gotMac), []byte(expectedMac)) {
		return TokenClaims{}, ErrInvalidToken
	}

	parts := strings.Split(payload, ":")
	if len(parts) != 5 {
		return TokenClaims{}, ErrInvalidToken
	}
	expUnix, err := strconv.ParseInt(parts[4], 10, 64)
	if err != nil {
		return TokenClaims{}, ErrInvalidToken
	}
	exp := time.Unix(expUnix, 0).UTC()
	if now.After(exp.Add(clockSkew)) {
		return TokenClaims{}, ErrInvalidToken
	}

	claims := TokenClaims{
		TenantID:  parts[0],
		UserID:    parts[1],
		AgentRole: parts[2],
		ChannelID: parts[3],
		ExpiresAt: exp,
	}
	if claims.TenantID == "" {
		return TokenClaims{}, ErrInvalidToken
	}
	return claims, nil
}

func hmacHex(secret []byte, payload string) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}

// AuthConfig configures HMACAuthMiddleware.
type AuthConfig struct {
	// Secret is the shared secret used to verify token MACs. Required.
	Secret []byte
	// ExemptPaths are paths bypassed by the auth check. /healthz is the
	// canonical exemption — liveness probes can't carry bearer tokens.
	ExemptPaths []string
	// ClockSkew is the allowance for clock drift between client and gateway.
	// Defaults to 30s when zero.
	ClockSkew time.Duration
	// Now overrides time.Now() for tests.
	Now func() time.Time
}

// identityContextKey holds parsed token claims for downstream middleware.
type identityContextKey struct{}

// IdentityFromContext returns the parsed token claims, if any, that were
// installed by HMACAuthMiddleware. Returns the zero value when absent —
// callers should check (claims.TenantID != "") before treating the context
// as authenticated.
func IdentityFromContext(ctx context.Context) TokenClaims {
	if v, ok := ctx.Value(identityContextKey{}).(TokenClaims); ok {
		return v
	}
	return TokenClaims{}
}

func withIdentity(ctx context.Context, claims TokenClaims) context.Context {
	return context.WithValue(ctx, identityContextKey{}, claims)
}

// HMACAuthMiddleware requires every non-exempt request to carry a valid
// `Authorization: Bearer <token>` header. On success, the parsed claims are
// injected into the request context so downstream middleware (tenant, policy)
// can read identity without trusting client-supplied headers. On failure it
// returns 401 with a generic body — the specific reason is logged but never
// echoed back.
func HMACAuthMiddleware(cfg AuthConfig, logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	skew := cfg.ClockSkew
	if skew == 0 {
		skew = 30 * time.Second
	}
	nowFn := cfg.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	exempt := make(map[string]bool, len(cfg.ExemptPaths))
	for _, p := range cfg.ExemptPaths {
		exempt[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if exempt[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			authz := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if !strings.HasPrefix(authz, prefix) {
				writeAuthError(w, "missing bearer token")
				return
			}
			token := strings.TrimSpace(authz[len(prefix):])
			if token == "" {
				writeAuthError(w, "missing bearer token")
				return
			}

			claims, err := VerifyToken(token, cfg.Secret, nowFn(), skew)
			if err != nil {
				logger.WarnContext(r.Context(), "token rejected",
					slog.String("error", err.Error()),
					slog.String("path", r.URL.Path),
					slog.String("remote_addr", r.RemoteAddr),
				)
				writeAuthError(w, "invalid token")
				return
			}

			ctx := withIdentity(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	// RFC 6750 §3 — Bearer challenge so clients know to retry with auth.
	w.Header().Set("WWW-Authenticate", `Bearer realm="irongolem"`)
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
