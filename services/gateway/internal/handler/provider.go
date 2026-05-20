// Package handler — provider listing endpoint.
//
// v0.3 Step 3 of `Plans/modular-puzzling-blum.md`. Exposes the
// `ListProviders` IPC verb (runtime/core/src/ipc.rs) over HTTP so the
// settings UI can render "currently active" alongside available
// providers without reaching into provider-specific code.
//
// The runtimed child owns provider definitions; this handler is a thin
// passthrough — it serializes the runtimed payload directly to the HTTP
// client so the Rust types stay the source of truth.

package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	gwruntime "github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/runtime"
)

// providersResponse mirrors ipc.ListProvidersResponse but with a stable
// HTTP shape — `Profiles` is `[]map[string]any` (not `[]json.RawMessage`)
// so the response is human-inspectable and the field names match the
// Rust serde output exactly.
type providersResponse struct {
	Active   string           `json:"active"`
	Profiles []map[string]any `json:"profiles"`
}

// ProviderHandler serves GET /api/v1/providers. Reads the active +
// known-profiles list from runtimed every call so the response always
// reflects current state (a future restart with a different
// IRONGOLEM_LLM_PROVIDER flips the answer without a cache flush).
type ProviderHandler struct {
	logger  *slog.Logger
	runtime providerRuntimeClient
	timeout time.Duration
}

// providerRuntimeClient is the live runtime client surface used by the
// handler. The real *gwruntime.Client satisfies it; tests substitute a
// fake. Kept private — the public interface is the handler itself.
type providerRuntimeClient interface {
	ListProvidersRaw(ctx context.Context) (active string, profiles []json.RawMessage, err error)
}

// NewProviderHandler builds a ProviderHandler against the real runtime
// client.
func NewProviderHandler(logger *slog.Logger, client *gwruntime.Client) *ProviderHandler {
	return &ProviderHandler{
		logger:  logger,
		runtime: realProviderRuntime{client: client},
		timeout: 5 * time.Second,
	}
}

// newProviderHandlerWithRuntime is the test seam. Unit tests inject a
// fake providerRuntimeClient instead of dialing runtimed.
func newProviderHandlerWithRuntime(logger *slog.Logger, runtime providerRuntimeClient) *ProviderHandler {
	return &ProviderHandler{logger: logger, runtime: runtime, timeout: 5 * time.Second}
}

// realProviderRuntime adapts *gwruntime.Client to providerRuntimeClient.
type realProviderRuntime struct {
	client *gwruntime.Client
}

func (r realProviderRuntime) ListProvidersRaw(ctx context.Context) (string, []json.RawMessage, error) {
	resp, err := r.client.ListProviders(ctx)
	if err != nil {
		return "", nil, err
	}
	return resp.Active, resp.Profiles, nil
}

// ListProviders responds with the active provider name and every known
// profile. GET /api/v1/providers.
//
// Errors map to 502 (runtime unavailable / IPC failure) so operators can
// distinguish a provider misconfiguration from a gateway bug.
func (h *ProviderHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	active, rawProfiles, err := h.runtime.ListProvidersRaw(ctx)
	if err != nil {
		h.logger.Warn("provider listing failed", slog.String("error", err.Error()))
		writeJSON(w, http.StatusBadGateway, map[string]string{
			"error":  "runtime_unavailable",
			"detail": err.Error(),
		})
		return
	}

	profiles := make([]map[string]any, 0, len(rawProfiles))
	for _, raw := range rawProfiles {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			// One bad profile shouldn't blank the whole list; skip and log.
			h.logger.Warn("skip undecodable provider profile", slog.String("error", err.Error()))
			continue
		}
		profiles = append(profiles, m)
	}

	resp := providersResponse{
		Active:   active,
		Profiles: profiles,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Warn("provider response encode failed", slog.String("error", err.Error()))
	}
}
