package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/runtime"
	plugin "agent-vivy/sdk/port/face"
	tuiface "agent-vivy/sdk/tui/face"
)

// textOnlyDeepSeekServer streams one plain assistant message over the OpenAI
// chat.completions wire protocol, split into per-word SSE chunks so the
// delta-integrity path is exercised.
func textOnlyDeepSeekServer(t *testing.T, words ...string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		streaming := strings.Contains(string(raw), `"stream":true`)
		if streaming {
			w.Header().Set("Content-Type", "text/event-stream")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		flusher, _ := w.(http.Flusher)
		writeChunk := func(payload string) {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			if flusher != nil {
				flusher.Flush()
			}
		}
		text := strings.Join(words, "")
		if !streaming {
			_, _ = io.WriteString(w, `{"id":"chatcmpl-text","object":"chat.completion","created":1,"model":"deepseek-flash","choices":[{"index":0,"message":{"role":"assistant","content":"`+text+`"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
			return
		}
		writeChunk(`{"id":"chatcmpl-text","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`)
		for _, word := range words {
			writeChunk(`{"id":"chatcmpl-text","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"content":"` + word + `"},"finish_reason":null}]}`)
		}
		writeChunk(`{"id":"chatcmpl-text","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv
}

// composeCodeFace launches a full kernel plus the real terminal face
// (RunFaceWithAppOptions) against a frozen-env model, so the print/json
// mode tests drive the same path `vivy-code` takes.
func runCodeFaceMode(t *testing.T, cfg config.Config, opts plugin.Options) (plugin.Result, string, string, error) {
	t.Helper()
	var out, errBuf bytes.Buffer
	opts.Out = &out
	opts.Err = &errBuf
	res, err := RunFaceWithAppOptions(context.Background(), cfg, tuiface.New, opts)
	return res, out.String(), errBuf.String(), err
}

func TestCodeFacePrintModeStreamsAssistantText(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "codemode-print-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	t.Setenv("VIVY_API_BASE", textOnlyDeepSeekServer(t, "hello", " ", "vivy").URL)

	res, out, errOut, err := runCodeFaceMode(t, newDeepSeekTestConfig(t), plugin.Options{
		Mode:   "print",
		Prompt: "hi",
	})
	if err != nil {
		t.Fatalf("print mode: %v (stderr %s)", err, errOut)
	}
	if res.Status != "completed" {
		t.Fatalf("status = %q, want completed (stderr %s)", res.Status, errOut)
	}
	if out != "hello vivy\n" {
		t.Fatalf("stdout = %q, want assistant text only", out)
	}
}

func TestCodeFaceJSONModeEmitsOrderedRecords(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "codemode-json-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	t.Setenv("VIVY_API_BASE", textOnlyDeepSeekServer(t, "hi", " ", "there").URL)

	res, out, errOut, err := runCodeFaceMode(t, newDeepSeekTestConfig(t), plugin.Options{
		Mode:   "json",
		Prompt: "hi",
	})
	if err != nil {
		t.Fatalf("json mode: %v (stderr %s)", err, errOut)
	}
	if res.Status != "completed" {
		t.Fatalf("status = %q, want completed", res.Status)
	}

	var types []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %q is not JSON: %v", line, err)
		}
		if rec["v"] != float64(1) {
			t.Fatalf("record %q missing v:1", line)
		}
		types = append(types, rec["type"].(string))
	}
	// The contract: session → turn_start → agent_start → message_start →
	// message_update+ → message_end → turn_end → agent_end → agent_settled.
	want := []string{"session", "turn_start", "agent_start", "message_start", "message_update", "message_end", "turn_end", "agent_end", "agent_settled"}
	pos := 0
	for _, typ := range types {
		if pos < len(want) && typ == want[pos] {
			pos++
		}
	}
	if pos != len(want) {
		t.Fatalf("ordered record sequence incomplete: reached %d/%d, types = %v", pos, len(want), types)
	}
	// Assistant text must arrive as message_update deltas, reassembled = "hi there".
	var deltas strings.Builder
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var rec map[string]any
		_ = json.Unmarshal([]byte(line), &rec)
		if rec["type"] == "message_update" {
			if d, ok := rec["delta"].(string); ok {
				deltas.WriteString(d)
			}
		}
	}
	if deltas.String() != "hi there" {
		t.Fatalf("reassembled deltas = %q, want %q", deltas.String(), "hi there")
	}
}

// blockedDeepSeekServer answers the first call with a write_note tool call
// (prompt-gated), so the run blocks on human approval.
func blockedDeepSeekServer(t *testing.T) *httptest.Server {
	t.Helper()
	return scriptedDeepSeekServer(t, "note body", "final answer")
}

func TestCodeFacePrintModeCancelsBlockedRunLoudly(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "codemode-blocked-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	t.Setenv("VIVY_API_BASE", blockedDeepSeekServer(t).URL)

	res, out, errOut, err := runCodeFaceMode(t, newDeepSeekTestConfig(t), plugin.Options{
		Mode:   "print",
		Prompt: "write a note",
	})
	if err != nil {
		t.Fatalf("print blocked: %v", err)
	}
	if res.Status != "cancelled" {
		t.Fatalf("status = %q, want cancelled", res.Status)
	}
	if !strings.Contains(errOut, "requires human approval") {
		t.Fatalf("stderr missing loud block notice: %q", errOut)
	}
	if strings.Contains(out, "final answer") {
		t.Fatalf("stdout leaked post-cancel text: %q", out)
	}
}

func firstSessionID(t *testing.T, out string) string {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var rec map[string]any
		_ = json.Unmarshal([]byte(line), &rec)
		if rec["type"] == "session" {
			id, _ := rec["session_id"].(string)
			return id
		}
	}
	t.Fatalf("no session record in output: %s", out)
	return ""
}

func TestCodeFaceJSONModeContinuesNewestSession(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "codemode-continue-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	t.Setenv("VIVY_API_BASE", textOnlyDeepSeekServer(t, "ack").URL)

	// Two invocations share the journal (sessions live there) while each gets
	// a private DataDir, matching how separate vivy-code processes isolate
	// their memory/settings homes.
	cfg := newDeepSeekTestConfig(t)
	cfg2 := cfg
	cfg2.Storage.DataDir = t.TempDir()

	res, out1, _, err := runCodeFaceMode(t, cfg, plugin.Options{Mode: "json", Prompt: "first"})
	if err != nil || res.Status != "completed" {
		t.Fatalf("first run: status=%q err=%v", res.Status, err)
	}
	res, out2, _, err := runCodeFaceMode(t, cfg2, plugin.Options{Mode: "json", Prompt: "second", ContinueNewest: true})
	if err != nil || res.Status != "completed" {
		t.Fatalf("continue run: status=%q err=%v", res.Status, err)
	}
	if id1, id2 := firstSessionID(t, out1), firstSessionID(t, out2); id1 != id2 {
		t.Fatalf("--continue opened a different session: %q vs %q", id1, id2)
	}
}
