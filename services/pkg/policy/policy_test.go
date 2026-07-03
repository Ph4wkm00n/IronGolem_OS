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
	if reason != "layer4 disabled in v0.2" {
		t.Fatalf("expected disabled reason, got %q", reason)
	}

	// Without a channel context the explanation should reflect that.
	_, reason, _ = checker.Check(context.Background(), EvalRequest{})
	if reason != "layer4 disabled in v0.2 (no channel context)" {
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
	if reason != "layer4 store not provisioned" {
		t.Fatalf("expected not-implemented reason, got %q", reason)
	}
}

// fakeChannelStore is an in-memory ChannelPolicyStore for the pkg/policy
// test surface. Lives here rather than in a separate file because it's
// only used to exercise the engine's wiring; the SQLite-backed impl gets
// its own integration tests in services/gateway/internal/policy.
type fakeChannelStore struct {
	rules     map[string]ChannelRule // keyed "channel|action"
	hasRules  bool
	lookupErr error
	hasErr    error
}

func newFakeChannelStore() *fakeChannelStore {
	return &fakeChannelStore{rules: map[string]ChannelRule{}}
}

func (f *fakeChannelStore) put(rule ChannelRule) {
	f.rules[rule.ChannelID+"|"+rule.Action] = rule
	f.hasRules = true
}

func (f *fakeChannelStore) Lookup(_ context.Context, channelID, action string) (ChannelRule, bool, error) {
	if f.lookupErr != nil {
		return ChannelRule{}, false, f.lookupErr
	}
	r, ok := f.rules[channelID+"|"+action]
	return r, ok, nil
}

func (f *fakeChannelStore) HasRules(_ context.Context) (bool, error) {
	if f.hasErr != nil {
		return false, f.hasErr
	}
	return f.hasRules, nil
}

// TestLayer4_DenyByRule proves a configured rule flips Layer 4 to deny
// with the rule's reason — the v0.2 enforcement path.
func TestLayer4_DenyByRule(t *testing.T) {
	t.Setenv("IRONGOLEM_LAYER4_ENABLED", "")
	store := newFakeChannelStore()
	store.put(ChannelRule{
		ChannelID: "chan-locked", Action: "execute",
		Decision: DecisionDeny, Reason: "channel paused for review",
	})

	engine := NewDefaultPolicyEngineWithStore(testLogger(), store)
	res, err := engine.Evaluate(context.Background(), EvalRequest{
		TenantID: "t", WorkspaceID: "w", UserID: "u",
		AgentRole: "executor", ChannelID: "chan-locked",
		Permission: Permission{Resource: "message.outbound", Action: "execute"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.Decision != DecisionDeny {
		t.Fatalf("decision: got %q, want deny", res.Decision)
	}
	if res.Layer != LayerPerChannelRestrict {
		t.Fatalf("layer: got %s, want per_channel_restriction", res.Layer)
	}
	if res.Reason != "channel paused for review" {
		t.Fatalf("reason: got %q, want %q", res.Reason, "channel paused for review")
	}
}

// TestLayer4_AllowByRule proves an explicit allow rule passes through —
// allow rules don't short-circuit other layers, but they still surface
// the right reason for the audit log.
func TestLayer4_AllowByRule(t *testing.T) {
	t.Setenv("IRONGOLEM_LAYER4_ENABLED", "")
	store := newFakeChannelStore()
	store.put(ChannelRule{
		ChannelID: "chan-trusted", Action: "execute",
		Decision: DecisionAllow, Reason: "explicit allow",
	})
	engine := NewDefaultPolicyEngineWithStore(testLogger(), store)

	res, err := engine.Evaluate(context.Background(), EvalRequest{
		TenantID: "t", WorkspaceID: "w", UserID: "u",
		AgentRole: "executor", ChannelID: "chan-trusted",
		Permission: Permission{Resource: "message.outbound", Action: "execute"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if res.Decision != DecisionAllow {
		t.Fatalf("decision: got %q, want allow", res.Decision)
	}
}

// TestLayer4_MissingRuleFallback proves that with the store provisioned
// (has rules) but no rule for THIS (channel, action), Layer 4 allows in
// deny-list mode. Allow-list mode is a v0.3 concern.
func TestLayer4_MissingRuleFallback(t *testing.T) {
	t.Setenv("IRONGOLEM_LAYER4_ENABLED", "")
	store := newFakeChannelStore()
	store.put(ChannelRule{
		ChannelID: "chan-other", Action: "write",
		Decision: DecisionDeny, Reason: "unrelated",
	})
	engine := NewDefaultPolicyEngineWithStore(testLogger(), store)

	res, _ := engine.Evaluate(context.Background(), EvalRequest{
		TenantID: "t", WorkspaceID: "w", UserID: "u",
		AgentRole: "executor", ChannelID: "chan-quiet",
		Permission: Permission{Resource: "message.outbound", Action: "execute"},
	})
	if res.Decision != DecisionAllow {
		t.Fatalf("missing rule: got %q, want allow", res.Decision)
	}
}

// TestLayer4_StoreErrorFailsClosedWhenEnabled proves the security-positive
// failure mode: with IRONGOLEM_LAYER4_ENABLED=true and a store error, we
// deny rather than fall through.
func TestLayer4_StoreErrorFailsClosedWhenEnabled(t *testing.T) {
	t.Setenv("IRONGOLEM_LAYER4_ENABLED", "true")
	store := newFakeChannelStore()
	store.hasErr = errSentinel("db down")
	engine := NewDefaultPolicyEngineWithStore(testLogger(), store)

	res, _ := engine.Evaluate(context.Background(), EvalRequest{
		TenantID: "t", WorkspaceID: "w", UserID: "u",
		AgentRole: "executor", ChannelID: "chan-x",
		Permission: Permission{Resource: "message.outbound", Action: "execute"},
	})
	if res.Decision != DecisionDeny {
		t.Fatalf("store error + enabled: got %q, want deny", res.Decision)
	}
}

// errSentinel keeps the test file dep-free.
type errSentinel string

func (e errSentinel) Error() string { return string(e) }
