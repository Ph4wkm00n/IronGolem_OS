package connector

import (
	"context"
	"errors"
	"fmt"
	"time"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
	"github.com/Ph4wkm00n/IronGolem_OS/connectors/telegram"
)

// TelegramSourceConfig captures what the gateway's main.go needs to know to
// stand up the Telegram InboundSource.
type TelegramSourceConfig struct {
	// ConnectorID is the id under which the source registers with the
	// Manager. Audit events attribute themselves to this string, so it
	// should be stable across restarts.
	ConnectorID string
	// BotToken is the Telegram bot token. Required.
	BotToken string
	// APIBase overrides the Telegram Bot API base URL. Defaults to
	// `https://api.telegram.org`. Tests point this at an httptest server.
	APIBase string
	// AllowedChatIDs is the comma-separated list of chat ids the connector
	// will accept inbound from + send outbound to. Empty allows all (solo
	// mode); production deployments should set this.
	AllowedChatIDs string
	// TenantID stamps every inbound message with this tenant. Solo mode
	// defaults to "default". Multi-tenant deployments resolve from
	// per-chat-id mapping at a later step in the v0.2 plan.
	TenantID string
}

// NewTelegramSource builds an InboundSource backed by a real
// connectors.Connector for Telegram. The returned source can be passed to
// Manager.RegisterSource; the source owns connector lifecycle (Connect on
// Receive, Disconnect on context cancellation).
func NewTelegramSource(ctx context.Context, cfg TelegramSourceConfig) (InboundSource, error) {
	if cfg.BotToken == "" {
		return nil, errors.New("telegram source: BotToken required")
	}
	if cfg.ConnectorID == "" {
		cfg.ConnectorID = "telegram"
	}
	if cfg.TenantID == "" {
		cfg.TenantID = "default"
	}

	c := &telegram.Connector{}
	connectCfg := map[string]string{
		"bot_token":        cfg.BotToken,
		"allowed_chat_ids": cfg.AllowedChatIDs,
	}
	if cfg.APIBase != "" {
		connectCfg["api_base"] = cfg.APIBase
	}
	if err := c.Connect(ctx, connectCfg); err != nil {
		return nil, fmt.Errorf("telegram source: connect: %w", err)
	}

	return &telegramSource{
		c:           c,
		connectorID: cfg.ConnectorID,
		tenantID:    cfg.TenantID,
	}, nil
}

// telegramSource adapts a connectors.Connector to the gateway's local
// InboundSource interface. The two sit in different Go modules deliberately:
// the gateway must not depend on the wire-level connector types in its
// inbound handler; the adapter is the single seam that translates between
// `connectors.Message` and the gateway-local `InboundMessage` shape.
type telegramSource struct {
	c           *telegram.Connector
	connectorID string
	tenantID    string
}

// Receive starts the underlying connector's poll loop and translates each
// inbound `*connectors.Message` into the gateway's InboundMessage shape.
// The returned channel closes when ctx cancels or the connector disconnects.
func (s *telegramSource) Receive(ctx context.Context) (<-chan InboundMessage, error) {
	raw, err := s.c.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("telegram source: receive: %w", err)
	}

	out := make(chan InboundMessage, 64)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				_ = s.c.Disconnect(context.Background())
				return
			case msg, ok := <-raw:
				if !ok {
					return
				}
				select {
				case out <- s.translateInbound(msg):
				case <-ctx.Done():
					_ = s.c.Disconnect(context.Background())
					return
				}
			}
		}
	}()
	return out, nil
}

// Send delivers a reply through the connector's `Send` method, packing the
// gateway-local OutboundMessage into the connector wire shape. The Telegram
// `Send` requires `chat_id` in the Metadata map; the pump preserves the
// inbound message's Metadata into the outbound, so the chat id flows
// through naturally.
func (s *telegramSource) Send(ctx context.Context, msg OutboundMessage) error {
	meta := msg.Metadata
	if meta == nil {
		meta = map[string]string{}
	}
	if _, ok := meta["chat_id"]; !ok && msg.ChannelID != "" {
		// Fallback: derive chat_id from the channel id when the inbound
		// metadata was lost in transit (defensive — shouldn't happen).
		meta = copyMeta(meta)
		meta["chat_id"] = msg.ChannelID
	}

	return s.c.Send(ctx, &connectors.Message{
		ConnectorID: s.connectorID,
		Type:        connectors.TypeTelegram,
		Direction:   connectors.Outbound,
		Content:     msg.Content,
		Metadata:    meta,
		Timestamp:   time.Now().UTC(),
	})
}

func (s *telegramSource) translateInbound(m *connectors.Message) InboundMessage {
	out := InboundMessage{
		ConnectorID: s.connectorID,
		Content:     m.Content,
		TenantID:    s.tenantID,
		// WorkspaceID stamped by the handler when empty.
		Metadata: copyMeta(m.Metadata),
	}
	if m.Metadata != nil {
		out.ChannelID = m.Metadata["chat_id"]
		out.UserID = m.Metadata["from_id"]
	}
	return out
}

func copyMeta(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
