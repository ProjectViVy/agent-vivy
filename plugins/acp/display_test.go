package acp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	acp "github.com/eino-contrib/acp"
)

// digestPayload returns a v2 model.completed payload matching the exact
// bytes emitted as model.delta strings.
func digestPayload(deltas ...string) string {
	h := sha256.New()
	total := 0
	for _, d := range deltas {
		h.Write([]byte(d))
		total += len(d)
	}
	return fmt.Sprintf(`{"content_sha256":%q,"byte_len":%d}`, hex.EncodeToString(h.Sum(nil)), total)
}

func emitDelta(h *runHost, sub, run string, seq int64, delta string) {
	payload, _ := json.Marshal(map[string]any{"delta": delta})
	h.emitRunEvent(sub, run, seq, "model.delta", string(payload))
}

func emitCompletedV2(h *runHost, sub, run string, seq int64, deltas ...string) {
	emitCompletedV2Raw(h, sub, run, seq, digestPayload(deltas...))
}

func emitCompletedV2Raw(h *runHost, sub, run string, seq int64, payload string) {
	params, _ := json.Marshal(map[string]any{
		"subscription_id": sub,
		"event": map[string]any{
			"run_id":          run,
			"seq":             seq,
			"type":            "model.completed",
			"created_at":      time.Now().UnixMilli(),
			"payload_version": 2,
			"payload":         json.RawMessage(payload),
		},
	})
	h.fakeHost.emit("run/event", params)
}

func chunkTexts(ups []acp.SessionUpdate) []string {
	var out []string
	for _, u := range ups {
		if u.AgentMessageChunk != nil && u.AgentMessageChunk.Content.Text != nil {
			out = append(out, u.AgentMessageChunk.Content.Text.Text)
		}
	}
	return out
}

func TestDisplayBoundary(t *testing.T) {
	setup := func(t *testing.T) (*runHost, *agent, *promptState, string, string, chan struct{}) {
		t.Helper()
		h := newRunHost()
		a := newTestAgent(t, h, 1)
		sess := h.sessionID(0)
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = a.Prompt(context.Background(), promptText(sess, "hi"))
		}()
		if !h.waitForSubs(1) {
			t.Fatal("no subscription")
		}
		p := activePromptOf(t, a, sess)
		return h, a, p, h.subID(0), h.runID(0), done
	}

	t.Run("completed lines emit as chunks, tail flushed at model.completed", func(t *testing.T) {
		h, _, p, sub, run, done := setup(t)
		d1, d2 := "first line\nsecond ", "line\npartial tail"
		emitDelta(h, sub, run, 1, d1)
		emitDelta(h, sub, run, 2, d2)
		emitCompletedV2(h, sub, run, 3, d1, d2)
		h.emitRunEvent(sub, run, 4, "run.completed", `{"summary":"ok"}`)
		<-done
		got := chunkTexts(promptUpdates(p))
		want := []string{"first line\n", "second line\n", "partial tail"}
		if len(got) != len(want) {
			t.Fatalf("chunks = %q", got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("chunk %d = %q want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("digest mismatch fails with integrity error, never end_turn", func(t *testing.T) {
		h, _, _, sub, run, done := setup(t)
		emitDelta(h, sub, run, 1, "alpha\n")
		emitCompletedV2Raw(h, sub, run, 2, `{"content_sha256":"`+strings.Repeat("0", 64)+`","byte_len":999}`) // bogus digest
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("hang")
		}
		// run cancelled by failStream
		if h.cancelCount() == 0 {
			t.Fatal("integrity failure did not cancel the run")
		}
	})

	t.Run("v1 model.completed fails explicitly", func(t *testing.T) {
		h, _, _, sub, run, done := setup(t)
		emitDelta(h, sub, run, 1, "x")
		h.emitRunEvent(sub, run, 2, "model.completed", `{"content":"full text","input_tokens":1,"output_tokens":1}`)
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("hang")
		}
		if h.cancelCount() == 0 {
			t.Fatal("no cancel on unsupported version")
		}
	})

	t.Run("oversized incomplete line suppressed with explicit marker", func(t *testing.T) {
		h, _, p, sub, run, done := setup(t)
		big := strings.Repeat("a", maxIncompleteLineBytes+1024) // no newline
		d2 := "more of same line"
		d3 := "\nnext line\n"
		emitDelta(h, sub, run, 1, big)
		emitDelta(h, sub, run, 2, d2)
		emitDelta(h, sub, run, 3, d3)
		emitCompletedV2(h, sub, run, 4, big, d2, d3)
		h.emitRunEvent(sub, run, 5, "run.completed", `{"summary":"ok"}`)
		<-done
		got := chunkTexts(promptUpdates(p))
		// Expect: marker (line suppression at bound), marker at newline
		// termination? The suppressed line ends on d3's newline — one
		// marker total, then "next line\n", then flush.
		joined := strings.Join(got, "")
		if !strings.Contains(joined, "line omitted") {
			t.Fatalf("no omission marker: %q", got)
		}
		if strings.Contains(joined, strings.Repeat("a", 4096)) {
			t.Fatal("oversized line leaked")
		}
		if !strings.Contains(joined, "next line\n") {
			t.Fatalf("next line missing: %q", got)
		}
	})

	t.Run("frame-bound split at rune boundary with escaped bytes", func(t *testing.T) {
		h, _, p, sub, run, done := setup(t)
		// 0x01 escapes to \u0001 — 6 bytes JSON per raw byte, forcing the
		// 256 KiB encoded frame bound even below the 64 KiB line cap.
		line := strings.Repeat("\x01", 48<<10) + "\n"
		emitDelta(h, sub, run, 1, line)
		emitCompletedV2(h, sub, run, 2, line)
		h.emitRunEvent(sub, run, 3, "run.completed", `{"summary":"ok"}`)
		<-done
		ups := promptUpdates(p)
		var joined strings.Builder
		for _, u := range ups {
			if u.AgentMessageChunk == nil {
				continue
			}
			raw, _ := json.Marshal(acp.SessionNotification{Update: u})
			if len(raw) > maxOutboundFrameBytes {
				t.Fatalf("frame %d bytes exceeds bound", len(raw))
			}
			joined.WriteString(u.AgentMessageChunk.Content.Text.Text)
		}
		if joined.String() != line {
			t.Fatal("split corrupted the line bytes")
		}
	})

	t.Run("malformed model.delta fails integrity", func(t *testing.T) {
		h, _, _, sub, run, done := setup(t)
		h.emitRunEvent(sub, run, 1, "model.delta", `{"delta":123}`) // non-string
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("hang")
		}
	})

	t.Run("all writes land before the final response", func(t *testing.T) {
		// The prompt response must not be observable before its updates
		// were recorded — check ordering via update count + completion.
		h, _, p, sub, run, done := setup(t)
		emitDelta(h, sub, run, 1, "a\nb\n")
		emitCompletedV2(h, sub, run, 2, "a\nb\n")
		h.emitRunEvent(sub, run, 3, "run.completed", `{"summary":"ok"}`)
		<-done
		if len(chunkTexts(promptUpdates(p))) != 2 {
			t.Fatal("updates missing at finalization")
		}
	})
}

func TestMalformedPayloadVersionRejected(t *testing.T) {
	h := newRunHost()
	a := newTestAgent(t, h, 1)
	sess := h.sessionID(0)
	done := make(chan struct{})
	var perr error
	go func() {
		defer close(done)
		_, perr = a.Prompt(context.Background(), promptText(sess, "hi"))
	}()
	if !h.waitForSubs(1) {
		t.Fatal("no subscription")
	}
	// model.completed with a digest mismatch must surface -32603.
	sub, run := h.subID(0), h.runID(0)
	emitDelta(h, sub, run, 1, "data")
	h.emitRunEvent(sub, run, 2, "model.completed", `{"content_sha256":"`+strings.Repeat("f", 64)+`","byte_len":4}`)
	<-done
	var re *acp.RPCError
	if !errors.As(perr, &re) || re.Code != -32603 {
		t.Fatalf("error = %v", perr)
	}
	var data map[string]string
	_ = json.Unmarshal(re.Data, &data)
	if data["reason"] != "STREAM_INTEGRITY_FAILED" {
		t.Fatalf("reason = %q", data["reason"])
	}
}
