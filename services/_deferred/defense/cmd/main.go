// Package main is the entry point for the IronGolem OS Defense service.
//
// The defense service is the self-defending layer of the platform. It checks
// inputs for prompt injection, SSRF attempts, and anomalous behavior patterns.
// It also manages quarantine, incidents, config rollback, destination
// allowlists, and dangerous command filtering.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/defense/internal"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"
)

func main() {
	cfg := telemetry.DefaultConfig("defense")
	logger := telemetry.SetupLogger(cfg)
	slog.SetDefault(logger)

	detector := internal.NewThreatDetector(logger, internal.DetectorConfig{
		InjectionThreshold: 0.7,
		AnomalyWindow:     5 * time.Minute,
		AnomalyMaxVolume:  100,
	})

	// Initialize subsystems.
	quarantineStore := internal.NewInMemoryQuarantineStore()
	quarantineMgr := internal.NewQuarantineManager(logger, quarantineStore, nil)

	incidentStore := internal.NewInMemoryIncidentStore()
	incidentMgr := internal.NewIncidentManager(logger, incidentStore)

	rollbackStore := internal.NewInMemoryRollbackStore()
	rollbackMgr := internal.NewRollbackManager(logger, rollbackStore)

	allowlistStore := internal.NewInMemoryAllowlistStore()
	allowlistMgr := internal.NewAllowlistManager(logger, allowlistStore)

	commandFilter := internal.NewCommandFilter(logger)

	mux := http.NewServeMux()

	// Health check.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "defense",
			"time":    time.Now().UTC(),
		})
	})

	// Check input for threats.
	mux.HandleFunc("POST /api/v1/defense/check", func(w http.ResponseWriter, r *http.Request) {
		ctx, span := telemetry.NewSpan(r.Context(), "defense.check_input")
		defer span.End(logger)

		var req internal.CheckRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		if req.Input == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "input is required",
			})
			return
		}

		assessment := detector.Assess(ctx, req)

		// If any threats were detected and blocked, emit an event.
		if assessment.Blocked {
			payload, _ := json.Marshal(events.ThreatPayload{
				ThreatType:  assessment.PrimaryThreat,
				Severity:    assessment.Severity,
				Description: assessment.Summary,
				Score:       assessment.Score,
				Blocked:     true,
			})
			evt := events.NewEvent(events.EventKindThreatDetected, req.TenantID, "defense", payload)
			logger.WarnContext(ctx, "threat blocked",
				slog.String("event_id", evt.ID),
				slog.String("threat_type", assessment.PrimaryThreat),
				slog.Float64("score", assessment.Score),
				slog.String("tenant_id", req.TenantID),
			)
		}

		writeJSON(w, http.StatusOK, assessment)
	})

	// List blocked actions.
	mux.HandleFunc("GET /api/v1/defense/blocked", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.list_blocked")
		defer span.End(logger)

		blocked := detector.ListBlocked()
		writeJSON(w, http.StatusOK, map[string]any{
			"blocked": blocked,
			"count":   len(blocked),
		})
	})

	// --- Quarantine endpoints ---

	// List quarantined items.
	mux.HandleFunc("GET /api/v1/defense/quarantine", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.list_quarantine")
		defer span.End(logger)

		items := quarantineMgr.List()
		writeJSON(w, http.StatusOK, map[string]any{
			"quarantine": items,
			"count":      len(items),
		})
	})

	// Create quarantine item.
	mux.HandleFunc("POST /api/v1/defense/quarantine", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.create_quarantine")
		defer span.End(logger)

		var item internal.QuarantineItem
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		actions, err := quarantineMgr.Quarantine(item)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}

		payload, _ := json.Marshal(map[string]any{
			"target":   item.Target,
			"type":     item.Type,
			"severity": item.Severity,
		})
		_ = events.NewEvent(events.EventKindQuarantined, item.TenantID, "defense", payload)

		// Check if auto-incident creation is needed.
		incidentMgr.CheckAutoCreate(item.Target, item.TenantID)

		writeJSON(w, http.StatusCreated, map[string]any{
			"item":    item,
			"actions": actions,
		})
	})

	// Release quarantine item.
	mux.HandleFunc("POST /api/v1/defense/quarantine/{id}/release", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.release_quarantine")
		defer span.End(logger)

		id := r.PathValue("id")

		var req struct {
			Reviewer string `json:"reviewer"`
			Reason   string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		if err := quarantineMgr.Release(id, req.Reviewer, req.Reason); err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			}
			writeJSON(w, status, map[string]string{
				"error": err.Error(),
			})
			return
		}

		payload, _ := json.Marshal(map[string]any{
			"id":       id,
			"reviewer": req.Reviewer,
			"reason":   req.Reason,
		})
		_ = events.NewEvent(events.EventKindQuarantineReleased, "", "defense", payload)

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "released",
		})
	})

	// Escalate quarantine item.
	mux.HandleFunc("POST /api/v1/defense/quarantine/{id}/escalate", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.escalate_quarantine")
		defer span.End(logger)

		id := r.PathValue("id")

		var req struct {
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		if err := quarantineMgr.Escalate(id, req.Reason); err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			}
			writeJSON(w, status, map[string]string{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "escalated",
		})
	})

	// --- Incident endpoints ---

	// List incidents.
	mux.HandleFunc("GET /api/v1/defense/incidents", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.list_incidents")
		defer span.End(logger)

		incidents := incidentMgr.List()
		writeJSON(w, http.StatusOK, map[string]any{
			"incidents": incidents,
			"count":     len(incidents),
		})
	})

	// Create incident.
	mux.HandleFunc("POST /api/v1/defense/incidents", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.create_incident")
		defer span.End(logger)

		var req struct {
			Title            string                  `json:"title"`
			Summary          string                  `json:"summary"`
			Severity         internal.IncidentSeverity `json:"severity"`
			AffectedServices []string                `json:"affected_services"`
			TenantID         string                  `json:"tenant_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		incident, err := incidentMgr.Create(req.Title, req.Summary, req.Severity, req.AffectedServices, req.TenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}

		payload, _ := json.Marshal(map[string]any{
			"id":       incident.ID,
			"title":    incident.Title,
			"severity": incident.Severity,
		})
		_ = events.NewEvent(events.EventKindIncidentCreated, req.TenantID, "defense", payload)

		writeJSON(w, http.StatusCreated, incident)
	})

	// Get incident by ID.
	mux.HandleFunc("GET /api/v1/defense/incidents/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.get_incident")
		defer span.End(logger)

		id := r.PathValue("id")
		incident, ok := incidentMgr.Get(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "incident not found",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"incident":               incident,
			"plain_language_summary": internal.PlainLanguageSummary(incident),
		})
	})

	// Resolve incident.
	mux.HandleFunc("POST /api/v1/defense/incidents/{id}/resolve", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.resolve_incident")
		defer span.End(logger)

		id := r.PathValue("id")

		var req struct {
			RootCause  string `json:"root_cause"`
			Resolution string `json:"resolution"`
			Actor      string `json:"actor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		if err := incidentMgr.Resolve(id, req.RootCause, req.Resolution, req.Actor); err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			}
			writeJSON(w, status, map[string]string{
				"error": err.Error(),
			})
			return
		}

		payload, _ := json.Marshal(map[string]any{
			"id":         id,
			"root_cause": req.RootCause,
			"resolution": req.Resolution,
		})
		_ = events.NewEvent(events.EventKindIncidentResolved, "", "defense", payload)

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "resolved",
		})
	})

	// --- Rollback endpoints ---

	// List config snapshots.
	mux.HandleFunc("GET /api/v1/defense/rollback/snapshots", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.list_snapshots")
		defer span.End(logger)

		service := r.URL.Query().Get("service")
		snapshots := rollbackMgr.ListSnapshots(service)
		writeJSON(w, http.StatusOK, map[string]any{
			"snapshots": snapshots,
			"count":     len(snapshots),
		})
	})

	// Take a config snapshot.
	mux.HandleFunc("POST /api/v1/defense/rollback/snapshots", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.take_snapshot")
		defer span.End(logger)

		var req struct {
			Service string          `json:"service"`
			Config  json.RawMessage `json:"config"`
			Label   string          `json:"label"`
			IsGood  bool            `json:"is_good"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		snapshot, err := rollbackMgr.TakeSnapshot(req.Service, req.Config, req.Label, req.IsGood)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusCreated, snapshot)
	})

	// Restore config from snapshot.
	mux.HandleFunc("POST /api/v1/defense/rollback/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.rollback_restore")
		defer span.End(logger)

		id := r.PathValue("id")

		var req struct {
			CurrentConfig json.RawMessage `json:"current_config"`
		}
		// Body is optional (current config for auto-backup).
		_ = json.NewDecoder(r.Body).Decode(&req)

		result, err := rollbackMgr.Rollback(id, req.CurrentConfig)
		if err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "not found") {
				status = http.StatusNotFound
			}
			writeJSON(w, status, map[string]string{
				"error": err.Error(),
			})
			return
		}

		payload, _ := json.Marshal(map[string]any{
			"service":     result.Service,
			"snapshot_id": result.SnapshotID,
		})
		_ = events.NewEvent(events.EventKindConfigRollback, "", "defense", payload)

		writeJSON(w, http.StatusOK, result)
	})

	// --- Allowlist endpoints ---

	// List allowlist entries.
	mux.HandleFunc("GET /api/v1/defense/allowlist", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.list_allowlist")
		defer span.End(logger)

		workspaceID := r.URL.Query().Get("workspace_id")
		entries := allowlistMgr.List(workspaceID)
		writeJSON(w, http.StatusOK, map[string]any{
			"entries": entries,
			"count":   len(entries),
		})
	})

	// Add allowlist entry.
	mux.HandleFunc("POST /api/v1/defense/allowlist", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.add_allowlist")
		defer span.End(logger)

		var entry internal.AllowlistEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		saved, err := allowlistMgr.AddEntry(entry)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusCreated, saved)
	})

	// Check destination against allowlist.
	mux.HandleFunc("POST /api/v1/defense/allowlist/check", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.check_allowlist")
		defer span.End(logger)

		var req struct {
			URL         string `json:"url"`
			WorkspaceID string `json:"workspace_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		result := allowlistMgr.CheckDestination(req.URL, req.WorkspaceID)
		writeJSON(w, http.StatusOK, result)
	})

	// --- Command filter endpoints ---

	// Check command against deny patterns.
	mux.HandleFunc("POST /api/v1/defense/commands/check", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.check_command")
		defer span.End(logger)

		var req struct {
			Command  string `json:"command"`
			UserID   string `json:"user_id"`
			TenantID string `json:"tenant_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid request body",
			})
			return
		}

		result := commandFilter.Check(req.Command, req.UserID, req.TenantID)

		if !result.Allowed {
			payload, _ := json.Marshal(map[string]any{
				"command":  req.Command,
				"pattern":  result.MatchedPattern,
				"severity": result.Severity,
			})
			_ = events.NewEvent(events.EventKindCommandBlocked, req.TenantID, "defense", payload)
		}

		writeJSON(w, http.StatusOK, result)
	})

	// List command audit log.
	mux.HandleFunc("GET /api/v1/defense/commands/audit", func(w http.ResponseWriter, r *http.Request) {
		_, span := telemetry.NewSpan(r.Context(), "defense.command_audit")
		defer span.End(logger)

		log := commandFilter.AuditLog()
		writeJSON(w, http.StatusOK, map[string]any{
			"audit_log": log,
			"count":     len(log),
		})
	})

	addr := envOrDefault("DEFENSE_ADDR", ":8083")
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("defense service starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("defense service shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}

	logger.Info("defense service stopped")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
