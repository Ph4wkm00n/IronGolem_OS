package runtime

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExecutePlanRequestRoundtrip(t *testing.T) {
	req := ExecutePlanRequest{
		Kind:        KindExecutePlanRequest,
		RequestID:   "11111111-1111-1111-1111-111111111111",
		WorkspaceID: "22222222-2222-2222-2222-222222222222",
		Plan: Plan{
			ID:          "33333333-3333-3333-3333-333333333333",
			Description: "smoke",
			AgentID:     "44444444-4444-4444-4444-444444444444",
			Status:      PlanStatusPending,
			Nodes: []PlanNode{
				{
					ID:          "55555555-5555-5555-5555-555555555555",
					Description: "echo",
					Kind: PlanNodeKind{
						Type:     NodeKindToolCall,
						ToolName: "echo",
						Input:    json.RawMessage(`{"hello":"world"}`),
					},
					Status:       NodeStatusPending,
					Dependencies: []string{},
				},
			},
		},
	}

	out, err := json.Marshal(&req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)

	mustContain(t, s, `"kind":"execute_plan_request"`)
	mustContain(t, s, `"type":"ToolCall"`)
	mustContain(t, s, `"tool_name":"echo"`)
	mustContain(t, s, `"status":"pending"`)

	var back ExecutePlanRequest
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Plan.Nodes[0].Kind.ToolName != "echo" {
		t.Fatalf("tool_name lost in round trip")
	}
}

func TestEnvelopeDispatch(t *testing.T) {
	rustWire := `{"kind":"event_notification","request_id":"abc","event":{"id":"e1","timestamp":"2026-05-10T00:00:00Z","workspace_id":"w1","kind":{"type":"PlanCompleted","data":{"plan_id":"p1"}}}}`

	var env Envelope
	if err := json.Unmarshal([]byte(rustWire), &env); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if env.Kind != KindEventNotification {
		t.Fatalf("got kind %q", env.Kind)
	}

	var n EventNotification
	if err := json.Unmarshal([]byte(rustWire), &n); err != nil {
		t.Fatalf("notification: %v", err)
	}

	var e RuntimeEvent
	if err := json.Unmarshal(n.Event, &e); err != nil {
		t.Fatalf("event: %v", err)
	}
	if e.Kind.Type != EventPlanCompleted {
		t.Fatalf("got event type %q", e.Kind.Type)
	}
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected %q to contain %q", haystack, needle)
	}
}
