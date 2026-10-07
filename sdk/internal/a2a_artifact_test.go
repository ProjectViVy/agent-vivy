package sdk

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestA2APackedArtifactSmoke is the physical artifact gate (A2A-06 plan
// Step 3): pack the explicit a2a recipe into a real executable, run it
// against an isolated scripted OpenAI-compatible model server and a
// temporary native config, then hand the listener origin + bearer to the
// plugin's official-client smoke through test-only env vars. The
// scripted model requests a real governed tool before its final text —
// an echo-only success would not prove the native governed path.
func TestA2APackedArtifactSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("packed-artifact smoke skipped in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	repoRoot, err := findRepoRoot(".")
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	modelSrv := scriptedA2AModelServer(t)

	a2aPort := freeTCPPort(t)
	token := "artifact-smoke-token"
	workDir := t.TempDir()

	artifact, err := Pack(ctx, packOptions{
		Recipe:  filepath.Join(repoRoot, "recipes", "a2a.vivy.yml"),
		Output:  filepath.Join(workDir, "dist"),
		Sources: []string{filepath.Join(repoRoot, "plugins", "a2a-server")},
	})
	if err != nil {
		t.Fatalf("pack a2a recipe: %v", err)
	}
	if artifact.Binary == "" {
		t.Fatalf("pack produced no binary: %+v", artifact)
	}

	configPath := filepath.Join(workDir, "config.yaml")
	config := fmt.Sprintf(`server:
  addr: "127.0.0.1:%d"
storage:
  backend: sqlite
  data_dir: %q
  sqlite:
    path: %q
providers:
  active: deepseek
runtime:
  workspace_root: %q
  sandbox:
    default_mode: workspace_write
    approval:
      default_policy: ask
      auto_approve_tools:
        - list_dir
tools:
  enabled:
    - list_dir
governance:
  profile: default
  profiles:
    default:
      default: allow
channels:
  a2a:
    enabled: true
    allow_from:
      - pens-local
    http:
      listen: "127.0.0.1:%d"
      public_base_url: "http://127.0.0.1:%d"
      principal:
        id: pens-local
        token_env: VIVY_A2A_TOKEN
`, freeTCPPort(t), workDir, filepath.Join(workDir, "vivy.db"),
		filepath.Join(workDir, "workspace"), a2aPort, a2aPort)
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	binary := exec.CommandContext(ctx, artifact.Binary)
	binary.Dir = workDir
	binary.Env = append(os.Environ(),
		"VIVY_CONFIG="+configPath,
		"DEEPSEEK_API_KEY=a2a-artifact-key",
		"VIVY_PROVIDER=deepseek",
		"VIVY_API_BASE="+modelSrv.URL,
		"VIVY_A2A_TOKEN="+token,
	)
	var stderr strings.Builder
	binary.Stderr = &stderr
	binary.Stdout = &stderr
	if err := binary.Start(); err != nil {
		t.Fatalf("launch binary: %v", err)
	}
	t.Cleanup(func() {
		_ = binary.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- binary.Wait() }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = binary.Process.Kill()
			<-done
		}
		if t.Failed() {
			t.Logf("packed binary output:\n%s", stderr.String())
		}
	})

	base := fmt.Sprintf("http://127.0.0.1:%d", a2aPort)
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := http.Get(base + "/.well-known/agent-card.json")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("a2a listener never came up (last err %v); binary output:\n%s", err, stderr.String())
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}

	// The official-client smoke lives in the plugin module (the only place
	// the a2a-go dependency is allowed). Drive it with test-only env vars.
	smoke := exec.CommandContext(ctx, "go", "test", "-count=1", "-run", "TestA2AArtifactClientSmoke", "-v", ".")
	smoke.Dir = filepath.Join(repoRoot, "plugins", "a2a-server")
	smoke.Env = append(os.Environ(),
		"VIVY_A2A_SMOKE_BASE="+base,
		"VIVY_A2A_SMOKE_TOKEN="+token,
	)
	out, err := smoke.CombinedOutput()
	if err != nil {
		t.Fatalf("official-client smoke failed: %v\n%s", err, out)
	}
	t.Logf("official-client smoke:\n%s", out)
}

// scriptedA2AModelServer speaks the DeepSeek/OpenAI chat.completions
// wire protocol: the first call demands the governed list_dir tool,
// subsequent calls answer with the artifact marker text.
func scriptedA2AModelServer(t *testing.T) *httptest.Server {
	t.Helper()
	var calls atomic.Int64
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		if calls.Add(1) == 1 {
			if !streaming {
				_, _ = io.WriteString(w, `{"id":"chatcmpl-tool","object":"chat.completion","created":1,"model":"deepseek-flash","choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"list_dir","arguments":"{}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
				return
			}
			writeChunk(`{"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"list_dir","arguments":""}}]},"finish_reason":null}]}`)
			writeChunk(`{"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{}"}}]},"finish_reason":null}]}`)
			writeChunk(`{"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			return
		}
		const finalText = "artifact answer: governed tool observed"
		if !streaming {
			_, _ = io.WriteString(w, `{"id":"chatcmpl-final","object":"chat.completion","created":1,"model":"deepseek-flash","choices":[{"index":0,"message":{"role":"assistant","content":"`+finalText+`"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
			return
		}
		writeChunk(`{"id":"chatcmpl-final","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`)
		writeChunk(`{"id":"chatcmpl-final","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{"content":"` + finalText + `"},"finish_reason":null}]}`)
		writeChunk(`{"id":"chatcmpl-final","object":"chat.completion.chunk","created":1,"model":"deepseek-flash","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
}

// freeTCPPort reserves a loopback port for the test's listeners.
func freeTCPPort(t *testing.T) int {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lis.Close()
	return lis.Addr().(*net.TCPAddr).Port
}
