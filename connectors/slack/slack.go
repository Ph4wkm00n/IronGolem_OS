// Package slack implements the IronGolem OS connector for the Slack
// Web API (chat.postMessage for outbound) and the Events API (inbound).
//
// v0.3 Step 8 of Plans/modular-puzzling-blum.md. Mirrors the structure
// of `connectors/telegram` to keep the connector adapter pattern
// uniform across channels.
//
// v0.3 shipped the OUTBOUND path (Send → chat.postMessage) end-to-end so
// the commitments runtime can fire reminders into Slack. v0.4 adds the
// INBOUND path via Socket Mode — see inbound.go. Inbound requires an
// app-level token (xapp-...); without one the connector stays
// outbound-only exactly as in v0.3.
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
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
	appToken      string // app-level token (xapp-...) for Socket Mode inbound; optional
	apiBase       string
	signingSecret string // reserved for Events API verification (unused with Socket Mode)
	httpClient    *http.Client
	selfUserID    string // our bot's user id from auth.test; used to ignore self-messages

	connected bool
	msgCh     chan *connectors.Message
	done      chan struct{}

	// receiveStarted guards the once-only inbound startup (worker spawn
	// or the one-time "inbound disabled" log). closeMsgOnce guarantees
	// msgCh closes exactly once whether the worker or Disconnect gets
	// there first.
	receiveStarted bool
	workerOwnsCh   bool
	closeMsgOnce   sync.Once
}

// Type returns the canonical connector identifier.
func (c *Connector) Type() connectors.ConnectorType {
	return connectors.TypeSlack
}

// Connect verifies the bot token via auth.test and stores config.
//
// config["app_token"] (fallback: IRONGOLEM_SLACK_APP_TOKEN) enables
// Socket Mode inbound. It is optional — outbound-only remains a fully
// supported configuration.
func (c *Connector) Connect(ctx context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.botToken = config["bot_token"]
	c.signingSecret = config["signing_secret"]
	c.appToken = config["app_token"]
	if c.appToken == "" {
		c.appToken = os.Getenv(envAppToken)
	}
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
	c.receiveStarted = false
	c.workerOwnsCh = false
	c.closeMsgOnce = sync.Once{}
	c.connected = true
	return nil
}

// Disconnect signals shutdown. If the Socket Mode worker is running it
// owns closing msgCh (it may still be mid-send); otherwise we close it
// here so the gateway pump observes the shutdown either way.
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

// Receive returns the inbound message channel and, when an app-level
// token is configured, starts the long-lived Socket Mode worker
// (connectors.StartWorker: context cancellation, panic recovery,
// exponential-backoff reconnect). Without an app token inbound stays
// silent — logged once — and the connector remains outbound-only,
// exactly as in v0.3.
func (c *Connector) Receive(ctx context.Context) (<-chan *connectors.Message, error) {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return nil, fmt.Errorf("slack: not connected")
	}
	msgCh := c.msgCh
	if c.receiveStarted {
		c.mu.Unlock()
		return msgCh, nil
	}
	c.receiveStarted = true

	if c.appToken == "" {
		c.mu.Unlock()
		slog.Info("slack: inbound disabled — no app-level token configured; connector is outbound-only",
			slog.String("hint", "set "+envAppToken+" (xapp-...) to enable Socket Mode inbound"))
		return msgCh, nil
	}

	c.workerOwnsCh = true
	done := c.done
	c.mu.Unlock()

	connectors.StartWorker(ctx, done,
		connectors.WorkerConfig{Name: "slack-socket-mode"},
		c.runSocketSession,
		func() { c.closeMsgOnce.Do(func() { close(msgCh) }) },
	)
	return msgCh, nil
}

// Capabilities reports what this connector can do. "receive" is
// advertised only when Socket Mode inbound is configured (app token
// present); outbound-only setups report just "send".
func (c *Connector) Capabilities() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.appToken != "" {
		return []string{"send", "receive"}
	}
	return []string{"send"}
}

// authTest validates the bot token by calling auth.test and captures
// our own bot user id so the inbound path can ignore self-messages.
// Called from Connect with c.mu held.
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
		OK     bool   `json:"ok"`
		Error  string `json:"error"`
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return err
	}
	if !parsed.OK {
		return fmt.Errorf("auth.test: %s", parsed.Error)
	}
	c.selfUserID = parsed.UserID
	return nil
}
