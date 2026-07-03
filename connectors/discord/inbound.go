// Discord inbound via the Gateway WebSocket. v0.4 adoption wave.
//
// Session flow (one iteration of the shared connector worker):
//
//	GET /gateway/bot            → wss URL
//	dial URL?v=10&encoding=json
//	← HELLO (op 10)             → start heartbeat loop (op 1, last seq)
//	→ IDENTIFY (op 2)           with GUILDS | GUILD_MESSAGES |
//	                            DIRECT_MESSAGES | MESSAGE_CONTENT intents
//	← DISPATCH (op 0)           MESSAGE_CREATE → normalize → msgCh
//	← HEARTBEAT ACK (op 11)     noted
//	← RECONNECT (op 7)          return error → worker reconnects
//	← INVALID SESSION (op 9)    return error → worker backs off, then a
//	                            fresh session re-identifies
//
// TODO(v0.5): full RESUME support (cache session_id + resume_gateway_url
// from the READY dispatch and send op 6 on reconnect instead of a fresh
// IDENTIFY). Reconnect-and-reidentify with backoff is acceptable for
// this wave, at the cost of replaying no missed events.
package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
	"github.com/coder/websocket"
)

// Gateway opcodes (https://discord.com/developers/docs/topics/opcodes-and-status-codes).
const (
	opDispatch       = 0
	opHeartbeat      = 1
	opIdentify       = 2
	opReconnect      = 7
	opInvalidSession = 9
	opHello          = 10
	opHeartbeatACK   = 11
)

// Gateway intents (https://discord.com/developers/docs/topics/gateway#gateway-intents).
const (
	intentGuilds         = 1 << 0  // 1
	intentGuildMessages  = 1 << 9  // 512
	intentDirectMessages = 1 << 12 // 4096
	intentMessageContent = 1 << 15 // 32768

	// gatewayIntents = 1 + 512 + 4096 + 32768 = 37377.
	gatewayIntents = intentGuilds | intentGuildMessages | intentDirectMessages | intentMessageContent
)

// gatewayEvent is one frame from the Gateway WebSocket.
type gatewayEvent struct {
	Op   int             `json:"op"`
	Seq  *int64          `json:"s"`
	Type string          `json:"t"`
	Data json.RawMessage `json:"d"`
}

// messageCreate is the MESSAGE_CREATE dispatch payload subset we consume.
type messageCreate struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	GuildID   string `json:"guild_id"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	Author    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Bot      bool   `json:"bot"`
	} `json:"author"`
}

// parseGatewayEvent decodes one Gateway frame. Malformed JSON is an
// error the session loop drops without crashing the worker.
func parseGatewayEvent(raw []byte) (*gatewayEvent, error) {
	var ev gatewayEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		return nil, fmt.Errorf("discord: malformed gateway frame: %w", err)
	}
	return &ev, nil
}

// buildIdentify constructs the op-2 IDENTIFY payload.
func buildIdentify(token string) []byte {
	payload, _ := json.Marshal(map[string]any{
		"op": opIdentify,
		"d": map[string]any{
			"token":   token,
			"intents": gatewayIntents,
			"properties": map[string]string{
				"os":      runtime.GOOS,
				"browser": "irongolem",
				"device":  "irongolem",
			},
		},
	})
	return payload
}

// buildHeartbeat constructs the op-1 heartbeat carrying the last seen
// sequence number (null before the first dispatch).
func buildHeartbeat(lastSeq *int64) []byte {
	payload, _ := json.Marshal(map[string]any{
		"op": opHeartbeat,
		"d":  lastSeq,
	})
	return payload
}

// normalizeMessageCreate turns a MESSAGE_CREATE dispatch into a
// connectors.Message. Returns (nil, false) for our own messages, other
// bots, and malformed JSON — dropped, never fatal.
func normalizeMessageCreate(data []byte, selfID string) (*connectors.Message, bool) {
	var m messageCreate
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	if m.Author.Bot {
		return nil, false
	}
	if selfID != "" && m.Author.ID == selfID {
		return nil, false
	}
	if m.ChannelID == "" || m.ID == "" {
		return nil, false
	}

	ts := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339, m.Timestamp); err == nil {
		ts = parsed.UTC()
	}

	msg := &connectors.Message{
		ID:        fmt.Sprintf("discord_%s_%s", m.ChannelID, m.ID),
		Type:      connectors.TypeDiscord,
		Direction: connectors.Inbound,
		Content:   m.Content,
		Metadata: map[string]string{
			// "channel" matches the key Send expects, so a reply can
			// reuse the inbound metadata unchanged.
			"channel":    m.ChannelID,
			"message_id": m.ID,
			"author_id":  m.Author.ID,
		},
		Timestamp: ts,
	}
	if m.Author.Username != "" {
		msg.Metadata["author_username"] = m.Author.Username
	}
	if m.GuildID != "" {
		msg.Metadata["guild_id"] = m.GuildID
	}
	return msg, true
}

// gatewayURL asks the REST API where the Gateway lives.
func (c *Connector) gatewayURL(ctx context.Context) (string, error) {
	c.mu.RLock()
	base := c.apiBase
	token := c.botToken
	client := c.httpClient
	c.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/gateway/bot", nil)
	if err != nil {
		return "", fmt.Errorf("discord: build /gateway/bot request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+token)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("discord: /gateway/bot: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("discord: /gateway/bot status %d: %s", resp.StatusCode, string(raw))
	}
	var parsed struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("discord: parse /gateway/bot response: %w", err)
	}
	if parsed.URL == "" {
		return "", fmt.Errorf("discord: /gateway/bot returned empty url")
	}
	if strings.Contains(parsed.URL, "?") {
		return parsed.URL, nil
	}
	return parsed.URL + "?v=10&encoding=json", nil
}

// runGatewaySession runs one Gateway connection: HELLO, IDENTIFY,
// heartbeats, dispatch. Returning an error hands control back to the
// worker, which reconnects (and re-identifies) with backoff.
func (c *Connector) runGatewaySession(ctx context.Context) error {
	gwURL, err := c.gatewayURL(ctx)
	if err != nil {
		return err
	}

	conn, _, err := websocket.Dial(ctx, gwURL, nil)
	if err != nil {
		return fmt.Errorf("discord: gateway dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "session over")

	c.mu.RLock()
	token := c.botToken
	selfID := c.selfID
	msgCh := c.msgCh
	c.mu.RUnlock()

	// First frame must be HELLO with the heartbeat interval.
	_, raw, err := conn.Read(ctx)
	if err != nil {
		return fmt.Errorf("discord: gateway read hello: %w", err)
	}
	hello, err := parseGatewayEvent(raw)
	if err != nil || hello.Op != opHello {
		return fmt.Errorf("discord: expected HELLO as first gateway frame")
	}
	var helloData struct {
		HeartbeatInterval int64 `json:"heartbeat_interval"` // ms
	}
	if err := json.Unmarshal(hello.Data, &helloData); err != nil || helloData.HeartbeatInterval <= 0 {
		return fmt.Errorf("discord: invalid HELLO heartbeat_interval")
	}

	if err := conn.Write(ctx, websocket.MessageText, buildIdentify(token)); err != nil {
		return fmt.Errorf("discord: gateway identify: %w", err)
	}

	// Reader goroutine feeds frames into the select loop below so the
	// heartbeat ticker never starves behind a blocking Read.
	frames := make(chan []byte)
	readErr := make(chan error, 1)
	go func() {
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				readErr <- err
				return
			}
			select {
			case frames <- raw:
			case <-ctx.Done():
				return
			}
		}
	}()

	ticker := time.NewTicker(time.Duration(helloData.HeartbeatInterval) * time.Millisecond)
	defer ticker.Stop()

	var lastSeq *int64
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-readErr:
			return fmt.Errorf("discord: gateway read: %w", err)

		case <-ticker.C:
			if err := conn.Write(ctx, websocket.MessageText, buildHeartbeat(lastSeq)); err != nil {
				return fmt.Errorf("discord: gateway heartbeat: %w", err)
			}

		case raw := <-frames:
			ev, err := parseGatewayEvent(raw)
			if err != nil {
				// Malformed frame: drop it, keep the session alive.
				continue
			}
			switch ev.Op {
			case opDispatch:
				if ev.Seq != nil {
					lastSeq = ev.Seq
				}
				if ev.Type != "MESSAGE_CREATE" {
					continue
				}
				msg, ok := normalizeMessageCreate(ev.Data, selfID)
				if !ok {
					continue
				}
				select {
				case msgCh <- msg:
				case <-ctx.Done():
					return ctx.Err()
				}

			case opHeartbeat:
				// Server asked for an immediate heartbeat.
				if err := conn.Write(ctx, websocket.MessageText, buildHeartbeat(lastSeq)); err != nil {
					return fmt.Errorf("discord: gateway heartbeat (requested): %w", err)
				}

			case opHeartbeatACK:
				// Healthy. TODO(v0.5): track missed ACKs to detect a
				// zombie connection before Discord closes it.

			case opReconnect:
				// TODO(v0.5): RESUME instead of a fresh IDENTIFY.
				return fmt.Errorf("discord: gateway requested reconnect")

			case opInvalidSession:
				// Worker backoff supplies the delay Discord asks for
				// before the next IDENTIFY.
				return fmt.Errorf("discord: gateway invalidated session")
			}
		}
	}
}
