package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/config"
	"agent-vivy/internal/modules/memory"
	controlrpc "agent-vivy/internal/rpc"
	"agent-vivy/internal/runtime"
	"github.com/ProjectViVy/agent-vivy/bml"
)

// The closeout fixture answers a real provider request from the request body
// alone: it extracts the BML context block that the composed ContextHost put
// in a user-role message and returns that body verbatim. The test never hands
// the handler an expected answer or database access.

const bmlCloseoutMarker = "[context: vivy.memory.bml/"

// bmlCloseoutReply derives the assistant answer from one captured
// chat.completions request. An absent BML block answers the explicit
// BML_UNKNOWN control value; an unexpected request shape is a fixture error
// so the run fails loudly instead of inventing an answer.
func bmlCloseoutReply(raw []byte) (string, error) {
	request, err := decodeBMLCloseoutRequest(raw)
	if err != nil {
		return "", err
	}
	for _, message := range request.Messages {
		if message.Role != "user" {
			continue
		}
		for _, text := range message.Texts {
			start := strings.Index(text, bmlCloseoutMarker)
			if start < 0 {
				continue
			}
			labelEnd := strings.Index(text[start:], "]\n")
			if labelEnd < 0 {
				return "", fmt.Errorf("bml context label has no body separator: %q", text[start:])
			}
			body := text[start+labelEnd+len("]\n"):]
			if strings.TrimSpace(body) == "" {
				return "", fmt.Errorf("bml context block has an empty body")
			}
			return body, nil
		}
	}
	return "BML_UNKNOWN", nil
}

type bmlCloseoutMessage struct {
	Role  string
	Texts []string
}

type bmlCloseoutRequest struct {
	Model    string
	Stream   bool
	Messages []bmlCloseoutMessage
}

// decodeBMLCloseoutRequest decodes the openai chat.completions wire shape at
// the real boundary: string content and text-part arrays are both supported;
// any other shape is rejected rather than coerced.
func decodeBMLCloseoutRequest(raw []byte) (bmlCloseoutRequest, error) {
	var wire struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&wire); err != nil {
		return bmlCloseoutRequest{}, fmt.Errorf("decode chat.completions request: %w", err)
	}
	request := bmlCloseoutRequest{Model: wire.Model, Stream: wire.Stream}
	for _, message := range wire.Messages {
		texts, err := bmlCloseoutTexts(message.Content)
		if err != nil {
			return bmlCloseoutRequest{}, fmt.Errorf("%s message content: %w", message.Role, err)
		}
		request.Messages = append(request.Messages, bmlCloseoutMessage{Role: message.Role, Texts: texts})
	}
	return request, nil
}

func bmlCloseoutTexts(raw json.RawMessage) ([]string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(trimmed, &text); err == nil {
		if text == "" {
			return nil, nil
		}
		return []string{text}, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(trimmed, &parts); err != nil {
		return nil, fmt.Errorf("unsupported content shape: %s", trimmed)
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return texts, nil
}

// newBMLCloseoutModel starts the scripted loopback provider and returns a
// snapshot function over the exact request bodies it received. The handler
// captures deep copies under a mutex and derives every reply from the body.
func newBMLCloseoutModel(t *testing.T) (*httptest.Server, func() [][]byte) {
	t.Helper()
	var mu sync.Mutex
	var captured [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			http.Error(w, "bml closeout fixture: read request: "+err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		captured = append(captured, append([]byte(nil), raw...))
		mu.Unlock()
		reply, err := bmlCloseoutReply(raw)
		if err != nil {
			http.Error(w, "bml closeout fixture: "+err.Error(), http.StatusInternalServerError)
			return
		}
		var probe struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			http.Error(w, "bml closeout fixture: decode model/stream: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if probe.Stream {
			writeBMLCloseoutStream(w, probe.Model, reply)
			return
		}
		writeBMLCloseoutJSON(w, probe.Model, reply)
	}))
	t.Cleanup(server.Close)
	snapshot := func() [][]byte {
		mu.Lock()
		defer mu.Unlock()
		out := make([][]byte, len(captured))
		for i, body := range captured {
			out[i] = append([]byte(nil), body...)
		}
		return out
	}
	return server, snapshot
}

func writeBMLCloseoutJSON(w http.ResponseWriter, modelID, reply string) {
	w.Header().Set("Content-Type", "application/json")
	payload, err := json.Marshal(map[string]any{
		"id":      "chatcmpl-bml-closeout",
		"object":  "chat.completion",
		"created": 1,
		"model":   modelID,
		"choices": []any{map[string]any{
			"index":         0,
			"message":       map[string]any{"role": "assistant", "content": reply},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	})
	if err != nil {
		http.Error(w, "bml closeout fixture: marshal reply: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(payload)
}

func writeBMLCloseoutStream(w http.ResponseWriter, modelID, reply string) {
	w.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := w.(http.Flusher)
	write := func(payload map[string]any) {
		raw, err := json.Marshal(payload)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(w, "data: %s\n\n", raw)
		if flusher != nil {
			flusher.Flush()
		}
	}
	write(map[string]any{
		"id": "chatcmpl-bml-closeout", "object": "chat.completion.chunk", "created": 1, "model": modelID,
		"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": reply}, "finish_reason": nil}},
	})
	write(map[string]any{
		"id": "chatcmpl-bml-closeout", "object": "chat.completion.chunk", "created": 1, "model": modelID,
		"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
		"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	})
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

// bmlCloseoutRoot builds one disposable default-App configuration whose data,
// SQLite and workspace paths live under the same scenario root, with the
// operator full_auto profile that admits authenticated module writes.
func bmlCloseoutRoot(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	cfg := uninitializedDeepSeekTestConfig(t)
	cfg.Storage.DataDir = filepath.Join(root, "data")
	cfg.Storage.SQLite.Path = filepath.Join(root, "state", "bml-closeout.db")
	cfg.Runtime.WorkspaceRoot = filepath.Join(root, "workspace")
	for _, dir := range []string{cfg.Storage.DataDir, filepath.Dir(cfg.Storage.SQLite.Path), cfg.Runtime.WorkspaceRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create scenario directory %q: %v", dir, err)
		}
	}
	cfg.Governance.Profile = "full_auto"
	cfg.Governance.Profiles["full_auto"] = config.GovernanceProfile{Default: "allow"}
	initializeTestPersona(t, cfg)
	return cfg
}

func bmlCloseoutNonce(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("random nonce: %v", err)
	}
	return hex.EncodeToString(buf)
}

func invokeBMLMemory(t *testing.T, client *controlrpc.Peer, actionID string, input map[string]any) map[string]any {
	t.Helper()
	return callControl(t, client, "module.action.invoke", map[string]any{
		"module_id": "vivy/memory-bml",
		"action_id": actionID,
		"input":     input,
	})
}

// bmlCloseoutEntry asserts an applied outcome and returns the native record
// identity the action plane reported.
func bmlCloseoutEntry(t *testing.T, outcome map[string]any) (string, int64) {
	t.Helper()
	if outcome["status"] != string(bml.CrudOutcomeApplied) {
		t.Fatalf("memory action outcome = %v, want applied", outcome)
	}
	entry, ok := outcome["entry"].(map[string]any)
	if !ok {
		t.Fatalf("memory action outcome has no entry: %v", outcome)
	}
	id, _ := entry["id"].(string)
	revision, _ := entry["revision"].(float64)
	if id == "" {
		t.Fatalf("memory action entry has no id: %v", entry)
	}
	return id, int64(revision)
}

// TestBMLCloseoutActualModelConsumption proves BML-R2/R3 through the ordinary
// default App and a real loopback provider: a full App close/reopen, a fresh
// session, and one actual model request consume present, corrected, absent and
// removed long-term state. Every subtest owns its root so no earlier inference
// can contaminate a later deletion control.
func TestBMLCloseoutActualModelConsumption(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })

	scenario := func(name string) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			cfg := bmlCloseoutRoot(t)
			nonce := bmlCloseoutNonce(t)
			oldSecret := "old-" + bmlCloseoutNonce(t)
			newSecret := "new-" + bmlCloseoutNonce(t)
			query := "bmlprobe" + nonce
			oldContent := query + " secret=" + oldSecret
			newContent := query + " secret=" + newSecret

			server, captured := newBMLCloseoutModel(t)
			t.Setenv("DEEPSEEK_API_KEY", "bml-closeout-dummy-key")
			t.Setenv("VIVY_PROVIDER", "deepseek")
			t.Setenv("VIVY_API_BASE", server.URL)

			// Compose the ordinary App, record the durable state through the
			// authenticated module action plane, then tear the whole App down.
			first, err := New(ctx, cfg, WithoutEars(), WithoutGateway())
			if err != nil {
				t.Fatalf("compose first App: %v", err)
			}
			firstClient, err := first.DialControl(ctx, nil)
			if err != nil {
				_ = first.Close()
				t.Fatalf("DialControl first App: %v", err)
			}
			callControl(t, firstClient, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})

			var wantID string
			var wantRevision int64
			switch name {
			case "absent":
			case "present":
				wantID, wantRevision = bmlCloseoutEntry(t, invokeBMLMemory(t, firstClient, memory.ActionAdd, map[string]any{
					"kind": "long_term", "content": newContent,
				}))
			case "corrected", "removed":
				id, _ := bmlCloseoutEntry(t, invokeBMLMemory(t, firstClient, memory.ActionAdd, map[string]any{
					"kind": "long_term", "content": oldContent,
				}))
				wantID, wantRevision = bmlCloseoutEntry(t, invokeBMLMemory(t, firstClient, memory.ActionUpdate, map[string]any{
					"id": id, "content": newContent, "base_revision": 1,
				}))
				if name == "removed" {
					bmlCloseoutEntry(t, invokeBMLMemory(t, firstClient, memory.ActionRemove, map[string]any{
						"id": id, "reason": "bmlcloseout removal", "base_revision": wantRevision,
					}))
				}
			default:
				t.Fatalf("unknown scenario %q", name)
			}

			if err := firstClient.Close(); err != nil {
				t.Fatalf("close first control peer: %v", err)
			}
			if err := first.Close(); err != nil {
				t.Fatalf("close first App: %v", err)
			}

			// Reopen the same config and root: a fresh composition, not the
			// closed service.
			second, err := New(ctx, cfg, WithoutEars(), WithoutGateway())
			if err != nil {
				t.Fatalf("compose second App: %v", err)
			}
			t.Cleanup(func() { _ = second.Close() })
			client, err := second.DialControl(ctx, nil)
			if err != nil {
				t.Fatalf("DialControl second App: %v", err)
			}
			t.Cleanup(func() { _ = client.Close() })
			callControl(t, client, "initialize", map[string]any{"protocol_version": "vivy.rpc.v1"})
			session := callControl(t, client, "session/create", map[string]any{"title": "bml closeout " + name})
			sessionID, _ := session["id"].(string)
			if sessionID == "" {
				t.Fatalf("session/create = %v", session)
			}

			turn := callControl(t, client, "turn/start", map[string]any{"session_id": sessionID, "text": query})
			runID, _ := turn["run_id"].(string)
			if runID == "" {
				t.Fatalf("turn/start = %v", turn)
			}
			waitFor(t, 30*time.Second, func() bool {
				run := callControl(t, client, "run/get", map[string]any{"run_id": runID})
				status, _ := run["status"].(string)
				if status == "failed" {
					for _, event := range runEvents(t, client, runID) {
						t.Logf("run event: type=%v payload=%v", event["type"], event["payload"])
					}
					t.Fatalf("run failed: %v", run)
				}
				return status == "completed"
			})

			// Inspect what the provider actually received.
			bodies := captured()
			if len(bodies) == 0 {
				t.Fatal("no model request reached the loopback provider")
			}
			userTexts := make([]string, 0)
			systemTexts := make([]string, 0)
			for _, body := range bodies {
				request, err := decodeBMLCloseoutRequest(body)
				if err != nil {
					t.Fatalf("decode captured request: %v", err)
				}
				if request.Model != "deepseek-flash" {
					t.Fatalf("outbound model = %q, want the raw selected id deepseek-flash", request.Model)
				}
				for _, message := range request.Messages {
					switch message.Role {
					case "user":
						userTexts = append(userTexts, message.Texts...)
					case "system":
						systemTexts = append(systemTexts, message.Texts...)
					}
				}
			}
			allUser := strings.Join(userTexts, "\n")
			for _, text := range systemTexts {
				if strings.Contains(text, "vivy.memory.bml") || strings.Contains(text, newSecret) || strings.Contains(text, oldSecret) {
					t.Fatalf("BML material leaked into a system message: %q", text)
				}
			}

			assistant := strings.TrimSpace(lastAssistantText(t, client, sessionID))
			switch name {
			case "present", "corrected":
				label := bmlCloseoutMarker + wantID + " version=" + strconv.FormatInt(wantRevision, 10) + " "
				if !strings.Contains(allUser, label) {
					t.Fatalf("model request lacks BML framing %q; user content = %q", label, allUser)
				}
				if !strings.Contains(allUser, newContent) {
					t.Fatalf("model request lacks the current BML body %q; user content = %q", newContent, allUser)
				}
				if name == "corrected" && strings.Contains(allUser, oldSecret) {
					t.Fatalf("corrected request still carries the superseded body %q", oldSecret)
				}
				if !strings.Contains(assistant, newContent) {
					t.Fatalf("assistant reply = %q, want the request-derived BML body %q", assistant, newContent)
				}
				if strings.Contains(assistant, oldSecret) {
					t.Fatalf("assistant reply resurrects the superseded body %q", oldSecret)
				}
			case "absent", "removed":
				if strings.Contains(allUser, bmlCloseoutMarker) {
					t.Fatalf("model request carries BML context for %q: %q", name, allUser)
				}
				if assistant != "BML_UNKNOWN" {
					t.Fatalf("assistant reply = %q, want BML_UNKNOWN", assistant)
				}
			}
		})
	}

	for _, name := range []string{"absent", "present", "corrected", "removed"} {
		scenario(name)
	}
}

// lastAssistantText returns the newest assistant projection from the visible
// session history.
func lastAssistantText(t *testing.T, client *controlrpc.Peer, sessionID string) string {
	t.Helper()
	messages := callControl(t, client, "session/messages", map[string]any{"session_id": sessionID})
	items, _ := messages["messages"].([]any)
	for i := len(items) - 1; i >= 0; i-- {
		message, _ := items[i].(map[string]any)
		if message["role"] != "assistant" {
			continue
		}
		content, _ := message["content"].(string)
		return content
	}
	t.Fatalf("no assistant message in session %q: %v", sessionID, messages)
	return ""
}
