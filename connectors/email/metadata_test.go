package email

import (
	"testing"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

func TestRegistration_Shape(t *testing.T) {
	r := Registration()
	if r.Type != connectors.TypeEmail {
		t.Fatalf("Type = %q, want %q", r.Type, connectors.TypeEmail)
	}
	if len(r.RequiredEnv) < 4 {
		t.Fatalf("RequiredEnv len = %d, want ≥4 (imap host/port + smtp host/port + creds)", len(r.RequiredEnv))
	}
}

func TestCheckFn_RequiresBothHosts(t *testing.T) {
	r := Registration()
	t.Setenv(envIMAPHost, "")
	t.Setenv(envSMTPHost, "")
	if r.CheckFn() {
		t.Fatal("CheckFn = true with no hosts")
	}
	t.Setenv(envIMAPHost, "imap.example.com")
	if r.CheckFn() {
		t.Fatal("CheckFn = true with IMAP-only")
	}
	t.Setenv(envSMTPHost, "smtp.example.com")
	if !r.CheckFn() {
		t.Fatal("CheckFn = false with both hosts set")
	}
}

func TestValidateConfig_RequiresHostsAndPorts(t *testing.T) {
	r := Registration()
	cases := []map[string]string{
		{},
		{"imap_host": "x"},
		{"imap_host": "x", "imap_port": "143"},
		{"imap_host": "x", "imap_port": "143", "smtp_host": "y"},
	}
	for i, c := range cases {
		if err := r.ValidateConfig(c); err == nil {
			t.Fatalf("case %d: expected error, got nil (cfg=%v)", i, c)
		}
	}
	ok := map[string]string{"imap_host": "x", "imap_port": "143", "smtp_host": "y", "smtp_port": "465"}
	if err := r.ValidateConfig(ok); err != nil {
		t.Fatalf("valid config returned %v", err)
	}
}
