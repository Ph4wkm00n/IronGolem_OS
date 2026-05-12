package policy

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestAllLayersPass(t *testing.T) {
	engine := NewDefaultPolicyEngine(testLogger())

	req := EvalRequest{
		TenantID:    "tenant-001",
		WorkspaceID: "ws-001",
		UserID:      "user-001",
		AgentRole:   "executor",
		ChannelID:   "channel-email",
		Permission: Permission{
			Resource: "connector.email",
			Action:   "read",
		},
	}

	result, err := engine.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Decision != DecisionAllow {
		t.Errorf("expected decision %q, got %q (layer: %s, reason: %s)",
			DecisionAllow, result.Decision, result.Layer, result.Reason)
	}
	if result.Reason != "all layers passed" {
		t.Errorf("expected reason %q, got %q", "all layers passed", result.Reason)
	}
}

func TestDenyMissingIdentity(t *testing.T) {
	engine := NewDefaultPolicyEngine(testLogger())

	tests := []struct {
		name string
		req  EvalRequest
	}{
		{
			name: "missing_tenant_id",
			req: EvalRequest{
				TenantID: "",
				UserID:   "user-001",
				Permission: Permission{
					Resource: "connector.email",
					Action:   "read",
				},
			},
		},
		{
			name: "missing_user_and_agent",
			req: EvalRequest{
				TenantID:  "tenant-001",
				UserID:    "",
				AgentRole: "",
				Permission: Permission{
					Resource: "connector.email",
					Action:   "read",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := engine.Evaluate(context.Background(), tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Decision != DecisionDeny {
				t.Errorf("expected decision %q, got %q", DecisionDeny, result.Decision)
			}
			if result.Layer != LayerGatewayIdentity {
				t.Errorf("expected denial at layer %s, got %s", LayerGatewayIdentity, result.Layer)
			}
		})
	}
}

func TestDenyBlockedTool(t *testing.T) {
	engine := NewDefaultPolicyEngine(testLogger())

	blockedResources := []struct {
		name     string
		resource string
	}{
		{"shell_exec", "tool.shell_exec"},
		{"raw_sql", "tool.raw_sql"},
		{"network_scan", "tool.network_scan"},
	}

	for _, tc := range blockedResources {
		t.Run(tc.name, func(t *testing.T) {
			req := EvalRequest{
				TenantID: "tenant-001",
				UserID:   "user-001",
				Permission: Permission{
					Resource: tc.resource,
					Action:   "execute",
				},
			}

			result, err := engine.Evaluate(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Decision != DecisionDeny {
				t.Errorf("expected decision %q for blocked tool %s, got %q",
					DecisionDeny, tc.resource, result.Decision)
			}
			if result.Layer != LayerGlobalToolPolicy {
				t.Errorf("expected denial at layer %s, got %s",
					LayerGlobalToolPolicy, result.Layer)
			}
		})
	}
}

func TestDenyUnknownAgentRole(t *testing.T) {
	engine := NewDefaultPolicyEngine(testLogger())

	req := EvalRequest{
		TenantID:  "tenant-001",
		AgentRole: "superadmin_hacker",
		Permission: Permission{
			Resource: "connector.email",
			Action:   "execute",
		},
	}

	result, err := engine.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Decision != DecisionDeny {
		t.Errorf("expected decision %q for unknown agent role, got %q",
			DecisionDeny, result.Decision)
	}
	if result.Layer != LayerPerAgentPermission {
		t.Errorf("expected denial at layer %s, got %s",
			LayerPerAgentPermission, result.Layer)
	}
}

func TestEmergencyStop(t *testing.T) {
	engine := NewDefaultPolicyEngine(testLogger())

	req := EvalRequest{
		TenantID:    "tenant-001",
		WorkspaceID: "ws-001",
		UserID:      "user-001",
		AgentRole:   "executor",
		ChannelID:   "channel-email",
		Permission: Permission{
			Resource: "connector.email",
			Action:   "read",
		},
		Metadata: map[string]string{
			"emergency_stop": "true",
		},
	}

	result, err := engine.Evaluate(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Decision != DecisionDeny {
		t.Errorf("expected decision %q during emergency stop, got %q",
			DecisionDeny, result.Decision)
	}
	if result.Layer != LayerAdminControls {
		t.Errorf("expected denial at layer %s, got %s",
			LayerAdminControls, result.Layer)
	}
	if result.Reason != "system is in emergency stop mode" {
		t.Errorf("expected emergency stop reason, got %q", result.Reason)
	}
}

// TestLayer4DisabledByDefault proves Step 7's fix: layer-4 never silently
// allows. With the env unset it explicitly returns the "disabled" reason
// so the operator sees the gap; with the env set to "true" it returns a
// deny rather than a stub-allow.
func TestLayer4DisabledByDefault(t *testing.T) {
	t.Setenv("IRONGOLEM_LAYER4_ENABLED", "")
	checker := &perChannelRestrictionChecker{}

	// With a channel context.
	dec, reason, err := checker.Check(context.Background(), EvalRequest{
		ChannelID: "channel-email",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec != DecisionAllow {
		t.Fatalf("expected allow with disabled reason, got %q (%s)", dec, reason)
	}
	if reason != "layer4 disabled in v0.1" {
		t.Fatalf("expected disabled reason, got %q", reason)
	}

	// Without a channel context the explanation should reflect that.
	_, reason, _ = checker.Check(context.Background(), EvalRequest{})
	if reason != "layer4 disabled in v0.1 (no channel context)" {
		t.Fatalf("expected no-channel disabled reason, got %q", reason)
	}
}

func TestLayer4EnabledDeniesUntilImplemented(t *testing.T) {
	t.Setenv("IRONGOLEM_LAYER4_ENABLED", "true")
	checker := &perChannelRestrictionChecker{}

	dec, reason, err := checker.Check(context.Background(), EvalRequest{
		ChannelID: "channel-email",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dec != DecisionDeny {
		t.Fatalf("expected deny when layer4 enabled, got %q", dec)
	}
	if reason != "layer4 store not implemented" {
		t.Fatalf("expected not-implemented reason, got %q", reason)
	}
}
