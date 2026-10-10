// Package codeclient embeds `vivy-code --mode rpc` as a child process and
// speaks its stdin/stdout JSONL protocol: typed commands with id-correlated
// responses plus an ordered event stream.
//
// It is the Go-side answer to pi's subprocess embedding surface: a host
// program (IDE, orchestrator, script) starts a Client, sends commands, and
// consumes run events without parsing the wire itself.
package codeclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Config controls how the vivy-code child process is spawned.
type Config struct {
	// Binary is the vivy-code executable path (required).
	Binary string
	// Args are extra CLI args appended after `--mode rpc` (e.g. --session-id,
	// --model, --provider). Flags that change the mode are rejected.
	Args []string
	// Dir is the child working directory (session storage resolves there).
	Dir string
	// Env is appended to the child environment (e.g. provider keys).
	Env []string
	// Stderr receives the child's stderr (logs); defaults to io.Discard.
	Stderr io.Writer
	// EventBuffer is the per-subscriber channel capacity; default 256.
	// When a subscriber falls behind, events are dropped and Dropped counts.
	EventBuffer int
}

// Event is one un-correlated protocol record from the child
// (session, agent_start, message_update, agent_settled, …). Type mirrors the
// record's "type" field; the full record is in Fields.
type Event struct {
	Type   string
	Fields map[string]any
}

// Subscription is a client's event view. Close it to unsubscribe.
type Subscription struct {
	Events  <-chan Event
	Dropped *atomic.Int64
	client  *Client
	id      int
	once    sync.Once
}

func (s *Subscription) Close() {
	s.once.Do(func() { s.client.unsubscribe(s.id) })
}

// Client owns one `vivy-code --mode rpc` child process.
type Client struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	waitCh chan error

	seq     atomic.Int64
	pendMu  sync.Mutex
	pending map[string]chan wireResponse

	subsMu      sync.Mutex
	subs        map[int]*sub
	subSeq      int
	eventBuffer int

	closeOnce sync.Once
	closed    atomic.Bool
}

type sub struct {
	ch      chan Event
	dropped *atomic.Int64
}

type wireResponse struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Command string          `json:"command"`
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

// New spawns the child and confirms the protocol loop is live via get_state.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.Binary) == "" {
		return nil, errors.New("codeclient: Binary is required")
	}
	for _, a := range cfg.Args {
		if a == "--mode" || strings.HasPrefix(a, "--mode=") || a == "-m" {
			return nil, fmt.Errorf("codeclient: Args must not override the mode (%q)", a)
		}
	}
	args := append([]string{"--mode", "rpc"}, cfg.Args...)
	cmd := exec.Command(cfg.Binary, args...)
	cmd.Dir = cfg.Dir
	cmd.Env = append(environ(), cfg.Env...)
	stderr := cfg.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	cmd.Stderr = stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("codeclient: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("codeclient: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("codeclient: spawn %s: %w", cfg.Binary, err)
	}
	c := &Client{
		cmd:         cmd,
		stdin:       stdin,
		waitCh:      make(chan error, 1),
		pending:     make(map[string]chan wireResponse),
		subs:        make(map[int]*sub),
		eventBuffer: cfg.EventBuffer,
	}
	if c.eventBuffer <= 0 {
		c.eventBuffer = 256
	}
	go c.readLoop(stdout, cfg.EventBuffer)
	go func() { c.waitCh <- cmd.Wait() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := c.GetState(ctx); err != nil {
		c.Close()
		return nil, fmt.Errorf("codeclient: handshake get_state: %w", err)
	}
	return c, nil
}

// Subscribe registers an ordered event consumer. Events publish until Close;
// a slow subscriber drops records and counts them on Dropped.
func (c *Client) Subscribe() *Subscription {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	c.subSeq++
	ch := make(chan Event, c.eventBuffer)
	s := &Subscription{Events: ch, Dropped: &atomic.Int64{}, client: c, id: c.subSeq}
	c.subs[c.subSeq] = &sub{ch: ch, dropped: s.Dropped}
	return s
}

func (c *Client) unsubscribe(id int) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	if s, ok := c.subs[id]; ok {
		delete(c.subs, id)
		close(s.ch)
	}
}

// readLoop decodes stdout lines forever: responses route to waiters by id,
// everything else broadcasts to subscribers.
func (c *Client) readLoop(r io.Reader, buffer int) {
	defer func() {
		c.pendMu.Lock()
		for id, ch := range c.pending {
			delete(c.pending, id)
			close(ch)
		}
		c.pendMu.Unlock()
		c.subsMu.Lock()
		for id, s := range c.subs {
			delete(c.subs, id)
			close(s.ch)
		}
		c.subsMu.Unlock()
	}()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue // protocol guarantees JSON; non-JSON is child noise → skip
		}
		typ, _ := rec["type"].(string)
		if typ == "response" {
			var wr wireResponse
			if err := json.Unmarshal([]byte(line), &wr); err == nil {
				c.pendMu.Lock()
				if ch, ok := c.pending[wr.ID]; ok {
					delete(c.pending, wr.ID)
					ch <- wr
				}
				c.pendMu.Unlock()
			}
			continue
		}
		ev := Event{Type: typ, Fields: rec}
		c.subsMu.Lock()
		for _, s := range c.subs {
			select {
			case s.ch <- ev:
			default:
				s.dropped.Add(1)
			}
		}
		c.subsMu.Unlock()
	}
}

// Call sends one command line and waits for its correlated response.
// Returns the response's data payload; protocol failures return an error
// carrying the child's error text.
func (c *Client) Call(ctx context.Context, command string, params map[string]any) (json.RawMessage, error) {
	if c.closed.Load() {
		return nil, errors.New("codeclient: closed")
	}
	id := fmt.Sprintf("cc-%d", c.seq.Add(1))
	req := map[string]any{"id": id, "type": command}
	for k, v := range params {
		req[k] = v
	}
	ch := make(chan wireResponse, 1)
	c.pendMu.Lock()
	c.pending[id] = ch
	c.pendMu.Unlock()
	line, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if _, err := c.stdin.Write(append(line, '\n')); err != nil {
		c.pendMu.Lock()
		delete(c.pending, id)
		c.pendMu.Unlock()
		return nil, fmt.Errorf("codeclient: write: %w", err)
	}
	select {
	case wr, ok := <-ch:
		if !ok {
			return nil, errors.New("codeclient: protocol stream ended")
		}
		if !wr.Success {
			return nil, fmt.Errorf("codeclient: %s: %s", command, wr.Error)
		}
		return wr.Data, nil
	case <-ctx.Done():
		c.pendMu.Lock()
		delete(c.pending, id)
		c.pendMu.Unlock()
		return nil, ctx.Err()
	}
}

// Close closes stdin (the child's graceful shutdown signal), waits, and
// kills the process if it does not exit in time.
func (c *Client) Close() error {
	var err error
	c.closeOnce.Do(func() {
		c.closed.Store(true)
		_ = c.stdin.Close()
		select {
		case err = <-c.waitCh:
		case <-time.After(5 * time.Second):
			_ = c.cmd.Process.Kill()
			err = <-c.waitCh
		}
	})
	if err != nil && strings.Contains(err.Error(), "signal:") {
		return nil // killed after graceful timeout
	}
	return err
}

// environ is a seam for tests; the child inherits the parent env plus
// Config.Env overrides.
var environ = os.Environ
