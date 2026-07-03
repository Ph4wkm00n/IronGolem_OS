// Signal inbound via a long-running `signal-cli receive` subprocess.
// v0.4 adoption wave.
//
// One worker session = one subprocess:
//
//	signal-cli -u <account> receive --timeout -1 --output=json
//
// signal-cli streams NDJSON envelopes on stdout; each line carrying a
// dataMessage is normalized into a connectors.Message. When the process
// exits (signal-cli crash, account hiccup, network), the session
// returns an error and connectors.RunWorker restarts it with capped
// exponential backoff. Context cancellation kills the subprocess via
// exec.CommandContext.
package signal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"time"

	connectors "github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

// maxLineBytes bounds a single NDJSON line. signal-cli envelopes are
// small; a line this large is malformed or hostile and gets dropped by
// the scanner erroring out (session restarts with backoff).
const maxLineBytes = 1 << 20 // 1 MiB

// receiveEnvelope mirrors the signal-cli --output=json receive shape,
// reduced to the fields the normalizer consumes.
type receiveEnvelope struct {
	Envelope struct {
		Source       string `json:"source"`
		SourceNumber string `json:"sourceNumber"`
		SourceName   string `json:"sourceName"`
		Timestamp    int64  `json:"timestamp"` // epoch millis
		DataMessage  *struct {
			Timestamp int64  `json:"timestamp"`
			Message   string `json:"message"`
			GroupInfo *struct {
				GroupID string `json:"groupId"`
			} `json:"groupInfo"`
		} `json:"dataMessage"`
	} `json:"envelope"`
	Account string `json:"account"`
}

// normalizeLine turns one signal-cli NDJSON line into a
// connectors.Message. Returns (nil, false) for receipts, typing
// indicators, empty messages, and malformed JSON — dropped, never
// fatal, so a hostile line can't take the worker down.
func normalizeLine(line []byte) (*connectors.Message, bool) {
	var env receiveEnvelope
	if err := json.Unmarshal(line, &env); err != nil {
		return nil, false
	}
	dm := env.Envelope.DataMessage
	if dm == nil || dm.Message == "" {
		// Read receipts, typing notifications, sync messages, etc.
		return nil, false
	}
	source := env.Envelope.SourceNumber
	if source == "" {
		source = env.Envelope.Source
	}
	if source == "" {
		return nil, false
	}

	ts := dm.Timestamp
	if ts == 0 {
		ts = env.Envelope.Timestamp
	}

	msg := &connectors.Message{
		ID:        fmt.Sprintf("signal_%s_%s", source, strconv.FormatInt(ts, 10)),
		Type:      connectors.TypeSignal,
		Direction: connectors.Inbound,
		Content:   dm.Message,
		Metadata: map[string]string{
			// "recipient" matches the key Send expects, so a reply can
			// reuse the inbound metadata unchanged.
			"recipient": source,
			"source":    source,
		},
		Timestamp: signalTS(ts),
	}
	if env.Envelope.SourceName != "" {
		msg.Metadata["source_name"] = env.Envelope.SourceName
	}
	if dm.GroupInfo != nil && dm.GroupInfo.GroupID != "" {
		msg.Metadata["group_id"] = dm.GroupInfo.GroupID
	}
	return msg, true
}

// signalTS converts signal-cli epoch-millisecond timestamps. Zero or
// negative values fall back to now so a weird timestamp never drops an
// otherwise valid message.
func signalTS(ms int64) time.Time {
	if ms <= 0 {
		return time.Now().UTC()
	}
	return time.UnixMilli(ms).UTC()
}

// runReceiveSession runs one `signal-cli receive` subprocess to
// completion, feeding normalized messages into msgCh. Returning an
// error hands control back to the worker, which restarts with backoff.
func (c *Connector) runReceiveSession(ctx context.Context) error {
	c.mu.RLock()
	cli := c.cliPath
	account := c.accountName
	msgCh := c.msgCh
	c.mu.RUnlock()

	cmd := exec.CommandContext(ctx, cli,
		"-u", account,
		"receive",
		"--timeout", "-1",
		"--output=json",
	)
	// Discard stderr: signal-cli logs progress there, and an unread
	// pipe would eventually block the process.
	cmd.Stderr = io.Discard
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("signal: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("signal: start signal-cli receive: %w", err)
	}
	// Always reap the subprocess, even on early return paths.
	defer func() { _ = cmd.Wait() }()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), maxLineBytes)
	for scanner.Scan() {
		msg, ok := normalizeLine(scanner.Bytes())
		if !ok {
			continue
		}
		select {
		case msgCh <- msg:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("signal: receive stream: %w", err)
	}
	// EOF: signal-cli exited. Treat as an abnormal session end so the
	// worker restarts it (receive --timeout -1 should never exit).
	return fmt.Errorf("signal: signal-cli receive exited")
}
