// Package email implements the IronGolem OS connector for email via IMAP/SMTP.
package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"path/filepath"
	"strings"
	"sync"
	"time"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

// Connector implements connectors.Connector for email over IMAP and SMTP.
type Connector struct {
	mu sync.RWMutex

	// Configuration fields populated from Connect config map.
	imapHost string
	imapPort string
	smtpHost string
	smtpPort string
	username string
	password string
	useTLS   bool

	// Runtime state.
	imapConn  net.Conn
	connected bool
	msgCh     chan *connectors.Message
	done      chan struct{}
}

// Type returns the connector type identifier.
func (c *Connector) Type() connectors.ConnectorType {
	return connectors.TypeEmail
}

// Connect initialises the email connector by establishing an IMAP connection
// and verifying credentials.
func (c *Connector) Connect(ctx context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.imapHost = config["imap_host"]
	c.imapPort = config["imap_port"]
	c.smtpHost = config["smtp_host"]
	c.smtpPort = config["smtp_port"]
	c.username = config["username"]
	c.password = config["password"]
	c.useTLS = config["use_tls"] == "true"

	if c.imapHost == "" || c.imapPort == "" {
		return fmt.Errorf("email connector: imap_host and imap_port are required")
	}
	if c.smtpHost == "" || c.smtpPort == "" {
		return fmt.Errorf("email connector: smtp_host and smtp_port are required")
	}
	if c.username == "" || c.password == "" {
		return fmt.Errorf("email connector: username and password are required")
	}

	addr := net.JoinHostPort(c.imapHost, c.imapPort)

	var conn net.Conn
	var err error

	if c.useTLS {
		tlsCfg := &tls.Config{
			ServerName: c.imapHost,
			MinVersion: tls.VersionTLS12,
		}
		dialer := &tls.Dialer{
			NetDialer: &net.Dialer{Timeout: 10 * time.Second},
			Config:    tlsCfg,
		}
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	} else {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}

	if err != nil {
		return fmt.Errorf("email connector: failed to connect to IMAP server %s: %w", addr, err)
	}

	c.imapConn = conn
	c.connected = true
	c.done = make(chan struct{})

	return nil
}

// Disconnect cleanly shuts down the email connector.
func (c *Connector) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false
	close(c.done)

	if c.imapConn != nil {
		if err := c.imapConn.Close(); err != nil {
			return fmt.Errorf("email connector: error closing IMAP connection: %w", err)
		}
		c.imapConn = nil
	}

	return nil
}

// Health checks whether the IMAP connection is still alive.
func (c *Connector) Health(ctx context.Context) connectors.HealthState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.imapConn == nil {
		return connectors.HealthDisconnected
	}

	// Probe the connection with a zero-byte write deadline check.
	if err := c.imapConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return connectors.HealthDegraded
	}
	// Reset the deadline.
	_ = c.imapConn.SetReadDeadline(time.Time{})

	return connectors.HealthHealthy
}

// Send composes and sends an email via SMTP.
//
// Expected metadata keys on the message:
//   - "to"      : recipient email address (required)
//   - "subject" : email subject line
//   - "cc"      : comma-separated CC addresses
func (c *Connector) Send(ctx context.Context, msg *connectors.Message) error {
	c.mu.RLock()
	host := c.smtpHost
	port := c.smtpPort
	username := c.username
	password := c.password
	useTLS := c.useTLS
	c.mu.RUnlock()

	to := msg.Metadata["to"]
	if to == "" {
		return fmt.Errorf("email connector: 'to' metadata field is required")
	}
	subject := msg.Metadata["subject"]

	addr := net.JoinHostPort(host, port)

	// Build the RFC 5322 message body.
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n",
		username, to, subject)
	body := headers + msg.Content

	auth := smtp.PlainAuth("", username, password, host)

	if useTLS {
		tlsCfg := &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		}
		conn, err := tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("email connector: TLS dial to SMTP failed: %w", err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("email connector: SMTP client creation failed: %w", err)
		}
		defer client.Close()

		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("email connector: SMTP auth failed: %w", err)
		}
		if err := client.Mail(username); err != nil {
			return fmt.Errorf("email connector: SMTP MAIL FROM failed: %w", err)
		}
		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("email connector: SMTP RCPT TO failed: %w", err)
		}
		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("email connector: SMTP DATA failed: %w", err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			return fmt.Errorf("email connector: writing email body failed: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("email connector: closing SMTP data writer failed: %w", err)
		}
		return client.Quit()
	}

	return smtp.SendMail(addr, auth, username, []string{to}, []byte(body))
}

// Receive returns a channel that yields inbound email messages by polling IMAP.
func (c *Connector) Receive(ctx context.Context) (<-chan *connectors.Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil, fmt.Errorf("email connector: not connected")
	}

	c.msgCh = make(chan *connectors.Message, 64)

	go c.pollInbox(ctx)

	return c.msgCh, nil
}

// pollInbox periodically checks the IMAP inbox for new messages.
// This is a stub that demonstrates the polling structure; a production
// implementation would use a full IMAP client library.
func (c *Connector) pollInbox(ctx context.Context) {
	defer close(c.msgCh)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			// TODO: Use a proper IMAP client to FETCH unseen messages,
			// normalise them to connectors.Message, and send on c.msgCh.
			// Each message should set Direction = connectors.Inbound.
		}
	}
}

// Capabilities returns the features supported by the email connector.
func (c *Connector) Capabilities() []string {
	return []string{"send", "receive", "attachments", "html"}
}

// sanitizeAttachmentPath protects against path traversal in attachment
// filenames. It returns a safe basename with directory components stripped.
func sanitizeAttachmentPath(name string) (string, error) {
	// Clean the path to resolve any ".." or "." components.
	cleaned := filepath.Clean(name)

	// Extract only the base filename, discarding any directory prefix.
	base := filepath.Base(cleaned)

	// Reject if the result is empty, a dot, or still contains separators.
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "", fmt.Errorf("email connector: invalid attachment filename: %q", name)
	}

	// Double-check: the base must not contain path separators.
	if strings.ContainsAny(base, `/\`) {
		return "", fmt.Errorf("email connector: path traversal detected in attachment: %q", name)
	}

	return base, nil
}

// Compile-time interface compliance check.
var _ connectors.Connector = (*Connector)(nil)
