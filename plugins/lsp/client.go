package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"agent-vivy/sdk/plugin"
)

// server is one language-server connection: a child process spawned
// through Env.Spawn plus the JSON-RPC client state layered on its stdio.
// All request/notification writes are serialized (callMu); responses are
// routed to waiters by ID from the single reader goroutine.
type server struct {
	proc  plugin.Proc
	stdin io.WriteCloser

	callMu sync.Mutex
	nextID int64

	pendingMu sync.Mutex
	pending   map[int64]chan rpcMessage

	diagMu  sync.Mutex
	diags   map[string][]diagnostic
	diagGen map[string]uint64
	signal  chan struct{}

	versions map[string]int

	// enc is the position encoding negotiated at initialize (utf-16 when
	// the server does not pick one).
	enc positionEncoding

	lastUsed atomic.Int64

	dead     chan struct{}
	deadOnce sync.Once
}

func startServer(ctx context.Context, env plugin.Env, lang language, root string) (*server, error) {
	proc, err := env.Spawn(ctx, plugin.SpawnSpec{Command: lang.Command, Args: lang.Args})
	if err != nil {
		return nil, fmt.Errorf("lsp: start %s: %w", lang.Command, err)
	}
	s := &server{
		proc:     proc,
		stdin:    proc.Stdin(),
		pending:  map[int64]chan rpcMessage{},
		diags:    map[string][]diagnostic{},
		diagGen:  map[string]uint64{},
		signal:   make(chan struct{}),
		versions: map[string]int{},
		dead:     make(chan struct{}),
	}
	s.touch()
	go s.readLoop(proc.Stdout())
	handshakeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	params := initializeParams{
		ProcessID: nil,
		RootURI:   pathToURI(root, "."),
	}
	params.Capabilities.General.PositionEncodings = offeredPositionEncodings
	result, err := s.call(handshakeCtx, "initialize", params)
	if err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("lsp: initialize %s: %w", lang.Command, err)
	}
	s.enc = negotiatedEncoding(result)
	if err := s.notify("initialized", map[string]any{}); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("lsp: initialized %s: %w", lang.Command, err)
	}
	return s, nil
}

func (s *server) touch() { s.lastUsed.Store(time.Now().UnixNano()) }

func (s *server) idleSince() time.Time {
	return time.Unix(0, s.lastUsed.Load())
}

func (s *server) exited() bool {
	select {
	case <-s.dead:
		return true
	default:
		return false
	}
}

// readLoop consumes server messages until the pipe ends. Responses route
// to their waiter; notifications are dispatched (publishDiagnostics is
// the only one this client consumes).
func (s *server) readLoop(r io.Reader) {
	br := bufio.NewReader(r)
	for {
		msg, err := readMessage(br)
		if err != nil {
			s.markDead()
			return
		}
		switch {
		case msg.isNotification() && msg.Method == "textDocument/publishDiagnostics":
			var p publishDiagnosticsParams
			if err := json.Unmarshal(msg.Params, &p); err != nil {
				continue
			}
			s.storeDiagnostics(p)
		case msg.isResponse():
			if ch := s.takeWaiter(*msg.ID); ch != nil {
				ch <- msg
				close(ch)
			}
		}
	}
}

func (s *server) markDead() {
	s.deadOnce.Do(func() { close(s.dead) })
	s.failWaiters()
}

func (s *server) takeWaiter(id int64) chan rpcMessage {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	ch := s.pending[id]
	delete(s.pending, id)
	return ch
}

// failWaiters unblocks every pending call when the connection dies.
func (s *server) failWaiters() {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	for id, ch := range s.pending {
		ch <- rpcMessage{Error: &rpcError{Code: -32800, Message: "lsp: server exited"}}
		close(ch)
		delete(s.pending, id)
	}
}

func (s *server) storeDiagnostics(p publishDiagnosticsParams) {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	s.diags[p.URI] = p.Diagnostics
	s.diagGen[p.URI]++
	close(s.signal)
	s.signal = make(chan struct{})
}

// call sends one request and waits for its response.
func (s *server) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	s.callMu.Lock()
	s.nextID++
	id := s.nextID
	body, err := json.Marshal(params)
	if err != nil {
		s.callMu.Unlock()
		return nil, fmt.Errorf("lsp: encode %s: %w", method, err)
	}
	ch := make(chan rpcMessage, 1)
	s.pendingMu.Lock()
	s.pending[id] = ch
	s.pendingMu.Unlock()
	if err := writeMessage(s.stdin, rpcMessage{ID: &id, Method: method, Params: body}); err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		s.callMu.Unlock()
		return nil, fmt.Errorf("lsp: send %s: %w", method, err)
	}
	s.callMu.Unlock()
	select {
	case msg := <-ch:
		if msg.Error != nil {
			return nil, msg.Error
		}
		return msg.Result, nil
	case <-ctx.Done():
		s.takeWaiter(id)
		return nil, fmt.Errorf("lsp: %s: %w", method, ctx.Err())
	}
}

// notify sends one notification (no response expected).
func (s *server) notify(method string, params any) error {
	s.callMu.Lock()
	defer s.callMu.Unlock()
	body, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("lsp: encode %s: %w", method, err)
	}
	if err := writeMessage(s.stdin, rpcMessage{Method: method, Params: body}); err != nil {
		return fmt.Errorf("lsp: send %s: %w", method, err)
	}
	return nil
}

// openText syncs one file into the server: didOpen on first sight,
// didChange (full text) afterwards. The plugin is read-only, so disk is
// always the source of truth for the buffer.
func (s *server) openText(ctx context.Context, languageID, uri, text string) error {
	s.callMu.Lock()
	s.versions[uri]++
	version := s.versions[uri]
	s.callMu.Unlock()
	if version == 1 {
		return s.notify("textDocument/didOpen", didOpenParams{
			TextDocument: textDocumentItem{URI: uri, LanguageID: languageID, Version: version, Text: text},
		})
	}
	changes := didChangeParams{
		TextDocument: versionedTextDocumentIdentifier{URI: uri, Version: version},
	}
	changes.ContentChanges = append(changes.ContentChanges, struct {
		Text string `json:"text"`
	}{Text: text})
	return s.notify("textDocument/didChange", changes)
}

// diagGeneration returns the current publish generation for uri. Callers
// must snapshot it BEFORE the request that triggers the next publish —
// snapshotting afterwards races a publish that already arrived and would
// wait forever for a fresh round that never comes.
func (s *server) diagGeneration(uri string) uint64 {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	return s.diagGen[uri]
}

// waitForDiagnostics blocks until a publishDiagnostics for uri arrives
// after the given base generation (or the deadline passes). timedOut
// distinguishes a fresh publish from a silent wait; a silent wait is not
// an error, it just means the current (possibly empty) view is the best
// available.
func (s *server) waitForDiagnostics(ctx context.Context, uri string, base uint64, timeout time.Duration) (timedOut bool, err error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	s.diagMu.Lock()
	sig := s.signal
	s.diagMu.Unlock()
	for {
		s.diagMu.Lock()
		fresh := s.diagGen[uri] > base
		s.diagMu.Unlock()
		if fresh {
			return false, nil
		}
		select {
		case <-sig:
			// A publish arrived; loop to check whether it was for our URI.
		case <-ctx.Done():
			return false, ctx.Err()
		case <-deadline.C:
			return true, nil
		}
	}
}

func (s *server) diagnosticsFor(uri string) []diagnostic {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	out := make([]diagnostic, len(s.diags[uri]))
	copy(out, s.diags[uri])
	return out
}

// Close kills the child process. A well-behaved language server also exits
// on stdin EOF when the species process dies, so no OS-level job objects
// are tracked here.
func (s *server) Close() error {
	s.markDead()
	if s.proc == nil {
		return nil
	}
	_ = s.stdin.Close()
	return s.proc.Close()
}
