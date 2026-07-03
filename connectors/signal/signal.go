// Package signal implements the IronGolem OS connector for Signal via
// the `signal-cli` JSON-RPC bridge.
//
// v0.3 Step 8 of Plans/modular-puzzling-blum.md. Native libsignal
// integration is a separate v0.4+ effort; v0.3 takes the pragmatic
// path of shelling out to `signal-cli`, which most operators already
// have installed for verified-account flows.
//
// v0.3 shipped the OUTBOUND path (Send → `signal-cli send`). v0.4 adds
// INBOUND via a long-running `signal-cli receive --output=json`
// subprocess — see inbound.go. If Connect succeeds (binary + account
// present), inbound is available; without them the connector simply
// never connects, exactly as in v0.3.
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

	// receiveStarted guards the once-only inbound worker spawn.
	// closeMsgOnce guarantees msgCh closes exactly once whether the
	// worker or Disconnect gets there first.
	receiveStarted bool
	workerOwnsCh   bool
	closeMsgOnce   sync.Once
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
	c.msgCh = make(chan *connectors.Message, 64)
	c.done = make(chan struct{})
	c.receiveStarted = false
	c.workerOwnsCh = false
	c.closeMsgOnce = sync.Once{}
	c.connected = true
	return nil
}

// Disconnect signals shutdown. If the receive worker is running it owns
// closing msgCh (it may still be mid-send); otherwise we close it here
// so the gateway pump observes the shutdown either way.
func (c *Connector) Disconnect(_ context.Context) error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil
	}
	c.connected = false
	workerOwnsCh := c.workerOwnsCh
	done := c.done
	msgCh := c.msgCh
	c.mu.Unlock()

	close(done)
	if !workerOwnsCh {
		c.closeMsgOnce.Do(func() { close(msgCh) })
	}
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

	// v1.2.2: argv hardening. `--` tells signal-cli's flag parser that
	// no more flags follow — protects against a future signal-cli
	// version that begins to recognize a flag whose name happens to
	// match user-supplied content. Strictly belt-and-suspenders: there
	// is no shell here, so there's no shell-injection vector. argv-
	// injection (e.g. a recipient starting with `-` that hits signal-
	// cli's flag parser) is the only attack surface left, and the
	// existing HasPrefix("+") check already blocks it for the
	// recipient slot. Adding `--` defends the message-content slot the
	// same way: if signal-cli ever grows a flag named after a phrase
	// the user might type, the parser still treats it as content.
	cmd := exec.CommandContext(ctx, cli,
		"-u", account,
		"send",
		"-m", msg.Content,
		"--",
		msg.Metadata["recipient"],
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("signal-cli send: %w; output: %s", err, string(out))
	}
	return nil
}

// Receive returns the inbound message channel and starts the
// long-lived `signal-cli receive` worker (connectors.StartWorker:
// context cancellation, panic recovery, exponential-backoff restart).
// Connect already guarantees the binary and account exist, so unlike
// Slack there is no partial outbound-only configuration to detect.
func (c *Connector) Receive(ctx context.Context) (<-chan *connectors.Message, error) {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil, fmt.Errorf("signal: not connected")
	}
	msgCh := c.msgCh
	if c.receiveStarted {
		c.mu.Unlock()
		return msgCh, nil
	}
	c.receiveStarted = true
	c.workerOwnsCh = true
	done := c.done
	c.mu.Unlock()

	connectors.StartWorker(ctx, done,
		connectors.WorkerConfig{Name: "signal-receive"},
		c.runReceiveSession,
		func() { c.closeMsgOnce.Do(func() { close(msgCh) }) },
	)
	return msgCh, nil
}

func (c *Connector) Capabilities() []string { return []string{"send", "receive"} }
