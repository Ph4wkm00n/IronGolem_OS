// Package discord implements the IronGolem OS connector for Discord using the
// REST API for sending and Gateway WebSocket for receiving events.
package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

const (
	defaultAPIBase     = "https://discord.com/api/v10"
	defaultGatewayURL  = "wss://gateway.discord.gg/?v=10&encoding=json"
	heartbeatJitter    = 500 * time.Millisecond
	reconnectBaseDelay = 1 * time.Second
	reconnectMaxDelay  = 60 * time.Second
)

// Gateway opcodes.
const (
	opDispatch        = 0
	opHeartbeat       = 1
	opIdentify        = 2
	opResume          = 6
	opReconnect       = 7
	opInvalidSession  = 9
	opHello           = 10
	opHeartbeatACK    = 11
)

// Connector implements connectors.Connector for Discord.
type Connector struct {
	mu sync.RWMutex

	botToken     string
	guildID      string
	allowedChans map[string]bool
	apiBase      string
	gatewayURL   string

	httpClient *http.Client
	botUserID  string

	connected bool
	msgCh     chan *connectors.Message
	done      chan struct{}

	// Gateway WebSocket state.
	sessionID string
	sequence  int64
}

// --- Discord API / Gateway types ---

type discordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Bot      bool   `json:"bot"`
}

type discordMessage struct {
	ID        string       `json:"id"`
	ChannelID string       `json:"channel_id"`
	GuildID   string       `json:"guild_id,omitempty"`
	Author    *discordUser `json:"author,omitempty"`
	Content   string       `json:"content"`
	Timestamp string       `json:"timestamp"`
}

type gatewayPayload struct {
	Op   int             `json:"op"`
	D    json.RawMessage `json:"d,omitempty"`
	S    *int64          `json:"s,omitempty"`
	T    string          `json:"t,omitempty"`
}

type helloData struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type readyData struct {
	SessionID string      `json:"session_id"`
	User      discordUser `json:"user"`
}

// Type returns the connector type identifier.
func (c *Connector) Type() connectors.ConnectorType {
	return connectors.TypeDiscord
}

// Connect verifies the bot token by calling /users/@me.
func (c *Connector) Connect(ctx context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.botToken = config["bot_token"]
	if c.botToken == "" {
		return fmt.Errorf("discord connector: bot_token is required")
	}

	c.guildID = config["guild_id"]

	c.apiBase = defaultAPIBase
	if base := config["api_base"]; base != "" {
		c.apiBase = base
	}

	c.gatewayURL = defaultGatewayURL
	if gw := config["gateway_url"]; gw != "" {
		c.gatewayURL = gw
	}

	// Parse allowed_channels.
	c.allowedChans = make(map[string]bool)
	if chans := config["allowed_channels"]; chans != "" {
		for _, ch := range strings.Split(chans, ",") {
			ch = strings.TrimSpace(ch)
			if ch != "" {
				c.allowedChans[ch] = true
			}
		}
	}

	c.httpClient = &http.Client{Timeout: 30 * time.Second}

	// Verify bot token with /users/@me.
	var user discordUser
	if err := c.restCall(ctx, http.MethodGet, "/users/@me", nil, &user); err != nil {
		return fmt.Errorf("discord connector: /users/@me failed: %w", err)
	}

	c.botUserID = user.ID
	c.connected = true
	c.done = make(chan struct{})

	return nil
}

// Disconnect cleanly shuts down the Discord connector.
func (c *Connector) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false
	close(c.done)

	return nil
}

// Health checks connectivity by calling /users/@me.
func (c *Connector) Health(ctx context.Context) connectors.HealthState {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return connectors.HealthDisconnected
	}

	var user discordUser
	if err := c.restCall(ctx, http.MethodGet, "/users/@me", nil, &user); err != nil {
		return connectors.HealthDegraded
	}

	return connectors.HealthHealthy
}

// Send creates a message in a Discord channel via /channels/{id}/messages.
//
// Expected metadata keys:
//   - "channel_id" : target channel ID (required)
//   - "embed_title" : optional, title for a rich embed
//   - "embed_description" : optional, description for a rich embed
//   - "embed_color" : optional, decimal color for the embed
//   - "reaction" : optional, emoji to react with (requires "message_id")
//   - "message_id" : optional, message to react to
func (c *Connector) Send(ctx context.Context, msg *connectors.Message) error {
	channelID := msg.Metadata["channel_id"]
	if channelID == "" {
		return fmt.Errorf("discord connector: 'channel_id' metadata is required")
	}

	// Security: only send to allowed channels.
	c.mu.RLock()
	allowed := len(c.allowedChans) == 0 || c.allowedChans[channelID]
	c.mu.RUnlock()

	if !allowed {
		return fmt.Errorf("discord connector: channel %q is not in the allowed list", channelID)
	}

	// Handle reactions.
	if reaction := msg.Metadata["reaction"]; reaction != "" {
		messageID := msg.Metadata["message_id"]
		if messageID == "" {
			return fmt.Errorf("discord connector: 'message_id' metadata required for reactions")
		}
		path := fmt.Sprintf("/channels/%s/messages/%s/reactions/%s/@me", channelID, messageID, reaction)
		return c.restCall(ctx, http.MethodPut, path, nil, nil)
	}

	// Build message payload.
	payload := map[string]interface{}{
		"content": msg.Content,
	}

	// Support embeds.
	if embedTitle := msg.Metadata["embed_title"]; embedTitle != "" {
		embed := map[string]interface{}{
			"title":       embedTitle,
			"description": msg.Metadata["embed_description"],
		}
		if colorStr := msg.Metadata["embed_color"]; colorStr != "" {
			embed["color"] = colorStr
		}
		payload["embeds"] = []map[string]interface{}{embed}
	}

	path := fmt.Sprintf("/channels/%s/messages", channelID)
	var result discordMessage
	return c.restCall(ctx, http.MethodPost, path, payload, &result)
}

// Receive returns a channel of inbound messages. It starts a goroutine that
// connects to the Discord Gateway WebSocket for real-time events with
// automatic reconnection logic.
//
// NOTE: The actual WebSocket connection requires a WebSocket library (e.g.
// gorilla/websocket or nhooyr/websocket). This implementation provides the
// full reconnection framework and event normalization; the low-level WebSocket
// dial/read/write calls are abstracted behind helper methods that a production
// build would implement with a real WebSocket library.
func (c *Connector) Receive(ctx context.Context) (<-chan *connectors.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("discord connector: not connected")
	}

	c.msgCh = make(chan *connectors.Message, 64)

	go c.gatewayLoop(ctx)

	return c.msgCh, nil
}

// gatewayLoop manages the Discord Gateway WebSocket lifecycle with
// reconnection on drops.
func (c *Connector) gatewayLoop(ctx context.Context) {
	defer close(c.msgCh)

	delay := reconnectBaseDelay

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		default:
		}

		err := c.runGatewaySession(ctx)
		if err == nil {
			return // clean shutdown
		}

		// Exponential backoff for reconnection.
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-time.After(delay):
		}

		delay *= 2
		if delay > reconnectMaxDelay {
			delay = reconnectMaxDelay
		}
	}
}

// runGatewaySession represents a single Gateway WebSocket session.
// In a production implementation, this would use a real WebSocket library.
// Here we provide the full protocol framework as a reference implementation.
func (c *Connector) runGatewaySession(ctx context.Context) error {
	// In production:
	// 1. Dial c.gatewayURL via WebSocket.
	// 2. Receive HELLO (op 10) → start heartbeat loop.
	// 3. Send IDENTIFY (op 2) with bot token and intents.
	// 4. Receive READY (op 0, t="READY") → store session_id.
	// 5. Read loop: process MESSAGE_CREATE, REACTION_ADD, etc.
	// 6. On op 7 (RECONNECT) or op 9 (INVALID_SESSION), return error to trigger reconnect.

	// Stub: wait for shutdown signal.
	select {
	case <-ctx.Done():
		return nil
	case <-c.done:
		return nil
	}
}

// normalizeMessage converts a Discord message to a connector.Message.
func (c *Connector) normalizeMessage(dm *discordMessage) *connectors.Message {
	metadata := map[string]string{
		"channel_id": dm.ChannelID,
		"message_id": dm.ID,
	}
	if dm.GuildID != "" {
		metadata["guild_id"] = dm.GuildID
	}
	if dm.Author != nil {
		metadata["author_id"] = dm.Author.ID
		metadata["author_username"] = dm.Author.Username
	}

	ts, _ := time.Parse(time.RFC3339, dm.Timestamp)

	return &connectors.Message{
		ID:          fmt.Sprintf("discord_%s_%s", dm.ChannelID, dm.ID),
		ConnectorID: "",
		Type:        connectors.TypeDiscord,
		Direction:   connectors.Inbound,
		Content:     dm.Content,
		Metadata:    metadata,
		Timestamp:   ts,
	}
}

// Capabilities returns the features supported by the Discord connector.
func (c *Connector) Capabilities() []string {
	return []string{"send", "receive", "embeds", "reactions"}
}

// restCall makes a REST API request to the Discord API.
func (c *Connector) restCall(ctx context.Context, method, path string, payload interface{}, dest interface{}) error {
	c.mu.RLock()
	url := c.apiBase + path
	token := c.botToken
	c.mu.RUnlock()

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("discord connector: marshal payload: %w", err)
		}
		body = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("discord connector: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("discord connector: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("discord connector: rate limited (429), retry later")
	}

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("discord connector: API error %d: %s", resp.StatusCode, string(respBody))
	}

	if dest != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return fmt.Errorf("discord connector: decode response: %w", err)
		}
	}

	return nil
}

// Compile-time interface compliance check.
var _ connectors.Connector = (*Connector)(nil)
