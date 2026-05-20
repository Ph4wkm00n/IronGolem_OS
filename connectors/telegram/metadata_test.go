package telegram

import (
	"testing"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

func TestRegistration_Shape(t *testing.T) {
	r := Registration()
	if r.Type != connectors.TypeTelegram {
		t.Fatalf("Type = %q, want %q", r.Type, connectors.TypeTelegram)
	}
	if r.Label == "" {
		t.Fatal("Label is empty")
	}
	if len(r.RequiredEnv) == 0 {
		t.Fatal("RequiredEnv is empty; expected at least IRONGOLEM_TELEGRAM_BOT_TOKEN")
	}
	if r.InstallHint == "" {
		t.Fatal("InstallHint is empty")
	}
	if r.Source != connectors.SourceBuiltin {
		t.Fatalf("Source = %q, want %q", r.Source, connectors.SourceBuiltin)
	}
}

func TestCheckFn_RespectsEnv(t *testing.T) {
	r := Registration()
	t.Setenv(envBotToken, "")
	if r.CheckFn() {
		t.Fatal("CheckFn = true with empty token")
	}
	t.Setenv(envBotToken, "  ")
	if r.CheckFn() {
		t.Fatal("CheckFn = true with whitespace-only token")
	}
	t.Setenv(envBotToken, "12345:abc")
	if !r.CheckFn() {
		t.Fatal("CheckFn = false with valid token")
	}
}

func TestValidateConfig(t *testing.T) {
	r := Registration()
	if err := r.ValidateConfig(map[string]string{}); err == nil {
		t.Fatal("ValidateConfig(empty) = nil, expected error")
	}
	if err := r.ValidateConfig(map[string]string{"bot_token": ""}); err == nil {
		t.Fatal("ValidateConfig(empty token) = nil, expected error")
	}
	if err := r.ValidateConfig(map[string]string{"bot_token": "12345:abc"}); err != nil {
		t.Fatalf("ValidateConfig(valid) = %v, want nil", err)
	}
}
