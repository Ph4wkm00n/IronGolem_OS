package email

import (
	"fmt"
	"os"
	"strings"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

// Email reads IMAP + SMTP credentials from env at boot. The hosts must
// both be set for CheckFn to pass; username/password are surfaced via
// RequiredEnv so the operator knows what else they need.
const (
	envIMAPHost = "IRONGOLEM_EMAIL_IMAP_HOST"
	envIMAPPort = "IRONGOLEM_EMAIL_IMAP_PORT"
	envSMTPHost = "IRONGOLEM_EMAIL_SMTP_HOST"
	envSMTPPort = "IRONGOLEM_EMAIL_SMTP_PORT"
	envUsername = "IRONGOLEM_EMAIL_USERNAME"
	envPassword = "IRONGOLEM_EMAIL_PASSWORD"
)

func Registration() connectors.Registration {
	return connectors.Registration{
		Type:  connectors.TypeEmail,
		Label: "Email (IMAP/SMTP)",
		CheckFn: func() bool {
			return strings.TrimSpace(os.Getenv(envIMAPHost)) != "" &&
				strings.TrimSpace(os.Getenv(envSMTPHost)) != ""
		},
		RequiredEnv: []string{envIMAPHost, envIMAPPort, envSMTPHost, envSMTPPort, envUsername, envPassword},
		InstallHint: "Set IRONGOLEM_EMAIL_IMAP_HOST/PORT + IRONGOLEM_EMAIL_SMTP_HOST/PORT plus " +
			"IRONGOLEM_EMAIL_USERNAME/PASSWORD. Gmail users: enable an App Password and use smtp.gmail.com:465.",
		ValidateConfig: func(cfg map[string]string) error {
			if strings.TrimSpace(cfg["imap_host"]) == "" || strings.TrimSpace(cfg["imap_port"]) == "" {
				return fmt.Errorf("imap_host and imap_port are required")
			}
			if strings.TrimSpace(cfg["smtp_host"]) == "" || strings.TrimSpace(cfg["smtp_port"]) == "" {
				return fmt.Errorf("smtp_host and smtp_port are required")
			}
			return nil
		},
		Source: connectors.SourceBuiltin,
	}
}

func init() { connectors.MustRegister(Registration()) }
