package discord

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

func TestGatewayIntentsBitmask(t *testing.T) {
	// GUILDS(1) + GUILD_MESSAGES(512) + DIRECT_MESSAGES(4096) +
	// MESSAGE_CONTENT(32768) = 37377.
	if gatewayIntents != 37377 {
		t.Fatalf("gatewayIntents = %d, want 37377", gatewayIntents)
	}
}

func TestBuildIdentify(t *testing.T) {
	raw := buildIdentify("token-abc")
	var parsed struct {
		Op int `json:"op"`
		D  struct {
			Token      string            `json:"token"`
			Intents    int               `json:"intents"`
			Properties map[string]string `json:"properties"`
		} `json:"d"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("IDENTIFY is not valid JSON: %v", err)
	}
	if parsed.Op != opIdentify {
		t.Errorf("op = %d, want %d", parsed.Op, opIdentify)
	}
	if parsed.D.Token != "token-abc" {
		t.Errorf("token = %q", parsed.D.Token)
	}
	if parsed.D.Intents != 37377 {
		t.Errorf("intents = %d, want 37377", parsed.D.Intents)
	}
	for _, key := range []string{"os", "browser", "device"} {
		if parsed.D.Properties[key] == "" {
			t.Errorf("properties[%q] empty", key)
		}
	}
}

func TestBuildHeartbeat(t *testing.T) {
	tests := []struct {
		name string
		seq  *int64
		want string
	}{
		{name: "null before first dispatch", seq: nil, want: `{"d":null,"op":1}`},
		{name: "carries last seq", seq: ptrInt64(42), want: `{"d":42,"op":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(buildHeartbeat(tt.seq)); got != tt.want {
				t.Fatalf("buildHeartbeat = %s, want %s", got, tt.want)
			}
		})
	}
}

func ptrInt64(v int64) *int64 { return &v }

func TestNormalizeMessageCreate(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		selfID      string
		wantOK      bool
		wantContent string
		wantChannel string
	}{
		{
			name: "plain user message",
			data: `{"id":"m1","channel_id":"ch9","guild_id":"g1","content":"hi there",` +
				`"timestamp":"2026-07-03T10:00:00.000000+00:00",` +
				`"author":{"id":"u5","username":"alice","bot":false}}`,
			selfID:      "self",
			wantOK:      true,
			wantContent: "hi there",
			wantChannel: "ch9",
		},
		{
			name:   "bot author ignored",
			data:   `{"id":"m1","channel_id":"ch9","content":"beep","author":{"id":"u5","bot":true}}`,
			selfID: "self",
			wantOK: false,
		},
		{
			name:   "own message ignored",
			data:   `{"id":"m1","channel_id":"ch9","content":"echo","author":{"id":"self","bot":false}}`,
			selfID: "self",
			wantOK: false,
		},
		{
			name:   "missing channel dropped",
			data:   `{"id":"m1","content":"lost","author":{"id":"u5"}}`,
			selfID: "self",
			wantOK: false,
		},
		{
			name:   "malformed JSON dropped without panic",
			data:   `{"id":"m1","channel_id":`,
			selfID: "self",
			wantOK: false,
		},
		{
			name:   "wrong JSON shape dropped without panic",
			data:   `"just a string"`,
			selfID: "self",
			wantOK: false,
		},
		{
			name:   "empty data dropped without panic",
			data:   ``,
			selfID: "self",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, ok := normalizeMessageCreate([]byte(tt.data), tt.selfID)
			if ok != tt.wantOK {
				t.Fatalf("normalizeMessageCreate ok = %v, want %v", ok, tt.wantOK)
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
			if msg.Type != connectors.TypeDiscord || msg.Direction != connectors.Inbound {
				t.Errorf("Type/Direction = %q/%q", msg.Type, msg.Direction)
			}
		})
	}
}

func TestParseGatewayEvent_Malformed(t *testing.T) {
	for _, raw := range []string{`{`, ``, `"string"`, `[1,2]`} {
		if _, err := parseGatewayEvent([]byte(raw)); err == nil {
			t.Errorf("parseGatewayEvent(%q) expected error", raw)
		}
	}
}

// TestGatewayEndToEnd runs a full inbound path against an in-process
// Gateway: /users/@me + /gateway/bot over httptest, then a WebSocket
// that sends HELLO, expects IDENTIFY with the right intents, and
// dispatches one MESSAGE_CREATE.
func TestGatewayEndToEnd(t *testing.T) {
	identified := make(chan int, 1)

	wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")
		ctx := r.Context()

		// HELLO with a long interval so the test never heartbeats.
		hello := `{"op":10,"d":{"heartbeat_interval":45000}}`
		if err := conn.Write(ctx, websocket.MessageText, []byte(hello)); err != nil {
			return
		}

		// Expect IDENTIFY.
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var ident struct {
			Op int `json:"op"`
			D  struct {
				Intents int `json:"intents"`
			} `json:"d"`
		}
		_ = json.Unmarshal(raw, &ident)
		if ident.Op == opIdentify {
			identified <- ident.D.Intents
		}

		// A malformed frame first — must be dropped, not fatal.
		_ = conn.Write(ctx, websocket.MessageText, []byte(`{not json`))

		dispatch := `{"op":0,"s":1,"t":"MESSAGE_CREATE","d":{"id":"m77","channel_id":"ch1",` +
			`"content":"gateway ping","timestamp":"2026-07-03T10:00:00Z",` +
			`"author":{"id":"u9","username":"bob","bot":false}}}`
		if err := conn.Write(ctx, websocket.MessageText, []byte(dispatch)); err != nil {
			return
		}

		// Hold the connection open until the client goes away.
		_, _, _ = conn.Read(ctx)
	}))
	defer wsSrv.Close()

	wsURL := "ws" + wsSrv.URL[len("http"):] + "?v=10&encoding=json"

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/@me":
			_, _ = w.Write([]byte(`{"id":"selfbot","username":"irongolem","bot":true}`))
		case "/gateway/bot":
			_ = json.NewEncoder(w).Encode(map[string]any{"url": wsURL})
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	c := &Connector{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := c.Connect(ctx, map[string]string{
		"bot_token": "bot-token",
		"api_base":  apiSrv.URL,
	}); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = c.Disconnect(context.Background()) }()

	ch, err := c.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}

	select {
	case intents := <-identified:
		if intents != 37377 {
			t.Fatalf("IDENTIFY intents = %d, want 37377", intents)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("connector never identified")
	}

	select {
	case msg := <-ch:
		if msg.Content != "gateway ping" || msg.Metadata["channel"] != "ch1" {
			t.Fatalf("unexpected message: %+v", msg)
		}
		if msg.Metadata["author_id"] != "u9" {
			t.Fatalf("author_id = %q, want u9", msg.Metadata["author_id"])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no inbound message within 5s")
	}
}
