package commitments

import (
	"context"
	"log/slog"
	"time"
)

// Dispatcher delivers a fired commitment via the gateway's connector
// manager. Carved out as an interface so the runtime + tests can mock
// the connector layer without depending on its concrete type.
type Dispatcher interface {
	Dispatch(ctx context.Context, c Commitment) error
}

// EventEmitter publishes commitment lifecycle events into the timeline.
// Implemented by the handler-level adapter (parallel to
// `handler.AuditFindingEmitter`). Nil-tolerant — the runtime checks
// before invoking.
type EventEmitter interface {
	EmitCommitmentExtracted(ctx context.Context, c Commitment) error
	EmitCommitmentFired(ctx context.Context, c Commitment) error
	EmitCommitmentDismissed(ctx context.Context, c Commitment) error
	EmitCommitmentSnoozed(ctx context.Context, c Commitment) error
	EmitCommitmentExpired(ctx context.Context, c Commitment) error
}

// RuntimeConfig tunes the commitments runtime. Zero-valued fields use
// defaults that match the plan's "M-tier 60s tick" sizing.
type RuntimeConfig struct {
	// Interval between firing-window scans. Default 60s — fast enough
	// that a "ping me in 10 minutes" commitment lands within 0-60s of
	// its target window, slow enough that the gateway's DB load stays
	// trivial.
	Interval time.Duration
	// ExpireGrace is how long after `latest_ms` to wait before marking
	// a missed commitment expired. Default 10min — covers a single
	// missed tick + a buffer for slow dispatchers.
	ExpireGrace time.Duration
	// DispatchTimeout bounds a single Dispatcher.Dispatch call. Default
	// 15s — Telegram + Slack ack times are tens of ms; 15s is a generous
	// outer bound that still keeps a hung delivery from wedging the
	// tick.
	DispatchTimeout time.Duration
}

const (
	defaultInterval        = 60 * time.Second
	defaultExpireGrace     = 10 * time.Minute
	defaultDispatchTimeout = 15 * time.Second
	dueNowBatch            = 50
)

// Runtime drives the firing + expiry side of commitments. Construction
// is wire-once: New() to assemble, Run(ctx) in a goroutine to start,
// ctx.Done() to stop.
type Runtime struct {
	store      Store
	dispatcher Dispatcher
	emitter    EventEmitter
	logger     *slog.Logger
	cfg        RuntimeConfig
}

// NewRuntime constructs a Runtime. dispatcher and emitter may be nil
// for tests; the runtime degrades gracefully (no delivery, no events).
func NewRuntime(
	store Store,
	dispatcher Dispatcher,
	emitter EventEmitter,
	logger *slog.Logger,
	cfg RuntimeConfig,
) *Runtime {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Interval == 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.ExpireGrace == 0 {
		cfg.ExpireGrace = defaultExpireGrace
	}
	if cfg.DispatchTimeout == 0 {
		cfg.DispatchTimeout = defaultDispatchTimeout
	}
	return &Runtime{
		store:      store,
		dispatcher: dispatcher,
		emitter:    emitter,
		logger:     logger.With(slog.String("component", "commitments_runtime")),
		cfg:        cfg,
	}
}

// Run blocks until ctx is cancelled. Runs an initial tick immediately
// so a freshly-started gateway picks up due commitments without waiting.
func (r *Runtime) Run(ctx context.Context) {
	r.logger.Info("commitments runtime starting",
		slog.Duration("interval", r.cfg.Interval),
	)
	r.tick(ctx)
	t := time.NewTicker(r.cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("commitments runtime stopping")
			return
		case <-t.C:
			r.tick(ctx)
		}
	}
}

// tick is the heart of the runtime: fire what's due, expire what's stale.
func (r *Runtime) tick(ctx context.Context) {
	r.fireDue(ctx)
	r.expireStale(ctx)
}

func (r *Runtime) fireDue(ctx context.Context) {
	nowMs := time.Now().UTC().UnixMilli()
	due, err := r.store.DueNow(ctx, nowMs, dueNowBatch)
	if err != nil {
		r.logger.Warn("commitments DueNow failed", slog.String("error", err.Error()))
		return
	}
	for _, c := range due {
		r.fireOne(ctx, c)
	}
}

func (r *Runtime) fireOne(ctx context.Context, c Commitment) {
	dispatchCtx, cancel := context.WithTimeout(ctx, r.cfg.DispatchTimeout)
	defer cancel()
	if r.dispatcher != nil {
		if err := r.dispatcher.Dispatch(dispatchCtx, c); err != nil {
			r.logger.Warn("commitment dispatch failed",
				slog.String("commitment_id", c.ID),
				slog.String("error", err.Error()),
			)
			// Leave status=pending; next tick retries. attempts is
			// incremented only by MarkSent on success — fail-and-retry
			// stays opaque in the row.
			return
		}
	}
	sentAtMs := time.Now().UTC().UnixMilli()
	if err := r.store.MarkSent(ctx, c.ID, sentAtMs); err != nil {
		r.logger.Warn("MarkSent failed", slog.String("commitment_id", c.ID), slog.String("error", err.Error()))
		return
	}
	c.Status = StatusSent
	c.SentAtMs = sentAtMs
	if r.emitter != nil {
		if err := r.emitter.EmitCommitmentFired(ctx, c); err != nil {
			r.logger.Warn("EmitCommitmentFired failed", slog.String("commitment_id", c.ID), slog.String("error", err.Error()))
		}
	}
}

func (r *Runtime) expireStale(ctx context.Context) {
	graceCutoff := time.Now().UTC().UnixMilli() - r.cfg.ExpireGrace.Milliseconds()
	// Reuse the List(pending) path; the SQL surface stays small.
	pending, err := r.store.List(ctx, ListFilter{Status: StatusPending, Limit: 200})
	if err != nil {
		r.logger.Warn("commitments List failed", slog.String("error", err.Error()))
		return
	}
	for _, c := range pending {
		if c.DueWindow.LatestMs == 0 || c.DueWindow.LatestMs >= graceCutoff {
			continue
		}
		if err := r.store.MarkExpired(ctx, c.ID); err != nil {
			r.logger.Warn("MarkExpired failed", slog.String("commitment_id", c.ID), slog.String("error", err.Error()))
			continue
		}
		c.Status = StatusExpired
		if r.emitter != nil {
			if err := r.emitter.EmitCommitmentExpired(ctx, c); err != nil {
				r.logger.Warn("EmitCommitmentExpired failed", slog.String("commitment_id", c.ID), slog.String("error", err.Error()))
			}
		}
	}
}

// Enqueue runs the extractor against a Turn, threshold-filters the
// candidates, and inserts the survivors. The handler calls this
// asynchronously (own goroutine) from the inbound flow so the user
// reply never blocks on extractor / DB latency.
func (r *Runtime) Enqueue(ctx context.Context, extractor Extractor, in Turn) {
	candidates, err := extractor.Extract(ctx, in)
	if err != nil {
		r.logger.Warn("extractor failed", slog.String("error", err.Error()))
		return
	}
	for _, cand := range candidates {
		if cand.Confidence < MinConfidence {
			continue
		}
		now := time.Now().UTC().UnixMilli()
		c := Commitment{
			WorkspaceID:   in.WorkspaceID,
			TenantID:      in.TenantID,
			Kind:          cand.Kind,
			Sensitivity:   cand.Sensitivity,
			Status:        StatusPending,
			Reason:        cand.Reason,
			SuggestedText: cand.SuggestedText,
			DedupeKey:     cand.DedupeKey,
			Confidence:    cand.Confidence,
			DueWindow:     cand.DueWindow,
			ConnectorID:   in.ConnectorID,
			ChannelID:     in.ChannelID,
			UserID:        in.UserID,
			SourceEventID: in.SourceEventID,
			CreatedAtMs:   now,
			UpdatedAtMs:   now,
		}
		id, err := r.store.Insert(ctx, c)
		if err != nil {
			r.logger.Warn("commitments Insert failed",
				slog.String("error", err.Error()),
				slog.String("dedupe_key", cand.DedupeKey),
			)
			continue
		}
		c.ID = id
		if r.emitter != nil {
			if err := r.emitter.EmitCommitmentExtracted(ctx, c); err != nil {
				r.logger.Warn("EmitCommitmentExtracted failed", slog.String("error", err.Error()))
			}
		}
	}
}
