// Package runtime is the gateway's NDJSON client for the runtimed child
// process. The gateway spawns runtimed once at boot, multiplexes plan
// execution + ping requests over its stdin/stdout, and restarts the
// child with exponential backoff if it crashes.
//
// Concurrency model:
//   - One writer goroutine drains an outbound channel onto child stdin.
//     All gateway code sends via Execute / Ping / Shutdown — never writes
//     directly to the process pipe.
//   - One reader goroutine consumes the child's stdout line by line and
//     routes each NDJSON message to the caller waiting on its request_id.
//   - One supervisor goroutine waits on Cmd.Wait(); on exit it cancels
//     all in-flight requests and re-spawns the child (up to 5 attempts
//     with backoff 1,2,4,8,16s). After the cap the client moves to
//     state=dead and every subsequent call returns ErrRuntimeUnavailable.
//
// The wire types live in services/pkg/runtime/types.go and mirror
// runtime/core/src/ipc.rs on the Rust side.
package runtime

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"time"

	ipc "github.com/Ph4wkm00n/IronGolem_OS/services/pkg/runtime"
)

// ErrRuntimeUnavailable is returned when the runtimed child has exhausted
// its restart budget. Callers should treat this as a hard outage signal.
var ErrRuntimeUnavailable = errors.New("runtime: child process unavailable")

// ErrClosed is returned when the client has been shut down explicitly.
var ErrClosed = errors.New("runtime: client closed")

// Config captures everything Client.New needs to spawn runtimed.
type Config struct {
	// BinaryPath is the absolute or PATH-relative path to runtimed.
	BinaryPath string
	// Env are extra environment variables passed to the child (e.g.
	// IRONGOLEM_LLM_PROVIDER=mock for tests). Parent env is inherited.
	Env []string
	// MaxRestartAttempts caps the supervisor's restart loop. Defaults to 5.
	MaxRestartAttempts int
	// BaseBackoff is the initial restart delay; doubles per attempt.
	// Defaults to 1s.
	BaseBackoff time.Duration
	// PingTimeout bounds a single Ping call. Defaults to 5s.
	PingTimeout time.Duration
}

// pendingRequest tracks one in-flight Execute or Ping call so the reader
// goroutine can deliver responses by request_id.
type pendingRequest struct {
	// kind is the wire-tag of the request issued. Used by the reader to
	// reject mismatched response types.
	kind string
	// events receives EventNotification objects (Execute only). Nil for Ping.
	events chan ipc.EventNotification
	// done receives the terminal response and is closed afterward.
	// For Execute it carries ipc.ExecutePlanResponse; for Ping ipc.PingResponse.
	done chan terminalResponse
}

type terminalResponse struct {
	execute       *ipc.ExecutePlanResponse
	ping          *ipc.PingResponse
	listProviders *ipc.ListProvidersResponse // v0.3 Step 3
	err           error
}

// Client owns the runtimed child process and the IPC plumbing.
type Client struct {
	cfg    Config
	logger *slog.Logger

	// outbound holds messages that the writer goroutine drains onto stdin.
	// Buffered so callers don't block on transient writer back-pressure.
	outbound chan []byte

	// pending maps request_id -> in-flight call. Guarded by pendingMu.
	pendingMu sync.Mutex
	pending   map[string]*pendingRequest

	// supervisorDone closes once the supervisor goroutine has stopped
	// trying to keep the child alive (either after ExitClose or after the
	// restart budget is exhausted).
	supervisorDone chan struct{}

	// stop signals the supervisor + writer + reader to wind down. Closed
	// exactly once by Close().
	stop chan struct{}

	// closeOnce ensures stop is only closed once.
	closeOnce sync.Once

	// state captures the supervisor's view of the child. Reads are
	// cheap; writes go through stateMu.
	stateMu sync.RWMutex
	state   clientState
}

type clientState int

const (
	stateBooting clientState = iota
	stateRunning
	stateRestarting
	stateDead
	stateClosed
)

// New spawns runtimed and starts the goroutines that own its I/O. The
// caller MUST invoke Close() to release resources.
func New(ctx context.Context, cfg Config, logger *slog.Logger) (*Client, error) {
	if cfg.BinaryPath == "" {
		return nil, errors.New("runtime: BinaryPath required")
	}
	if cfg.MaxRestartAttempts == 0 {
		cfg.MaxRestartAttempts = 5
	}
	if cfg.BaseBackoff == 0 {
		cfg.BaseBackoff = time.Second
	}
	if cfg.PingTimeout == 0 {
		cfg.PingTimeout = 5 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}

	c := &Client{
		cfg:            cfg,
		logger:         logger.With(slog.String("component", "runtime.client")),
		outbound:       make(chan []byte, 64),
		pending:        make(map[string]*pendingRequest),
		supervisorDone: make(chan struct{}),
		stop:           make(chan struct{}),
		state:          stateBooting,
	}

	// First spawn happens synchronously so callers see a "boot failed"
	// error rather than a phantom ErrRuntimeUnavailable later.
	cmd, stdin, stdout, err := c.spawn()
	if err != nil {
		return nil, fmt.Errorf("runtime: initial spawn: %w", err)
	}
	c.setState(stateRunning)

	go c.supervise(ctx, cmd, stdin, stdout)

	return c, nil
}

// spawn forks runtimed and returns its handles. The caller is responsible
// for closing stdin/stdout when the child exits.
func (c *Client) spawn() (*exec.Cmd, io.WriteCloser, io.ReadCloser, error) {
	cmd := exec.Command(c.cfg.BinaryPath)
	if len(c.cfg.Env) > 0 {
		cmd.Env = append(cmd.Env, c.cfg.Env...)
	}
	// Forward child stderr to our own — runtimed logs there in JSON.
	cmd.Stderr = stderrTeeWriter{logger: c.logger}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, nil, nil, fmt.Errorf("start runtimed: %w", err)
	}
	c.logger.Info("runtimed spawned", slog.Int("pid", cmd.Process.Pid))
	return cmd, stdin, stdout, nil
}

// supervise owns the writer + reader goroutines for one child lifecycle
// and re-spawns on crash up to the restart budget.
func (c *Client) supervise(ctx context.Context, cmd *exec.Cmd, stdin io.WriteCloser, stdout io.ReadCloser) {
	defer close(c.supervisorDone)

	attempts := 0
	for {
		// Per-child wait group: writer + reader must exit before we
		// restart so the next pair gets fresh pipes.
		childWG := &sync.WaitGroup{}
		childWG.Add(2)
		writerDone := make(chan struct{})
		readerDone := make(chan struct{})

		go func() {
			defer childWG.Done()
			defer close(writerDone)
			c.runWriter(stdin)
		}()
		go func() {
			defer childWG.Done()
			defer close(readerDone)
			c.runReader(stdout)
		}()

		// Wait for the child to exit, the context to cancel, or Close.
		waitErr := make(chan error, 1)
		go func() { waitErr <- cmd.Wait() }()

		select {
		case err := <-waitErr:
			c.logger.Warn("runtimed exited",
				slog.Int("pid", cmd.Process.Pid),
				slog.String("error", errString(err)),
			)
		case <-ctx.Done():
			c.logger.Info("supervisor context cancelled; terminating child")
			_ = cmd.Process.Kill()
			<-waitErr
			c.failAllPending(ctx.Err())
			c.setState(stateClosed)
			return
		case <-c.stop:
			c.logger.Info("Close() called; terminating child")
			_ = cmd.Process.Kill()
			<-waitErr
			c.failAllPending(ErrClosed)
			c.setState(stateClosed)
			return
		}

		// Child exited unexpectedly. Drain writer/reader, fail in-flight
		// requests, then attempt restart.
		_ = stdin.Close()
		_ = stdout.Close()
		<-writerDone
		<-readerDone
		c.failAllPending(fmt.Errorf("runtime: child exited"))

		attempts++
		if attempts >= c.cfg.MaxRestartAttempts {
			c.logger.Error("runtime restart budget exhausted",
				slog.Int("attempts", attempts),
			)
			c.setState(stateDead)
			return
		}

		backoff := c.cfg.BaseBackoff << (attempts - 1)
		c.logger.Warn("scheduling runtime restart",
			slog.Int("attempt", attempts),
			slog.Duration("backoff", backoff),
		)
		c.setState(stateRestarting)
		select {
		case <-time.After(backoff):
		case <-c.stop:
			c.setState(stateClosed)
			return
		case <-ctx.Done():
			c.setState(stateClosed)
			return
		}

		var err error
		cmd, stdin, stdout, err = c.spawn()
		if err != nil {
			c.logger.Error("respawn failed", slog.String("error", err.Error()))
			// Spawn failure counts toward the budget but doesn't burn it
			// instantly — loop will reattempt after a fresh backoff.
			continue
		}
		c.setState(stateRunning)
	}
}

// runWriter drains the outbound channel onto child stdin one NDJSON line
// at a time. Exits when stdin closes (child crash) or stop fires.
func (c *Client) runWriter(stdin io.WriteCloser) {
	for {
		select {
		case msg, ok := <-c.outbound:
			if !ok {
				return
			}
			// NDJSON: every payload terminated by a single \n.
			if _, err := stdin.Write(append(msg, '\n')); err != nil {
				c.logger.Warn("stdin write failed", slog.String("error", err.Error()))
				return
			}
		case <-c.stop:
			return
		}
	}
}

// runReader parses NDJSON from child stdout and routes each message to
// the pending request by request_id.
func (c *Client) runReader(stdout io.ReadCloser) {
	scanner := bufio.NewScanner(stdout)
	// Default Scanner buf is 64 KB; bump to 1 MB for long event payloads.
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var env ipc.Envelope
		if err := json.Unmarshal(line, &env); err != nil {
			c.logger.Warn("failed to parse envelope",
				slog.String("error", err.Error()),
				slog.Int("line_len", len(line)),
			)
			continue
		}

		switch env.Kind {
		case ipc.KindExecutePlanResponse:
			var resp ipc.ExecutePlanResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				c.logger.Warn("bad execute response", slog.String("error", err.Error()))
				continue
			}
			c.deliverTerminal(resp.RequestID, terminalResponse{execute: &resp})

		case ipc.KindEventNotification:
			var evt ipc.EventNotification
			if err := json.Unmarshal(line, &evt); err != nil {
				c.logger.Warn("bad event notification", slog.String("error", err.Error()))
				continue
			}
			c.deliverEvent(evt)

		case ipc.KindPingResponse:
			var resp ipc.PingResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				c.logger.Warn("bad ping response", slog.String("error", err.Error()))
				continue
			}
			c.deliverTerminal(resp.RequestID, terminalResponse{ping: &resp})

		case ipc.KindListProvidersResponse:
			var resp ipc.ListProvidersResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				c.logger.Warn("bad list_providers response", slog.String("error", err.Error()))
				continue
			}
			c.deliverTerminal(resp.RequestID, terminalResponse{listProviders: &resp})

		default:
			c.logger.Warn("unknown message kind", slog.String("kind", env.Kind))
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		c.logger.Warn("stdout scanner error", slog.String("error", err.Error()))
	}
}

// deliverEvent routes an EventNotification to its waiting Execute call.
// If no caller is waiting (rare race during shutdown), the event is logged
// and dropped — we never block the reader.
func (c *Client) deliverEvent(evt ipc.EventNotification) {
	c.pendingMu.Lock()
	pr, ok := c.pending[evt.RequestID]
	c.pendingMu.Unlock()
	if !ok || pr.events == nil {
		c.logger.Debug("event dropped (no waiter)",
			slog.String("request_id", evt.RequestID),
		)
		return
	}
	select {
	case pr.events <- evt:
	default:
		// Caller's buffer is full; drop oldest by skipping. Production
		// callers should size the buffer for their max burst, but we
		// refuse to block the reader for any single slow consumer.
		c.logger.Warn("event channel full; dropping notification",
			slog.String("request_id", evt.RequestID),
		)
	}
}

// nilUUID is the canonical "no request id" marker runtimed uses when it
// can't even parse the incoming envelope. We detect it explicitly so we
// can fail in-flight callers instead of letting them time out.
const nilUUID = "00000000-0000-0000-0000-000000000000"

// deliverTerminal closes out a pending call by request_id. A response
// carrying the nil request_id is treated as an envelope-level parse
// failure from runtimed (it had nothing else to put in the field) and
// fails ALL in-flight requests so callers see a useful error rather
// than a timeout.
func (c *Client) deliverTerminal(reqID string, t terminalResponse) {
	if reqID == nilUUID {
		err := fmt.Errorf("runtime: child rejected request envelope")
		if t.execute != nil && t.execute.Error != "" {
			err = fmt.Errorf("runtime: %s", t.execute.Error)
		}
		c.logger.Warn("runtime returned nil-uuid error; failing all in-flight requests",
			slog.String("reason", err.Error()),
		)
		c.failAllPending(err)
		return
	}

	c.pendingMu.Lock()
	pr, ok := c.pending[reqID]
	if ok {
		delete(c.pending, reqID)
	}
	c.pendingMu.Unlock()
	if !ok {
		c.logger.Warn("terminal response without waiter",
			slog.String("request_id", reqID),
		)
		return
	}
	pr.done <- t
	close(pr.done)
	if pr.events != nil {
		close(pr.events)
	}
}

// failAllPending fails every in-flight call with err. Called on child
// crash, shutdown, or supervisor exit.
func (c *Client) failAllPending(err error) {
	c.pendingMu.Lock()
	pending := c.pending
	c.pending = make(map[string]*pendingRequest)
	c.pendingMu.Unlock()

	for id, pr := range pending {
		c.logger.Debug("failing pending request",
			slog.String("request_id", id),
			slog.String("error", err.Error()),
		)
		pr.done <- terminalResponse{err: err}
		close(pr.done)
		if pr.events != nil {
			close(pr.events)
		}
	}
}

// register adds a pending request, returning an error if the client is
// no longer accepting work.
func (c *Client) register(reqID string, pr *pendingRequest) error {
	if !c.acceptingWork() {
		return ErrRuntimeUnavailable
	}
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if _, dup := c.pending[reqID]; dup {
		return fmt.Errorf("runtime: duplicate request id %s", reqID)
	}
	c.pending[reqID] = pr
	return nil
}

func (c *Client) acceptingWork() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state == stateRunning || c.state == stateBooting || c.state == stateRestarting
}

func (c *Client) setState(s clientState) {
	c.stateMu.Lock()
	c.state = s
	c.stateMu.Unlock()
}

// Ping sends a PingRequest and returns once the matching PingResponse
// arrives. Returns ErrRuntimeUnavailable if the child is dead, context
// error if ctx is cancelled, or wraps any transport error.
func (c *Client) Ping(ctx context.Context) error {
	if !c.acceptingWork() {
		return ErrRuntimeUnavailable
	}
	if c.cfg.PingTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.PingTimeout)
		defer cancel()
	}

	reqID := c.newID()
	pr := &pendingRequest{
		kind: ipc.KindPingRequest,
		done: make(chan terminalResponse, 1),
	}
	if err := c.register(reqID, pr); err != nil {
		return err
	}

	req := ipc.PingRequest{Kind: ipc.KindPingRequest, RequestID: reqID}
	body, err := json.Marshal(req)
	if err != nil {
		c.unregister(reqID)
		return fmt.Errorf("runtime: marshal ping: %w", err)
	}

	if err := c.send(ctx, body); err != nil {
		c.unregister(reqID)
		return err
	}

	select {
	case t := <-pr.done:
		if t.err != nil {
			return t.err
		}
		if t.ping == nil || t.ping.RequestID != reqID {
			return errors.New("runtime: ping response mismatch")
		}
		return nil
	case <-ctx.Done():
		c.unregister(reqID)
		return ctx.Err()
	}
}

// ListProviders sends a ListProvidersRequest and returns once runtimed
// responds. Mirrors Ping's shape — no events, single terminal response.
// v0.3 Step 3 of Plans/modular-puzzling-blum.md.
func (c *Client) ListProviders(ctx context.Context) (*ipc.ListProvidersResponse, error) {
	if !c.acceptingWork() {
		return nil, ErrRuntimeUnavailable
	}
	// Reuse the ping timeout — this is a cheap synchronous call, the
	// runtime is doing no work beyond serializing a static list.
	if c.cfg.PingTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.PingTimeout)
		defer cancel()
	}

	reqID := c.newID()
	pr := &pendingRequest{
		kind: ipc.KindListProvidersRequest,
		done: make(chan terminalResponse, 1),
	}
	if err := c.register(reqID, pr); err != nil {
		return nil, err
	}

	req := ipc.ListProvidersRequest{Kind: ipc.KindListProvidersRequest, RequestID: reqID}
	body, err := json.Marshal(req)
	if err != nil {
		c.unregister(reqID)
		return nil, fmt.Errorf("runtime: marshal list_providers: %w", err)
	}

	if err := c.send(ctx, body); err != nil {
		c.unregister(reqID)
		return nil, err
	}

	select {
	case t := <-pr.done:
		if t.err != nil {
			return nil, t.err
		}
		if t.listProviders == nil || t.listProviders.RequestID != reqID {
			return nil, errors.New("runtime: list_providers response mismatch")
		}
		return t.listProviders, nil
	case <-ctx.Done():
		c.unregister(reqID)
		return nil, ctx.Err()
	}
}

// ExecuteResult is what Execute returns: a channel of streamed events
// (closed when the run terminates) and a future for the terminal response.
type ExecuteResult struct {
	Events <-chan ipc.EventNotification
	// Done returns the terminal response. It blocks until the run completes,
	// the context cancels, or the child crashes.
	Done <-chan ExecuteTerminal
}

// ExecuteTerminal carries either the runtime's terminal response or a
// transport-level error. Exactly one of Response / Err is non-nil.
type ExecuteTerminal struct {
	Response *ipc.ExecutePlanResponse
	Err      error
}

// Execute sends an ExecutePlanRequest. The caller gets a stream of
// EventNotifications and a future for the terminal response. The caller
// MUST drain Events (or rely on the reader's drop-on-full safety net).
//
// The events channel is closed when the terminal response is delivered.
func (c *Client) Execute(ctx context.Context, workspaceID string, plan ipc.Plan) (*ExecuteResult, error) {
	if !c.acceptingWork() {
		return nil, ErrRuntimeUnavailable
	}
	reqID := c.newID()
	events := make(chan ipc.EventNotification, 64)
	done := make(chan terminalResponse, 1)

	pr := &pendingRequest{
		kind:   ipc.KindExecutePlanRequest,
		events: events,
		done:   done,
	}
	if err := c.register(reqID, pr); err != nil {
		return nil, err
	}

	req := ipc.ExecutePlanRequest{
		Kind:        ipc.KindExecutePlanRequest,
		RequestID:   reqID,
		WorkspaceID: workspaceID,
		Plan:        plan,
	}
	body, err := json.Marshal(req)
	if err != nil {
		c.unregister(reqID)
		return nil, fmt.Errorf("runtime: marshal execute: %w", err)
	}
	if err := c.send(ctx, body); err != nil {
		c.unregister(reqID)
		return nil, err
	}

	// Adapt terminalResponse → ExecuteTerminal for the caller.
	out := make(chan ExecuteTerminal, 1)
	go func() {
		t, ok := <-done
		if !ok {
			out <- ExecuteTerminal{Err: ErrClosed}
			close(out)
			return
		}
		if t.err != nil {
			out <- ExecuteTerminal{Err: t.err}
		} else {
			out <- ExecuteTerminal{Response: t.execute}
		}
		close(out)
	}()

	return &ExecuteResult{Events: events, Done: out}, nil
}

// Close sends a Shutdown message to the child and waits for the
// supervisor to exit. Idempotent.
func (c *Client) Close(ctx context.Context) error {
	c.closeOnce.Do(func() {
		// Best-effort Shutdown so the child can drain in-flight requests
		// before exiting. Don't block forever — the supervisor will kill
		// the process if it doesn't honor the shutdown.
		reqID := c.newID()
		req := ipc.Shutdown{Kind: ipc.KindShutdown, RequestID: reqID}
		if body, err := json.Marshal(req); err == nil {
			ctxSend, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			_ = c.send(ctxSend, body)
		}
		close(c.stop)
	})

	select {
	case <-c.supervisorDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// send enqueues a payload onto the outbound channel.
func (c *Client) send(ctx context.Context, payload []byte) error {
	select {
	case c.outbound <- payload:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.stop:
		return ErrClosed
	}
}

func (c *Client) unregister(reqID string) {
	c.pendingMu.Lock()
	delete(c.pending, reqID)
	c.pendingMu.Unlock()
}

// newID returns a fresh request_id. runtimed parses this as a UUID v4 on
// the Rust side, so we emit the canonical 8-4-4-4-12 hex layout with the
// version + variant bits set. crypto/rand keeps the impl zero-dep.
func (c *Client) newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure on a healthy host is essentially impossible;
		// log and return a degraded-but-still-shaped UUID so the call
		// fails downstream rather than silently corrupting wire data.
		c.logger.Error("crypto/rand failed", slog.String("error", err.Error()))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	hexStr := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", hexStr[0:8], hexStr[8:12], hexStr[12:16], hexStr[16:20], hexStr[20:32])
}

// stderrTeeWriter forwards child-stderr bytes into the gateway logger so
// we never lose runtimed's structured logs.
type stderrTeeWriter struct {
	logger *slog.Logger
}

func (w stderrTeeWriter) Write(p []byte) (int, error) {
	// runtimed emits structured logs already; preserve as-is and let the
	// operator's log aggregator parse them. We trim trailing newlines so
	// slog doesn't double-stamp.
	msg := string(p)
	for len(msg) > 0 && (msg[len(msg)-1] == '\n' || msg[len(msg)-1] == '\r') {
		msg = msg[:len(msg)-1]
	}
	if msg != "" {
		w.logger.Info("runtimed.stderr", slog.String("line", msg))
	}
	return len(p), nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
