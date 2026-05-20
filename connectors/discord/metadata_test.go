package discord

import (
	"testing"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

func TestRegistration_Shape(t *testing.T) {
	r := Registration()
	if r.Type != connectors.TypeDiscord {
		t.Fatalf("Type = %q", r.Type)
	}
	if r.CheckFn == nil {
		t.Fatal("CheckFn nil")
	}
}

func TestCheckFn_RespectsBotToken(t *testing.T) {
	r := Registration()
	t.Setenv(envBotToken, "")
	if r.CheckFn() {
		t.Fatal("CheckFn = true with empty token")
	}
	t.Setenv(envBotToken, "discord-bot-token-test")
	if !r.CheckFn() {
		t.Fatal("CheckFn = false with token set")
	}
}

func TestValidateConfig(t *testing.T) {
	r := Registration()
	if err := r.ValidateConfig(map[string]string{}); err == nil {
		t.Fatal("expected error for empty config")
	}
	if err := r.ValidateConfig(map[string]string{"bot_token": "x"}); err != nil {
		t.Fatalf("valid returned %v", err)
	}
}
