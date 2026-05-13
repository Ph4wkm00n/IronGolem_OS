// Command smoke-telegram is the v0.2 Step 1 verification harness (Gate 4 of
// Plans/v0.2-foundation.md). It stands up an in-process httptest server
// impersonating the Telegram Bot API, boots a gateway subprocess pointed at
// that impersonator, drives a fake inbound update through it, and asserts
// that the gateway's outbound path landed a `sendMessage` call back at the
// impersonator with the expected `chat_id` and reply text.
//
// Wire:
//
//	[impersonator] /bot<TOK>/getMe       — Connect verifies the bot id
//	[impersonator] /bot<TOK>/getUpdates  — first long-poll delivers one inbound
//	[impersonator] /bot<TOK>/sendMessage — outbound assertion target
//
// Exits 0 only when sendMessage receives the expected payload within the
// configured timeout; otherwise prints the gateway log tail to stderr and
// exits 1. Wired as `make smoke-telegram`.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// expected reply text the gateway should produce. Configured via the
// MockProvider's IRONGOLEM_LLM_MOCK_RESPONSE env var.
const expectedReply = "pong"

// expected chat id the fake update originates from.
const fakeChatID int64 = 1001

// fake bot id returned by getMe.
const fakeBotID int64 = 42

func main() {
	gatewayBin := flag.String("gateway-bin", "", "path to the gateway binary (required)")
	runtimedBin := flag.String("runtimed-bin", "", "path to the runtimed binary (required)")
	port := flag.Int("port", 18100, "port the spawned gateway listens on")
	timeout := flag.Duration("timeout", 30*time.Second, "max wall-clock for the whole smoke")
	flag.Parse()

	if *gatewayBin == "" || *runtimedBin == "" {
		fmt.Fprintln(os.Stderr, "usage: smoke-telegram --gateway-bin=PATH --runtimed-bin=PATH [--port N] [--timeout DUR]")
		os.Exit(2)
	}

	if err := run(*gatewayBin, *runtimedBin, *port, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "smoke-telegram · FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("smoke-telegram · ALL GATES PASSED")
}

// recorder captures the sendMessage call the impersonator receives.
type recorder struct {
	mu      sync.Mutex
	got     []sendMessage
	updated bool // flipped after the first getUpdates poll delivers the fake update
}

type sendMessage struct {
	ChatID int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func run(gatewayBin, runtimedBin string, port int, timeout time.Duration) error {
	rec := &recorder{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Telegram URLs look like /bot<token>/<method>. We accept any
		// token here — the test binary owns both ends of the connection.
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/getMe"):
			writeOK(w, map[string]any{
				"id":       fakeBotID,
				"is_bot":   true,
				"username": "irongolem_smoke_bot",
			})
		case strings.HasSuffix(path, "/getUpdates"):
			rec.mu.Lock()
			if rec.updated {
				// Second + subsequent polls return empty so the connector
				// loop doesn't spin.
				rec.mu.Unlock()
				writeOK(w, []any{})
				return
			}
			rec.updated = true
			rec.mu.Unlock()
			writeOK(w, []map[string]any{{
				"update_id": 1,
				"message": map[string]any{
					"message_id": 100,
					"from": map[string]any{
						"id":       100,
						"is_bot":   false,
						"username": "smoke_user",
					},
					"chat": map[string]any{
						"id":   fakeChatID,
						"type": "private",
					},
					"date": time.Now().Unix(),
					"text": "ping",
				},
			}})
		case strings.HasSuffix(path, "/sendMessage"):
			body, _ := io.ReadAll(r.Body)
			var msg sendMessage
			if err := json.Unmarshal(body, &msg); err != nil {
				http.Error(w, "bad sendMessage body", http.StatusBadRequest)
				return
			}
			rec.mu.Lock()
			rec.got = append(rec.got, msg)
			rec.mu.Unlock()
			writeOK(w, map[string]any{"message_id": 999})
		default:
			http.Error(w, "unhandled "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Working dir for the gateway subprocess (db file + log tail).
	workDir, err := os.MkdirTemp("", "smoke-telegram-*")
	if err != nil {
		return fmt.Errorf("mkdtemp: %w", err)
	}
	defer os.RemoveAll(workDir)

	logPath := filepath.Join(workDir, "gateway.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer logFile.Close()

	hmacSecret := fmt.Sprintf("smoke-telegram-%d", time.Now().UnixNano())

	cmd := exec.Command(gatewayBin)
	cmd.Env = append(os.Environ(),
		"IRONGOLEM_HMAC_SECRET="+hmacSecret,
		"IRONGOLEM_GATEWAY_DB="+filepath.Join(workDir, "gateway.db"),
		"IRONGOLEM_RUNTIMED_PATH="+runtimedBin,
		"IRONGOLEM_LLM_PROVIDER=mock",
		"IRONGOLEM_LLM_MOCK_RESPONSE="+expectedReply,
		"GATEWAY_ADDR="+fmt.Sprintf(":%d", port),
		"DEPLOYMENT_MODE=solo",
		// Telegram source — token can be anything; impersonator ignores it.
		"IRONGOLEM_TELEGRAM_BOT_TOKEN=smoke-bot-token",
		"IRONGOLEM_TELEGRAM_API_BASE="+server.URL,
		"IRONGOLEM_TELEGRAM_CONNECTOR_ID=telegram-smoke",
		// Pre-authorize the fake chat so the connector accepts inbound + outbound.
		"IRONGOLEM_TELEGRAM_ALLOWED_CHAT_IDS=1001",
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start gateway: %w", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(os.Interrupt)
			done := make(chan struct{})
			go func() { _, _ = cmd.Process.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				_ = cmd.Process.Kill()
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Wait for /healthz to come up — gateway boot includes runtimed spawn
	// + db open + Telegram Connect call, so first-byte latency varies.
	healthURL := fmt.Sprintf("http://localhost:%d/healthz", port)
	if err := waitFor(ctx, func() bool {
		resp, err := http.Get(healthURL)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}); err != nil {
		return fmt.Errorf("gateway never reached /healthz: %w; log tail:\n%s", err, tailLog(logPath))
	}
	fmt.Println("smoke-telegram · gateway healthy")

	// Wait for the impersonator to record a sendMessage. The gateway
	// inbound pump polls getUpdates, gets our fake update, runs it through
	// the planner + mock runtimed, and ships the reply via telegramSource.Send.
	if err := waitFor(ctx, func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		return len(rec.got) > 0
	}); err != nil {
		return fmt.Errorf("sendMessage never landed: %w; log tail:\n%s", err, tailLog(logPath))
	}

	rec.mu.Lock()
	got := append([]sendMessage(nil), rec.got...)
	rec.mu.Unlock()

	if len(got) != 1 {
		return fmt.Errorf("expected exactly 1 sendMessage, got %d: %+v", len(got), got)
	}
	if got[0].ChatID != fakeChatID {
		return fmt.Errorf("sendMessage chat_id mismatch: got %d, want %d", got[0].ChatID, fakeChatID)
	}
	if got[0].Text != expectedReply {
		return fmt.Errorf("sendMessage text mismatch: got %q, want %q", got[0].Text, expectedReply)
	}
	fmt.Printf("smoke-telegram · ✅ sendMessage(chat_id=%d, text=%q)\n", got[0].ChatID, got[0].Text)
	return nil
}

func writeOK(w http.ResponseWriter, result any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"result": result,
	})
}

func waitFor(ctx context.Context, predicate func() bool) error {
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		if predicate() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func tailLog(path string) string {
	const maxBytes = 4096
	f, err := os.Open(path)
	if err != nil {
		return "(no log)"
	}
	defer f.Close()
	stat, _ := f.Stat()
	size := stat.Size()
	if size > maxBytes {
		_, _ = f.Seek(size-maxBytes, io.SeekStart)
	}
	buf, _ := io.ReadAll(f)
	return string(buf)
}

// Stuff `runtime` keeps the import non-empty if a future refactor wants
// platform-specific behavior (e.g. signal handling on Windows).
var _ = runtime.GOOS
