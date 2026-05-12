package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// tenantContextKey is the context key for tenant ID. Kept as its own
// context value so handlers can pull tenant ID without depending on the
// full TokenClaims struct.
type tenantContextKey struct{}

// TenantIDFromContext retrieves the tenant ID installed by TenantMiddleware.
func TenantIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(tenantContextKey{}).(string); ok {
		return v
	}
	return ""
}

// UserIDFromContext returns the authenticated user id from the HMAC token,
// or "" when the request was anonymous (solo mode + no token).
func UserIDFromContext(ctx context.Context) string {
	return IdentityFromContext(ctx).UserID
}

// AgentRoleFromContext returns the authenticated agent role from the token.
func AgentRoleFromContext(ctx context.Context) string {
	return IdentityFromContext(ctx).AgentRole
}

// ChannelIDFromContext returns the channel id from the token.
func ChannelIDFromContext(ctx context.Context) string {
	return IdentityFromContext(ctx).ChannelID
}

// withTenantID stores the tenant ID in the request context.
func withTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantContextKey{}, tenantID)
}

// DeploymentMode determines whether the gateway runs in solo or team mode.
type DeploymentMode string

const (
	// ModeSolo is the single-user local mode with auto-injected tenant.
	ModeSolo DeploymentMode = "solo"

	// ModeTeam is the multi-tenant mode requiring explicit tenant headers.
	ModeTeam DeploymentMode = "team"
)

// defaultTenantID is the auto-injected tenant for solo mode.
const defaultTenantID = "default"

// TenantMiddleware pulls the tenant id from the HMAC token claims that
// HMACAuthMiddleware installed earlier in the chain. In team mode the
// caller must supply a token (so a tenant is always present); in solo
// mode the tenant defaults to "default" when no token claim is present
// (e.g. for the /healthz path which is auth-exempt and shouldn't carry
// identity).
//
// Step 7 of the v0.1 plan replaced the prior `X-Tenant-ID` header trust
// model with the HMAC token. The previous header-reading behavior is gone:
// downstream code reads tenant id exclusively through TenantIDFromContext.
func TenantMiddleware(logger *slog.Logger, mode DeploymentMode) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := IdentityFromContext(r.Context()).TenantID

			switch mode {
			case ModeTeam:
				if tenantID == "" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error": "authenticated tenant required in team mode",
					})
					return
				}
			default:
				if tenantID == "" {
					tenantID = defaultTenantID
				}
			}

			ctx := withTenantID(r.Context(), tenantID)
			ctx = telemetry.WithTenantID(ctx, tenantID)

			logger.DebugContext(ctx, "tenant context set",
				slog.String("tenant_id", tenantID),
				slog.String("mode", string(mode)),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
