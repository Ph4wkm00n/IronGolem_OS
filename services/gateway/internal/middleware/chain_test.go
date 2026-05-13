package middleware_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/middleware"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/policy"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestChain_AuthThenTenantThenPolicy exercises the full Step 7 middleware
// chain (HMACAuth → Tenant → Policy) against a real route mapping. Verifies:
//   - exempt /healthz bypasses auth
//   - missing token returns 401 before the policy layer ever runs
//   - a valid-but-no-role token reaches the policy layer (which 403s)
//   - a valid token with sufficient role passes through to the handler
func TestChain_AuthThenTenantThenPolicy(t *testing.T) {
	secret := []byte("test-secret")
	exp := time.Now().Add(time.Hour)

	tokOperator, err := middleware.MintToken(middleware.TokenClaims{
		TenantID:    "tenant-x",
		WorkspaceID: "ws-22",
		UserID:      "alice",
		AgentRole:   "executor",
		ChannelID:   "chat-1",
		ExpiresAt:   exp,
	}, secret)
	if err != nil {
		t.Fatalf("mint operator: %v", err)
	}

	called := false
	terminal := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// Echo back tenant + workspace so the test can confirm BOTH
		// claims flowed through (v0.2 Step 2 added workspace).
		tenant := middleware.TenantIDFromContext(r.Context())
		workspace := middleware.WorkspaceIDFromContext(r.Context())
		_, _ = w.Write([]byte(tenant + "/" + workspace))
	})

	logger := quietLogger()
	engine := policy.NewDefaultPolicyEngine(logger)

	var handler http.Handler = terminal
	// Build the chain in the SAME order as main.go (innermost first below
	// because Go middleware wraps the inner handler).
	handler = middleware.PolicyMiddleware(engine, logger, nil)(handler)
	handler = middleware.TenantMiddleware(logger, middleware.ModeSolo)(handler)
	handler = middleware.HMACAuthMiddleware(middleware.AuthConfig{
		Secret:      secret,
		ExemptPaths: []string{"/healthz"},
	}, logger)(handler)

	// 1. /healthz — exempt; should reach the terminal handler even without auth.
	called = false
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/healthz", nil))
	if !called {
		t.Fatalf("/healthz did not reach handler; status=%d body=%q", rr.Code, rr.Body.String())
	}

	// 2. Protected route, no token → 401 (auth fires before policy).
	called = false
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/recipes", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: expected 401, got %d", rr.Code)
	}
	if called {
		t.Fatal("missing token: handler should not have been called")
	}

	// 3. Protected route, valid operator token → reaches handler.
	called = false
	rr = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/recipes", nil)
	req.Header.Set("Authorization", "Bearer "+tokOperator)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("valid token: expected 200, got %d body=%q", rr.Code, rr.Body.String())
	}
	if !called {
		t.Fatal("valid token: handler was not called")
	}
	if rr.Body.String() != "tenant-x/ws-22" {
		t.Fatalf("tenant+workspace did not flow through chain: got %q want tenant-x/ws-22", rr.Body.String())
	}

	// 4. Protected route, token with an UNKNOWN agent role → policy 403.
	// The role list lives in policy.go; "wizard" is intentionally absent.
	tokWizard, _ := middleware.MintToken(middleware.TokenClaims{
		TenantID:    "tenant-x",
		WorkspaceID: "ws-22",
		UserID:      "alice",
		AgentRole:   "wizard",
		ChannelID:   "chat-1",
		ExpiresAt:   exp,
	}, secret)
	called = false
	rr = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/v1/messages/inbound", nil)
	req.Header.Set("Authorization", "Bearer "+tokWizard)
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("unknown role: expected 403 from policy, got %d body=%q", rr.Code, rr.Body.String())
	}
	if called {
		t.Fatal("unknown role: handler should not have been called")
	}

}
