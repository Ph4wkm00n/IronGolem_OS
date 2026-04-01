// Package whatsapp implements the IronGolem OS connector for the WhatsApp
// Business API, supporting text and template messages with webhook-based
// message reception.
package whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	defaultAPIBase = "https://graph.facebook.com/v18.0"
)

// Connector implements connectors.Connector for the WhatsApp Business API.
type Connector struct {
	mu sync.RWMutex

	phoneNumberID  string
	accessToken    string
	verifyToken    string
	appSecret      string
	allowedNumbers map[string]bool
	apiBase        string

	httpClient *http.Client

	connected    bool
	msgCh        chan *connectors.Message
	done         chan struct{}
	webhookAddr  string
	webhookServer *http.Server
}

// --- WhatsApp API types ---

type waMessagePayload struct {
	MessagingProduct string      `json:"messaging_product"`
	RecipientType    string      `json:"recipient_type"`
	To               string      `json:"to"`
	Type             string      `json:"type"`
	Text             *waText     `json:"text,omitempty"`
	Template         *waTemplate `json:"template,omitempty"`
}

type waText struct {
	PreviewURL bool   `json:"preview_url"`
	Body       string `json:"body"`
}

type waTemplate struct {
	Name     string         `json:"name"`
	Language waLanguage     `json:"language"`
}

type waLanguage struct {
	Code string `json:"code"`
}

type waAPIResponse struct {
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages,omitempty"`
	Error *waAPIError `json:"error,omitempty"`
}

type waAPIError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Webhook payload types.
type waWebhookPayload struct {
	Object string    `json:"object"`
	Entry  []waEntry `json:"entry"`
}

type waEntry struct {
	ID      string     `json:"id"`
	Changes []waChange `json:"changes"`
}

type waChange struct {
	Value waChangeValue `json:"value"`
	Field string        `json:"field"`
}

type waChangeValue struct {
	MessagingProduct string      `json:"messaging_product"`
	Metadata         waMetadata  `json:"metadata"`
	Messages         []waMessage `json:"messages,omitempty"`
	Contacts         []waContact `json:"contacts,omitempty"`
}

type waMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type waMessage struct {
	From      string    `json:"from"`
	ID        string    `json:"id"`
	Timestamp string    `json:"timestamp"`
	Type      string    `json:"type"`
	Text      *waText   `json:"text,omitempty"`
}

type waContact struct {
	WaID    string    `json:"wa_id"`
	Profile waProfile `json:"profile"`
}

type waProfile struct {
	Name string `json:"name"`
}

// Type returns the connector type identifier.
func (c *Connector) Type() connectors.ConnectorType {
	return connectors.TypeWhatsApp
}

// Connect verifies credentials with the WhatsApp Business API and stores
// configuration.
func (c *Connector) Connect(ctx context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.phoneNumberID = config["phone_number_id"]
	if c.phoneNumberID == "" {
		return fmt.Errorf("whatsapp connector: phone_number_id is required")
	}

	c.accessToken = config["access_token"]
	if c.accessToken == "" {
		return fmt.Errorf("whatsapp connector: access_token is required")
	}

	c.verifyToken = config["verify_token"]
	if c.verifyToken == "" {
		return fmt.Errorf("whatsapp connector: verify_token is required")
	}

	c.appSecret = config["app_secret"]

	c.apiBase = defaultAPIBase
	if base := config["api_base"]; base != "" {
		c.apiBase = base
	}

	// Parse allowed_numbers as a comma-separated list.
	c.allowedNumbers = make(map[string]bool)
	if nums := config["allowed_numbers"]; nums != "" {
		for _, n := range strings.Split(nums, ",") {
			n = strings.TrimSpace(n)
			if n != "" {
				c.allowedNumbers[n] = true
			}
		}
	}

	c.webhookAddr = config["webhook_addr"]
	if c.webhookAddr == "" {
		c.webhookAddr = ":3102"
	}

	c.httpClient = &http.Client{Timeout: 30 * time.Second}

	// Verify credentials by fetching the phone number info.
	url := fmt.Sprintf("%s/%s", c.apiBase, c.phoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("whatsapp connector: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp connector: credential verification failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("whatsapp connector: API error %d: %s", resp.StatusCode, string(body))
	}

	c.connected = true
	c.done = make(chan struct{})

	return nil
}

// Disconnect cleanly shuts down the WhatsApp connector.
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
			return fmt.Errorf("whatsapp connector: error shutting down webhook server: %w", err)
		}
		c.webhookServer = nil
	}

	return nil
}

// Health checks connectivity by verifying the access token.
func (c *Connector) Health(ctx context.Context) connectors.HealthState {
	c.mu.RLock()
	connected := c.connected
	phoneNumberID := c.phoneNumberID
	accessToken := c.accessToken
	apiBase := c.apiBase
	c.mu.RUnlock()

	if !connected {
		return connectors.HealthDisconnected
	}

	url := fmt.Sprintf("%s/%s", apiBase, phoneNumberID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return connectors.HealthDegraded
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return connectors.HealthDegraded
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return connectors.HealthExpired
	}

	if resp.StatusCode >= 400 {
		return connectors.HealthDegraded
	}

	return connectors.HealthHealthy
}

// Send delivers a message via the WhatsApp Business API.
//
// Expected metadata keys:
//   - "to"            : recipient phone number (required)
//   - "type"          : "text" (default) or "template"
//   - "template_name" : template name (required if type is "template")
//   - "language_code" : template language code (default "en_US")
func (c *Connector) Send(ctx context.Context, msg *connectors.Message) error {
	to := msg.Metadata["to"]
	if to == "" {
		return fmt.Errorf("whatsapp connector: 'to' metadata is required")
	}

	// Security: only send to allowed numbers.
	c.mu.RLock()
	allowed := len(c.allowedNumbers) == 0 || c.allowedNumbers[to]
	c.mu.RUnlock()

	if !allowed {
		return fmt.Errorf("whatsapp connector: number %q is not in the allowed list", to)
	}

	msgType := msg.Metadata["type"]
	if msgType == "" {
		msgType = "text"
	}

	payload := waMessagePayload{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               to,
		Type:             msgType,
	}

	switch msgType {
	case "text":
		payload.Text = &waText{
			PreviewURL: false,
			Body:       msg.Content,
		}
	case "template":
		templateName := msg.Metadata["template_name"]
		if templateName == "" {
			return fmt.Errorf("whatsapp connector: 'template_name' metadata is required for template messages")
		}
		langCode := msg.Metadata["language_code"]
		if langCode == "" {
			langCode = "en_US"
		}
		payload.Template = &waTemplate{
			Name:     templateName,
			Language: waLanguage{Code: langCode},
		}
	default:
		return fmt.Errorf("whatsapp connector: unsupported message type %q", msgType)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("whatsapp connector: marshal payload: %w", err)
	}

	c.mu.RLock()
	url := fmt.Sprintf("%s/%s/messages", c.apiBase, c.phoneNumberID)
	token := c.accessToken
	c.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("whatsapp connector: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("whatsapp connector: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("whatsapp connector: API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Receive returns a channel of inbound messages by starting an HTTP server
// that handles WhatsApp webhook callbacks.
func (c *Connector) Receive(ctx context.Context) (<-chan *connectors.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("whatsapp connector: not connected")
	}

	c.msgCh = make(chan *connectors.Message, 64)

	mux := http.NewServeMux()
	mux.HandleFunc("/whatsapp/webhook", c.handleWebhook)

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

// handleWebhook processes WhatsApp webhook HTTP callbacks including
// verification and incoming messages.
func (c *Connector) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Handle webhook verification (GET request).
	if r.Method == http.MethodGet {
		mode := r.URL.Query().Get("hub.mode")
		token := r.URL.Query().Get("hub.verify_token")
		challenge := r.URL.Query().Get("hub.challenge")

		c.mu.RLock()
		verifyToken := c.verifyToken
		c.mu.RUnlock()

		if mode == "subscribe" && token == verifyToken {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(challenge))
			return
		}

		http.Error(w, "verification failed", http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	// Verify webhook signature if app_secret is configured.
	c.mu.RLock()
	appSecret := c.appSecret
	c.mu.RUnlock()

	if appSecret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if !c.verifySignature(body, signature, appSecret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	var payload waWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Process messages.
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if change.Field != "messages" {
				continue
			}
			for _, msg := range change.Value.Messages {
				c.processInboundMessage(&msg, &change.Value)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// processInboundMessage normalizes a WhatsApp message and sends it on the
// message channel.
func (c *Connector) processInboundMessage(msg *waMessage, value *waChangeValue) {
	// Security: only accept messages from allowed numbers.
	c.mu.RLock()
	allowed := len(c.allowedNumbers) == 0 || c.allowedNumbers[msg.From]
	ch := c.msgCh
	c.mu.RUnlock()

	if !allowed || ch == nil {
		return
	}

	content := ""
	if msg.Text != nil {
		content = msg.Text.Body
	}

	// Find contact name.
	contactName := ""
	for _, contact := range value.Contacts {
		if contact.WaID == msg.From {
			contactName = contact.Profile.Name
			break
		}
	}

	normalized := &connectors.Message{
		ID:          fmt.Sprintf("wa_%s", msg.ID),
		ConnectorID: "",
		Type:        connectors.TypeWhatsApp,
		Direction:   connectors.Inbound,
		Content:     content,
		Metadata: map[string]string{
			"from":         msg.From,
			"message_type": msg.Type,
			"message_id":   msg.ID,
			"contact_name": contactName,
			"phone_number": value.Metadata.DisplayPhoneNumber,
		},
		Timestamp: parseTimestamp(msg.Timestamp),
	}

	select {
	case ch <- normalized:
	default:
		// Drop if channel is full.
	}
}

// verifySignature validates the X-Hub-Signature-256 header against the payload.
func (c *Connector) verifySignature(body []byte, signature, secret string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	expectedSig := signature[7:]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	computedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSig), []byte(computedSig))
}

// parseTimestamp parses a Unix timestamp string into time.Time.
func parseTimestamp(ts string) time.Time {
	// WhatsApp timestamps are Unix epoch seconds as strings.
	var sec int64
	for _, ch := range ts {
		if ch >= '0' && ch <= '9' {
			sec = sec*10 + int64(ch-'0')
		}
	}
	if sec == 0 {
		return time.Now()
	}
	return time.Unix(sec, 0)
}

// Capabilities returns the features supported by the WhatsApp connector.
func (c *Connector) Capabilities() []string {
	return []string{"send", "receive", "templates"}
}

// Compile-time interface compliance check.
var _ connectors.Connector = (*Connector)(nil)
