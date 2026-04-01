// Package telegram implements the IronGolem OS connector for the Telegram Bot API.
package telegram

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
	defaultAPIBase    = "https://api.telegram.org"
	pollTimeout       = 30 // seconds, for long-polling getUpdates
	minPollIntervalMs = 500
)

// Connector implements connectors.Connector for the Telegram Bot API.
type Connector struct {
	mu sync.RWMutex

	botToken       string
	allowedChatIDs map[int64]bool
	apiBase        string

	httpClient *http.Client
	botID      int64
	botName    string

	connected bool
	lastUpdate int64 // offset for getUpdates
	msgCh      chan *connectors.Message
	done       chan struct{}
}

// --- Telegram API response types ---

type apiResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

type botUser struct {
	ID       int64  `json:"id"`
	IsBot    bool   `json:"is_bot"`
	Username string `json:"username"`
}

type update struct {
	UpdateID int64    `json:"update_id"`
	Message  *tgMessage `json:"message,omitempty"`
}

type tgMessage struct {
	MessageID int64  `json:"message_id"`
	From      *botUser `json:"from,omitempty"`
	Chat      tgChat `json:"chat"`
	Date      int64  `json:"date"`
	Text      string `json:"text"`
}

type tgChat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title,omitempty"`
}

// Type returns the connector type identifier.
func (c *Connector) Type() connectors.ConnectorType {
	return connectors.TypeTelegram
}

// Connect verifies the bot token by calling getMe and stores allowed chat IDs.
func (c *Connector) Connect(ctx context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.botToken = config["bot_token"]
	if c.botToken == "" {
		return fmt.Errorf("telegram connector: bot_token is required")
	}

	c.apiBase = defaultAPIBase
	if base := config["api_base"]; base != "" {
		c.apiBase = base
	}

	// Parse allowed_chat_ids as a comma-separated list of integers.
	c.allowedChatIDs = make(map[int64]bool)
	if ids := config["allowed_chat_ids"]; ids != "" {
		for _, raw := range strings.Split(ids, ",") {
			raw = strings.TrimSpace(raw)
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return fmt.Errorf("telegram connector: invalid chat ID %q: %w", raw, err)
			}
			c.allowedChatIDs[id] = true
		}
	}

	c.httpClient = &http.Client{Timeout: time.Duration(pollTimeout+10) * time.Second}

	// Verify the bot token with getMe.
	var bot botUser
	if err := c.apiCall(ctx, "getMe", nil, &bot); err != nil {
		return fmt.Errorf("telegram connector: getMe failed: %w", err)
	}

	c.botID = bot.ID
	c.botName = bot.Username
	c.connected = true
	c.done = make(chan struct{})

	return nil
}

// Disconnect cleanly shuts down the Telegram connector.
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

// Health checks bot connectivity by calling getMe.
func (c *Connector) Health(ctx context.Context) connectors.HealthState {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return connectors.HealthDisconnected
	}

	var bot botUser
	if err := c.apiCall(ctx, "getMe", nil, &bot); err != nil {
		return connectors.HealthDegraded
	}

	return connectors.HealthHealthy
}

// Send delivers a message to a Telegram chat via the sendMessage API.
//
// Expected metadata keys:
//   - "chat_id" : target chat ID (required)
//   - "parse_mode" : optional, e.g. "Markdown" or "HTML"
func (c *Connector) Send(ctx context.Context, msg *connectors.Message) error {
	chatIDStr := msg.Metadata["chat_id"]
	if chatIDStr == "" {
		return fmt.Errorf("telegram connector: 'chat_id' metadata is required")
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram connector: invalid chat_id: %w", err)
	}

	// Security: only send to allowed chats.
	c.mu.RLock()
	allowed := len(c.allowedChatIDs) == 0 || c.allowedChatIDs[chatID]
	c.mu.RUnlock()

	if !allowed {
		return fmt.Errorf("telegram connector: chat %d is not in the allowed list", chatID)
	}

	params := map[string]interface{}{
		"chat_id": chatID,
		"text":    msg.Content,
	}
	if pm := msg.Metadata["parse_mode"]; pm != "" {
		params["parse_mode"] = pm
	}

	var result json.RawMessage
	return c.apiCall(ctx, "sendMessage", params, &result)
}

// Receive returns a channel of inbound messages using long-polling (getUpdates).
func (c *Connector) Receive(ctx context.Context) (<-chan *connectors.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("telegram connector: not connected")
	}

	c.msgCh = make(chan *connectors.Message, 64)

	go c.pollUpdates(ctx)

	return c.msgCh, nil
}

// pollUpdates long-polls the Telegram getUpdates endpoint.
func (c *Connector) pollUpdates(ctx context.Context) {
	defer close(c.msgCh)

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		default:
		}

		c.mu.RLock()
		offset := c.lastUpdate + 1
		c.mu.RUnlock()

		params := map[string]interface{}{
			"offset":  offset,
			"timeout": pollTimeout,
		}

		var updates []update
		if err := c.apiCall(ctx, "getUpdates", params, &updates); err != nil {
			// Rate-limit awareness: back off on errors.
			select {
			case <-ctx.Done():
				return
			case <-c.done:
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		for _, u := range updates {
			c.mu.Lock()
			if u.UpdateID >= c.lastUpdate {
				c.lastUpdate = u.UpdateID
			}
			c.mu.Unlock()

			if u.Message == nil {
				continue
			}

			// Security: only accept messages from allowed chats.
			c.mu.RLock()
			allowed := len(c.allowedChatIDs) == 0 || c.allowedChatIDs[u.Message.Chat.ID]
			c.mu.RUnlock()

			if !allowed {
				continue
			}

			normalized := &connectors.Message{
				ID:          fmt.Sprintf("tg_%d_%d", u.Message.Chat.ID, u.Message.MessageID),
				ConnectorID: "",
				Type:        connectors.TypeTelegram,
				Direction:   connectors.Inbound,
				Content:     u.Message.Text,
				Metadata: map[string]string{
					"chat_id":    strconv.FormatInt(u.Message.Chat.ID, 10),
					"chat_type":  u.Message.Chat.Type,
					"message_id": strconv.FormatInt(u.Message.MessageID, 10),
				},
				Timestamp: time.Unix(u.Message.Date, 0),
			}

			if u.Message.From != nil {
				normalized.Metadata["from_id"] = strconv.FormatInt(u.Message.From.ID, 10)
				normalized.Metadata["from_username"] = u.Message.From.Username
			}

			select {
			case c.msgCh <- normalized:
			case <-ctx.Done():
				return
			case <-c.done:
				return
			}
		}
	}
}

// Capabilities returns the features supported by the Telegram connector.
func (c *Connector) Capabilities() []string {
	return []string{"send", "receive", "groups"}
}

// apiCall makes a request to the Telegram Bot API and decodes the result.
func (c *Connector) apiCall(ctx context.Context, method string, params map[string]interface{}, dest interface{}) error {
	c.mu.RLock()
	url := fmt.Sprintf("%s/bot%s/%s", c.apiBase, c.botToken, method)
	c.mu.RUnlock()

	var body io.Reader
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("telegram connector: marshal params: %w", err)
		}
		body = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("telegram connector: build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram connector: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Rate-limit awareness: Telegram returns 429 on throttling.
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("telegram connector: rate limited (429), retry later")
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("telegram connector: decode response: %w", err)
	}

	if !apiResp.OK {
		return fmt.Errorf("telegram connector: API error: %s", apiResp.Description)
	}

	if dest != nil && apiResp.Result != nil {
		if err := json.Unmarshal(apiResp.Result, dest); err != nil {
			return fmt.Errorf("telegram connector: unmarshal result: %w", err)
		}
	}

	return nil
}

// Compile-time interface compliance check.
var _ connectors.Connector = (*Connector)(nil)
