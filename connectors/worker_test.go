package connectors

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestBackoff_ProgressionCapped(t *testing.T) {
	tests := []struct {
		name    string
		initial time.Duration
		max     time.Duration
		steps   []time.Duration // expected un-jittered base per Next() call
	}{
		{
			name:    "doubles then caps",
			initial: 1 * time.Second,
			max:     60 * time.Second,
			steps: []time.Duration{
				1 * time.Second, 2 * time.Second, 4 * time.Second,
				8 * time.Second, 16 * time.Second, 32 * time.Second,
				60 * time.Second, 60 * time.Second, // capped
			},
		},
		{
			name:    "cap below first double",
			initial: 40 * time.Second,
			max:     60 * time.Second,
			steps:   []time.Duration{40 * time.Second, 60 * time.Second, 60 * time.Second},
		},
		{
			name:    "defaults on non-positive input",
			initial: 0,
			max:     0,
			steps:   []time.Duration{DefaultInitialBackoff, 2 * DefaultInitialBackoff},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBackoff(tt.initial, tt.max)
			b.jitter = func() float64 { return 0.5 } // midpoint → factor exactly 1.0
			for i, want := range tt.steps {
				if got := b.Next(); got != want {
					t.Fatalf("Next() call %d = %v, want %v", i+1, got, want)
				}
			}
		})
	}
}

func TestBackoff_JitterBounds(t *testing.T) {
	b := NewBackoff(10*time.Second, 60*time.Second)
	lo := time.Duration(float64(10*time.Second) * (1 - backoffJitterFraction))
	hi := time.Duration(float64(10*time.Second) * (1 + backoffJitterFraction))
	for i := 0; i < 200; i++ {
		b.Reset()
		if got := b.Next(); got < lo || got > hi {
			t.Fatalf("jittered delay %v outside [%v, %v]", got, lo, hi)
		}
	}
}

func TestBackoff_ResetReturnsToInitial(t *testing.T) {
	b := NewBackoff(1*time.Second, 60*time.Second)
	b.jitter = func() float64 { return 0.5 }
	b.Next()
	b.Next()
	b.Reset()
	if got := b.Next(); got != 1*time.Second {
		t.Fatalf("Next() after Reset = %v, want 1s", got)
	}
}

func TestRunWorker_CancellationStopsLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var iterations atomic.Int64

	doneCh := make(chan struct{})
	go func() {
		RunWorker(ctx, WorkerConfig{Name: "test", InitialBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond},
			func(ctx context.Context) error {
				iterations.Add(1)
				<-ctx.Done()
				return ctx.Err()
			})
		close(doneCh)
	}()

	// Give the worker a moment to enter its first iteration, then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("RunWorker did not return after context cancellation")
	}
	if n := iterations.Load(); n != 1 {
		t.Fatalf("iterations = %d, want 1 (blocked session, cancelled once)", n)
	}
}

func TestRunWorker_PanicRecoveredAndRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int64
	recovered := make(chan struct{})

	doneCh := make(chan struct{})
	go func() {
		RunWorker(ctx, WorkerConfig{Name: "test", InitialBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond},
			func(ctx context.Context) error {
				n := calls.Add(1)
				if n == 1 {
					panic("poisoned event")
				}
				// Second iteration proves the panic was recovered and
				// the loop retried.
				close(recovered)
				<-ctx.Done()
				return ctx.Err()
			})
		close(doneCh)
	}()

	select {
	case <-recovered:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not retry after panic")
	}
	cancel()
	select {
	case <-doneCh:
	case <-time.After(2 * time.Second):
		t.Fatal("RunWorker did not return after cancellation")
	}
}

func TestRunWorker_ErrorBacksOffThenRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls atomic.Int64
	second := make(chan struct{})

	go RunWorker(ctx, WorkerConfig{Name: "test", InitialBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond},
		func(ctx context.Context) error {
			if calls.Add(1) == 2 {
				close(second)
				<-ctx.Done()
			}
			return errors.New("session died")
		})

	select {
	case <-second:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not retry after error")
	}
}

func TestStartWorker_DoneChannelStopsWorkerAndRunsOnExit(t *testing.T) {
	done := make(chan struct{})
	exited := make(chan struct{})

	StartWorker(context.Background(), done,
		WorkerConfig{Name: "test", InitialBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond},
		func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
		func() { close(exited) },
	)

	close(done) // simulate Disconnect
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("onExit did not run after done channel closed")
	}
}

func TestStartWorker_ParentContextStopsWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	exited := make(chan struct{})

	StartWorker(ctx, make(chan struct{}),
		WorkerConfig{Name: "test", InitialBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond},
		func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
		func() { close(exited) },
	)

	cancel()
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("onExit did not run after parent context cancellation")
	}
}
