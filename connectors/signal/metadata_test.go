package signal

import (
	"testing"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

func TestRegistration_Shape(t *testing.T) {
	r := Registration()
	if r.Type != connectors.TypeSignal {
		t.Fatalf("Type = %q", r.Type)
	}
	if r.Label != "Signal" {
		t.Errorf("Label = %q", r.Label)
	}
}

func TestCheckFn_RequiresAccountAndCLI(t *testing.T) {
	r := Registration()
	t.Setenv(envAccount, "")
	if r.CheckFn() {
		t.Fatal("CheckFn = true with empty account")
	}
	// Even with an account, missing binary on PATH fails.
	t.Setenv(envAccount, "+15551234567")
	t.Setenv(envCLIPath, "definitely-not-on-path-12345")
	if r.CheckFn() {
		t.Fatal("CheckFn = true with missing binary")
	}
}

func TestValidateConfig(t *testing.T) {
	r := Registration()
	if err := r.ValidateConfig(map[string]string{}); err == nil {
		t.Fatal("expected error for empty config")
	}
	if err := r.ValidateConfig(map[string]string{"account": "+15551234567"}); err != nil {
		t.Fatalf("valid returned %v", err)
	}
}
