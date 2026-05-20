package discord

import (
	"fmt"
	"os"
	"strings"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

const envBotToken = "IRONGOLEM_DISCORD_BOT_TOKEN"

func Registration() connectors.Registration {
	return connectors.Registration{
		Type:        connectors.TypeDiscord,
		Label:       "Discord",
		CheckFn:     func() bool { return strings.TrimSpace(os.Getenv(envBotToken)) != "" },
		RequiredEnv: []string{envBotToken},
		InstallHint: "Create a Discord application at https://discord.com/developers/applications, " +
			"add a bot user, copy the bot token into IRONGOLEM_DISCORD_BOT_TOKEN, and invite the " +
			"bot to your server with at least the 'Send Messages' permission.",
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
