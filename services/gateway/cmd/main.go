// Package main is the entry point for the IronGolem OS Gateway service.
//
// The gateway is the front door for all external communication. It handles
// message ingress and egress, connector lifecycle management, recipe gallery
// and activation, approval workflows, event timeline, and applies Layer 1
// (Gateway Identity) of the five-layer security model.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/audit/probes"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/commitments"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/connector"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/handler"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/middleware"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/persist"
	"github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/planner"
	gwpolicy "github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/policy"
	gwruntime "github.com/Ph4wkm00n/IronGolem_OS/services/gateway/internal/runtime"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/policy"
	ipcruntime "github.com/Ph4wkm00n/IronGolem_OS/services/pkg/runtime"
	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/telemetry"

	// Blank imports: each connector subpackage's init() self-registers
	// in the connectors registry. v0.3 Step 1 introduced this; new
	// connectors land here AND in services/gateway/cmd/doctor/main.go.
	_ "github.com/Ph4wkm00n/IronGolem_OS/connectors/discord"
	_ "github.com/Ph4wkm00n/IronGolem_OS/connectors/email"
	_ "github.com/Ph4wkm00n/IronGolem_OS/connectors/signal"
	_ "github.com/Ph4wkm00n/IronGolem_OS/connectors/slack"
	_ "github.com/Ph4wkm00n/IronGolem_OS/connectors/telegram"
	_ "github.com/Ph4wkm00n/IronGolem_OS/connectors/webhook"
)

func main() {
	cfg := telemetry.DefaultConfig("gateway")
	logger := telemetry.SetupLogger(cfg)

	slog.SetDefault(logger)

	connMgr := connector.NewManager(logger)

	// Spawn the runtimed child. Boot fails closed if the binary is missing
	// or the initial spawn errors — the gateway has no value without it.
	// Set IRONGOLEM_RUNTIMED_PATH to point at the runtimed binary (defaults
	// to ./runtimed alongside the gateway).
	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
	runtimeClient, err := gwruntime.New(runtimeCtx, gwruntime.Config{
		BinaryPath: envOrDefault("IRONGOLEM_RUNTIMED_PATH", "./runtimed"),
	}, logger)
	if err != nil {
		logger.Error("runtime client init failed", slog.String("error", err.Error()))
		runtimeCancel()
		os.Exit(1)
	}

	// Open the persistent SQLite database and run migrations. Boot fails
	// closed if the file is unwritable — silent fallback to in-memory
	// would mean restarts silently lose state, which is exactly what
	// Step 6 was meant to fix.
	dbPath := envOrDefault("IRONGOLEM_GATEWAY_DB", persist.DefaultDBPath)
	db, err := persist.Open(dbPath)
	if err != nil {
		logger.Error("gateway db open failed",
			slog.String("path", dbPath),
			slog.String("error", err.Error()),
		)
		runtimeCancel()
		os.Exit(1)
	}
	logger.Info("gateway db ready", slog.String("path", dbPath))

	// Shared SQLite-backed stores. Recipe and squad stores seed built-ins
	// on first run; event and approval stores stay empty until traffic
	// arrives.
	eventStore := handler.NewSQLiteEventStore(db, logger)
	recipeStore, err := handler.NewSQLiteRecipeStore(db, logger)
	if err != nil {
		logger.Error("recipe store init failed", slog.String("error", err.Error()))
		runtimeCancel()
		os.Exit(1)
	}
	approvalStore := handler.NewSQLiteApprovalStore(db, logger)
	squadStore, err := handler.NewSQLiteSquadStore(db, logger)
	if err != nil {
		logger.Error("squad store init failed", slog.String("error", err.Error()))
		runtimeCancel()
		os.Exit(1)
	}

	// Wire the runtime client + event store into the inbound handler so
	// MessageInbound synthesizes a plan, executes it via runtimed, and
	// returns the LLM reply.
	h := handler.NewWithOptions(logger, connMgr, handler.Options{
		Runtime:    runtimeClient,
		EventStore: eventStore,
	})

	// The connector pump shares the same inbound path as HTTP /messages/inbound.
	connMgr.SetInboundHandler(func(ctx context.Context, msg connector.InboundMessage) (string, error) {
		res, err := h.HandleInbound(ctx, planner.InboundMessage{
			ConnectorID: msg.ConnectorID,
			ChannelID:   msg.ChannelID,
			UserID:      msg.UserID,
			Content:     msg.Content,
			TenantID:    msg.TenantID,
			WorkspaceID: msg.WorkspaceID,
		})
		if err != nil {
			return "", err
		}
		return res.Reply, nil
	})

	// v0.2 Step 1: register the real Telegram connector when a bot token is
	// supplied. The IRONGOLEM_TELEGRAM_API_BASE env override is the seam
	// `smoke-telegram` uses to point the connector at an httptest server.
	// Absence of the token is the explicit "Telegram is off" signal — no
	// reflection on whether the connectors module is linked.
	if token := os.Getenv("IRONGOLEM_TELEGRAM_BOT_TOKEN"); token != "" {
		tgCtx, tgCancel := context.WithCancel(context.Background())
		tgSource, tgErr := connector.NewTelegramSource(tgCtx, connector.TelegramSourceConfig{
			ConnectorID:    envOrDefault("IRONGOLEM_TELEGRAM_CONNECTOR_ID", "telegram"),
			BotToken:       token,
			APIBase:        os.Getenv("IRONGOLEM_TELEGRAM_API_BASE"),
			AllowedChatIDs: os.Getenv("IRONGOLEM_TELEGRAM_ALLOWED_CHAT_IDS"),
			TenantID:       envOrDefault("IRONGOLEM_TELEGRAM_TENANT_ID", "default"),
		})
		if tgErr != nil {
			logger.Error("telegram source init failed", slog.String("error", tgErr.Error()))
			tgCancel()
			runtimeCancel()
			os.Exit(1)
		}
		if err := connMgr.RegisterSource(envOrDefault("IRONGOLEM_TELEGRAM_CONNECTOR_ID", "telegram"), tgSource); err != nil {
			logger.Error("telegram source register failed", slog.String("error", err.Error()))
			tgCancel()
			runtimeCancel()
			os.Exit(1)
		}
		// The source goroutine inherits tgCtx; DisconnectAll on shutdown
		// cancels the pump context, which in turn drains the Telegram
		// poll goroutine.
		defer tgCancel()
		logger.Info("telegram connector registered",
			slog.String("connector_id", envOrDefault("IRONGOLEM_TELEGRAM_CONNECTOR_ID", "telegram")),
			slog.Bool("custom_api_base", os.Getenv("IRONGOLEM_TELEGRAM_API_BASE") != ""),
		)
	}

	recipeHandler := handler.NewRecipeHandler(logger, recipeStore, eventStore)
	approvalHandler := handler.NewApprovalHandler(logger, approvalStore, eventStore)
	timelineHandler := handler.NewTimelineHandler(logger, eventStore)
	squadHandler := handler.NewSquadHandler(logger, squadStore, eventStore)
	inboxHandler := handler.NewInboxHandler(logger, eventStore)
	homeHandler := handler.NewHomeHandler(logger, eventStore, connMgr, db)
	healthStatusHandler := handler.NewHealthStatusHandler(logger, connMgr, db)
	// v0.3 Step 3 — provider listing surfaces the active LLM provider +
	// every profile runtimed knows about for the settings UI.
	providerHandler := handler.NewProviderHandler(logger, runtimeClient)

	// v0.3 Step 5 — Audit probe subsystem. The registry is constructed
	// here (rather than as a package global) so tests can spin up an
	// isolated runtime without touching live probe state.
	auditRegistry := audit.NewRegistry()
	auditRegistry.MustRegister(probes.NewWorkspaceSkillEscape())
	auditRegistry.MustRegister(probes.NewTrustModel())
	auditRegistry.MustRegister(probes.NewChannelDMPolicy(db))
	auditRegistry.MustRegister(probes.NewConnectorHealthDrift())
	// v0.4 — ports of openclaw's highest-leverage probes (2026-07 scan):
	// exposure composition (channel openness x action privilege),
	// state-file permissions, and gateway bind/secret drift.
	auditRegistry.MustRegister(probes.NewExposureComposition(db))
	auditRegistry.MustRegister(probes.NewFSPermissions(dbPath))
	auditRegistry.MustRegister(probes.NewGatewayExposure())
	auditFindingStore := audit.NewSQLiteFindingStore(db)
	auditEmitter := handler.NewAuditFindingEmitter(eventStore)
	auditRuntime := audit.NewRuntime(auditRegistry, auditFindingStore, auditEmitter, logger, audit.RuntimeConfig{})
	auditCtx, auditCancel := context.WithCancel(context.Background())
	go auditRuntime.Run(auditCtx)
	auditFindingsHandler := handler.NewAuditFindingsHandler(auditFindingStore, logger)

	// v0.3 Step 4 — Commitments subsystem. The heuristic (regex +
	// keyword) extractor remains the default. v0.4: setting
	// IRONGOLEM_COMMITMENTS_EXTRACTOR=llm opts into LLM extraction over
	// runtimed's direct llm_call verb, with the heuristic as fallback
	// whenever the model path fails or is cooling down. The dispatcher
	// fires reminders through the connector manager's SendOutbound path.
	commitmentsStore := commitments.NewSQLiteStore(db)
	var commitmentsExtractor commitments.Extractor = commitments.NewHeuristicExtractor()
	if os.Getenv("IRONGOLEM_COMMITMENTS_EXTRACTOR") == "llm" {
		llmCall := func(ctx context.Context, workspaceID, system, prompt string) (string, error) {
			resp, err := runtimeClient.LlmCall(ctx, workspaceID, "commitments.extract", system, prompt, 1024)
			if err != nil {
				return "", err
			}
			if resp.Status != ipcruntime.StatusCompleted {
				return "", fmt.Errorf("llm_call failed: %s", resp.Error)
			}
			return resp.Content, nil
		}
		commitmentsExtractor = commitments.NewLLMExtractor(llmCall, commitments.NewHeuristicExtractor(), logger)
		logger.Info("commitments: LLM extractor enabled (heuristic fallback armed)")
	}
	commitmentsDispatcher := handler.NewCommitmentDispatcher(connMgr, logger)
	commitmentsEmitter := handler.NewCommitmentEventEmitter(eventStore)
	commitmentsRuntime := commitments.NewRuntime(commitmentsStore, commitmentsDispatcher, commitmentsEmitter, logger, commitments.RuntimeConfig{})
	commitmentsCtx, commitmentsCancel := context.WithCancel(context.Background())
	go commitmentsRuntime.Run(commitmentsCtx)
	commitmentsHandler := handler.NewCommitmentsHandler(commitmentsStore, logger)

	// Hook into the inbound flow: every successful assistant reply
	// kicks off an async extraction. The reply path can't block on
	// extractor + DB latency, so we run it in its own goroutine.
	h.SetPostReplyHook(func(ctx context.Context, msg planner.InboundMessage, reply, inboundEventID string) {
		go commitmentsRuntime.Enqueue(context.Background(), commitmentsExtractor, commitments.Turn{
			UserText:      msg.Content,
			AssistantText: reply,
			WorkspaceID:   msg.WorkspaceID,
			TenantID:      msg.TenantID,
			SourceEventID: inboundEventID,
			ConnectorID:   msg.ConnectorID,
			ChannelID:     msg.ChannelID,
			UserID:        msg.UserID,
		})
	})

	mux := http.NewServeMux()

	// Health check.
	mux.HandleFunc("GET /healthz", h.HealthCheck)

	// Message routes.
	mux.HandleFunc("POST /api/v1/messages/inbound", h.MessageInbound)
	mux.HandleFunc("POST /api/v1/messages/outbound", h.MessageOutbound)

	// Connector routes.
	mux.HandleFunc("GET /api/v1/connectors/{id}/status", h.ConnectorStatus)
	mux.HandleFunc("POST /api/v1/connectors/{id}/connect", h.ConnectorConnect)
	mux.HandleFunc("POST /api/v1/connectors/{id}/disconnect", h.ConnectorDisconnect)
	mux.HandleFunc("POST /api/v1/connectors/{id}/heartbeat", h.ConnectorHeartbeat)

	// Recipe routes.
	mux.HandleFunc("GET /api/v1/recipes", recipeHandler.ListRecipes)
	mux.HandleFunc("GET /api/v1/recipes/{id}", recipeHandler.GetRecipe)
	mux.HandleFunc("POST /api/v1/recipes/{id}/activate", recipeHandler.ActivateRecipe)
	mux.HandleFunc("POST /api/v1/recipes/{id}/deactivate", recipeHandler.DeactivateRecipe)

	// Approval routes.
	mux.HandleFunc("GET /api/v1/approvals", approvalHandler.ListApprovals)
	mux.HandleFunc("GET /api/v1/approvals/{id}", approvalHandler.GetApproval)
	mux.HandleFunc("POST /api/v1/approvals/{id}/approve", approvalHandler.ApproveAction)
	mux.HandleFunc("POST /api/v1/approvals/{id}/deny", approvalHandler.DenyAction)

	// Squad routes.
	mux.HandleFunc("GET /api/v1/squads", squadHandler.ListSquads)
	mux.HandleFunc("GET /api/v1/squads/{id}", squadHandler.GetSquad)
	mux.HandleFunc("POST /api/v1/squads", squadHandler.CreateSquad)
	mux.HandleFunc("POST /api/v1/squads/{id}/activate", squadHandler.ActivateSquad)
	mux.HandleFunc("POST /api/v1/squads/{id}/pause", squadHandler.PauseSquad)
	mux.HandleFunc("POST /api/v1/squads/{id}/run", squadHandler.RunSquad)

	// Timeline / event routes.
	mux.HandleFunc("GET /api/v1/events", timelineHandler.ListEvents)
	mux.HandleFunc("GET /api/v1/events/{id}", timelineHandler.GetEvent)

	// v0.2 Step 3: inbox listing for the v2 frontend.
	mux.HandleFunc("GET /api/v1/inbox", inboxHandler.ListInbox)

	// v0.2 Step 6: home dashboard + rich health status.
	mux.HandleFunc("GET /api/v1/home", homeHandler.ListHome)
	mux.HandleFunc("GET /api/v1/health/status", healthStatusHandler.GetStatus)

	// v0.3 Step 3: provider listing for the settings UI.
	mux.HandleFunc("GET /api/v1/providers", providerHandler.ListProviders)

	// v0.3 Step 5: audit probe findings for the v2 Audit page.
	mux.HandleFunc("GET /api/v2/audit/findings", auditFindingsHandler.ListFindings)

	// v0.3 Step 4: commitments lifecycle endpoints.
	mux.HandleFunc("GET /api/v2/commitments", commitmentsHandler.ListCommitments)
	mux.HandleFunc("POST /api/v2/commitments/{id}/dismiss", commitmentsHandler.DismissCommitment)
	mux.HandleFunc("POST /api/v2/commitments/{id}/snooze", commitmentsHandler.SnoozeCommitment)
	mux.HandleFunc("DELETE /api/v2/commitments/{id}", commitmentsHandler.DeleteCommitment)

	// HMAC token authentication. The secret comes from IRONGOLEM_HMAC_SECRET
	// and is required at boot — fail-closed per Step 7 of the v0.1 plan
	// (Plans/create-a-plan-to-glowing-nest.md). Silent fallback to header
	// trust is exactly what Step 7 removed; we won't reintroduce it under a
	// "dev mode" loophole. Mint dev tokens locally via the helper in
	// services/gateway/internal/middleware/auth.go (MintToken).
	hmacSecret := []byte(os.Getenv("IRONGOLEM_HMAC_SECRET"))
	if len(hmacSecret) == 0 {
		logger.Error("IRONGOLEM_HMAC_SECRET is required; refusing to start without it")
		runtimeCancel()
		os.Exit(1)
	}

	// Build middleware chain (outermost first):
	// security headers -> rate limit -> request size -> CORS -> logging
	//   -> auth (HMAC) -> tenant -> policy -> handler.
	deployMode := middleware.DeploymentMode(envOrDefault("DEPLOYMENT_MODE", "solo"))
	// v0.2 Step 4: wire the SQLite-backed channel-policy store into Layer 4.
	// An empty store + unset IRONGOLEM_LAYER4_ENABLED keeps the v0.1 disabled
	// behavior; rules become enforcing as soon as the table has rows.
	channelPolicyStore := gwpolicy.NewSQLiteChannelPolicyStore(db, logger)
	policyEngine := policy.NewDefaultPolicyEngineWithStore(logger, channelPolicyStore)

	var finalHandler http.Handler = mux
	finalHandler = middleware.PolicyMiddleware(policyEngine, logger, eventStore)(finalHandler)
	finalHandler = middleware.TenantMiddleware(logger, deployMode)(finalHandler)
	finalHandler = middleware.HMACAuthMiddleware(middleware.AuthConfig{
		Secret:      hmacSecret,
		ExemptPaths: []string{"/healthz"},
	}, logger)(finalHandler)
	finalHandler = middleware.LoggingMiddleware(logger)(finalHandler)
	finalHandler = middleware.CORSMiddleware(middleware.DefaultCORSConfig())(finalHandler)
	finalHandler = middleware.RequestSizeMiddleware(1 << 20)(finalHandler) // 1 MB
	finalHandler = middleware.RateLimitMiddleware(middleware.DefaultRateLimitConfig())(finalHandler)
	finalHandler = middleware.SecurityHeadersMiddleware()(finalHandler)

	addr := envOrDefault("GATEWAY_ADDR", ":8080")
	srv := &http.Server{
		Addr:              addr,
		Handler:           finalHandler,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("gateway starting", slog.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("gateway shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", slog.String("error", err.Error()))
	}

	connMgr.DisconnectAll()

	// v0.3 Step 5 — stop the audit ticker. Bounded by the surrounding
	// shutdownCtx; the runtime's Run() returns promptly on cancel.
	auditCancel()
	// v0.3 Step 4 — stop the commitments ticker.
	commitmentsCancel()

	// Tell runtimed to drain in-flight work, then signal the supervisor
	// to stop trying to keep it alive. Bounded so a hung child can't
	// stall our shutdown.
	runtimeCloseCtx, runtimeCloseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := runtimeClient.Close(runtimeCloseCtx); err != nil {
		logger.Warn("runtime shutdown error", slog.String("error", err.Error()))
	}
	runtimeCloseCancel()
	runtimeCancel()

	if err := db.Close(); err != nil {
		logger.Warn("gateway db close error", slog.String("error", err.Error()))
	}

	logger.Info("gateway stopped")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
