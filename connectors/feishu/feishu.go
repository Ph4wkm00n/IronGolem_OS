// Package feishu implements the IronGolem OS connector for Feishu/Lark,
// supporting text, rich text (post), and interactive card messages with
// automatic tenant access token refresh.
package feishu

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
	defaultAPIBase       = "https://open.feishu.cn/open-apis"
	tokenRefreshMargin   = 5 * time.Minute
	tokenRefreshInterval = 30 * time.Minute
)

// Connector implements connectors.Connector for Feishu/Lark.
type Connector struct {
	mu sync.RWMutex

	appID             string
	appSecret         string
	verificationToken string
	apiBase           string

	tenantAccessToken string
	tokenExpiry       time.Time

	httpClient *http.Client

	connected     bool
	msgCh         chan *connectors.Message
	done          chan struct{}
	webhookAddr   string
	webhookServer *http.Server
}

// --- Feishu API types ---

type tenantTokenRequest struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

type tenantTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"` // seconds
}

type sendMessageRequest struct {
	ReceiveID string `json:"receive_id"`
	MsgType   string `json:"msg_type"`
	Content   string `json:"content"`
}

type feishuAPIResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Event subscription types.
type feishuEventPayload struct {
	Schema    string          `json:"schema,omitempty"`
	Header    *feishuHeader   `json:"header,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
	Challenge string          `json:"challenge,omitempty"`
	Token     string          `json:"token,omitempty"`
	Type      string          `json:"type,omitempty"`
}

type feishuHeader struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	Token     string `json:"token"`
	CreateTime string `json:"create_time"`
}

type feishuMessageEvent struct {
	Sender  feishuSender  `json:"sender"`
	Message feishuMessage `json:"message"`
}

type feishuSender struct {
	SenderID   feishuSenderID `json:"sender_id"`
	SenderType string         `json:"sender_type"`
}

type feishuSenderID struct {
	OpenID  string `json:"open_id"`
	UserID  string `json:"user_id,omitempty"`
	UnionID string `json:"union_id,omitempty"`
}

type feishuMessage struct {
	MessageID   string `json:"message_id"`
	RootID      string `json:"root_id,omitempty"`
	ParentID    string `json:"parent_id,omitempty"`
	ChatID      string `json:"chat_id"`
	ChatType    string `json:"chat_type"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
	CreateTime  string `json:"create_time"`
}

type feishuTextContent struct {
	Text string `json:"text"`
}

// Type returns the connector type identifier.
func (c *Connector) Type() connectors.ConnectorType {
	return connectors.TypeFeishu
}

// Connect obtains a tenant access token and verifies connectivity.
func (c *Connector) Connect(ctx context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.appID = config["app_id"]
	if c.appID == "" {
		return fmt.Errorf("feishu connector: app_id is required")
	}

	c.appSecret = config["app_secret"]
	if c.appSecret == "" {
		return fmt.Errorf("feishu connector: app_secret is required")
	}

	c.verificationToken = config["verification_token"]

	c.apiBase = defaultAPIBase
	if base := config["api_base"]; base != "" {
		c.apiBase = base
	}

	c.webhookAddr = config["webhook_addr"]
	if c.webhookAddr == "" {
		c.webhookAddr = ":3103"
	}

	c.httpClient = &http.Client{Timeout: 30 * time.Second}

	// Obtain initial tenant access token.
	if err := c.refreshToken(ctx); err != nil {
		return fmt.Errorf("feishu connector: failed to obtain tenant access token: %w", err)
	}

	c.connected = true
	c.done = make(chan struct{})

	// Start background token refresh.
	go c.tokenRefreshLoop(ctx)

	return nil
}

// Disconnect cleanly shuts down the Feishu connector.
func (c *Connector) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false
	close(c.done)

	if c.webhookServer != nil {
		if err := c.webhookServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("feishu connector: error shutting down webhook server: %w", err)
		}
		c.webhookServer = nil
	}

	return nil
}

// Health checks connectivity by verifying the tenant access token is valid.
func (c *Connector) Health(ctx context.Context) connectors.HealthState {
	c.mu.RLock()
	connected := c.connected
	tokenExpiry := c.tokenExpiry
	c.mu.RUnlock()

	if !connected {
		return connectors.HealthDisconnected
	}

	if time.Now().After(tokenExpiry) {
		return connectors.HealthExpired
	}

	return connectors.HealthHealthy
}

// Send delivers a message via the Feishu /im/v1/messages API.
//
// Expected metadata keys:
//   - "receive_id"      : recipient ID (required)
//   - "receive_id_type" : "open_id" (default), "user_id", "union_id", "email", or "chat_id"
//   - "msg_type"        : "text" (default), "post", or "interactive"
func (c *Connector) Send(ctx context.Context, msg *connectors.Message) error {
	receiveID := msg.Metadata["receive_id"]
	if receiveID == "" {
		return fmt.Errorf("feishu connector: 'receive_id' metadata is required")
	}

	receiveIDType := msg.Metadata["receive_id_type"]
	if receiveIDType == "" {
		receiveIDType = "open_id"
	}

	msgType := msg.Metadata["msg_type"]
	if msgType == "" {
		msgType = "text"
	}

	// Build content based on message type.
	var content string
	switch msgType {
	case "text":
		textContent := feishuTextContent{Text: msg.Content}
		data, err := json.Marshal(textContent)
		if err != nil {
			return fmt.Errorf("feishu connector: marshal text content: %w", err)
		}
		content = string(data)
	case "post", "interactive":
		// For post and interactive types, content is passed as-is (pre-formatted JSON).
		content = msg.Content
	default:
		return fmt.Errorf("feishu connector: unsupported message type %q", msgType)
	}

	payload := sendMessageRequest{
		ReceiveID: receiveID,
		MsgType:   msgType,
		Content:   content,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("feishu connector: marshal payload: %w", err)
	}

	c.mu.RLock()
	url := fmt.Sprintf("%s/im/v1/messages?receive_id_type=%s", c.apiBase, receiveIDType)
	token := c.tenantAccessToken
	c.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("feishu connector: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("feishu connector: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	var apiResp feishuAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return fmt.Errorf("feishu connector: decode response: %w", err)
	}

	if apiResp.Code != 0 {
		return fmt.Errorf("feishu connector: API error (code %d): %s", apiResp.Code, apiResp.Msg)
	}

	return nil
}

// Receive returns a channel of inbound messages by starting an HTTP server
// that handles Feishu event subscription webhooks.
func (c *Connector) Receive(ctx context.Context) (<-chan *connectors.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("feishu connector: not connected")
	}

	c.msgCh = make(chan *connectors.Message, 64)

	mux := http.NewServeMux()
	mux.HandleFunc("/feishu/events", c.handleEvents)

	c.webhookServer = &http.Server{
		Addr:              c.webhookAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := c.webhookServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

// handleEvents processes Feishu event subscription HTTP callbacks.
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

	var payload feishuEventPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Handle URL verification challenge (v1 schema).
	if payload.Type == "url_verification" {
		c.mu.RLock()
		verifyToken := c.verificationToken
		c.mu.RUnlock()

		if verifyToken != "" && payload.Token != verifyToken {
			http.Error(w, "token mismatch", http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		resp, _ := json.Marshal(map[string]string{"challenge": payload.Challenge})
		w.Write(resp)
		return
	}

	// Verify event token (v2 schema).
	if payload.Header != nil {
		c.mu.RLock()
		verifyToken := c.verificationToken
		c.mu.RUnlock()

		if verifyToken != "" && payload.Header.Token != verifyToken {
			http.Error(w, "token mismatch", http.StatusForbidden)
			return
		}
	}

	// Process message events.
	if payload.Header != nil && payload.Event != nil {
		switch payload.Header.EventType {
		case "im.message.receive_v1":
			var evt feishuMessageEvent
			if err := json.Unmarshal(payload.Event, &evt); err == nil {
				c.processMessageEvent(&evt, payload.Header.EventID)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// processMessageEvent normalizes a Feishu message event and sends it on the
// message channel.
func (c *Connector) processMessageEvent(evt *feishuMessageEvent, eventID string) {
	c.mu.RLock()
	ch := c.msgCh
	c.mu.RUnlock()

	if ch == nil {
		return
	}

	// Extract text content from the message.
	content := evt.Message.Content
	if evt.Message.MessageType == "text" {
		var textContent feishuTextContent
		if err := json.Unmarshal([]byte(content), &textContent); err == nil {
			content = textContent.Text
		}
	}

	normalized := &connectors.Message{
		ID:          fmt.Sprintf("feishu_%s", evt.Message.MessageID),
		ConnectorID: "",
		Type:        connectors.TypeFeishu,
		Direction:   connectors.Inbound,
		Content:     content,
		Metadata: map[string]string{
			"message_id":   evt.Message.MessageID,
			"chat_id":      evt.Message.ChatID,
			"chat_type":    evt.Message.ChatType,
			"message_type": evt.Message.MessageType,
			"sender_id":    evt.Sender.SenderID.OpenID,
			"sender_type":  evt.Sender.SenderType,
			"event_id":     eventID,
		},
		Timestamp: parseFeishuTimestamp(evt.Message.CreateTime),
	}

	if evt.Message.RootID != "" {
		normalized.Metadata["root_id"] = evt.Message.RootID
	}
	if evt.Message.ParentID != "" {
		normalized.Metadata["parent_id"] = evt.Message.ParentID
	}

	select {
	case ch <- normalized:
	default:
		// Drop if channel is full.
	}
}

// refreshToken obtains or refreshes the tenant access token.
func (c *Connector) refreshToken(ctx context.Context) error {
	payload := tenantTokenRequest{
		AppID:     c.appID,
		AppSecret: c.appSecret,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal token request: %w", err)
	}

	url := fmt.Sprintf("%s/auth/v3/tenant_access_token/internal", c.apiBase)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp tenantTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if tokenResp.Code != 0 {
		return fmt.Errorf("API error (code %d): %s", tokenResp.Code, tokenResp.Msg)
	}

	c.tenantAccessToken = tokenResp.TenantAccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.Expire) * time.Second)

	return nil
}

// tokenRefreshLoop periodically refreshes the tenant access token before
// it expires.
func (c *Connector) tokenRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(tokenRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			c.mu.Lock()
			needsRefresh := time.Until(c.tokenExpiry) < tokenRefreshMargin
			c.mu.Unlock()

			if needsRefresh {
				c.mu.Lock()
				_ = c.refreshToken(ctx)
				c.mu.Unlock()
			}
		}
	}
}

// parseFeishuTimestamp parses a Feishu timestamp (milliseconds as string)
// into time.Time.
func parseFeishuTimestamp(ts string) time.Time {
	var ms int64
	for _, ch := range ts {
		if ch >= '0' && ch <= '9' {
			ms = ms*10 + int64(ch-'0')
		}
	}
	if ms == 0 {
		return time.Now()
	}
	return time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond))
}

// Capabilities returns the features supported by the Feishu connector.
func (c *Connector) Capabilities() []string {
	return []string{"send", "receive", "rich_text", "cards"}
}

// Compile-time interface compliance check.
var _ connectors.Connector = (*Connector)(nil)
