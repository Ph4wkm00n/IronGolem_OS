// Slack inbound via Socket Mode. v0.4 adoption wave.
//
// Decision (final): Socket Mode, NOT the Events API webhook. Socket Mode
// needs no public HTTPS endpoint and no request-signing plumbing, which
// fits IronGolem's local-first deployment story. Flow:
//
//	POST /apps.connections.open (app-level token, xapp-...)
//	  → wss URL → WebSocket → envelopes
//	  → ack every envelope with {"envelope_id": ...} immediately
//	  → events_api envelopes with event.type == "message" are
//	    normalized into connectors.Message and fed to msgCh.
//
// The long-lived WebSocket session runs under connectors.StartWorker,
// which handles context cancellation, panic recovery, and exponential
// backoff reconnect.
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
	"github.com/coder/websocket"
)

// socketEnvelope is the outer frame Slack sends over a Socket Mode
// connection. Every envelope with an envelope_id must be acked.
type socketEnvelope struct {
	EnvelopeID string          `json:"envelope_id"`
	Type       string          `json:"type"` // "hello", "events_api", "disconnect", ...
	Payload    json.RawMessage `json:"payload,omitempty"`
}

// eventsAPIPayload is the payload of an "events_api" envelope.
type eventsAPIPayload struct {
	TeamID string     `json:"team_id"`
	Event  slackEvent `json:"event"`
}

// slackEvent is the inner Events API event.
type slackEvent struct {
	Type        string `json:"type"`
	Subtype     string `json:"subtype"`
	User        string `json:"user"`
	BotID       string `json:"bot_id"`
	Text        string `json:"text"`
	Channel     string `json:"channel"`
	ChannelType string `json:"channel_type"`
	TS          string `json:"ts"`
	EventTS     string `json:"event_ts"`
}

// parseEnvelope decodes one Socket Mode frame. Malformed JSON is an
// error the caller drops without crashing the worker.
func parseEnvelope(raw []byte) (*socketEnvelope, error) {
	var env socketEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("slack: malformed envelope: %w", err)
	}
	return &env, nil
}

// buildAck constructs the acknowledgement frame Slack requires for
// every envelope. Sent immediately on receipt, before any processing.
func buildAck(envelopeID string) []byte {
	ack, _ := json.Marshal(map[string]string{"envelope_id": envelopeID})
	return ack
}

// normalizeEvent turns an events_api payload into a connectors.Message.
// Returns (nil, false) for anything that isn't a plain user message:
// bot messages (bot_id set, or sender == our own bot user), subtypes
// like message_changed / message_deleted, non-message event types, and
// malformed JSON. Dropping instead of erroring keeps a hostile or
// malformed event from ever taking the worker down.
func normalizeEvent(payload []byte, selfUserID string) (*connectors.Message, bool) {
	var p eventsAPIPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, false
	}
	ev := p.Event
	if ev.Type != "message" {
		return nil, false
	}
	// Subtypes (message_changed, message_deleted, bot_message, ...) are
	// not fresh user messages.
	if ev.Subtype != "" {
		return nil, false
	}
	// Ignore bots — including ourselves, or we'd loop on our own replies.
	if ev.BotID != "" {
		return nil, false
	}
	if selfUserID != "" && ev.User == selfUserID {
		return nil, false
	}
	if ev.Channel == "" {
		return nil, false
	}

	msg := &connectors.Message{
		ID:        fmt.Sprintf("slack_%s_%s", ev.Channel, ev.TS),
		Type:      connectors.TypeSlack,
		Direction: connectors.Inbound,
		Content:   ev.Text,
		Metadata: map[string]string{
			// "channel" matches the key Send expects, so a reply can
			// reuse the inbound metadata unchanged.
			"channel": ev.Channel,
			"user":    ev.User,
			"ts":      ev.TS,
		},
		Timestamp: slackTS(ev.TS),
	}
	if ev.ChannelType != "" {
		msg.Metadata["channel_type"] = ev.ChannelType
	}
	if ev.EventTS != "" {
		msg.Metadata["event_ts"] = ev.EventTS
	}
	if p.TeamID != "" {
		msg.Metadata["team"] = p.TeamID
	}
	return msg, true
}

// slackTS parses Slack's "1712345678.001200" second.fraction timestamps.
// Falls back to the current time on parse failure so a weird ts never
// drops an otherwise valid message.
func slackTS(ts string) time.Time {
	sec, frac, _ := strings.Cut(ts, ".")
	s, err := strconv.ParseInt(sec, 10, 64)
	if err != nil {
		return time.Now().UTC()
	}
	var nanos int64
	if frac != "" {
		// Right-pad/truncate the fraction to microsecond precision.
		if len(frac) > 6 {
			frac = frac[:6]
		}
		if f, err := strconv.ParseInt(frac, 10, 64); err == nil {
			for i := len(frac); i < 6; i++ {
				f *= 10
			}
			nanos = f * 1000
		}
	}
	return time.Unix(s, nanos).UTC()
}

// openSocketURL calls apps.connections.open with the app-level token
// and returns the wss URL to dial.
func (c *Connector) openSocketURL(ctx context.Context) (string, error) {
	c.mu.RLock()
	base := c.apiBase
	appToken := c.appToken
	client := c.httpClient
	c.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/apps.connections.open", bytes.NewReader(nil))
	if err != nil {
		return "", fmt.Errorf("slack: build apps.connections.open request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+appToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("slack: apps.connections.open: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("slack: apps.connections.open status %d: %s", resp.StatusCode, string(raw))
	}
	var parsed struct {
		OK    bool   `json:"ok"`
		URL   string `json:"url"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("slack: parse apps.connections.open response: %w", err)
	}
	if !parsed.OK {
		return "", fmt.Errorf("slack: apps.connections.open api error: %s", parsed.Error)
	}
	if parsed.URL == "" {
		return "", fmt.Errorf("slack: apps.connections.open returned empty url")
	}
	return parsed.URL, nil
}

// runSocketSession runs one Socket Mode connection: open the URL, dial,
// then read envelopes until the connection dies. Returning an error
// hands control back to the worker, which reconnects with backoff.
func (c *Connector) runSocketSession(ctx context.Context) error {
	wsURL, err := c.openSocketURL(ctx)
	if err != nil {
		return err
	}

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("slack: socket mode dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "session over")

	c.mu.RLock()
	selfUserID := c.selfUserID
	msgCh := c.msgCh
	c.mu.RUnlock()

	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("slack: socket mode read: %w", err)
		}

		env, err := parseEnvelope(raw)
		if err != nil {
			// Malformed frame: drop it, keep the session alive.
			continue
		}

		// Ack first, always — Slack retries unacked envelopes and will
		// eventually close connections that don't ack.
		if env.EnvelopeID != "" {
			if err := conn.Write(ctx, websocket.MessageText, buildAck(env.EnvelopeID)); err != nil {
				return fmt.Errorf("slack: socket mode ack: %w", err)
			}
		}

		switch env.Type {
		case "disconnect":
			// Slack recycles Socket Mode connections; reconnect cleanly.
			return fmt.Errorf("slack: server requested disconnect")
		case "events_api":
			msg, ok := normalizeEvent(env.Payload, selfUserID)
			if !ok {
				continue
			}
			select {
			case msgCh <- msg:
			case <-ctx.Done():
				return ctx.Err()
			}
		default:
			// "hello" and anything unknown: nothing to do beyond the ack.
		}
	}
}
