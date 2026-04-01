package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

// tenantContextKey is the context key for tenant ID.
type tenantContextKey struct{}

// TenantIDFromContext retrieves the tenant ID injected by TenantMiddleware.
func TenantIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(tenantContextKey{}).(string); ok {
		return v
	}
	return ""
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

// TenantMiddleware extracts X-Tenant-ID from the request header and injects
// it into the request context. In team mode the header is required; in solo
// mode a default tenant is auto-injected when the header is absent.
//
// The middleware also propagates the tenant ID into the telemetry context
// so that all downstream log lines are correlated.
func TenantMiddleware(logger *slog.Logger, mode DeploymentMode) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.Header.Get("X-Tenant-ID")

			switch mode {
			case ModeTeam:
				if tenantID == "" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error": "X-Tenant-ID header is required in team mode",
					})
					return
				}
			default:
				// Solo / household mode: auto-inject default tenant.
				if tenantID == "" {
					tenantID = defaultTenantID
				}
			}

			// Inject tenant into request context for downstream handlers.
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
