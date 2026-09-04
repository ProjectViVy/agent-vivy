// Package headless is the first seam-face organ (VIVY-FACE-PACK.md §6):
// `vivy run` as a packed face. It speaks only control-plane JSON-RPC
// through plugin.FaceEnv — no kernel import, no listener, no engine. The
// flow mirrors the kernel headless loop: resolve the session (newest with
// --continue, else one titled by the prompt), start one turn, render
// deltas to Out and diagnostics to Err, and fail loudly when the run
// blocks on approval/question (§14④: cancel durably — a face that cannot
// ask must not wait forever and must not bypass HITL).
package headless

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"agent-vivy/sdk/plugin"
)

// FaceKind is the presentation family this organ serves.
const FaceKind = "headless"

func New(opts plugin.FaceOptions) plugin.Face {
	return &face{opts: opts}
}

type face struct {
	opts           plugin.FaceOptions
	streamed       atomic.Bool
	protocolFailed atomic.Bool
	stateMu        sync.Mutex
	completionHash hash.Hash
	completionLen  int
	protocolErr    string
	cancel         sync.Once
}

func (f *face) Kind() string { return FaceKind }

// Run drives one prompt to its terminal event. The subscription replays
// journal events from seq 0, so events emitted between turn/start and
// run/subscribe are still delivered exactly once, in seq order.
func (f *face) Run(ctx context.Context, env plugin.FaceEnv) (plugin.FaceResult, error) {
	prompt := strings.TrimSpace(f.opts.Prompt)
	if prompt == "" {
		return plugin.FaceResult{}, fmt.Errorf("headless: prompt is empty")
	}
	f.stateMu.Lock()
	f.resetCompletionLocked()
	f.protocolErr = ""
	f.streamed.Store(false)
	f.protocolFailed.Store(false)
	f.stateMu.Unlock()
	if _, err := env.Call(ctx, "initialize", nil); err != nil {
		return plugin.FaceResult{}, fmt.Errorf("headless: initialize: %w", err)
	}
	sessionID, err := f.resolveSession(ctx, env)
	if err != nil {
		return plugin.FaceResult{}, err
	}
	turn, err := env.Call(ctx, "turn/start", map[string]any{
		"session_id": sessionID,
		"text":       prompt,
		"face":       FaceKind,
	})
	if err != nil {
		return plugin.FaceResult{}, fmt.Errorf("headless: turn/start: %w", err)
	}
	var start struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(turn, &start); err != nil || start.RunID == "" {
		return plugin.FaceResult{}, fmt.Errorf("headless: turn/start returned %s", turn)
	}
	terminal := make(chan string, 1)
	// FaceHost drops notifications delivered before OnEvent is registered.
	// Register before subscribing because run/subscribe may synchronously
	// replay a terminal event for a fast run.
	env.OnEvent(func(method string, params json.RawMessage) {
		if method == "run/event" {
			f.onEvent(params, start.RunID, env, terminal)
		}
	})
	sub, err := env.Call(ctx, "run/subscribe", map[string]any{"run_id": start.RunID})
	if err != nil {
		return plugin.FaceResult{}, fmt.Errorf("headless: run/subscribe: %w", err)
	}
	var stream struct {
		SubscriptionID string `json:"subscription_id"`
	}
	_ = json.Unmarshal(sub, &stream)

	select {
	case status := <-terminal:
		return plugin.FaceResult{Status: status}, nil
	case <-ctx.Done():
		return plugin.FaceResult{}, fmt.Errorf("headless: %w", ctx.Err())
	}
}

// resolveSession picks the session the prompt lands in — the newest one
// with --continue, or a fresh one titled by the prompt (the same Journal
// the web face lists, so the turn shows up there too).
func (f *face) resolveSession(ctx context.Context, env plugin.FaceEnv) (string, error) {
	if f.opts.ContinueNewest {
		raw, err := env.Call(ctx, "session/list", nil)
		if err != nil {
			return "", fmt.Errorf("headless: session/list: %w", err)
		}
		var list struct {
			Sessions []struct {
				ID string `json:"id"`
			} `json:"sessions"`
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return "", fmt.Errorf("headless: decode session/list: %w", err)
		}
		if len(list.Sessions) == 0 {
			return "", fmt.Errorf("headless: no sessions to continue; run without --continue to start one")
		}
		return list.Sessions[0].ID, nil
	}
	title := strings.TrimSpace(f.opts.Prompt)
	if runes := []rune(title); len(runes) > 60 {
		title = string(runes[:60]) + "..."
	}
	raw, err := env.Call(ctx, "session/create", map[string]any{"title": title})
	if err != nil {
		return "", fmt.Errorf("headless: session/create: %w", err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == "" {
		return "", fmt.Errorf("headless: session/create returned %s", raw)
	}
	return created.ID, nil
}

// wireEvent is one run/event notification: the server wraps the journal
// event in {"subscription_id": ..., "event": {...}}.
type wireEvent struct {
	Event struct {
		RunID          string          `json:"run_id"`
		Seq            int64           `json:"seq"`
		Type           string          `json:"type"`
		PayloadVersion int             `json:"payload_version"`
		Payload        json.RawMessage `json:"payload"`
	} `json:"event"`
}

// onEvent renders the stream and reports the terminal. It runs on the
// notification goroutine; writers are direct like the kernel headless
// sink, the terminal channel dedupes, and the §14④ cancel fires once.
func (f *face) onEvent(params json.RawMessage, runID string, env plugin.FaceEnv, terminal chan<- string) {
	var wire wireEvent
	if err := json.Unmarshal(params, &wire); err != nil || wire.Event.RunID != runID {
		return
	}
	payload := wire.Event.Payload
	switch wire.Event.Type {
	case "model.request":
		f.stateMu.Lock()
		if f.protocolErr == "" {
			f.resetCompletionLocked()
		}
		f.stateMu.Unlock()
	case "model.delta":
		f.stateMu.Lock()
		if f.protocolErr == "" {
			delta, err := parseDelta(payload)
			if err != nil {
				f.failProtocolLocked(err)
			} else if delta != "" {
				_, _ = f.completionHash.Write([]byte(delta))
				f.completionLen += len([]byte(delta))
				_, _ = fmt.Fprint(f.opts.Out, delta)
				f.streamed.Store(true)
			}
		}
		f.stateMu.Unlock()
	case "model.completed":
		f.stateMu.Lock()
		streamed := f.streamed.Swap(false)
		if f.protocolErr == "" {
			content, err := f.completeModelLocked(wire.Event.PayloadVersion, payload)
			if err != nil {
				f.failProtocolLocked(err)
			} else if streamed {
				_, _ = fmt.Fprintln(f.opts.Out)
			} else if wire.Event.PayloadVersion == 0 || wire.Event.PayloadVersion == 1 {
				if strings.TrimSpace(content) != "" {
					_, _ = fmt.Fprintln(f.opts.Out, content)
				}
			}
		}
		f.stateMu.Unlock()
	case "tool.started":
		var p struct {
			ToolName string `json:"tool_name"`
		}
		if json.Unmarshal(payload, &p) == nil {
			_, _ = fmt.Fprintf(f.opts.Err, "> %s\n", p.ToolName)
		}
	case "tool.finished":
		var p struct {
			ToolName string `json:"tool_name"`
			Error    string `json:"error"`
		}
		if json.Unmarshal(payload, &p) == nil && p.Error != "" {
			_, _ = fmt.Fprintf(f.opts.Err, "  %s failed: %s\n", p.ToolName, p.Error)
		}
	case "tool.approval_required":
		f.cancelLoudly(env, runID, terminal, func() string {
			var p struct {
				ToolName string `json:"tool_name"`
				Action   string `json:"action"`
				Target   string `json:"target"`
			}
			_ = json.Unmarshal(payload, &p)
			return fmt.Sprintf("vivy: blocked: %s (%s %s) requires human approval and the headless face cannot ask for it. Approve in the web UI or adjust the permission preset; the run was cancelled.\n", p.ToolName, p.Action, p.Target)
		})
	case "user.question_required":
		f.cancelLoudly(env, runID, terminal, func() string {
			var p struct {
				Prompt string `json:"prompt"`
			}
			_ = json.Unmarshal(payload, &p)
			return fmt.Sprintf("vivy: blocked: the model asked %q and the headless face cannot answer. Re-run in the web UI; the run was cancelled.\n", p.Prompt)
		})
	case "run.failed":
		var p struct {
			CauseCategory string `json:"cause_category"`
			Message       string `json:"message"`
		}
		_ = json.Unmarshal(payload, &p)
		_, _ = fmt.Fprintf(f.opts.Err, "vivy: run failed (%s): %s\n", p.CauseCategory, p.Message)
		emitTerminal(terminal, "failed")
	case "run.cancelled":
		_, _ = fmt.Fprintln(f.opts.Err, "vivy: run cancelled")
		emitTerminal(terminal, "cancelled")
	case "run.completed":
		if f.protocolFailed.Load() {
			emitTerminal(terminal, "failed")
		} else {
			emitTerminal(terminal, "completed")
		}
	}
}

func (f *face) resetCompletionLocked() {
	f.completionHash = sha256.New()
	f.completionLen = 0
}

func parseDelta(payload json.RawMessage) (string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil || len(fields) != 1 {
		return "", fmt.Errorf("model.delta: invalid payload")
	}
	raw, ok := fields["delta"]
	if !ok {
		return "", fmt.Errorf("model.delta: missing delta")
	}
	var delta string
	if err := json.Unmarshal(raw, &delta); err != nil {
		return "", fmt.Errorf("model.delta: delta must be a string")
	}
	return delta, nil
}

func (f *face) completeModelLocked(version int, payload json.RawMessage) (string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil || fields == nil {
		return "", fmt.Errorf("invalid model.completed payload")
	}
	if version == 0 {
		if _, ok := fields["content"]; ok {
			version = 1
		}
	}
	if version == 1 {
		raw, ok := fields["content"]
		if !ok {
			return "", fmt.Errorf("model.completed v1: missing content")
		}
		var content string
		if err := json.Unmarshal(raw, &content); err != nil {
			return "", fmt.Errorf("model.completed v1: content must be a string")
		}
		f.resetCompletionLocked()
		return content, nil
	}
	if version != 2 {
		return "", fmt.Errorf("unsupported model.completed payload version %d", version)
	}
	if len(fields) != 2 {
		return "", fmt.Errorf("model.completed v2: invalid fields")
	}
	var digest string
	if raw, ok := fields["content_sha256"]; !ok || json.Unmarshal(raw, &digest) != nil || len(digest) != 64 || strings.ToLower(digest) != digest {
		return "", fmt.Errorf("model.completed v2: invalid content_sha256")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return "", fmt.Errorf("model.completed v2: invalid content_sha256")
	}
	var byteLen int
	if raw, ok := fields["byte_len"]; !ok || json.Unmarshal(raw, &byteLen) != nil || byteLen < 0 {
		return "", fmt.Errorf("model.completed v2: invalid byte_len")
	}
	if byteLen != f.completionLen || digest != hex.EncodeToString(f.completionHash.Sum(nil)) {
		return "", fmt.Errorf("model.completed v2: content integrity mismatch")
	}
	f.resetCompletionLocked()
	return "", nil
}

func (f *face) failProtocolLocked(err error) {
	if f.protocolErr != "" {
		return
	}
	message := "invalid stream protocol"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	f.protocolErr = message
	f.protocolFailed.Store(true)
	_, _ = fmt.Fprintln(f.opts.Err, "vivy: "+message)
}

// cancelLoudly renders the blocking notice and cancels the run once —
// the durable cancelled terminal then arrives through the subscription.
func (f *face) cancelLoudly(env plugin.FaceEnv, runID string, terminal chan<- string, notice func() string) {
	f.cancel.Do(func() {
		_, _ = fmt.Fprint(f.opts.Err, notice())
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = env.Call(ctx, "run/cancel", map[string]any{"run_id": runID})
		}()
	})
}

func emitTerminal(terminal chan<- string, status string) {
	select {
	case terminal <- status:
	default:
	}
}
