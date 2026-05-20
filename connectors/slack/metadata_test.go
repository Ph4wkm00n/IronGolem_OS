package slack

import (
	"testing"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

func TestRegistration_Shape(t *testing.T) {
	r := Registration()
	if r.Type != connectors.TypeSlack {
		t.Fatalf("Type = %q", r.Type)
	}
	if r.Label != "Slack" {
		t.Errorf("Label = %q", r.Label)
	}
	if len(r.RequiredEnv) < 2 {
		t.Errorf("RequiredEnv len = %d, want ≥2", len(r.RequiredEnv))
	}
}

func TestCheckFn_RespectsBotToken(t *testing.T) {
	r := Registration()
	t.Setenv(envBotToken, "")
	if r.CheckFn() {
		t.Fatal("CheckFn = true with no bot token")
	}
	t.Setenv(envBotToken, "xoxb-test")
	if !r.CheckFn() {
		t.Fatal("CheckFn = false with valid token")
	}
}

func TestValidateConfig(t *testing.T) {
	r := Registration()
	if err := r.ValidateConfig(map[string]string{}); err == nil {
		t.Fatal("expected error for missing bot_token")
	}
	if err := r.ValidateConfig(map[string]string{"bot_token": "xoxb-test"}); err != nil {
		t.Fatalf("valid config returned %v", err)
	}
}
