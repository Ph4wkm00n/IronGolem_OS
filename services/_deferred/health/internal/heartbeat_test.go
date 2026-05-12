package internal

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/services/pkg/events"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestHealthyOnReceipt(t *testing.T) {
	mgr := NewHeartbeatManager(testLogger(), HeartbeatConfig{
		Timeout:       5 * time.Second,
		CheckInterval: 1 * time.Second,
	})

	ctx := context.Background()
	mgr.RecordHeartbeat(ctx, events.HeartbeatPayload{
		ServiceName: "test-service",
		Status:      events.HeartbeatHealthy,
		Message:     "all good",
	})

	records := mgr.ListAll(ctx)
	if len(records) != 1 {
		t.Fatalf("expected 1 service record, got %d", len(records))
	}

	rec := records[0]
	if rec.ServiceName != "test-service" {
		t.Errorf("expected service name %q, got %q", "test-service", rec.ServiceName)
	}
	if rec.State != StateHealthy {
		t.Errorf("expected state %q, got %q", StateHealthy, rec.State)
	}
	if rec.LastStatus != string(events.HeartbeatHealthy) {
		t.Errorf("expected last_status %q, got %q", events.HeartbeatHealthy, rec.LastStatus)
	}
}

func TestNeedsAttentionOnMissed(t *testing.T) {
	// Use very short timeouts so the test runs fast.
	mgr := NewHeartbeatManager(testLogger(), HeartbeatConfig{
		Timeout:             50 * time.Millisecond,
		CheckInterval:       10 * time.Millisecond,
		AttentionThreshold:  2,
		QuarantineThreshold: 100, // high enough to not trigger during test
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register the service with a single heartbeat.
	mgr.RecordHeartbeat(ctx, events.HeartbeatPayload{
		ServiceName: "flaky-service",
		Status:      events.HeartbeatHealthy,
	})

	// Start the monitor loop in the background.
	go mgr.Run(ctx)
	defer mgr.Stop()

	// Wait long enough for multiple missed heartbeats to trigger NeedsAttention.
	// Timeout=50ms, AttentionThreshold=2, so we need at least ~150ms of silence.
	time.Sleep(200 * time.Millisecond)

	records := mgr.ListAll(ctx)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.State != StateNeedsAttention {
		t.Errorf("expected state %q after missed heartbeats, got %q", StateNeedsAttention, rec.State)
	}
	if rec.MissedBeats < 2 {
		t.Errorf("expected at least 2 missed beats, got %d", rec.MissedBeats)
	}
}

func TestRecoveryAfterMissed(t *testing.T) {
	mgr := NewHeartbeatManager(testLogger(), HeartbeatConfig{
		Timeout:             50 * time.Millisecond,
		CheckInterval:       10 * time.Millisecond,
		AttentionThreshold:  2,
		QuarantineThreshold: 100, // high enough to not trigger during test
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register
	mgr.RecordHeartbeat(ctx, events.HeartbeatPayload{
		ServiceName: "recovering-service",
		Status:      events.HeartbeatHealthy,
	})

	// Run monitor
	go mgr.Run(ctx)
	defer mgr.Stop()

	// Wait for degradation to NeedsAttention (not quarantine).
	time.Sleep(200 * time.Millisecond)

	records := mgr.ListAll(ctx)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].State != StateNeedsAttention {
		t.Fatalf("expected NeedsAttention before recovery, got %q", records[0].State)
	}

	// Now send a heartbeat to trigger recovery.
	mgr.RecordHeartbeat(ctx, events.HeartbeatPayload{
		ServiceName: "recovering-service",
		Status:      events.HeartbeatHealthy,
		Message:     "back online",
	})

	records = mgr.ListAll(ctx)
	rec := records[0]

	// After a heartbeat arrives while in NeedsAttention, the service should
	// transition to QuietlyRecovering (not immediately to Healthy).
	if rec.State != StateQuietlyRecovering {
		t.Errorf("expected state %q or %q after recovery heartbeat, got %q",
			StateQuietlyRecovering, StateHealthy, rec.State)
	}
	if rec.MissedBeats != 0 {
		t.Errorf("expected missed_beats reset to 0, got %d", rec.MissedBeats)
	}
}
