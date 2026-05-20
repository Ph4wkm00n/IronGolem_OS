package webhook

import (
	"testing"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

func TestRegistration_AlwaysAvailable(t *testing.T) {
	r := Registration()
	if r.Type != connectors.TypeWebhook {
		t.Fatalf("Type = %q, want %q", r.Type, connectors.TypeWebhook)
	}
	if !r.CheckFn() {
		t.Fatal("Webhook CheckFn should pass unconditionally; it's config-driven, not env-driven")
	}
}

func TestValidateConfig_RequiresTargetURL(t *testing.T) {
	r := Registration()
	if err := r.ValidateConfig(map[string]string{}); err == nil {
		t.Fatal("ValidateConfig(empty) = nil, expected target_url required")
	}
	if err := r.ValidateConfig(map[string]string{"target_url": "https://example.com/hook"}); err != nil {
		t.Fatalf("valid config returned %v", err)
	}
}

func TestEnvIsSet_HelperReachable(t *testing.T) {
	// envIsSet is unexported but tests touch it to keep the helper
	// honest. If we later delete it, this test fails fast.
	t.Setenv(envWebhookURL, "")
	if envIsSet(envWebhookURL) {
		t.Fatal("envIsSet = true with empty value")
	}
	t.Setenv(envWebhookURL, "https://example.com/hook")
	if !envIsSet(envWebhookURL) {
		t.Fatal("envIsSet = false with non-empty value")
	}
}
