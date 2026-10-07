package sdk

import (
	assemblyv1 "agent-vivy/sdk/internal/assembly"
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

// TestA2ASelectedAndOmittedArtifacts proves physical selection and
// physical omission in one fixture (A2A-06 plan Task .3 Step 1): the
// a2a recipe's artifact carries the module, port edges, grants and SDK
// dependency; the default and minimal recipes' artifacts carry none of
// them — a successful default build alone is not omission evidence.
func TestA2ASelectedAndOmittedArtifacts(t *testing.T) {
	if testing.Short() {
		t.Skip("artifact packing skipped in -short mode")
	}
	root := t.TempDir()
	repoRoot, err := findRepoRoot(".")
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	pack := func(name string) Artifact {
		t.Helper()
		artifact, err := Pack(context.Background(), packOptions{
			Recipe:  filepath.Join(repoRoot, "recipes", name+".vivy.yml"),
			Output:  filepath.Join(root, name),
			Sources: []string{filepath.Join(repoRoot, "plugins", "a2a-server")},
		})
		if err != nil {
			t.Fatalf("pack %s: %v", name, err)
		}
		return artifact
	}

	selected := pack("a2a")
	inspected, err := InspectArtifact(selected.Directory)
	if err != nil {
		t.Fatal(err)
	}

	var module *assemblyv1.ManifestModule
	for i := range inspected.Manifest.Modules {
		if inspected.Manifest.Modules[i].ID == "projectvivy/a2a-server" {
			module = &inspected.Manifest.Modules[i]
		}
	}
	if module == nil {
		t.Fatalf("selected artifact missing projectvivy/a2a-server: %+v", inspected.Manifest.Modules)
	}
	var sawProvides bool
	for _, p := range module.Provides {
		if p.Port == "std/channel@v1" && p.ID == "vivy.a2a" {
			sawProvides = true
		}
	}
	if !sawProvides {
		t.Fatalf("selected module provides = %+v", module.Provides)
	}
	grants := map[string]bool{}
	for _, g := range module.EffectiveGrants {
		grants[string(g.Name)] = true
	}
	if !grants["channel.a2a"] || !grants["secret.read"] {
		t.Fatalf("selected module grants = %+v", module.EffectiveGrants)
	}
	var sawEdge bool
	for _, e := range inspected.Manifest.PortEdges {
		if e.Provider == "projectvivy/a2a-server" && e.Consumer == "vivy/channel-host" && e.Port.ID == "vivy.a2a" {
			sawEdge = true
		}
	}
	if !sawEdge {
		t.Fatalf("selected artifact missing a2a provider edge: %+v", inspected.Manifest.PortEdges)
	}

	// The generated binder and the built binary must carry the module:
	// selection is physical, not just manifest metadata.
	binder, err := os.ReadFile(filepath.Join(selected.Directory, "zz_assembly.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"a2aserver", "projectvivy/a2a-server"} {
		if !strings.Contains(string(binder), needle) {
			t.Fatalf("selected binder missing %q", needle)
		}
	}
	if !strings.Contains(binaryBuildInfo(t, selected.Binary), "a2a-go") {
		t.Fatal("selected binary build info lacks the a2a-go dependency")
	}

	// Physical omission: default and minimal artifacts must carry neither
	// the module nor the official SDK dependency.
	for _, name := range []string{"default", "minimal"} {
		other := pack(name)
		insp, err := InspectArtifact(other.Directory)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range insp.Manifest.Modules {
			if m.ID == "projectvivy/a2a-server" {
				t.Errorf("%s manifest retained a2a module", name)
			}
		}
		for _, e := range insp.Manifest.PortEdges {
			if e.Provider == "projectvivy/a2a-server" || e.Consumer == "projectvivy/a2a-server" {
				t.Errorf("%s manifest retained a2a port edge: %+v", name, e)
			}
		}
		b, err := os.ReadFile(filepath.Join(other.Directory, "zz_assembly.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, needle := range []string{"a2aserver", "projectvivy/a2a-server"} {
			if strings.Contains(string(b), needle) {
				t.Errorf("%s binder retained %q", name, needle)
			}
		}
		if strings.Contains(binaryBuildInfo(t, other.Binary), "a2a-go") {
			t.Errorf("%s binary retained the a2a-go dependency", name)
		}
	}

	// The plugin keeps no server ownership and no Core imports: the Host
	// owns the bind, and isolation stays inside the optional Module.
	pluginDir := filepath.Join(repoRoot, "plugins", "a2a-server")
	err = filepath.WalkDir(pluginDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, banned := range []string{"net.Listen", "ListenAndServe", "agent-vivy/internal"} {
			if strings.Contains(string(body), banned) {
				t.Errorf("plugin source %s references %q", path, banned)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// binaryBuildInfo returns the module/dependency metadata embedded in a
// Go binary — the physical proof of what was actually compiled in.
func binaryBuildInfo(t *testing.T, binary string) string {
	t.Helper()
	out, err := exec.Command("go", "version", "-m", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("go version -m %s: %v\n%s", binary, err, out)
	}
	return string(out)
}
