package signal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

const (
	envAccount = "IRONGOLEM_SIGNAL_ACCOUNT"
	envCLIPath = "IRONGOLEM_SIGNAL_CLI_PATH" // override; otherwise PATH lookup
	defaultCLI = "signal-cli"
)

func Registration() connectors.Registration {
	return connectors.Registration{
		Type:  connectors.TypeSignal,
		Label: "Signal",
		CheckFn: func() bool {
			if strings.TrimSpace(os.Getenv(envAccount)) == "" {
				return false
			}
			binary := os.Getenv(envCLIPath)
			if binary == "" {
				binary = defaultCLI
			}
			_, err := exec.LookPath(binary)
			return err == nil
		},
		RequiredEnv: []string{envAccount, envCLIPath},
		InstallHint: "Install signal-cli (`brew install signal-cli` on macOS, or download the JAR " +
			"from https://github.com/AsamK/signal-cli/releases), register an account, and export " +
			"IRONGOLEM_SIGNAL_ACCOUNT=+15551234567 (your verified phone number).",
		ValidateConfig: func(cfg map[string]string) error {
			if strings.TrimSpace(cfg["account"]) == "" {
				return fmt.Errorf("account (phone number with country code) is required")
			}
			return nil
		},
		Source: connectors.SourceBuiltin,
	}
}

func init() { connectors.MustRegister(Registration()) }
