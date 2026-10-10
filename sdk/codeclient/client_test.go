package codeclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The protocol fixture is this test executable, so it runs on Windows as well
// as Unix without a shebang, shell, or external text-processing commands.
func TestMain(m *testing.M) {
	if os.Getenv("VIVY_CODECLIENT_RPC_FIXTURE") != "1" {
		os.Exit(m.Run())
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var command struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &command); err != nil {
			os.Exit(1)
		}
		response := map[string]any{"id": command.ID, "type": "response", "command": command.Type, "success": true, "data": nil}
		switch command.Type {
		case "get_state":
			response["data"] = map[string]any{"session_id": "fake-1", "isStreaming": false, "pendingMessageCount": 0}
		case "prompt":
			for _, event := range []map[string]any{{"type": "agent_start", "run_id": "r1"}, {"type": "message_update", "delta": "hi "}, {"type": "message_update", "delta": "there"}, {"type": "agent_settled", "status": "completed"}} {
				if err := encoder.Encode(event); err != nil {
					os.Exit(1)
				}
			}
			response["data"] = map[string]any{"disposition": "started", "run_id": "r1"}
		case "steer":
			response["success"], response["error"] = false, "steer not implemented yet (lands with B1 steering)"
		case "abort":
		default:
			response["success"], response["error"] = false, "unknown"
		}
		if err := encoder.Encode(response); err != nil {
			os.Exit(1)
		}
	}
	if scanner.Err() != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func fakeRPCConfig() Config {
	return Config{Binary: os.Args[0], Env: []string{"VIVY_CODECLIENT_RPC_FIXTURE=1"}}
}

func TestClientCorrelatesResponsesAndStreamsEvents(t *testing.T) {
	c, err := New(fakeRPCConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	sub := c.Subscribe()
	defer sub.Close()

	disp, err := c.Prompt(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if disp.Disposition != "started" || disp.RunID != "r1" {
		t.Fatalf("disposition: %+v", disp)
	}

	var types []string
	timeout := time.After(5 * time.Second)
	for len(types) < 4 {
		select {
		case ev, ok := <-sub.Events:
			if !ok {
				t.Fatal("event channel closed early")
			}
			types = append(types, ev.Type)
		case <-timeout:
			t.Fatalf("events: %v", types)
		}
	}
	want := []string{"agent_start", "message_update", "message_update", "agent_settled"}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("events = %v, want prefix %v", types, want)
		}
	}
}

func TestClientPropagatesProtocolErrors(t *testing.T) {
	c, err := New(fakeRPCConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Steer(context.Background(), "nudge"); err == nil ||
		!strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("steer err = %v", err)
	}
}

func TestClientRejectsModeOverrideAndMissingBinary(t *testing.T) {
	if _, err := New(Config{Binary: "x", Args: []string{"--mode", "json"}}); err == nil {
		t.Fatal("mode override accepted")
	}
	if _, err := New(Config{}); err == nil {
		t.Fatal("empty Binary accepted")
	}
	if _, err := New(Config{Binary: "/nonexistent/binary"}); err == nil {
		t.Fatal("missing binary accepted")
	}
}

// E2E: a real `vivy-code --mode rpc` binary driven through prompt → events →
// last text, against the same frozen-env scripted model the in-process tests
// use.
func TestClientAgainstRealVivyCode(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the vivy-code binary")
	}
	bin := filepath.Join(t.TempDir(), "vivy-code")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", bin, "./cmd/vivy-code")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build vivy-code: %v\n%s", err, out)
	}

	var serveSSE = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, word := range []string{"codeclient", " ", "e2e"} {
			chunk := fmt.Sprintf(`{"choices":[{"delta":{"content":%q}}]}`, word)
			fmt.Fprintf(w, "data: %s\n\n", chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}
	srv := httptest.NewServer(http.HandlerFunc(serveSSE))
	defer srv.Close()

	c, err := New(Config{
		Binary: bin,
		Dir:    t.TempDir(),
		Env: []string{
			"DEEPSEEK_API_KEY=codeclient-e2e",
			"VIVY_PROVIDER=deepseek",
			"VIVY_MODEL=deepseek-chat",
			"VIVY_API_BASE=" + srv.URL,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	sub := c.Subscribe()
	defer sub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	disp, err := c.Prompt(ctx, "say hi")
	if err != nil {
		t.Fatal(err)
	}
	if disp.Disposition != "started" {
		t.Fatalf("disposition: %+v", disp)
	}
	deadline := time.After(30 * time.Second)
	for {
		select {
		case ev, ok := <-sub.Events:
			if !ok {
				t.Fatal("event stream ended before agent_settled")
			}
			if ev.Type == "agent_settled" {
				goto settled
			}
		case <-deadline:
			t.Fatal("no agent_settled within deadline")
		}
	}
settled:
	text, err := c.LastAssistantText(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if text != "codeclient e2e" {
		t.Fatalf("last assistant text = %q", text)
	}
	st, err := c.GetState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if st.IsStreaming || st.SessionID == "" {
		t.Fatalf("state after settle: %+v", st)
	}
	if err := c.SetSessionName(ctx, "codeclient e2e"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Steer(ctx, "x"); err != nil {
		t.Fatalf("steer on settled session should start a run: %v", err)
	}
}
