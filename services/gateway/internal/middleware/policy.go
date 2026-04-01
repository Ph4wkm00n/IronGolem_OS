package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/policy"
)

// routeMapping maps HTTP method + path pattern to a policy Permission.
// The keys use the format "METHOD /path/pattern" where path segments
// containing {id} are normalized away.
var routeMapping = map[string]policy.Permission{
	// Recipe actions
	"POST /api/v1/recipes/activate":   {Resource: "recipe.activate", Action: "execute"},
	"POST /api/v1/recipes/deactivate": {Resource: "recipe.deactivate", Action: "execute"},
	"GET /api/v1/recipes":             {Resource: "recipe", Action: "read"},

	// Squad actions
	"POST /api/v1/squads/activate": {Resource: "squad.activate", Action: "execute"},
	"POST /api/v1/squads/pause":    {Resource: "squad.pause", Action: "execute"},
	"POST /api/v1/squads/run":      {Resource: "squad.run", Action: "execute"},
	"POST /api/v1/squads":          {Resource: "squad.create", Action: "write"},
	"GET /api/v1/squads":           {Resource: "squad", Action: "read"},

	// Approval actions
	"POST /api/v1/approvals/approve": {Resource: "approval.approve", Action: "execute"},
	"POST /api/v1/approvals/deny":    {Resource: "approval.deny", Action: "execute"},
	"GET /api/v1/approvals":          {Resource: "approval", Action: "read"},

	// Message actions
	"POST /api/v1/messages/inbound":  {Resource: "message.inbound", Action: "write"},
	"POST /api/v1/messages/outbound": {Resource: "message.outbound", Action: "write"},

	// Connector actions
	"POST /api/v1/connectors/connect":    {Resource: "connector.connect", Action: "execute"},
	"POST /api/v1/connectors/disconnect": {Resource: "connector.disconnect", Action: "execute"},
	"POST /api/v1/connectors/heartbeat":  {Resource: "connector.heartbeat", Action: "write"},
	"GET /api/v1/connectors":             {Resource: "connector", Action: "read"},

	// Event / timeline actions
	"GET /api/v1/events": {Resource: "event", Action: "read"},
}

// normalizeRoute strips path parameter values to produce a canonical route key.
// Example: "POST /api/v1/recipes/abc123/activate" -> "POST /api/v1/recipes/activate"
func normalizeRoute(method, path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	var cleaned []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		// Skip segments that look like IDs (contain digits or hyphens
		// typical of UUIDs / generated IDs) unless they are known keywords.
		if looksLikeID(p) {
			continue
		}
		cleaned = append(cleaned, p)
	}
	return method + " /" + strings.Join(cleaned, "/")
}

// knownSegments are path segments that should never be treated as IDs.
var knownSegments = map[string]bool{
	"api": true, "v1": true, "recipes": true, "squads": true,
	"approvals": true, "messages": true, "connectors": true, "events": true,
	"activate": true, "deactivate": true, "pause": true, "run": true,
	"approve": true, "deny": true, "connect": true, "disconnect": true,
	"heartbeat": true, "inbound": true, "outbound": true, "status": true,
	"healthz": true,
}

func looksLikeID(segment string) bool {
	if knownSegments[segment] {
		return false
	}
	return true
}

// PolicyDeniedResponse is the JSON body returned when a policy layer denies a request.
type PolicyDeniedResponse struct {
	Error    string `json:"error"`
	Layer    string `json:"layer"`
	Reason   string `json:"reason"`
	Decision string `json:"decision"`
}

// PolicyApprovalResponse is the JSON body returned when action needs approval.
type PolicyApprovalResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Layer   string `json:"layer"`
}

// EventEmitter is an interface for appending events to the audit trail.
// This avoids coupling the middleware to a concrete event store implementation.
type EventEmitter interface {
	Append(evt events.Event)
}

// PolicyMiddleware evaluates every request against the five-layer policy
// engine. It extracts identity from request headers and maps the HTTP route
// to a policy resource. On denial it returns 403 with a structured JSON body
// explaining which layer denied and why. All decisions are logged for the
// audit trail.
func PolicyMiddleware(engine policy.PolicyEngine, logger *slog.Logger, emitter EventEmitter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip policy check for health endpoint.
			if r.URL.Path == "/healthz" {
				next.ServeHTTP(w, r)
				return
			}

			tenantID := TenantIDFromContext(r.Context())
			userID := r.Header.Get("X-User-ID")
			agentRole := r.Header.Get("X-Agent-Role")
			channelID := r.Header.Get("X-Channel-ID")

			// Resolve the permission for this route.
			routeKey := normalizeRoute(r.Method, r.URL.Path)
			perm, found := routeMapping[routeKey]
			if !found {
				// No policy mapping means no enforcement needed (e.g. healthz).
				next.ServeHTTP(w, r)
				return
			}

			evalReq := policy.EvalRequest{
				TenantID:   tenantID,
				UserID:     userID,
				AgentRole:  agentRole,
				ChannelID:  channelID,
				Permission: perm,
			}

			result, err := engine.Evaluate(r.Context(), evalReq)
			if err != nil {
				logger.ErrorContext(r.Context(), "policy evaluation error",
					slog.String("error", err.Error()),
					slog.String("route", routeKey),
					slog.String("tenant_id", tenantID),
				)
			}

			// Log every decision for the audit trail.
			logger.InfoContext(r.Context(), "policy decision",
				slog.String("decision", string(result.Decision)),
				slog.String("layer", result.Layer.String()),
				slog.String("reason", result.Reason),
				slog.String("resource", perm.Resource),
				slog.String("action", perm.Action),
				slog.String("tenant_id", tenantID),
				slog.String("user_id", userID),
				slog.String("agent_role", agentRole),
				slog.String("channel_id", channelID),
			)

			switch result.Decision {
			case policy.DecisionDeny:
				// Emit an ActionBlocked event for the audit trail.
				if emitter != nil {
					payload, _ := json.Marshal(map[string]string{
						"resource":   perm.Resource,
						"action":     perm.Action,
						"layer":      result.Layer.String(),
						"reason":     result.Reason,
						"user_id":    userID,
						"agent_role": agentRole,
						"channel_id": channelID,
					})
					evt := events.NewEvent(events.EventKindPolicyDenied, tenantID, "gateway", payload)
					emitter.Append(evt)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(PolicyDeniedResponse{
					Error:    "action denied by policy",
					Layer:    result.Layer.String(),
					Reason:   result.Reason,
					Decision: string(result.Decision),
				})
				return

			case policy.DecisionAudit:
				// Allow but flag for review: return 202 to indicate approval is needed.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				_ = json.NewEncoder(w).Encode(PolicyApprovalResponse{
					Status:  "approval_required",
					Message: "This action requires approval before execution.",
					Layer:   result.Layer.String(),
				})
				return
			}

			// DecisionAllow: proceed to the handler.
			next.ServeHTTP(w, r)
		})
	}
}
