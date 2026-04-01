// Package slack implements the IronGolem OS connector for Slack using the
// Bot API (chat.postMessage) and Events API (HTTP webhook mode).
package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

const (
	defaultAPIBase = "https://slack.com/api"
)

// Connector implements connectors.Connector for the Slack API.
type Connector struct {
	mu sync.RWMutex

	botToken       string
	signingSecret  string
	defaultChannel string
	allowedChans   map[string]bool
	apiBase        string

	httpClient *http.Client
	botUserID  string
	teamName   string

	connected bool
	msgCh     chan *connectors.Message
	done      chan struct{}

	// eventsAddr is the listen address for the Events API HTTP server.
	eventsAddr   string
	eventsServer *http.Server
}

// --- Slack API response types ---

type slackResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Warning  string `json:"warning,omitempty"`
	TeamName string `json:"team,omitempty"`
	UserID   string `json:"user_id,omitempty"`
}

type authTestResponse struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
	UserID string `json:"user_id"`
	Team   string `json:"team"`
}

type chatPostResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Channel string `json:"channel"`
	TS      string `json:"ts"`
}

// eventsAPIPayload is the outer wrapper for Slack Events API callbacks.
type eventsAPIPayload struct {
	Token     string          `json:"token"`
	Type      string          `json:"type"`
	Challenge string          `json:"challenge,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
}

type slackEvent struct {
	Type    string `json:"type"`
	User    string `json:"user,omitempty"`
	Text    string `json:"text,omitempty"`
	Channel string `json:"channel,omitempty"`
	TS      string `json:"ts,omitempty"`
	ThreadTS string `json:"thread_ts,omitempty"`
	SubType string `json:"subtype,omitempty"`
	// File upload fields.
	FileID string `json:"file_id,omitempty"`
	// Reaction fields.
	Reaction string `json:"reaction,omitempty"`
	ItemUser string `json:"item_user,omitempty"`
}

// Type returns the connector type identifier.
func (c *Connector) Type() connectors.ConnectorType {
	return connectors.TypeSlack
}

// Connect verifies the bot token by calling auth.test and stores configuration.
func (c *Connector) Connect(ctx context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.botToken = config["bot_token"]
	if c.botToken == "" {
		return fmt.Errorf("slack connector: bot_token is required")
	}

	c.signingSecret = config["signing_secret"]
	if c.signingSecret == "" {
		return fmt.Errorf("slack connector: signing_secret is required")
	}

	c.defaultChannel = config["default_channel"]

	c.apiBase = defaultAPIBase
	if base := config["api_base"]; base != "" {
		c.apiBase = base
	}

	// Parse allowed_channels as a comma-separated list.
	c.allowedChans = make(map[string]bool)
	if chans := config["allowed_channels"]; chans != "" {
		for _, ch := range strings.Split(chans, ",") {
			ch = strings.TrimSpace(ch)
			if ch != "" {
				c.allowedChans[ch] = true
			}
		}
	}

	c.eventsAddr = config["events_addr"]
	if c.eventsAddr == "" {
		c.eventsAddr = ":3101"
	}

	c.httpClient = &http.Client{Timeout: 30 * time.Second}

	// Verify bot token with auth.test.
	var authResp authTestResponse
	if err := c.apiCall(ctx, "auth.test", nil, &authResp); err != nil {
		return fmt.Errorf("slack connector: auth.test failed: %w", err)
	}
	if !authResp.OK {
		return fmt.Errorf("slack connector: auth.test returned error: %s", authResp.Error)
	}

	c.botUserID = authResp.UserID
	c.teamName = authResp.Team
	c.connected = true
	c.done = make(chan struct{})

	return nil
}

// Disconnect cleanly shuts down the Slack connector.
func (c *Connector) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false
	close(c.done)

	if c.eventsServer != nil {
		if err := c.eventsServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("slack connector: error shutting down events server: %w", err)
		}
		c.eventsServer = nil
	}

	return nil
}

// Health checks connectivity by calling auth.test.
func (c *Connector) Health(ctx context.Context) connectors.HealthState {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return connectors.HealthDisconnected
	}

	var authResp authTestResponse
	if err := c.apiCall(ctx, "auth.test", nil, &authResp); err != nil {
		return connectors.HealthDegraded
	}

	return connectors.HealthHealthy
}

// Send delivers a message to a Slack channel via chat.postMessage.
//
// Expected metadata keys:
//   - "channel" : target channel ID (falls back to default_channel)
//   - "thread_ts" : optional, for thread replies
//   - "file_url" : optional, URL for file sharing
//   - "reaction" : optional, emoji name to react with (requires "timestamp")
//   - "timestamp" : optional, message ts to react to
func (c *Connector) Send(ctx context.Context, msg *connectors.Message) error {
	channel := msg.Metadata["channel"]
	if channel == "" {
		c.mu.RLock()
		channel = c.defaultChannel
		c.mu.RUnlock()
	}
	if channel == "" {
		return fmt.Errorf("slack connector: 'channel' metadata or default_channel is required")
	}

	// Security: only send to allowed channels.
	c.mu.RLock()
	allowed := len(c.allowedChans) == 0 || c.allowedChans[channel]
	c.mu.RUnlock()

	if !allowed {
		return fmt.Errorf("slack connector: channel %q is not in the allowed list", channel)
	}

	// Handle reaction adds.
	if reaction := msg.Metadata["reaction"]; reaction != "" {
		ts := msg.Metadata["timestamp"]
		if ts == "" {
			return fmt.Errorf("slack connector: 'timestamp' metadata is required for reactions")
		}
		params := map[string]interface{}{
			"channel":   channel,
			"name":      reaction,
			"timestamp": ts,
		}
		var resp slackResponse
		return c.apiCall(ctx, "reactions.add", params, &resp)
	}

	params := map[string]interface{}{
		"channel": channel,
		"text":    msg.Content,
	}
	if threadTS := msg.Metadata["thread_ts"]; threadTS != "" {
		params["thread_ts"] = threadTS
	}

	var resp chatPostResponse
	return c.apiCall(ctx, "chat.postMessage", params, &resp)
}

// Receive returns a channel of inbound messages by starting an HTTP server
// that listens for Slack Events API callbacks.
func (c *Connector) Receive(ctx context.Context) (<-chan *connectors.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("slack connector: not connected")
	}

	c.msgCh = make(chan *connectors.Message, 64)

	mux := http.NewServeMux()
	mux.HandleFunc("/slack/events", c.handleEvents)

	c.eventsServer = &http.Server{
		Addr:              c.eventsAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := c.eventsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error; in production this would use structured logging.
			_ = err
		}
	}()

	go func() {
		select {
		case <-ctx.Done():
		case <-c.done:
		}
		close(c.msgCh)
	}()

	return c.msgCh, nil
}

// handleEvents processes Slack Events API HTTP callbacks.
func (c *Connector) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var payload eventsAPIPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Handle URL verification challenge.
	if payload.Type == "url_verification" {
		w.Header().Set("Content-Type", "application/json")
		resp, _ := json.Marshal(map[string]string{"challenge": payload.Challenge})
		w.Write(resp)
		return
	}

	// Process event callbacks.
	if payload.Type == "event_callback" && payload.Event != nil {
		var evt slackEvent
		if err := json.Unmarshal(payload.Event, &evt); err != nil {
			http.Error(w, "invalid event", http.StatusBadRequest)
			return
		}

		// Skip bot's own messages.
		c.mu.RLock()
		botUserID := c.botUserID
		allowedChans := c.allowedChans
		c.mu.RUnlock()

		if evt.User == botUserID || evt.SubType == "bot_message" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Security: only accept from allowed channels.
		if len(allowedChans) > 0 && !allowedChans[evt.Channel] {
			w.WriteHeader(http.StatusOK)
			return
		}

		normalized := c.normalizeEvent(&evt)
		if normalized != nil {
			c.mu.RLock()
			ch := c.msgCh
			c.mu.RUnlock()

			if ch != nil {
				select {
				case ch <- normalized:
				default:
					// Drop if channel is full.
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// normalizeEvent converts a Slack event into a connector.Message.
func (c *Connector) normalizeEvent(evt *slackEvent) *connectors.Message {
	if evt.Type != "message" && evt.Type != "reaction_added" {
		return nil
	}

	content := evt.Text
	if evt.Type == "reaction_added" {
		content = fmt.Sprintf("reaction:%s", evt.Reaction)
	}

	metadata := map[string]string{
		"channel":    evt.Channel,
		"user":       evt.User,
		"event_type": evt.Type,
		"ts":         evt.TS,
	}
	if evt.ThreadTS != "" {
		metadata["thread_ts"] = evt.ThreadTS
	}
	if evt.FileID != "" {
		metadata["file_id"] = evt.FileID
	}

	return &connectors.Message{
		ID:          fmt.Sprintf("slack_%s_%s", evt.Channel, evt.TS),
		ConnectorID: "",
		Type:        connectors.TypeSlack,
		Direction:   connectors.Inbound,
		Content:     content,
		Metadata:    metadata,
		Timestamp:   parseSlackTS(evt.TS),
	}
}

// parseSlackTS parses a Slack timestamp ("1234567890.123456") into time.Time.
func parseSlackTS(ts string) time.Time {
	parts := strings.SplitN(ts, ".", 2)
	if len(parts) == 0 {
		return time.Now()
	}
	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Now()
	}
	return time.Unix(sec, 0)
}

// Capabilities returns the features supported by the Slack connector.
func (c *Connector) Capabilities() []string {
	return []string{"send", "receive", "threads", "reactions", "file_upload"}
}

// apiCall makes a request to the Slack Web API and decodes the result.
// It respects rate-limit headers (Retry-After) by waiting and retrying once.
func (c *Connector) apiCall(ctx context.Context, method string, params map[string]interface{}, dest interface{}) error {
	c.mu.RLock()
	url := fmt.Sprintf("%s/%s", c.apiBase, method)
	token := c.botToken
	c.mu.RUnlock()

	return c.apiCallWithRetry(ctx, url, token, params, dest, 1)
}

func (c *Connector) apiCallWithRetry(ctx context.Context, url, token string, params map[string]interface{}, dest interface{}, retriesLeft int) error {
	var body io.Reader
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("slack connector: marshal params: %w", err)
		}
		body = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("slack connector: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack connector: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Rate-limit awareness: respect Retry-After header.
	if resp.StatusCode == http.StatusTooManyRequests && retriesLeft > 0 {
		retryAfter := 1
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if parsed, err := strconv.Atoi(ra); err == nil && parsed > 0 {
				retryAfter = parsed
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(retryAfter) * time.Second):
		}
		return c.apiCallWithRetry(ctx, url, token, params, dest, retriesLeft-1)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("slack connector: rate limited (429), retry later")
	}

	if dest != nil {
		if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
			return fmt.Errorf("slack connector: decode response: %w", err)
		}
	}

	return nil
}

// Compile-time interface compliance check.
var _ connectors.Connector = (*Connector)(nil)
