// Package webhook implements the IronGolem OS connector for generic
// webhook/API integrations, supporting configurable HTTP requests with
// retries and backoff, and an HTTP endpoint for incoming webhooks with
// signature verification.
package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

const (
	defaultRetryCount  = 3
	defaultMethod      = "POST"
	baseBackoffDelay   = 1 * time.Second
	maxBackoffDelay    = 30 * time.Second
)

// Connector implements connectors.Connector for generic webhook/API
// integrations.
type Connector struct {
	mu sync.RWMutex

	targetURL  string
	method     string
	headers    map[string]string
	authType   string // "none", "bearer", "basic", "api_key"
	authValue  string
	retryCount int
	secret     string // for incoming webhook signature verification

	httpClient *http.Client

	connected     bool
	msgCh         chan *connectors.Message
	done          chan struct{}
	listenAddr    string
	webhookServer *http.Server
}

// Type returns the connector type identifier.
func (c *Connector) Type() connectors.ConnectorType {
	return connectors.TypeWebhook
}

// Connect initializes the webhook connector and verifies the target URL
// is reachable.
func (c *Connector) Connect(ctx context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.targetURL = config["target_url"]
	if c.targetURL == "" {
		return fmt.Errorf("webhook connector: target_url is required")
	}

	c.method = config["method"]
	if c.method == "" {
		c.method = defaultMethod
	}

	// Parse headers from JSON string.
	c.headers = make(map[string]string)
	if headersJSON := config["headers"]; headersJSON != "" {
		if err := json.Unmarshal([]byte(headersJSON), &c.headers); err != nil {
			return fmt.Errorf("webhook connector: invalid headers JSON: %w", err)
		}
	}

	c.authType = config["auth_type"]
	if c.authType == "" {
		c.authType = "none"
	}
	c.authValue = config["auth_value"]

	c.retryCount = defaultRetryCount
	if rc := config["retry_count"]; rc != "" {
		var parsed int
		for _, ch := range rc {
			if ch >= '0' && ch <= '9' {
				parsed = parsed*10 + int(ch-'0')
			}
		}
		if parsed > 0 {
			c.retryCount = parsed
		}
	}

	c.secret = config["secret"]

	c.listenAddr = config["listen_addr"]
	if c.listenAddr == "" {
		c.listenAddr = ":3104"
	}

	c.httpClient = &http.Client{Timeout: 30 * time.Second}

	// Verify target URL is reachable with a HEAD request.
	if err := c.verifyTarget(ctx); err != nil {
		return fmt.Errorf("webhook connector: target URL not reachable: %w", err)
	}

	c.connected = true
	c.done = make(chan struct{})

	return nil
}

// Disconnect cleanly shuts down the webhook connector.
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
			return fmt.Errorf("webhook connector: error shutting down server: %w", err)
		}
		c.webhookServer = nil
	}

	return nil
}

// Health checks if the target URL is reachable.
func (c *Connector) Health(ctx context.Context) connectors.HealthState {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return connectors.HealthDisconnected
	}

	if err := c.verifyTarget(ctx); err != nil {
		return connectors.HealthDegraded
	}

	return connectors.HealthHealthy
}

// Send makes an HTTP request to the configured target URL with retries and
// exponential backoff.
//
// The message Content is sent as the request body. Additional metadata keys:
//   - "url"    : override target URL for this request
//   - "method" : override HTTP method for this request
//   - Any key starting with "header_" sets a request header (e.g. "header_X-Custom" -> "X-Custom: value")
func (c *Connector) Send(ctx context.Context, msg *connectors.Message) error {
	c.mu.RLock()
	targetURL := c.targetURL
	method := c.method
	headers := c.headers
	authType := c.authType
	authValue := c.authValue
	retryCount := c.retryCount
	c.mu.RUnlock()

	// Allow per-request overrides.
	if u := msg.Metadata["url"]; u != "" {
		targetURL = u
	}
	if m := msg.Metadata["method"]; m != "" {
		method = m
	}

	var lastErr error
	for attempt := 0; attempt <= retryCount; attempt++ {
		if attempt > 0 {
			delay := backoffDelay(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := c.doRequest(ctx, method, targetURL, msg.Content, headers, authType, authValue, msg.Metadata)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return fmt.Errorf("webhook connector: all %d attempts failed: %w", retryCount+1, lastErr)
}

// doRequest executes a single HTTP request.
func (c *Connector) doRequest(ctx context.Context, method, targetURL, body string, headers map[string]string, authType, authValue string, metadata map[string]string) error {
	var bodyReader io.Reader
	if body != "" && method != http.MethodGet && method != http.MethodHead {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	// Apply configured headers.
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Apply per-request headers from metadata.
	for k, v := range metadata {
		if strings.HasPrefix(k, "header_") {
			headerName := strings.TrimPrefix(k, "header_")
			req.Header.Set(headerName, v)
		}
	}

	// Set default Content-Type if body is present and no Content-Type set.
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply authentication.
	switch authType {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+authValue)
	case "basic":
		req.Header.Set("Authorization", "Basic "+authValue)
	case "api_key":
		req.Header.Set("X-API-Key", authValue)
	case "none", "":
		// No authentication.
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Drain body to allow connection reuse.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error: status %d", resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("client error: status %d", resp.StatusCode)
	}

	return nil
}

// Receive returns a channel of inbound messages by starting an HTTP server
// that listens for incoming webhook callbacks.
func (c *Connector) Receive(ctx context.Context) (<-chan *connectors.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("webhook connector: not connected")
	}

	c.msgCh = make(chan *connectors.Message, 64)

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook/incoming", c.handleIncoming)

	c.webhookServer = &http.Server{
		Addr:              c.listenAddr,
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

// handleIncoming processes incoming webhook HTTP callbacks.
func (c *Connector) handleIncoming(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	// Verify signature if secret is configured.
	c.mu.RLock()
	secret := c.secret
	c.mu.RUnlock()

	if secret != "" {
		signature := r.Header.Get("X-Signature-256")
		if signature == "" {
			signature = r.Header.Get("X-Hub-Signature-256")
		}
		if !verifySignature(body, signature, secret) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Extract metadata from headers.
	metadata := map[string]string{
		"content_type": r.Header.Get("Content-Type"),
		"remote_addr":  r.RemoteAddr,
	}

	// Copy select headers to metadata.
	for _, h := range []string{"X-Request-ID", "X-Correlation-ID", "User-Agent"} {
		if v := r.Header.Get(h); v != "" {
			metadata[strings.ToLower(strings.ReplaceAll(h, "-", "_"))] = v
		}
	}

	normalized := &connectors.Message{
		ID:          fmt.Sprintf("webhook_%d", time.Now().UnixNano()),
		ConnectorID: "",
		Type:        connectors.TypeWebhook,
		Direction:   connectors.Inbound,
		Content:     string(body),
		Metadata:    metadata,
		Timestamp:   time.Now(),
	}

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

	w.WriteHeader(http.StatusOK)
}

// verifyTarget checks that the target URL is reachable.
func (c *Connector) verifyTarget(ctx context.Context) error {
	c.mu.RLock()
	targetURL := c.targetURL
	c.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Accept any response as "reachable" (even 4xx means the server is up).
	return nil
}

// verifySignature validates an HMAC-SHA256 signature against the payload.
func verifySignature(body []byte, signature, secret string) bool {
	if signature == "" {
		return false
	}

	// Support "sha256=..." prefix format.
	sig := signature
	if strings.HasPrefix(sig, "sha256=") {
		sig = sig[7:]
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sig), []byte(expectedSig))
}

// backoffDelay calculates exponential backoff delay for a given attempt number.
func backoffDelay(attempt int) time.Duration {
	delay := time.Duration(math.Pow(2, float64(attempt-1))) * baseBackoffDelay
	if delay > maxBackoffDelay {
		delay = maxBackoffDelay
	}
	return delay
}

// Capabilities returns the features supported by the webhook connector.
func (c *Connector) Capabilities() []string {
	return []string{"send", "receive", "retries", "auth"}
}

// Compile-time interface compliance check.
var _ connectors.Connector = (*Connector)(nil)
