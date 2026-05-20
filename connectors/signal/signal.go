// Package signal implements the IronGolem OS connector for Signal via
// the `signal-cli` JSON-RPC bridge.
//
// v0.3 Step 8 of Plans/modular-puzzling-blum.md. Native libsignal
// integration is a separate v0.4+ effort; v0.3 takes the pragmatic
// path of shelling out to `signal-cli`, which most operators already
// have installed for verified-account flows.
//
// v0.3 ships the OUTBOUND path (Send → `signal-cli send`). INBOUND
// requires `signal-cli daemon --receive-mode` + JSON-RPC subscription,
// which lands in v0.4.
package signal

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

// Connector implements connectors.Connector for Signal.
type Connector struct {
	mu sync.RWMutex

	cliPath     string // resolved path to signal-cli binary
	accountName string // Signal account (phone number with +) to send as
	connected   bool

	msgCh chan *connectors.Message
	done  chan struct{}
}

func (c *Connector) Type() connectors.ConnectorType { return connectors.TypeSignal }

func (c *Connector) Connect(_ context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accountName = config["account"]
	if c.accountName == "" {
		return fmt.Errorf("signal: account (phone number) required in config")
	}
	binary := config["signal_cli_path"]
	if binary == "" {
		binary = "signal-cli"
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return fmt.Errorf("signal: %s not found in PATH; install via `brew install signal-cli` or apt", binary)
	}
	c.cliPath = path
	c.msgCh = make(chan *connectors.Message, 1)
	c.done = make(chan struct{})
	c.connected = true
	return nil
}

func (c *Connector) Disconnect(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.connected {
		return nil
	}
	close(c.done)
	close(c.msgCh)
	c.connected = false
	return nil
}

func (c *Connector) Health(_ context.Context) connectors.HealthState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.connected {
		return connectors.HealthDisconnected
	}
	return connectors.HealthHealthy
}

// Send invokes `signal-cli -u <account> send -m <text> <recipient>`.
// msg.Metadata["recipient"] must contain a phone number (with country
// code prefix) for individual chats. Group support lands in v0.4.
func (c *Connector) Send(ctx context.Context, msg *connectors.Message) error {
	c.mu.RLock()
	connected := c.connected
	cli := c.cliPath
	account := c.accountName
	c.mu.RUnlock()
	if !connected {
		return fmt.Errorf("signal: not connected")
	}
	if msg == nil || msg.Metadata["recipient"] == "" {
		return fmt.Errorf("signal: msg.Metadata[\"recipient\"] required")
	}
	if !strings.HasPrefix(msg.Metadata["recipient"], "+") {
		return fmt.Errorf("signal: recipient must be a phone number with country code (got %q)", msg.Metadata["recipient"])
	}

	cmd := exec.CommandContext(ctx, cli, "-u", account, "send", "-m", msg.Content, msg.Metadata["recipient"])
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("signal-cli send: %w; output: %s", err, string(out))
	}
	return nil
}

// Receive returns a closed inbound channel — Signal inbound delivery
// requires `signal-cli daemon` mode which v0.3 doesn't wire. The
// closed channel makes the manager's pump exit cleanly instead of
// blocking forever on an unused subscription.
func (c *Connector) Receive(_ context.Context) (<-chan *connectors.Message, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.connected {
		return nil, fmt.Errorf("signal: not connected")
	}
	ch := make(chan *connectors.Message)
	close(ch)
	return ch, nil
}

func (c *Connector) Capabilities() []string { return []string{"send"} }
