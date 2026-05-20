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
)

// Registration returns Slack's declarative connector metadata.
func Registration() connectors.Registration {
	return connectors.Registration{
		Type:  connectors.TypeSlack,
		Label: "Slack",
		CheckFn: func() bool {
			return strings.TrimSpace(os.Getenv(envBotToken)) != ""
		},
		RequiredEnv: []string{envBotToken, envSigningSecret},
		InstallHint: "Create a Slack app at https://api.slack.com/apps, install it to your " +
			"workspace, copy the Bot Token (xoxb-...) into IRONGOLEM_SLACK_BOT_TOKEN, and the " +
			"app's Signing Secret into IRONGOLEM_SLACK_SIGNING_SECRET.",
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
