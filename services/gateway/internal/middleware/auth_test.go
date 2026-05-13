package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestMintAndVerify_RoundTrip(t *testing.T) {
	secret := []byte("hunter2-please-use-a-real-secret")
	// `now` must be wall-clock current because MintToken rejects expiries in
	// the past against time.Now(); a hardcoded date drifts past tolerance
	// the moment the calendar moves. Round to seconds so the equality check
	// at the bottom holds.
	now := time.Now().UTC().Truncate(time.Second)
	exp := now.Add(15 * time.Minute)

	tok, err := MintToken(TokenClaims{
		TenantID:    "tenant-a",
		WorkspaceID: "ws-7",
		UserID:      "alice",
		AgentRole:   "operator",
		ChannelID:   "chat-1",
		ExpiresAt:   exp,
	}, secret)
	if err != nil {
		t.Fatalf("MintToken: %v", err)
	}

	claims, err := VerifyToken(tok, secret, now, 30*time.Second)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if claims.TenantID != "tenant-a" || claims.WorkspaceID != "ws-7" || claims.UserID != "alice" || claims.AgentRole != "operator" || claims.ChannelID != "chat-1" {
		t.Fatalf("claims roundtrip mismatch: %+v", claims)
	}
	// Expiry is rounded to the second; allow exact match.
	if !claims.ExpiresAt.Equal(exp.Truncate(time.Second)) {
		t.Fatalf("exp drift: got %v, want %v", claims.ExpiresAt, exp.Truncate(time.Second))
	}
}

func TestVerifyToken_RejectsTampering(t *testing.T) {
	secret := []byte("s")
	exp := time.Now().Add(time.Hour)
	tok, err := MintToken(TokenClaims{TenantID: "t", ExpiresAt: exp}, secret)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	// Flip a bit in the MAC.
	tampered := tok[:len(tok)-1] + flip(tok[len(tok)-1])
	if _, err := VerifyToken(tampered, secret, time.Now(), 30*time.Second); err == nil {
		t.Fatal("VerifyToken accepted tampered MAC")
	}
}

func TestVerifyToken_RejectsExpired(t *testing.T) {
	secret := []byte("s")
	// Same calendar-drift rationale as TestMintAndVerify_RoundTrip: mint
	// against wall-clock now, then verify with a synthetic future `now`
	// that's past the configured skew window.
	now := time.Now().UTC().Truncate(time.Second)
	tok, _ := MintToken(TokenClaims{TenantID: "t", ExpiresAt: now.Add(5 * time.Minute)}, secret)

	// Verify 10 minutes after exp (well past clock skew).
	if _, err := VerifyToken(tok, secret, now.Add(15*time.Minute), 30*time.Second); err == nil {
		t.Fatal("VerifyToken accepted expired token")
	}
}

func TestVerifyToken_RejectsWrongSecret(t *testing.T) {
	good := []byte("right")
	bad := []byte("wrong")
	tok, _ := MintToken(TokenClaims{TenantID: "t", ExpiresAt: time.Now().Add(time.Hour)}, good)
	if _, err := VerifyToken(tok, bad, time.Now(), 30*time.Second); err == nil {
		t.Fatal("VerifyToken accepted wrong secret")
	}
}

// TestVerifyToken_RejectsV01TokenShape proves Step 2's no-compat promise:
// a token minted under the v0.1 5-field payload (tenant:user:role:channel:exp)
// is rejected even when its MAC is correctly recomputed against the same
// secret. v0.2 requires exactly 6 fields with the workspace_id in slot 1.
func TestVerifyToken_RejectsV01TokenShape(t *testing.T) {
	secret := []byte("s")
	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Second).Unix()
	// Hand-craft a v0.1-shape payload (no workspace_id slot).
	payload := "tenant-a:alice:operator:chat-1:" + strconvI(exp)
	mac := hmacHex(secret, payload)
	v01Token := payload + "." + mac

	if _, err := VerifyToken(v01Token, secret, time.Now(), 30*time.Second); err == nil {
		t.Fatal("VerifyToken accepted a v0.1 (5-field) token; v0.2 must reject")
	}
}

// strconvI is a local helper so the v0.1-shape constructor stays inline
// without importing strconv just for one test.
func strconvI(n int64) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var out [20]byte
	i := len(out)
	for n > 0 {
		i--
		out[i] = digits[n%10]
		n /= 10
	}
	if negative {
		i--
		out[i] = '-'
	}
	return string(out[i:])
}

func TestMintToken_RejectsForbiddenChars(t *testing.T) {
	secret := []byte("s")
	exp := time.Now().Add(time.Hour)
	for _, c := range []string{":", "."} {
		_, err := MintToken(TokenClaims{TenantID: "t" + c + "x", ExpiresAt: exp}, secret)
		if err == nil {
			t.Fatalf("MintToken accepted forbidden char %q in tenant_id", c)
		}
	}
}

func TestHMACAuthMiddleware_AllowsExemptPath(t *testing.T) {
	mw := HMACAuthMiddleware(AuthConfig{
		Secret:      []byte("s"),
		ExemptPaths: []string{"/healthz"},
	}, quietLogger())

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/healthz", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("/healthz blocked: %d", rr.Code)
	}
}

func TestHMACAuthMiddleware_Rejects401_MissingToken(t *testing.T) {
	mw := HMACAuthMiddleware(AuthConfig{Secret: []byte("s")}, quietLogger())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/v1/messages/inbound", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if !strings.HasPrefix(rr.Header().Get("WWW-Authenticate"), "Bearer ") {
		t.Fatalf("missing Bearer challenge header: %q", rr.Header().Get("WWW-Authenticate"))
	}
}

func TestHMACAuthMiddleware_InjectsClaims(t *testing.T) {
	secret := []byte("s")
	exp := time.Now().Add(time.Hour)
	tok, _ := MintToken(TokenClaims{
		TenantID:  "tenant-b",
		UserID:    "bob",
		AgentRole: "operator",
		ChannelID: "chat-9",
		ExpiresAt: exp,
	}, secret)

	mw := HMACAuthMiddleware(AuthConfig{Secret: secret}, quietLogger())
	var seen TokenClaims
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = IdentityFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/v1/messages/inbound", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if seen.TenantID != "tenant-b" || seen.UserID != "bob" {
		t.Fatalf("claims not injected: %+v", seen)
	}
}

func flip(b byte) string {
	// Flip the low bit; produces a different hex digit (mostly).
	if b == '0' {
		return "1"
	}
	return string(b - 1)
}
