package internal

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func canaryLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestCanarySuccess(t *testing.T) {
	mgr := NewCanaryManager(canaryLogger())

	check := CanaryCheck{
		ID:               "canary-conn-1",
		ConnectorID:      "email-connector",
		Type:             CanaryConnectivity,
		Schedule:         "30s",
		Interval:         30 * time.Second,
		FailureThreshold: 3,
	}

	probe := &ConnectivityCanary{
		Checker: func(_ context.Context, _ string) error {
			return nil // healthy
		},
	}

	if err := mgr.Register(check, probe); err != nil {
		t.Fatalf("failed to register canary: %v", err)
	}

	result, err := mgr.RunCheck(context.Background(), "canary-conn-1")
	if err != nil {
		t.Fatalf("RunCheck returned error: %v", err)
	}

	if !result.Passed {
		t.Error("expected canary to pass for healthy connector")
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}

	// Verify consecutive failures is zero.
	checks := mgr.List()
	if len(checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(checks))
	}
	if checks[0].ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures, got %d", checks[0].ConsecutiveFailures)
	}
}

func TestCanaryFailureAlert(t *testing.T) {
	mgr := NewCanaryManager(canaryLogger())

	var alertCount int32
	mgr.AlertCallback = func(check CanaryCheck) {
		atomic.AddInt32(&alertCount, 1)
	}

	check := CanaryCheck{
		ID:               "canary-fail-1",
		ConnectorID:      "slack-connector",
		Type:             CanaryConnectivity,
		Schedule:         "10s",
		Interval:         10 * time.Second,
		FailureThreshold: 3,
	}

	probe := &ConnectivityCanary{
		Checker: func(_ context.Context, _ string) error {
			return fmt.Errorf("connection refused")
		},
	}

	if err := mgr.Register(check, probe); err != nil {
		t.Fatalf("failed to register canary: %v", err)
	}

	ctx := context.Background()

	// Run the check 3 times to reach the threshold.
	for i := 0; i < 3; i++ {
		result, err := mgr.RunCheck(ctx, "canary-fail-1")
		if err != nil {
			t.Fatalf("RunCheck returned error: %v", err)
		}
		if result.Passed {
			t.Error("expected canary to fail")
		}
	}

	// Give the async alert callback a moment to fire.
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&alertCount) == 0 {
		t.Error("expected alert callback to be invoked after reaching failure threshold")
	}

	checks := mgr.List()
	for _, c := range checks {
		if c.ID == "canary-fail-1" && c.ConsecutiveFailures < 3 {
			t.Errorf("expected at least 3 consecutive failures, got %d", c.ConsecutiveFailures)
		}
	}
}

func TestLatencyCanary(t *testing.T) {
	mgr := NewCanaryManager(canaryLogger())

	var alertFired int32
	mgr.AlertCallback = func(_ CanaryCheck) {
		atomic.AddInt32(&alertFired, 1)
	}

	check := CanaryCheck{
		ID:               "canary-latency-1",
		ConnectorID:      "calendar-connector",
		Type:             CanaryLatency,
		Schedule:         "30s",
		Interval:         30 * time.Second,
		FailureThreshold: 2,
	}

	probe := &LatencyCanary{
		MaxLatency: 100 * time.Millisecond,
		Pinger: func(_ context.Context, _ string) (time.Duration, error) {
			return 500 * time.Millisecond, nil // exceeds SLA
		},
	}

	if err := mgr.Register(check, probe); err != nil {
		t.Fatalf("failed to register canary: %v", err)
	}

	ctx := context.Background()

	// First run: fails but no alert yet (threshold is 2).
	result, err := mgr.RunCheck(ctx, "canary-latency-1")
	if err != nil {
		t.Fatalf("RunCheck returned error: %v", err)
	}
	if result.Passed {
		t.Error("expected latency canary to fail when latency exceeds SLA")
	}
	if result.Error == "" {
		t.Error("expected error message describing latency violation")
	}

	// Second run: should trigger alert.
	result, err = mgr.RunCheck(ctx, "canary-latency-1")
	if err != nil {
		t.Fatalf("RunCheck returned error: %v", err)
	}
	if result.Passed {
		t.Error("expected latency canary to fail")
	}

	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&alertFired) == 0 {
		t.Error("expected alert to fire after 2 consecutive latency failures")
	}
}

func TestAuthCanary(t *testing.T) {
	mgr := NewCanaryManager(canaryLogger())

	check := CanaryCheck{
		ID:               "canary-auth-1",
		ConnectorID:      "telegram-connector",
		Type:             CanaryAuth,
		Schedule:         "5m",
		Interval:         5 * time.Minute,
		FailureThreshold: 1,
	}

	probe := &AuthCanary{
		Verifier: func(_ context.Context, _ string) error {
			return fmt.Errorf("token expired")
		},
	}

	if err := mgr.Register(check, probe); err != nil {
		t.Fatalf("failed to register canary: %v", err)
	}

	result, err := mgr.RunCheck(context.Background(), "canary-auth-1")
	if err != nil {
		t.Fatalf("RunCheck returned error: %v", err)
	}
	if result.Passed {
		t.Error("expected auth canary to fail for expired token")
	}
	if result.Error == "" {
		t.Error("expected error message about auth failure")
	}
}

func TestDataIntegrityCanary(t *testing.T) {
	mgr := NewCanaryManager(canaryLogger())

	check := CanaryCheck{
		ID:               "canary-integrity-1",
		ConnectorID:      "webhook-connector",
		Type:             CanaryDataIntegrity,
		Schedule:         "5m",
		Interval:         5 * time.Minute,
		FailureThreshold: 2,
	}

	probe := &DataIntegrityCanary{
		RoundTripper: func(_ context.Context, _ string, _ string) error {
			return nil // round-trip passes
		},
	}

	if err := mgr.Register(check, probe); err != nil {
		t.Fatalf("failed to register canary: %v", err)
	}

	result, err := mgr.RunCheck(context.Background(), "canary-integrity-1")
	if err != nil {
		t.Fatalf("RunCheck returned error: %v", err)
	}
	if !result.Passed {
		t.Error("expected data integrity canary to pass")
	}
}
