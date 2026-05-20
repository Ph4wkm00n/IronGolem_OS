// Package slack implements the IronGolem OS connector for the Slack
// Web API (chat.postMessage for outbound) and the Events API (inbound).
//
// v0.3 Step 8 of Plans/modular-puzzling-blum.md. Mirrors the structure
// of `connectors/telegram` to keep the connector adapter pattern
// uniform across channels.
//
// v0.3 ships the OUTBOUND path (Send → chat.postMessage) end-to-end so
// the commitments runtime can fire reminders into Slack. INBOUND is
// stubbed pending Events API webhook + request-signing plumbing — see
// the TODO comments in `Receive` and the `connectors/slack/README.md`
// for the v0.4 roadmap.
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

const (
	defaultAPIBase = "https://slack.com/api"
	requestTimeout = 10 * time.Second
)

// Connector implements connectors.Connector for Slack.
type Connector struct {
	mu sync.RWMutex

	botToken      string
	apiBase       string
	signingSecret string // reserved for Events API verification (v0.4)
	httpClient    *http.Client

	connected bool
	msgCh     chan *connectors.Message
	done      chan struct{}
}

// Type returns the canonical connector identifier.
func (c *Connector) Type() connectors.ConnectorType {
	return connectors.TypeSlack
}

// Connect verifies the bot token via auth.test and stores config.
func (c *Connector) Connect(ctx context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.botToken = config["bot_token"]
	c.signingSecret = config["signing_secret"]
	c.apiBase = config["api_base"]
	if c.apiBase == "" {
		c.apiBase = defaultAPIBase
	}
	if c.botToken == "" {
		return fmt.Errorf("slack: bot_token is required")
	}

	c.httpClient = &http.Client{Timeout: requestTimeout}

	// Token validation via auth.test. Failure here is fail-closed:
	// we'd rather a clear setup error than a silent send-failure later.
	if err := c.authTest(ctx); err != nil {
		return fmt.Errorf("slack: auth.test failed: %w", err)
	}

	c.msgCh = make(chan *connectors.Message, 64)
	c.done = make(chan struct{})
	c.connected = true
	return nil
}

// Disconnect closes the inbound channel.
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

// Health returns the current health state.
func (c *Connector) Health(_ context.Context) connectors.HealthState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.connected {
		return connectors.HealthDisconnected
	}
	return connectors.HealthHealthy
}

// Send delivers an outbound message via chat.postMessage.
func (c *Connector) Send(ctx context.Context, msg *connectors.Message) error {
	c.mu.RLock()
	connected := c.connected
	token := c.botToken
	base := c.apiBase
	client := c.httpClient
	c.mu.RUnlock()
	if !connected {
		return fmt.Errorf("slack: not connected")
	}
	if msg == nil || msg.Metadata["channel"] == "" {
		return fmt.Errorf("slack: msg.Metadata[\"channel\"] required (Slack channel id)")
	}

	body, _ := json.Marshal(map[string]any{
		"channel": msg.Metadata["channel"],
		"text":    msg.Content,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat.postMessage", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: send: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("slack: send status %d: %s", resp.StatusCode, string(raw))
	}
	var parsed struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("slack: parse response: %w", err)
	}
	if !parsed.OK {
		return fmt.Errorf("slack: api error: %s", parsed.Error)
	}
	return nil
}

// Receive returns the inbound message channel.
//
// v0.3 status: the channel is wired (so the manager's pump doesn't
// nil-deref) but no messages arrive on it. Inbound delivery requires:
//
//   1. A public HTTPS endpoint for the Slack Events API to POST to.
//   2. Request signing verification using `signing_secret`.
//   3. A normalizer that turns Slack message events into
//      connectors.Message values and writes them to msgCh.
//
// All three land together in v0.4. The webhook handler will likely
// live in `services/gateway/cmd/slack-webhook/main.go` so the gateway
// itself doesn't have to grow per-channel route surface.
func (c *Connector) Receive(_ context.Context) (<-chan *connectors.Message, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.connected {
		return nil, fmt.Errorf("slack: not connected")
	}
	return c.msgCh, nil
}

// Capabilities reports what this connector can do.
func (c *Connector) Capabilities() []string {
	return []string{"send"} // "receive" lands in v0.4 with Events API wiring
}

// authTest validates the bot token by calling auth.test.
func (c *Connector) authTest(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"/auth.test", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.botToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}
	var parsed struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	if !parsed.OK {
		return fmt.Errorf("auth.test: %s", parsed.Error)
	}
	return nil
}
