package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
	"github.com/coder/websocket"
)

func TestNormalizeEvent(t *testing.T) {
	tests := []struct {
		name        string
		payload     string
		selfUserID  string
		wantOK      bool
		wantContent string
		wantChannel string
		wantUser    string
	}{
		{
			name: "plain user message",
			payload: `{"team_id":"T123","event":{"type":"message","user":"U777","text":"hello world",` +
				`"channel":"C42","channel_type":"channel","ts":"1712345678.001200","event_ts":"1712345678.001200"}}`,
			selfUserID:  "UBOT",
			wantOK:      true,
			wantContent: "hello world",
			wantChannel: "C42",
			wantUser:    "U777",
		},
		{
			name:       "bot message ignored via bot_id",
			payload:    `{"event":{"type":"message","bot_id":"B99","text":"beep","channel":"C42","ts":"1.0"}}`,
			selfUserID: "UBOT",
			wantOK:     false,
		},
		{
			name:       "own message ignored via self user id",
			payload:    `{"event":{"type":"message","user":"UBOT","text":"echo","channel":"C42","ts":"1.0"}}`,
			selfUserID: "UBOT",
			wantOK:     false,
		},
		{
			name:       "message_changed subtype ignored",
			payload:    `{"event":{"type":"message","subtype":"message_changed","channel":"C42","ts":"1.0"}}`,
			selfUserID: "UBOT",
			wantOK:     false,
		},
		{
			name:       "non-message event ignored",
			payload:    `{"event":{"type":"reaction_added","user":"U777","channel":"C42"}}`,
			selfUserID: "UBOT",
			wantOK:     false,
		},
		{
			name:       "missing channel dropped",
			payload:    `{"event":{"type":"message","user":"U777","text":"where am i","ts":"1.0"}}`,
			selfUserID: "UBOT",
			wantOK:     false,
		},
		{
			name:       "malformed JSON dropped without panic",
			payload:    `{"event":{"type":"message","user":`,
			selfUserID: "UBOT",
			wantOK:     false,
		},
		{
			name:       "wrong JSON shape dropped without panic",
			payload:    `[1,2,3]`,
			selfUserID: "UBOT",
			wantOK:     false,
		},
		{
			name:       "empty payload dropped without panic",
			payload:    ``,
			selfUserID: "UBOT",
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, ok := normalizeEvent([]byte(tt.payload), tt.selfUserID)
			if ok != tt.wantOK {
				t.Fatalf("normalizeEvent ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if msg.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", msg.Content, tt.wantContent)
			}
			if msg.Metadata["channel"] != tt.wantChannel {
				t.Errorf("Metadata[channel] = %q, want %q", msg.Metadata["channel"], tt.wantChannel)
			}
			if msg.Metadata["user"] != tt.wantUser {
				t.Errorf("Metadata[user] = %q, want %q", msg.Metadata["user"], tt.wantUser)
			}
			if msg.Type != connectors.TypeSlack || msg.Direction != connectors.Inbound {
				t.Errorf("Type/Direction = %q/%q", msg.Type, msg.Direction)
			}
		})
	}
}

func TestSlackTS(t *testing.T) {
	got := slackTS("1712345678.001200")
	want := time.Unix(1712345678, 1200000).UTC()
	if !got.Equal(want) {
		t.Errorf("slackTS = %v, want %v", got, want)
	}
	// Garbage falls back to ~now rather than zero.
	if slackTS("not-a-ts").IsZero() {
		t.Error("slackTS fallback returned zero time")
	}
}

func TestBuildAck(t *testing.T) {
	raw := buildAck("env-123")
	var parsed map[string]string
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("ack is not valid JSON: %v", err)
	}
	if parsed["envelope_id"] != "env-123" {
		t.Fatalf("ack envelope_id = %q, want env-123", parsed["envelope_id"])
	}
	if len(parsed) != 1 {
		t.Fatalf("ack has extra fields: %v", parsed)
	}
}

func TestParseEnvelope_Malformed(t *testing.T) {
	for _, raw := range []string{`{`, ``, `"string"`, `[1]`} {
		if _, err := parseEnvelope([]byte(raw)); err == nil {
			t.Errorf("parseEnvelope(%q) expected error", raw)
		}
	}
}

// TestSocketModeEndToEnd runs a full inbound path against an in-process
// Socket Mode server: auth.test + apps.connections.open over httptest,
// then a real WebSocket serving one envelope. Verifies the envelope is
// acked and the message lands normalized on the Receive channel.
func TestSocketModeEndToEnd(t *testing.T) {
	acked := make(chan string, 1)

	// WebSocket endpoint standing in for wss://...slack.com.
	wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")
		ctx := r.Context()

		envelope := `{"envelope_id":"env-1","type":"events_api","payload":` +
			`{"team_id":"T1","event":{"type":"message","user":"U777","text":"ping",` +
			`"channel":"C42","ts":"1712345678.000100"}}}`
		if err := conn.Write(ctx, websocket.MessageText, []byte(envelope)); err != nil {
			return
		}

		_, raw, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var ack struct {
			EnvelopeID string `json:"envelope_id"`
		}
		_ = json.Unmarshal(raw, &ack)
		acked <- ack.EnvelopeID

		// Hold the connection open until the client goes away.
		_, _, _ = conn.Read(ctx)
	}))
	defer wsSrv.Close()

	wsURL := "ws" + wsSrv.URL[len("http"):]

	// Web API endpoint for auth.test and apps.connections.open.
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth.test":
			_, _ = w.Write([]byte(`{"ok":true,"user_id":"UBOT"}`))
		case "/apps.connections.open":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "url": wsURL})
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	c := &Connector{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.Connect(ctx, map[string]string{
		"bot_token": "xoxb-test",
		"app_token": "xapp-test",
		"api_base":  apiSrv.URL,
	}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = c.Disconnect(context.Background()) }()

	caps := c.Capabilities()
	if len(caps) != 2 || caps[1] != "receive" {
		t.Fatalf("Capabilities = %v, want [send receive]", caps)
	}

	ch, err := c.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	select {
	case msg := <-ch:
		if msg.Content != "ping" || msg.Metadata["channel"] != "C42" {
			t.Fatalf("unexpected message: %+v", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no inbound message within 5s")
	}

	select {
	case id := <-acked:
		if id != "env-1" {
			t.Fatalf("acked envelope %q, want env-1", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("envelope was never acked")
	}
}

// TestReceive_NoAppToken_SilentOpenChannel checks the clean-skip path:
// without an app token, Receive returns an open channel that never
// yields, and capabilities stay outbound-only.
func TestReceive_NoAppToken_SilentOpenChannel(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"user_id":"UBOT"}`))
	}))
	defer apiSrv.Close()

	// Ensure the env fallback doesn't kick in from the host environment.
	t.Setenv(envAppToken, "")

	c := &Connector{}
	ctx := context.Background()
	if err := c.Connect(ctx, map[string]string{
		"bot_token": "xoxb-test",
		"api_base":  apiSrv.URL,
	}); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if caps := c.Capabilities(); len(caps) != 1 || caps[0] != "send" {
		t.Fatalf("Capabilities = %v, want [send]", caps)
	}

	ch, err := c.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	select {
	case msg, ok := <-ch:
		t.Fatalf("channel yielded (%v, %v); want silence", msg, ok)
	case <-time.After(50 * time.Millisecond):
	}

	// Disconnect closes the channel so the pump exits.
	if err := c.Disconnect(ctx); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected closed channel after Disconnect")
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after Disconnect")
	}
}
