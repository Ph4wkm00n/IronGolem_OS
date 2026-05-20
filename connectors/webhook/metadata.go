package webhook

import (
	"fmt"
	"os"
	"strings"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

// Webhook is opt-in by config map, not env, so CheckFn passes
// unconditionally — the connector is "always available" but only
// activates once a target URL is supplied at Connect time. The hint
// still points the operator at the env override smoke tests use.
const envWebhookURL = "IRONGOLEM_WEBHOOK_URL"

func Registration() connectors.Registration {
	return connectors.Registration{
		Type:        connectors.TypeWebhook,
		Label:       "Webhook (generic HTTP)",
		CheckFn:     func() bool { return true },
		RequiredEnv: []string{},
		InstallHint: "No env vars required — configure target_url + optional auth via " +
			"POST /api/v1/connectors/{id}/connect. The smoke scripts set " +
			envWebhookURL + " for end-to-end checks.",
		ValidateConfig: func(cfg map[string]string) error {
			if strings.TrimSpace(cfg["target_url"]) == "" {
				return fmt.Errorf("target_url is required")
			}
			return nil
		},
		Source: connectors.SourceBuiltin,
	}
}

// envIsSet is a small belt-and-suspenders helper used by tests so they
// can verify the webhook hint references the live env name. Not exported.
func envIsSet(name string) bool { return strings.TrimSpace(os.Getenv(name)) != "" }

func init() { connectors.MustRegister(Registration()) }
