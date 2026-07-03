// Long-lived connector worker. v0.4 adoption wave.
//
// Slack (Socket Mode WebSocket), Discord (Gateway WebSocket), and Signal
// (signal-cli subprocess) all need the same shape: a goroutine that runs
// one "session" at a time (a WS connection, a subprocess), feeds the
// connector's Receive channel, and — when the session dies — reconnects
// with capped exponential backoff. This file is that shared mechanism so
// each connector only writes its session body.
//
// Contract for the iterate function passed to RunWorker / StartWorker:
//
//   - It should BLOCK for the lifetime of one session (connection,
//     subprocess) and return an error when the session ends abnormally.
//   - Returning nil resets the backoff schedule and re-runs immediately,
//     so a nil return from a non-blocking body would hot-loop.
//   - Panics inside iterate are recovered, logged, and treated as errors
//     (backoff applies). A malformed inbound event must never take the
//     worker down.
//   - It must honor ctx cancellation promptly; RunWorker stops looping as
//     soon as ctx is done.
package connectors

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"time"
)

// Backoff defaults. MaxBackoff caps the exponential schedule so an
// outage never pushes retry intervals beyond ~a minute.
const (
	DefaultInitialBackoff = 1 * time.Second
	DefaultMaxBackoff     = 60 * time.Second

	// backoffJitterFraction spreads each delay uniformly across
	// [d*(1-f), d*(1+f)] so a fleet of connectors doesn't reconnect in
	// lockstep after a shared outage.
	backoffJitterFraction = 0.25
)

// WorkerConfig configures one long-lived connector worker.
type WorkerConfig struct {
	// Name tags log lines, e.g. "slack-socket-mode".
	Name string
	// Logger for worker lifecycle events. Defaults to slog.Default().
	Logger *slog.Logger
	// InitialBackoff is the first retry delay. Defaults to
	// DefaultInitialBackoff.
	InitialBackoff time.Duration
	// MaxBackoff caps the exponential schedule. Defaults to
	// DefaultMaxBackoff.
	MaxBackoff time.Duration
}

// Backoff computes capped exponential retry delays with jitter.
type Backoff struct {
	initial time.Duration
	max     time.Duration
	current time.Duration

	// jitter returns a uniform value in [0,1). Overridable in tests for
	// deterministic assertions; nil means math/rand/v2.
	jitter func() float64
}

// NewBackoff returns a Backoff starting at initial and doubling up to max.
// Non-positive arguments fall back to the package defaults.
func NewBackoff(initial, max time.Duration) *Backoff {
	if initial <= 0 {
		initial = DefaultInitialBackoff
	}
	if max <= 0 {
		max = DefaultMaxBackoff
	}
	if max < initial {
		max = initial
	}
	return &Backoff{initial: initial, max: max, current: initial}
}

// Next returns the delay to sleep before the next attempt and advances
// the schedule. The returned value is jittered; the internal schedule
// (initial, 2x, 4x, ... capped at max) is not, so the cap is stable.
func (b *Backoff) Next() time.Duration {
	base := b.current
	next := b.current * 2
	if next > b.max {
		next = b.max
	}
	b.current = next

	roll := rand.Float64()
	if b.jitter != nil {
		roll = b.jitter()
	}
	// factor in [1-f, 1+f)
	factor := 1 - backoffJitterFraction + 2*backoffJitterFraction*roll
	return time.Duration(float64(base) * factor)
}

// Reset returns the schedule to its initial delay. Called after a
// session that ended cleanly so a healthy reconnect isn't penalized by
// earlier failures.
func (b *Backoff) Reset() {
	b.current = b.initial
}

// RunWorker runs iterate in a loop until ctx is cancelled. Each
// iteration is panic-recovered; an error (or recovered panic) sleeps
// per the backoff schedule before retrying, a nil return resets the
// schedule and retries immediately. Blocks until ctx is done.
func RunWorker(ctx context.Context, cfg WorkerConfig, iterate func(context.Context) error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	backoff := NewBackoff(cfg.InitialBackoff, cfg.MaxBackoff)

	for {
		if ctx.Err() != nil {
			return
		}

		err := runIteration(ctx, iterate)

		if ctx.Err() != nil {
			logger.Info("connector worker stopped", slog.String("worker", cfg.Name))
			return
		}
		if err == nil {
			backoff.Reset()
			continue
		}

		delay := backoff.Next()
		logger.Warn("connector worker session ended; backing off",
			slog.String("worker", cfg.Name),
			slog.String("error", err.Error()),
			slog.Duration("retry_in", delay),
		)
		select {
		case <-ctx.Done():
			logger.Info("connector worker stopped", slog.String("worker", cfg.Name))
			return
		case <-time.After(delay):
		}
	}
}

// runIteration invokes iterate with panic recovery so one poisoned
// event can't kill the worker loop.
func runIteration(ctx context.Context, iterate func(context.Context) error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("connector worker panic recovered: %v", r)
		}
	}()
	return iterate(ctx)
}

// StartWorker launches RunWorker on a goroutine whose context is
// cancelled when either the parent ctx is done or the connector's done
// channel closes (the Disconnect signal). onExit runs after the worker
// fully stops — connectors use it to close their Receive channel so the
// gateway pump observes shutdown. Returns immediately.
//
// This is the lifecycle glue that lets Slack/Discord/Signal start their
// inbound worker inside Receive (exactly where telegram starts its
// polling loop) without any change to the gateway pump.
func StartWorker(ctx context.Context, done <-chan struct{}, cfg WorkerConfig, iterate func(context.Context) error, onExit func()) {
	wctx, cancel := context.WithCancel(ctx)

	// Bridge the connector's done channel into context cancellation.
	go func() {
		select {
		case <-done:
			cancel()
		case <-wctx.Done():
		}
	}()

	go func() {
		defer cancel()
		if onExit != nil {
			defer onExit()
		}
		RunWorker(wctx, cfg, iterate)
	}()
}
