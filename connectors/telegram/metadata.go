package telegram

import (
	"fmt"
	"os"
	"strings"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

// envBotToken is the canonical env var carrying the Telegram bot token.
// Kept here (not in telegram.go) so the metadata stays adjacent to the
// CheckFn that reads it.
const envBotToken = "IRONGOLEM_TELEGRAM_BOT_TOKEN"

// Registration returns Telegram's declarative metadata. Exported so
// tests can exercise CheckFn / ValidateConfig without going through the
// package-level registry.
func Registration() connectors.Registration {
	return connectors.Registration{
		Type:        connectors.TypeTelegram,
		Label:       "Telegram",
		CheckFn:     func() bool { return strings.TrimSpace(os.Getenv(envBotToken)) != "" },
		RequiredEnv: []string{envBotToken},
		InstallHint: "Create a bot via @BotFather and export IRONGOLEM_TELEGRAM_BOT_TOKEN. " +
			"Optionally set IRONGOLEM_TELEGRAM_ALLOWED_CHAT_IDS for a chat allowlist.",
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
