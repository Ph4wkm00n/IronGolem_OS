package audit

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// EventEmitter is the subset of the gateway's event store this package
// needs. Implemented by `handler.EventStore` — the runtime publishes one
// `audit-finding` event per non-info finding so the timeline + Pulse
// can surface them alongside other gateway activity.
//
// `info` findings are stored but not emitted to the timeline — they're
// confirmation that the probe ran, not actionable.
type EventEmitter interface {
	EmitAuditFinding(ctx context.Context, f StoredFinding) error
}

// RuntimeConfig tunes the audit runtime. Zero-valued fields use defaults.
type RuntimeConfig struct {
	// Interval between ticks. Default 5min — fast enough that an
	// operator notices drift within one coffee break, slow enough that
	// probes don't dominate the gateway's CPU budget.
	Interval time.Duration
	// ProbeTimeout bounds a single probe invocation. Default 15s — a
	// probe that can't make up its mind in 15s is broken; the runtime
	// records the timeout as a critical finding and moves on.
	ProbeTimeout time.Duration
}

const (
	defaultInterval     = 5 * time.Minute
	defaultProbeTimeout = 15 * time.Second
)

// Runtime drives a Registry on a fixed cadence, persisting findings and
// emitting timeline events. The lifecycle is: New() to construct, Run(ctx)
// in a goroutine to start, ctx.Done() to stop.
type Runtime struct {
	registry *Registry
	store    FindingStore
	emitter  EventEmitter
	logger   *slog.Logger
	cfg      RuntimeConfig
}

// NewRuntime wires the dependencies. The emitter may be nil — useful
// in tests where no event store is needed.
func NewRuntime(
	registry *Registry,
	store FindingStore,
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
	if cfg.ProbeTimeout == 0 {
		cfg.ProbeTimeout = defaultProbeTimeout
	}
	return &Runtime{
		registry: registry,
		store:    store,
		emitter:  emitter,
		logger:   logger.With(slog.String("component", "audit_runtime")),
		cfg:      cfg,
	}
}

// Run blocks until ctx is cancelled. Runs an initial tick immediately
// (so the gateway reaches a known audit baseline at boot) then ticks on
// cfg.Interval.
func (r *Runtime) Run(ctx context.Context) {
	r.logger.Info("audit runtime starting",
		slog.Duration("interval", r.cfg.Interval),
		slog.Int("probes", len(r.registry.List())),
	)

	// Initial tick at boot so the audit baseline isn't 5 minutes stale.
	r.tick(ctx)

	t := time.NewTicker(r.cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			r.logger.Info("audit runtime stopping")
			return
		case <-t.C:
			r.tick(ctx)
		}
	}
}

// tick runs every probe once and persists each finding. Panics in a
// probe are recovered so one bad probe never wedges the runtime.
func (r *Runtime) tick(ctx context.Context) {
	for _, p := range r.registry.List() {
		r.runOne(ctx, p)
	}
}

func (r *Runtime) runOne(ctx context.Context, p Probe) {
	probeCtx, cancel := context.WithTimeout(ctx, r.cfg.ProbeTimeout)
	defer cancel()

	finding := safeRun(probeCtx, p, r.logger)
	finding.Timestamp = time.Now().UTC()

	id, err := r.store.Insert(ctx, finding)
	if err != nil {
		r.logger.Warn("audit finding insert failed",
			slog.String("probe_id", p.ID()),
			slog.String("error", err.Error()),
		)
		return
	}

	// Only emit to the timeline for non-info findings — info is "I ran
	// and confirmed fine" which would spam the timeline every 5 minutes.
	if r.emitter != nil && finding.Severity != SeverityInfo {
		stored := StoredFinding{
			ID:       id,
			Finding:  finding,
			StoredAt: time.Now().UTC(),
		}
		if err := r.emitter.EmitAuditFinding(ctx, stored); err != nil {
			r.logger.Warn("audit finding event emit failed",
				slog.String("probe_id", p.ID()),
				slog.String("error", err.Error()),
			)
		}
	}
}

// safeRun wraps Probe.Run with panic recovery + timeout detection so a
// broken probe surfaces as a critical Finding rather than crashing the
// runtime goroutine.
func safeRun(ctx context.Context, p Probe, logger *slog.Logger) (out Finding) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.Error("audit probe panic",
				slog.String("probe_id", p.ID()),
				slog.Any("panic", rec),
			)
			out = Finding{
				ProbeID:  p.ID(),
				Severity: SeverityCritical,
				Reason:   "probe panicked during execution; see gateway logs",
				Evidence: map[string]any{"panic": stringifyPanic(rec)},
			}
		}
	}()

	out = p.Run(ctx)

	// Defensive: if a probe returns an unsupported severity, normalize
	// to critical with an explanation so we don't silently swallow it.
	if !out.Severity.Valid() {
		logger.Warn("probe returned invalid severity",
			slog.String("probe_id", p.ID()),
			slog.String("severity", string(out.Severity)),
		)
		out = Finding{
			ProbeID:  p.ID(),
			Severity: SeverityCritical,
			Reason:   "probe returned invalid severity value",
			Evidence: map[string]any{"returned_severity": string(out.Severity)},
		}
	}
	if out.ProbeID == "" {
		out.ProbeID = p.ID()
	}

	// Surface context-deadline timeouts explicitly. A probe that
	// returned because ctx expired (and was well-behaved enough to
	// return SeverityInfo) deserves a stronger signal.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		out.Severity = SeverityCritical
		out.Reason = "probe exceeded timeout"
		if out.Evidence == nil {
			out.Evidence = map[string]any{}
		}
		out.Evidence["timeout"] = true
	}
	return out
}

func stringifyPanic(rec any) string {
	switch v := rec.(type) {
	case string:
		return v
	case error:
		return v.Error()
	default:
		return "non-string panic value"
	}
}
