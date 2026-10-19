package acpcompat

// Concurrency, transport-boundary, and diagnostics probes for
// eino-contrib/acp v0.0.4. These characterize behavior the pilot design
// depends on; observations feed docs/research/acp-sdk-compatibility.md.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	acp "github.com/eino-contrib/acp"
	acpconn "github.com/eino-contrib/acp/conn"
	"github.com/eino-contrib/acp/transport/stdio"
)

// TestPromptCancelAdmissionOrder exercises the pilot's cancel-admission
// contract: a cancel that arrives while a prompt is parked before its active
// slot is installed must still cancel that prompt (latch), and must not leak
// into a later prompt on the same session.
func TestPromptCancelAdmissionOrder(t *testing.T) {
	run := func(t *testing.T, occupied int) {
		h := newWireHarness(t, 1<<20)
		defer h.close()

		gate := make(chan struct{})
		var promptParams []map[string]any

		// Admission model: turn starts -> barrier -> slot install -> check latch.
		h.agent.promptHook = func(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
			st := h.agent.session(req.SessionID)
			st.mu.Lock()
			st.turnActive = true
			st.mu.Unlock()

			<-gate // barrier: prompt received, active slot not yet installed

			st.mu.Lock()
			st.slotInstalled = true
			cancelled := st.latched && time.Since(st.latchedAt) <= cancelLatchTTL
			st.turnActive = false
			st.slotInstalled = false
			st.latched = false
			st.mu.Unlock()

			if cancelled {
				return acp.PromptResponse{StopReason: "cancelled"}, nil
			}
			return acp.PromptResponse{StopReason: "end_turn"}, nil
		}

		h.peer.call(t, "initialize", map[string]any{"protocolVersion": 1})

		// occupy `occupied` prompt handlers on distinct sessions
		for i := 0; i < occupied; i++ {
			ns := h.peer.call(t, "session/new", map[string]any{"cwd": "/x", "mcpServers": []any{}})
			var r struct {
				SessionID string `json:"sessionId"`
			}
			mustUnmarshal(t, ns.Result, &r)
			params := map[string]any{
				"sessionId": r.SessionID,
				"prompt":    []any{map[string]any{"type": "text", "text": "go"}},
			}
			promptParams = append(promptParams, params)
			h.peer.sendAsync("session/prompt", params)
		}

		// prompts are parked at the barrier; cancels race ahead of slot install
		for _, p := range promptParams {
			h.peer.notify("session/cancel", map[string]string{"sessionId": p["sessionId"].(string)})
		}
		for i := 0; i < occupied; i++ {
			select {
			case <-h.agent.cancels:
			case <-time.After(5 * time.Second):
				t.Fatalf("cancel %d not delivered", i)
			}
		}
		h.peer.drainLog() // drop setup responses; prompt output starts now
		close(gate)       // release all parked prompts; each sees its latch

		for i := 0; i < occupied; i++ {
			resp := h.peer.awaitResponse(t, 5*time.Second)
			var r struct {
				StopReason string `json:"stopReason"`
			}
			mustUnmarshal(t, resp.Result, &r)
			if r.StopReason != "cancelled" {
				t.Fatalf("prompt %d stopReason = %q, want cancelled", i, r.StopReason)
			}
		}

		// contract check: a cancel arriving between turns applies to the NEXT
		// prompt within the TTL window (indistinguishable from a racing cancel
		// for a queued prompt at the SDK boundary)
		sid := promptParams[0]["sessionId"].(string)
		h.peer.notify("session/cancel", map[string]string{"sessionId": sid})
		select {
		case <-h.agent.cancels:
		case <-time.After(5 * time.Second):
			t.Fatal("cancel not delivered")
		}
		resp := h.peer.call(t, "session/prompt", promptParams[0])
		var r2 struct {
			StopReason string `json:"stopReason"`
		}
		mustUnmarshal(t, resp.Result, &r2)
		if r2.StopReason != "cancelled" {
			t.Fatalf("prompt after recent cancel stopReason = %q, want cancelled", r2.StopReason)
		}

		// an idle cancel ages out: a prompt after the TTL is unaffected
		h.peer.notify("session/cancel", map[string]string{"sessionId": sid})
		select {
		case <-h.agent.cancels:
		case <-time.After(5 * time.Second):
			t.Fatal("idle cancel not delivered")
		}
		time.Sleep(cancelLatchTTL + 150*time.Millisecond)
		resp = h.peer.call(t, "session/prompt", promptParams[0])
		mustUnmarshal(t, resp.Result, &r2)
		if r2.StopReason != "end_turn" {
			t.Fatalf("prompt after expired cancel stopReason = %q, want end_turn", r2.StopReason)
		}
	}

	t.Run("single", func(t *testing.T) { run(t, 1) })
	t.Run("four-occupied", func(t *testing.T) { run(t, 4) })
}

func mustUnmarshal(t *testing.T, raw []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
}

// TestTransportBoundaries exercises frame-size limits, EOF, blocked/broken
// pipes, queue flood, and shutdown timing on real OS pipes.
func TestTransportBoundaries(t *testing.T) {
	const MiB = 1 << 20

	frameFor := func(t *testing.T, sessionID string, size int) []byte {
		t.Helper()
		base := fmt.Sprintf(`{"jsonrpc":"2.0","id":900,"method":"session/prompt","params":{"sessionId":%q,"prompt":[{"type":"text","text":"`, sessionID)
		suffix := `"}]}}`
		pad := size - len(base) - len(suffix)
		if pad < 0 {
			t.Fatalf("frame base %d exceeds target %d", len(base)+len(suffix), size)
		}
		return []byte(base + strings.Repeat("a", pad) + suffix)
	}

	t.Run("frame-exactly-1MiB", func(t *testing.T) {
		// bufio.Scanner's max applies to the raw line: a 1MiB payload plus its
		// newline is over the 1MiB cap. Largest accepted token = limit-1.
		h := newWireHarness(t, MiB)
		defer h.close()
		h.peer.call(t, "initialize", map[string]any{"protocolVersion": 1})
		ns := h.peer.call(t, "session/new", map[string]any{"cwd": "/x", "mcpServers": []any{}})
		var r struct {
			SessionID string `json:"sessionId"`
		}
		mustUnmarshal(t, ns.Result, &r)
		h.peer.drainLog()
		if werr := h.peer.writeRaw(t, frameFor(t, r.SessionID, MiB-1)); werr != nil {
			t.Fatalf("sub-1MiB frame write: %v", werr)
		}
		resp := h.peer.awaitResponse(t, 10*time.Second)
		if resp.Err != nil {
			t.Fatalf("sub-1MiB frame rejected: %s", resp.Err)
		}
	})

	t.Run("frame-1MiB-plus-1", func(t *testing.T) {
		h := newWireHarness(t, MiB)
		defer h.close()
		h.peer.call(t, "initialize", map[string]any{"protocolVersion": 1})
		ns := h.peer.call(t, "session/new", map[string]any{"cwd": "/x", "mcpServers": []any{}})
		var r struct {
			SessionID string `json:"sessionId"`
		}
		mustUnmarshal(t, ns.Result, &r)
		// token = cap; newline pushes line over -> scanner error kills the
		// read loop and transport; the blocked write surfaces ErrClosedPipe.
		werr := h.peer.writeRaw(t, frameFor(t, r.SessionID, MiB))
		if werr != nil {
			t.Logf("oversized frame write rejected mid-flight: %v", werr)
		}

		select {
		case <-h.conn.Done():
			t.Logf("conn torn down, terminal err: %v", h.conn.Err())
		case <-time.After(5 * time.Second):
			t.Fatal("oversized frame did not tear down connection")
		}
	})

	t.Run("EOF", func(t *testing.T) {
		h := newWireHarness(t, MiB)
		defer h.close()
		h.peer.call(t, "initialize", map[string]any{"protocolVersion": 1})
		// close client->agent write end: agent sees EOF
		for _, p := range h.pipes {
			if w, ok := p.(*io.PipeWriter); ok {
				w.Close()
			}
		}
		select {
		case <-h.conn.Done():
			t.Logf("conn done after EOF, terminal err: %v", h.conn.Err())
		case <-time.After(5 * time.Second):
			t.Fatal("EOF did not terminate connection")
		}
	})

	t.Run("blocked-stdout", func(t *testing.T) {
		// agent writes into a real OS pipe that nobody drains: the kernel
		// buffer (~64KiB) fills, then writes block until the caller's write
		// deadline fires.
		c2ar, c2aw := io.Pipe()
		a2cr, a2cw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		defer a2cr.Close()

		agent := &probeAgent{cancels: make(chan acp.SessionID, 16)}
		tr := stdio.NewTransport(c2ar, a2cw)
		conn := acpconn.NewAgentConnectionFromTransport(agent, tr)
		agent.conn = conn
		go conn.Start(context.Background())

		peer := newWirePeer(io.NopCloser(bytes.NewReader(nil)), c2aw)
		defer peer.close()
		defer conn.Close()
		defer c2ar.Close()
		defer c2aw.Close()
		defer a2cw.Close()

		// responses go into the unread OS pipe; don't await them
		peer.sendAsync("initialize", map[string]any{"protocolVersion": 1})
		time.Sleep(200 * time.Millisecond) // let init reach the handler

		// flood the outbound pipe past the kernel buffer via prompt updates;
		// every write carries a caller deadline — the first blocked one must
		// surface that deadline rather than hang.
		blocked := make(chan error, 1)
		big := strings.Repeat("b", 8<<10)
		agent.promptHook = func(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
			for i := 0; i < 64; i++ {
				wctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
				err := conn.SessionUpdate(wctx, acp.SessionNotification{
					SessionID: req.SessionID,
					Update: acp.SessionUpdate{
						AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
							ContentChunk: acp.ContentChunk{Content: acp.ContentBlock{
								Text: &acp.ContentBlockText{TextContent: acp.TextContent{Text: big}, Type: "text"},
							}},
						},
					},
				})
				cancel()
				if err != nil {
					blocked <- err
					return acp.PromptResponse{StopReason: "end_turn"}, nil
				}
			}
			blocked <- nil
			return acp.PromptResponse{StopReason: "end_turn"}, nil
		}
		peer.sendAsync("session/prompt", map[string]any{"sessionId": "s1", "prompt": []any{map[string]any{"type": "text", "text": "x"}}})

		select {
		case err := <-blocked:
			if err == nil {
				t.Fatal("all 64 8KiB writes landed on an undrained pipe")
			}
			t.Logf("blocked write error: %v", err)
		case <-time.After(15 * time.Second):
			t.Fatal("blocked write did not surface within deadline")
		}
	})

	t.Run("broken-pipe", func(t *testing.T) {
		c2ar, c2aw := io.Pipe()
		a2cr, a2cw, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		agent := &probeAgent{cancels: make(chan acp.SessionID, 16)}
		tr := stdio.NewTransport(c2ar, a2cw)
		conn := acpconn.NewAgentConnectionFromTransport(agent, tr)
		agent.conn = conn
		go conn.Start(context.Background())

		peer := newWirePeer(io.NopCloser(bytes.NewReader(nil)), c2aw)
		defer peer.close()
		defer conn.Close()
		defer c2ar.Close()
		defer c2aw.Close()

		peer.sendAsync("initialize", map[string]any{"protocolVersion": 1})
		time.Sleep(200 * time.Millisecond) // init response lands in kernel buffer
		a2cr.Close()                       // reader gone: next outbound write hits EPIPE

		writeErr := make(chan error, 1)
		agent.promptHook = func(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
			err := conn.SessionUpdate(ctx, acp.SessionNotification{
				SessionID: req.SessionID,
				Update: acp.SessionUpdate{
					AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
						ContentChunk: acp.ContentChunk{Content: acp.ContentBlock{
							Text: &acp.ContentBlockText{TextContent: acp.TextContent{Text: "x"}, Type: "text"},
						}},
					},
				},
			})
			writeErr <- err
			return acp.PromptResponse{StopReason: "end_turn"}, nil
		}
		peer.sendAsync("session/prompt", map[string]any{"sessionId": "s1", "prompt": []any{}})

		select {
		case err := <-writeErr:
			if err == nil {
				t.Fatal("write on broken pipe reported success")
			}
			t.Logf("broken-pipe write error surfaces to caller: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("write on broken pipe did not return")
		}
		// Record: an outbound write failure does NOT tear the connection
		// down; the adapter must close the stream itself on write failure.
		select {
		case <-conn.Done():
			t.Logf("conn also tore down, err: %v", conn.Err())
		case <-time.After(1 * time.Second):
			t.Log("conn remains open after write failure (observed)")
		}
	})

	t.Run("flood-shared-queue", func(t *testing.T) {
		h := newWireHarness(t, MiB)
		defer h.close()
		h.peer.call(t, "initialize", map[string]any{"protocolVersion": 1})

		gate := make(chan struct{})
		h.agent.promptHook = func(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
			<-gate
			return acp.PromptResponse{StopReason: "end_turn"}, nil
		}
		ns := h.peer.call(t, "session/new", map[string]any{"cwd": "/x", "mcpServers": []any{}})
		var r struct {
			SessionID string `json:"sessionId"`
		}
		mustUnmarshal(t, ns.Result, &r)

		// occupy all 8 default request workers with parked prompts
		for i := 0; i < 8; i++ {
			h.peer.sendAsync("session/prompt", map[string]any{"sessionId": r.SessionID, "prompt": []any{}})
		}
		// flood the shared queue behind them
		for i := 0; i < 200; i++ {
			h.peer.notify("session/cancel", map[string]string{"sessionId": r.SessionID})
		}
		close(gate)
		for i := 0; i < 8; i++ {
			h.peer.awaitResponse(t, 10*time.Second)
		}
		// 200 cancels were queued and still delivered (≤4096 default)
		for i := 0; i < 200; i++ {
			select {
			case <-h.agent.cancels:
			case <-time.After(10 * time.Second):
				t.Fatalf("only %d of 200 flooded cancels delivered", i)
			}
		}
		select {
		case <-h.conn.Done():
			t.Fatal("conn died under bounded flood")
		default:
		}
	})

	t.Run("shutdown-vs-10s-budget", func(t *testing.T) {
		h := newWireHarness(t, MiB)
		defer h.close()
		h.peer.call(t, "initialize", map[string]any{"protocolVersion": 1})

		release := make(chan struct{})
		h.agent.promptHook = func(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
			<-release // parked handler; Close must wait or abandon
			return acp.PromptResponse{StopReason: "end_turn"}, nil
		}
		h.peer.sendAsync("session/prompt", map[string]any{"sessionId": "s1", "prompt": []any{}})

		// give the prompt time to reach the handler
		time.Sleep(200 * time.Millisecond)

		done := make(chan struct{})
		go func() { h.conn.Close(); close(done) }()
		select {
		case <-done:
			t.Fatal("Close returned while a handler was still parked")
		case <-time.After(2 * time.Second):
			t.Log("Close still blocked at 2s with a parked handler (internal default 30s, not publicly tunable)")
		}
		close(release)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("Close did not return after handler released")
		}
	})

	t.Run("shutdown-cooperative", func(t *testing.T) {
		h := newWireHarness(t, MiB)
		defer h.close()
		h.peer.call(t, "initialize", map[string]any{"protocolVersion": 1})
		start := time.Now()
		h.conn.Close()
		t.Logf("idle Close latency: %v", time.Since(start))
	})
}

// TestSDKWireErrorsAndLogging checks what reaches the wire and the SDK's
// diagnostic sink when decode fails, a handler returns a raw error, and a
// handler panics — using fixed synthetic canaries.
func TestSDKWireErrorsAndLogging(t *testing.T) {
	const (
		secretCanary = "SK-CANARY-9f8e7d6c5b"
		pathCanary   = "/home/ci/secrets/vault-CANARY.json"
	)

	logBuf := &captureLogger{}
	acp.SetLogger(logBuf, acp.LevelDisabled)
	t.Cleanup(func() { acp.SetLogger(nil, acp.LevelDebug) })

	h := newWireHarness(t, 1<<20)
	defer h.close()
	h.peer.call(t, "initialize", map[string]any{"protocolVersion": 1})
	ns := h.peer.call(t, "session/new", map[string]any{"cwd": "/x", "mcpServers": []any{}})
	var r struct {
		SessionID string `json:"sessionId"`
	}
	mustUnmarshal(t, ns.Result, &r)

	assertWireError := func(resp wireEnvelope, name string) (hasCanary, hasOriginError, hasStack bool) {
		if resp.Err == nil {
			t.Fatalf("%s: expected error response, got %s", name, resp.Raw)
		}
		body := string(resp.Err)
		hasCanary = strings.Contains(body, secretCanary) || strings.Contains(body, pathCanary)
		hasOriginError = strings.Contains(body, "originError")
		hasStack = strings.Contains(body, "goroutine ") || strings.Contains(body, ".go:")
		t.Logf("%s wire error: %s", name, body)
		return
	}

	// 1) decode failure: params carry canary in a wrong-typed field
	h.peer.drainLog()
	decodeBad := fmt.Sprintf(`{"jsonrpc":"2.0","id":901,"method":"session/prompt","params":{"sessionId":%q,"prompt":%q}}`, r.SessionID, secretCanary)
	h.peer.writeRaw(t, []byte(decodeBad))
	resp := h.peer.awaitResponse(t, 5*time.Second)
	canaryInDecode, _, stackInDecode := assertWireError(resp, "decode-failure")

	// 2) handler returns a raw error containing canaries
	h.agent.promptHook = func(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
		return acp.PromptResponse{}, fmt.Errorf("upstream %s from %s", secretCanary, pathCanary)
	}
	resp = h.peer.call(t, "session/prompt", map[string]any{"sessionId": r.SessionID, "prompt": []any{}})
	canaryInHandler, _, stackInHandler := assertWireError(resp, "handler-error")

	// 3) handler panics with a value containing the canary
	h.agent.promptHook = func(ctx context.Context, req acp.PromptRequest) (acp.PromptResponse, error) {
		panic(fmt.Sprintf("boom %s", secretCanary))
	}
	resp = h.peer.call(t, "session/prompt", map[string]any{"sessionId": r.SessionID, "prompt": []any{}})
	canaryInPanic, _, stackInPanic := assertWireError(resp, "panic")

	// SDK diagnostics must carry nothing at LevelDisabled
	logBuf.mu.Lock()
	got := logBuf.b.String()
	logBuf.mu.Unlock()
	if got != "" {
		t.Fatalf("disabled logger received %d bytes", len(got))
	}

	// record the leak matrix for the verdict file
	t.Logf("leak matrix: decode canary=%v stack=%v | handler canary=%v stack=%v | panic canary=%v stack=%v",
		canaryInDecode, stackInDecode, canaryInHandler, stackInHandler, canaryInPanic, stackInPanic)
}

// captureLogger implements acp.Logger for the diagnostics probe.
type captureLogger struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *captureLogger) log(format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(&l.b, format, v...)
}
func (l *captureLogger) Debug(format string, v ...interface{}) { l.log(format, v...) }
func (l *captureLogger) Info(format string, v ...interface{})  { l.log(format, v...) }
func (l *captureLogger) Warn(format string, v ...interface{})  { l.log(format, v...) }
func (l *captureLogger) Error(format string, v ...interface{}) { l.log(format, v...) }
func (l *captureLogger) CtxDebug(ctx context.Context, format string, v ...interface{}) {
	l.log(format, v...)
}
func (l *captureLogger) CtxInfo(ctx context.Context, format string, v ...interface{}) {
	l.log(format, v...)
}
func (l *captureLogger) CtxWarn(ctx context.Context, format string, v ...interface{}) {
	l.log(format, v...)
}
func (l *captureLogger) CtxError(ctx context.Context, format string, v ...interface{}) {
	l.log(format, v...)
}
