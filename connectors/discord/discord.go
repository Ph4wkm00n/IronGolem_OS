// Package discord implements the IronGolem OS connector for Discord.
//
// v0.3 Step 8 of Plans/modular-puzzling-blum.md. Mirrors the structure
// of `connectors/slack` and `connectors/telegram`.
//
// v0.3 shipped the OUTBOUND path (Send → channels/{id}/messages)
// end-to-end. v0.4 adds INBOUND via the Discord Gateway WebSocket —
// see inbound.go for the session protocol (HELLO, IDENTIFY, heartbeat,
// MESSAGE_CREATE dispatch) and the shared connectors worker that
// reconnects with backoff.
package discord

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
	defaultAPIBase = "https://discord.com/api/v10"
	requestTimeout = 10 * time.Second
)

// Connector implements connectors.Connector for Discord.
type Connector struct {
	mu sync.RWMutex

	botToken   string
	apiBase    string
	httpClient *http.Client
	selfID     string // our bot's user id from /users/@me; used to ignore self-messages

	connected bool
	msgCh     chan *connectors.Message
	done      chan struct{}

	// receiveStarted guards the once-only worker spawn. closeMsgOnce
	// guarantees msgCh closes exactly once whether the gateway worker
	// or Disconnect gets there first.
	receiveStarted bool
	workerOwnsCh   bool
	closeMsgOnce   sync.Once
}

func (c *Connector) Type() connectors.ConnectorType { return connectors.TypeDiscord }

func (c *Connector) Connect(ctx context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.botToken = config["bot_token"]
	c.apiBase = config["api_base"]
	if c.apiBase == "" {
		c.apiBase = defaultAPIBase
	}
	if c.botToken == "" {
		return fmt.Errorf("discord: bot_token is required")
	}
	c.httpClient = &http.Client{Timeout: requestTimeout}

	// Validate the token by hitting /users/@me — equivalent of Slack's
	// auth.test. Fail-closed: surface bad tokens at setup, not at
	// first message-send time.
	if err := c.getMe(ctx); err != nil {
		return fmt.Errorf("discord: token validation failed: %w", err)
	}

	c.msgCh = make(chan *connectors.Message, 64)
	c.done = make(chan struct{})
	c.receiveStarted = false
	c.workerOwnsCh = false
	c.closeMsgOnce = sync.Once{}
	c.connected = true
	return nil
}

// Disconnect signals shutdown. If the gateway worker is running it owns
// closing msgCh (it may still be mid-send); otherwise close it here so
// the gateway pump observes the shutdown either way.
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

// Send posts an outbound message. msg.Metadata["channel"] must contain
// the Discord channel ID; the caller is responsible for capturing this
// from inbound messages or from setup config.
func (c *Connector) Send(ctx context.Context, msg *connectors.Message) error {
	c.mu.RLock()
	connected := c.connected
	token := c.botToken
	base := c.apiBase
	client := c.httpClient
	c.mu.RUnlock()
	if !connected {
		return fmt.Errorf("discord: not connected")
	}
	if msg == nil || msg.Metadata["channel"] == "" {
		return fmt.Errorf("discord: msg.Metadata[\"channel\"] required (Discord channel id)")
	}

	body, _ := json.Marshal(map[string]any{
		"content": msg.Content,
	})
	url := fmt.Sprintf("%s/channels/%s/messages", base, msg.Metadata["channel"])
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bot "+token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("discord: send: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("discord: send status %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

// Receive returns the inbound message channel and starts the long-lived
// Gateway WebSocket worker on first call (connectors.StartWorker:
// context cancellation, panic recovery, exponential-backoff reconnect —
// each reconnect re-identifies; see inbound.go).
func (c *Connector) Receive(ctx context.Context) (<-chan *connectors.Message, error) {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil, fmt.Errorf("discord: not connected")
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
		connectors.WorkerConfig{Name: "discord-gateway"},
		c.runGatewaySession,
		func() { c.closeMsgOnce.Do(func() { close(msgCh) }) },
	)
	return msgCh, nil
}

func (c *Connector) Capabilities() []string { return []string{"send", "receive"} }

// getMe validates the token against /users/@me and captures our own
// user id so the inbound path can ignore self-messages. Called from
// Connect with c.mu held.
func (c *Connector) getMe(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"/users/@me", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+c.botToken)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid token (401 from /users/@me)")
	}
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &me); err == nil {
		c.selfID = me.ID
	}
	return nil
}
