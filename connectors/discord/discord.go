// Package discord implements the IronGolem OS connector for Discord.
//
// v0.3 Step 8 of Plans/modular-puzzling-blum.md. Mirrors the structure
// of `connectors/slack` and `connectors/telegram`.
//
// v0.3 ships the OUTBOUND path (Send → channels/{id}/messages)
// end-to-end. INBOUND requires the Discord Gateway WebSocket, which
// lands in v0.4 — see `Receive` for the contract.
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

	connected bool
	msgCh     chan *connectors.Message
	done      chan struct{}
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

// Receive returns the inbound channel.
//
// v0.3 status: stub. Real inbound delivery requires the Discord
// Gateway WebSocket (wss://gateway.discord.gg) + identification +
// heartbeat + event dispatch handling. Lands in v0.4 alongside the
// Slack Events API receiver — both will likely share a "long-lived
// connector worker" pattern that the current pump doesn't model yet.
func (c *Connector) Receive(_ context.Context) (<-chan *connectors.Message, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.connected {
		return nil, fmt.Errorf("discord: not connected")
	}
	return c.msgCh, nil
}

func (c *Connector) Capabilities() []string { return []string{"send"} }

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
	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}
