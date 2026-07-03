package slack

import (
	"fmt"
	"os"
	"strings"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

const (
	envBotToken      = "IRONGOLEM_SLACK_BOT_TOKEN"
	envSigningSecret = "IRONGOLEM_SLACK_SIGNING_SECRET"
	// envAppToken carries the app-level token (xapp-...) that enables
	// Socket Mode inbound. Optional: without it the connector runs
	// outbound-only, which remains a fully supported configuration.
	envAppToken = "IRONGOLEM_SLACK_APP_TOKEN"
)

// Registration returns Slack's declarative connector metadata.
func Registration() connectors.Registration {
	return connectors.Registration{
		Type:  connectors.TypeSlack,
		Label: "Slack",
		// CheckFn intentionally requires only the bot token: the app
		// token is optional (outbound-only is valid without it).
		CheckFn: func() bool {
			return strings.TrimSpace(os.Getenv(envBotToken)) != ""
		},
		RequiredEnv: []string{envBotToken, envSigningSecret, envAppToken},
		InstallHint: "Create a Slack app at https://api.slack.com/apps, install it to your " +
			"workspace, copy the Bot Token (xoxb-...) into IRONGOLEM_SLACK_BOT_TOKEN, and the " +
			"app's Signing Secret into IRONGOLEM_SLACK_SIGNING_SECRET. For inbound messages, " +
			"enable Socket Mode, subscribe to message events, and put an app-level token " +
			"(xapp-...) with connections:write scope into IRONGOLEM_SLACK_APP_TOKEN (optional; " +
			"outbound-only works without it).",
		ValidateConfig: func(cfg map[string]string) error {
			if strings.TrimSpace(cfg["bot_token"]) == "" {
				return fmt.Errorf("bot_token is required")
			}
			return nil
		},
		Source: connectors.SourceBuiltin,
	}
}

func init() { connectors.MustRegister(Registration()) }
