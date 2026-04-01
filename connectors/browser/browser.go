// Package browser implements the IronGolem OS connector for browser automation
// using a Chrome DevTools Protocol (CDP) interface design. It supports
// navigation, screenshots, text extraction, clicking, and form filling with
// domain allowlist enforcement.
package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

const (
	defaultCDPPort = "9222"
)

// Connector implements connectors.Connector for browser automation via CDP.
type Connector struct {
	mu sync.RWMutex

	headless       bool
	userDataDir    string
	allowedDomains map[string]bool
	cdpHost        string
	cdpPort        string

	httpClient *http.Client

	connected bool
	msgCh     chan *connectors.Message
	done      chan struct{}
}

// --- CDP types ---

type cdpVersionResponse struct {
	Browser              string `json:"Browser"`
	ProtocolVersion      string `json:"Protocol-Version"`
	UserAgent            string `json:"User-Agent"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// browserCommand represents a command to execute in the browser.
type browserCommand struct {
	Action string            `json:"action"`
	Params map[string]string `json:"params,omitempty"`
}

// Type returns the connector type identifier.
func (c *Connector) Type() connectors.ConnectorType {
	return connectors.TypeBrowser
}

// Connect initializes the browser connector by verifying that the CDP
// endpoint is reachable.
func (c *Connector) Connect(ctx context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.headless = config["headless"] != "false" // default true

	c.userDataDir = config["user_data_dir"]

	// Parse allowed_domains as a comma-separated list.
	c.allowedDomains = make(map[string]bool)
	if domains := config["allowed_domains"]; domains != "" {
		for _, d := range strings.Split(domains, ",") {
			d = strings.TrimSpace(d)
			if d != "" {
				c.allowedDomains[strings.ToLower(d)] = true
			}
		}
	}

	c.cdpHost = config["cdp_host"]
	if c.cdpHost == "" {
		c.cdpHost = "127.0.0.1"
	}

	c.cdpPort = config["cdp_port"]
	if c.cdpPort == "" {
		c.cdpPort = defaultCDPPort
	}

	c.httpClient = &http.Client{Timeout: 10 * time.Second}

	// Verify CDP endpoint is reachable.
	if err := c.verifyCDP(ctx); err != nil {
		return fmt.Errorf("browser connector: CDP endpoint not reachable: %w", err)
	}

	c.connected = true
	c.done = make(chan struct{})

	return nil
}

// Disconnect cleanly shuts down the browser connector.
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

// Health checks if the browser process is alive by pinging the CDP endpoint.
func (c *Connector) Health(ctx context.Context) connectors.HealthState {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return connectors.HealthDisconnected
	}

	if err := c.verifyCDP(ctx); err != nil {
		return connectors.HealthDegraded
	}

	return connectors.HealthHealthy
}

// Send executes a browser command. The message Content should be a JSON-encoded
// browserCommand, or the action can be specified via metadata.
//
// Supported actions (via metadata["action"]):
//   - "navigate"     : navigate to URL in metadata["url"]
//   - "screenshot"   : take a screenshot (result returned as base64 in response)
//   - "extract_text" : extract text content from the current page
//   - "click"        : click element matching metadata["selector"]
//   - "fill_form"    : fill form field metadata["selector"] with metadata["value"]
//
// Alternatively, send a JSON-encoded browserCommand as the message Content.
func (c *Connector) Send(ctx context.Context, msg *connectors.Message) error {
	var cmd browserCommand

	// Try to get action from metadata first, then from content JSON.
	if action := msg.Metadata["action"]; action != "" {
		cmd.Action = action
		cmd.Params = make(map[string]string)
		for k, v := range msg.Metadata {
			if k != "action" {
				cmd.Params[k] = v
			}
		}
	} else {
		if err := json.Unmarshal([]byte(msg.Content), &cmd); err != nil {
			return fmt.Errorf("browser connector: invalid command: %w", err)
		}
	}

	return c.executeCommand(ctx, &cmd)
}

// executeCommand dispatches browser commands to the appropriate handler.
func (c *Connector) executeCommand(ctx context.Context, cmd *browserCommand) error {
	switch cmd.Action {
	case "navigate":
		return c.navigate(ctx, cmd.Params["url"])
	case "screenshot":
		return c.screenshot(ctx)
	case "extract_text":
		return c.extractText(ctx)
	case "click":
		return c.click(ctx, cmd.Params["selector"])
	case "fill_form":
		return c.fillForm(ctx, cmd.Params["selector"], cmd.Params["value"])
	default:
		return fmt.Errorf("browser connector: unsupported action %q", cmd.Action)
	}
}

// navigate navigates the browser to the specified URL after verifying the
// domain is in the allowlist.
func (c *Connector) navigate(ctx context.Context, rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("browser connector: 'url' parameter is required for navigate")
	}

	// Domain allowlist enforcement.
	if err := c.checkDomainAllowed(rawURL); err != nil {
		return err
	}

	// In production, this would send a Page.navigate CDP command via WebSocket.
	// Here we demonstrate the validation and structure.
	cdpCmd := map[string]interface{}{
		"method": "Page.navigate",
		"params": map[string]interface{}{
			"url": rawURL,
		},
	}
	_ = cdpCmd

	return nil
}

// screenshot takes a screenshot of the current page.
func (c *Connector) screenshot(ctx context.Context) error {
	// In production: send Page.captureScreenshot CDP command.
	cdpCmd := map[string]interface{}{
		"method": "Page.captureScreenshot",
		"params": map[string]interface{}{
			"format": "png",
		},
	}
	_ = cdpCmd

	return nil
}

// extractText extracts text from the current page.
func (c *Connector) extractText(ctx context.Context) error {
	// In production: evaluate document.body.innerText via Runtime.evaluate.
	cdpCmd := map[string]interface{}{
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{
			"expression":    "document.body.innerText",
			"returnByValue": true,
		},
	}
	_ = cdpCmd

	return nil
}

// click clicks an element matching the given CSS selector.
func (c *Connector) click(ctx context.Context, selector string) error {
	if selector == "" {
		return fmt.Errorf("browser connector: 'selector' parameter is required for click")
	}

	// In production: use DOM.querySelector + DOM.getBoxModel + Input.dispatchMouseEvent.
	cdpCmd := map[string]interface{}{
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{
			"expression": fmt.Sprintf("document.querySelector(%q).click()", selector),
		},
	}
	_ = cdpCmd

	return nil
}

// fillForm fills a form field matching the selector with the given value.
func (c *Connector) fillForm(ctx context.Context, selector, value string) error {
	if selector == "" {
		return fmt.Errorf("browser connector: 'selector' parameter is required for fill_form")
	}

	// In production: focus element, then use Input.dispatchKeyEvent or
	// Runtime.evaluate to set the value.
	cdpCmd := map[string]interface{}{
		"method": "Runtime.evaluate",
		"params": map[string]interface{}{
			"expression": fmt.Sprintf(
				"(function(){var el=document.querySelector(%q);el.focus();el.value=%q;el.dispatchEvent(new Event('input',{bubbles:true}));})()",
				selector, value,
			),
		},
	}
	_ = cdpCmd

	return nil
}

// checkDomainAllowed verifies that the given URL's domain is in the allowlist.
func (c *Connector) checkDomainAllowed(rawURL string) error {
	c.mu.RLock()
	allowedDomains := c.allowedDomains
	c.mu.RUnlock()

	// If no allowlist is configured, all domains are allowed.
	if len(allowedDomains) == 0 {
		return nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("browser connector: invalid URL %q: %w", rawURL, err)
	}

	hostname := strings.ToLower(parsed.Hostname())
	if hostname == "" {
		return fmt.Errorf("browser connector: URL %q has no hostname", rawURL)
	}

	// Check exact match and parent domain match.
	if allowedDomains[hostname] {
		return nil
	}

	// Check if hostname is a subdomain of an allowed domain.
	for domain := range allowedDomains {
		if strings.HasSuffix(hostname, "."+domain) {
			return nil
		}
	}

	return fmt.Errorf("browser connector: domain %q is not in the allowed list", hostname)
}

// Receive returns a channel that yields results from browser events.
// The browser connector primarily operates via Send (commands), but this
// channel can receive page events (e.g., console messages, navigation events).
func (c *Connector) Receive(ctx context.Context) (<-chan *connectors.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("browser connector: not connected")
	}

	c.msgCh = make(chan *connectors.Message, 64)

	go func() {
		defer close(c.msgCh)
		// In production, this would listen for CDP events via WebSocket
		// (e.g., Console.messageAdded, Page.loadEventFired) and normalize
		// them to connector.Message format.
		select {
		case <-ctx.Done():
		case <-c.done:
		}
	}()

	return c.msgCh, nil
}

// verifyCDP checks that the Chrome DevTools Protocol endpoint is reachable.
func (c *Connector) verifyCDP(ctx context.Context) error {
	c.mu.RLock()
	cdpURL := fmt.Sprintf("http://%s:%s/json/version", c.cdpHost, c.cdpPort)
	c.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cdpURL, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("CDP returned status %d", resp.StatusCode)
	}

	return nil
}

// Capabilities returns the features supported by the browser connector.
func (c *Connector) Capabilities() []string {
	return []string{"navigate", "screenshot", "extract_text", "click", "fill_form"}
}

// Compile-time interface compliance check.
var _ connectors.Connector = (*Connector)(nil)
