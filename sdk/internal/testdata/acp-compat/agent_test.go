package acpcompat

// Probe agent and a test-only standard-library JSON-RPC peer.
// Research fixture for the eino-contrib/acp v0.0.4 compatibility verdict.
// The agent embeds acp.BaseAgent and overrides only the four pilot handlers.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	acp "github.com/eino-contrib/acp"
	acpconn "github.com/eino-contrib/acp/conn"
	"github.com/eino-contrib/acp/transport/stdio"
)

// probeAgent embeds acp.BaseAgent and overrides only the four pilot handlers.
type probeAgent struct {
	acp.BaseAgent
	conn *acpconn.AgentConnection

	// promptHook, when set, is invoked inside Prompt. Tests install behavior
	// such as outbound SessionUpdate / RequestPermission / elicitation calls.
	promptHook func(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error)

	cancels chan acp.SessionID

	answered int32 // decoded form-accept count
	approved int32 // decoded permission-selected count

	sessionSeq int64
	sessionsMu sync.Mutex
	sessions   map[string]*sessionState
}

// sessionState models the minimum adapter admission state the cancel-admission
// probe needs: whether a prompt turn is in flight (handler entered), whether
// its "active slot" is installed, and the pending-cancel latch. Observed SDK
// behavior under -race: a session/cancel notification can dispatch BEFORE the
// session/prompt handler runs, so a latch gated on turnActive loses real
// cancels. The latch is therefore unconditional with a bounded TTL (spec's 5s
// gap bound): a prompt admission consumes a fresh latch and is cancelled; an
// expired latch is dropped, so a prompt long after an idle cancel is unaffected.
const cancelLatchTTL = 300 * time.Millisecond // probe knob; pilot bound is contract-owned

type sessionState struct {
	mu            sync.Mutex
	turnActive    bool
	slotInstalled bool
	latched       bool
	latchedAt     time.Time
}

func (a *probeAgent) Initialize(ctx context.Context, req acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion: 1,
		AgentInfo:       &acp.Implementation{Name: "acp-compat-probe", Title: "ACP compat probe", Version: "0.0.0"},
		AgentCapabilities: &acp.AgentCapabilities{
			LoadSession:         false,
			PromptCapabilities:  &acp.PromptCapabilities{},
			SessionCapabilities: &acp.SessionCapabilities{},
		},
	}, nil
}

func (a *probeAgent) NewSession(ctx context.Context, req acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	n := atomic.AddInt64(&a.sessionSeq, 1)
	id := acp.SessionID(fmt.Sprintf("probe-session-%d", n))
	a.sessionsMu.Lock()
	if a.sessions == nil {
		a.sessions = map[string]*sessionState{}
	}
	a.sessions[string(id)] = &sessionState{}
	a.sessionsMu.Unlock()
	return acp.NewSessionResponse{SessionID: id}, nil
}

func (a *probeAgent) session(id acp.SessionID) *sessionState {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	return a.sessions[string(id)]
}

func (a *probeAgent) Prompt(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
	if a.promptHook != nil {
		return a.promptHook(ctx, req)
	}
	return acp.PromptResponse{StopReason: "end_turn"}, nil
}

func (a *probeAgent) SessionCancel(ctx context.Context, n acp.CancelNotification) error {
	if st := a.session(n.SessionID); st != nil {
		st.mu.Lock()
		st.latched = true
		st.latchedAt = time.Now()
		st.mu.Unlock()
	}
	a.cancels <- n.SessionID
	return nil
}

// wireEnvelope is one decoded NDJSON JSON-RPC message on the client side.
type wireEnvelope struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Err    json.RawMessage `json:"error,omitempty"`
	Raw    json.RawMessage `json:"-"`
}

// wirePeer is a test-only standard-library JSON-RPC peer (the "client").
// It reads NDJSON frames, routes responses to pending calls, answers
// agent-issued reverse requests via onRequest, and records arrival order.
type wirePeer struct {
	out io.Writer
	mu  sync.Mutex // serialize writes

	pendingMu sync.Mutex
	pending   map[string]chan wireEnvelope
	nextID    int64

	// onRequest answers agent→client requests (request_permission,
	// elicitation/create). Returns the raw result object, or an *RPCError.
	onRequest func(method string, params json.RawMessage) (result json.RawMessage, rpcErr *acp.RPCError)

	log   chan wireEnvelope // every inbound envelope, arrival order
	close func()
}

func newWirePeer(r io.Reader, w io.Writer) *wirePeer {
	p := &wirePeer{
		out:     w,
		pending: map[string]chan wireEnvelope{},
		log:     make(chan wireEnvelope, 4096),
	}
	done := make(chan struct{})
	p.close = func() { close(done) }
	go p.readLoop(r, done)
	return p
}

func (p *wirePeer) readLoop(r io.Reader, done chan struct{}) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		select {
		case <-done:
			return
		default:
		}
		line := sc.Bytes()
		var env wireEnvelope
		if err := json.Unmarshal(line, &env); err != nil {
			continue
		}
		env.Raw = append(json.RawMessage(nil), line...)
		p.log <- env
		switch {
		case env.Method != "" && len(env.ID) > 0:
			// Agent-issued reverse request; answer it.
			go p.answerRequest(env)
		case env.Method == "" && len(env.ID) > 0:
			p.routeResponse(env)
		}
	}
}

func (p *wirePeer) routeResponse(env wireEnvelope) {
	key := string(env.ID)
	p.pendingMu.Lock()
	ch, ok := p.pending[key]
	delete(p.pending, key)
	p.pendingMu.Unlock()
	if ok {
		ch <- env
	}
}

func (p *wirePeer) answerRequest(env wireEnvelope) {
	var result json.RawMessage
	var rpcErr *acp.RPCError
	if p.onRequest != nil {
		result, rpcErr = p.onRequest(env.Method, env.Params)
	}
	resp := map[string]json.RawMessage{"jsonrpc": mustJSON("2.0"), "id": env.ID}
	if rpcErr != nil {
		e := map[string]any{"code": rpcErr.Code, "message": rpcErr.Message}
		if rpcErr.Data != nil {
			e["data"] = rpcErr.Data
		}
		resp["error"] = mustJSON(e)
	} else if result != nil {
		resp["result"] = result
	} else {
		resp["result"] = mustJSON(map[string]any{})
	}
	p.write(resp)
}

// call issues a JSON-RPC request and waits for its response.
func (p *wirePeer) call(t *testing.T, method string, params any) wireEnvelope {
	t.Helper()
	id := atomic.AddInt64(&p.nextID, 1)
	idRaw := mustJSON(id)
	ch := make(chan wireEnvelope, 1)
	p.pendingMu.Lock()
	p.pending[string(idRaw)] = ch
	p.pendingMu.Unlock()

	msg := map[string]json.RawMessage{
		"jsonrpc": mustJSON("2.0"),
		"id":      idRaw,
		"method":  mustJSON(method),
	}
	if params != nil {
		msg["params"] = mustJSON(params)
	}
	p.write(msg)

	select {
	case env := <-ch:
		return env
	case <-time.After(10 * time.Second):
		t.Fatalf("timeout waiting for response to %s", method)
		return wireEnvelope{}
	}
}

// sendAsync issues a JSON-RPC request without blocking; the returned channel
// receives exactly one response envelope.
func (p *wirePeer) sendAsync(method string, params any) <-chan wireEnvelope {
	id := atomic.AddInt64(&p.nextID, 1)
	idRaw := mustJSON(id)
	ch := make(chan wireEnvelope, 1)
	p.pendingMu.Lock()
	p.pending[string(idRaw)] = ch
	p.pendingMu.Unlock()

	msg := map[string]json.RawMessage{
		"jsonrpc": mustJSON("2.0"),
		"id":      idRaw,
		"method":  mustJSON(method),
	}
	if params != nil {
		msg["params"] = mustJSON(params)
	}
	p.write(msg)
	return ch
}

// writeRaw emits one pre-encoded frame (a '\n' terminator is appended when
// missing). Used to control exact frame byte size and inject malformed wire
// data that a typed helper could not produce.
func (p *wirePeer) writeRaw(t *testing.T, frame []byte) error {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(frame) == 0 || frame[len(frame)-1] != '\n' {
		frame = append(frame, '\n')
	}
	_, err := p.out.Write(frame)
	return err
}

// awaitResponse waits for the next inbound response envelope (a message with
// an id and no method) regardless of pending-call registration.
func (p *wirePeer) awaitResponse(t *testing.T, timeout time.Duration) wireEnvelope {
	t.Helper()
	deadline := time.After(timeout)
	var stash []wireEnvelope
	defer func() {
		for _, e := range stash {
			p.log <- e
		}
	}()
	for {
		select {
		case e := <-p.log:
			if e.Method == "" && len(e.ID) > 0 {
				return e
			}
			stash = append(stash, e)
		case <-deadline:
			t.Fatalf("timeout waiting for response envelope")
			return wireEnvelope{}
		}
	}
}

// notify sends a JSON-RPC notification (no response expected).
func (p *wirePeer) notify(method string, params any) {
	msg := map[string]json.RawMessage{
		"jsonrpc": mustJSON("2.0"),
		"method":  mustJSON(method),
	}
	if params != nil {
		msg["params"] = mustJSON(params)
	}
	p.write(msg)
}

func (p *wirePeer) write(msg map[string]json.RawMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.out.Write(append(data, '\n'))
}

// drainLog returns every envelope logged so far, in arrival order.
func (p *wirePeer) drainLog() []wireEnvelope {
	var out []wireEnvelope
	for {
		select {
		case e := <-p.log:
			out = append(out, e)
		default:
			return out
		}
	}
}

// waitEnvelopes blocks until at least n envelopes have arrived or timeout.
func (p *wirePeer) waitEnvelopes(n int, timeout time.Duration) []wireEnvelope {
	var out []wireEnvelope
	deadline := time.After(timeout)
	for len(out) < n {
		select {
		case e := <-p.log:
			out = append(out, e)
		case <-deadline:
			return out
		}
	}
	return out
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// wireHarness wires a probeAgent over real OS-pipe NDJSON to a wirePeer.
type wireHarness struct {
	peer  *wirePeer
	agent *probeAgent
	conn  *acpconn.AgentConnection
	pipes []io.Closer
}

func newWireHarness(t *testing.T, maxFrame int) *wireHarness {
	t.Helper()
	// client->agent (agent reads c2a.r, writes a2c.w)
	c2ar, c2aw := io.Pipe()
	a2cr, a2cw := io.Pipe()

	agent := &probeAgent{cancels: make(chan acp.SessionID, 16)}
	var tr *stdio.Transport
	if maxFrame > 0 {
		tr = stdio.NewTransport(c2ar, a2cw, stdio.WithMaxMessageSize(maxFrame))
	} else {
		tr = stdio.NewTransport(c2ar, a2cw)
	}
	c := acpconn.NewAgentConnectionFromTransport(agent, tr)
	agent.conn = c
	go c.Start(context.Background())

	peer := newWirePeer(a2cr, c2aw)
	return &wireHarness{
		peer:  peer,
		agent: agent,
		conn:  c,
		pipes: []io.Closer{c2ar, c2aw, a2cr, a2cw},
	}
}

func (h *wireHarness) close() {
	h.conn.Close()
	for _, p := range h.pipes {
		p.Close()
	}
	h.peer.close()
}

// readCaseFile loads a raw JSON fixture line/object map by name.
func readCaseFile(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	data, err := readFileBytes(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse fixture %s: %v", path, err)
	}
	return m
}

func readFileBytes(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

var _ = fmt.Sprintf // keep fmt import stable across edits
