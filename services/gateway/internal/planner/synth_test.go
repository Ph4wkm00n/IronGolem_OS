package planner

import (
	"strings"
	"testing"
	"time"

	ipc "github.com/Ph4wkm00n/IronGolem_OS/services/pkg/runtime"
)

func TestSynthesizePlan_ShapeAndContent(t *testing.T) {
	fixed := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	plan := SynthesizePlan(InboundMessage{
		ConnectorID: "telegram",
		ChannelID:   "chat-1",
		UserID:      "u-42",
		Content:     "hello there",
		TenantID:    "default",
		WorkspaceID: "ws-1",
	}, Options{
		AgentID: "agent-fixed",
		Now:     func() time.Time { return fixed },
	})

	if plan.AgentID != "agent-fixed" {
		t.Fatalf("AgentID: got %q, want agent-fixed", plan.AgentID)
	}
	if plan.Status != ipc.PlanStatusPending {
		t.Fatalf("Status: got %q, want pending", plan.Status)
	}
	if !plan.CreatedAt.Equal(fixed) {
		t.Fatalf("CreatedAt: got %v, want %v", plan.CreatedAt, fixed)
	}
	if got := len(plan.Nodes); got != 1 {
		t.Fatalf("Nodes: got %d, want 1", got)
	}
	node := plan.Nodes[0]
	if node.Kind.Type != ipc.NodeKindLlmCall {
		t.Fatalf("node Kind: got %q, want LlmCall", node.Kind.Type)
	}
	if !strings.Contains(node.Kind.Prompt, "hello there") {
		t.Fatalf("prompt missing user content: %q", node.Kind.Prompt)
	}
	if !strings.HasPrefix(node.Kind.Prompt, SystemPrompt) {
		t.Fatalf("prompt missing system header: %q", node.Kind.Prompt[:min(80, len(node.Kind.Prompt))])
	}
	if node.Kind.Model != "" {
		t.Fatalf("Model: got %q, want empty (runtime default)", node.Kind.Model)
	}
}

func TestSynthesizePlan_SystemPromptOverride(t *testing.T) {
	plan := SynthesizePlan(InboundMessage{Content: "ping"}, Options{
		SystemPromptOverride: "You are a test bot.",
	})
	got := plan.Nodes[0].Kind.Prompt
	if !strings.HasPrefix(got, "You are a test bot.") {
		t.Fatalf("override not applied: %q", got)
	}
	if strings.Contains(got, SystemPrompt) {
		t.Fatalf("default system prompt leaked through: %q", got)
	}
}
