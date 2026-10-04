package host_test

// Boundary tests for the public VIVY Go host (DIVA Next W3-1 contract).
// The package under test is the only API a native embedder (agent-diva's
// Wails shell) is allowed to see: no internal types may leak into its
// surface, and only a sealed Generation may open.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-vivy/internal/runtime"
	"agent-vivy/sdk/generation"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"

	host "agent-vivy/sdk/host/v1"
)

// sealedFixtureGeneration produces a manifest through the real assembly
// sealer (not a hand-typed JSON blob) and installs it as the process's
// embedded Generation for the duration of the test.
func sealedFixtureGeneration(t *testing.T) {
	t.Helper()
	descriptor := module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: "fixture/search", Version: "1.0.0"},
		Source: module.Source{
			Ref:    "git:fixture/search@0123456",
			SHA256: strings.Repeat("ab", 32),
		},
		Provides:  []module.PortRef{{Port: "std/tool@v1", ID: "fixture.search"}},
		Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration},
	}
	plan := assemblyv1.AssemblyPlan{
		Modules:        []assemblyv1.ResolvedModule{{Descriptor: descriptor, Trust: assemblyv1.TrustT2}},
		LifecycleOrder: []string{"fixture/search"},
	}
	_, raw, err := assemblyv1.SealManifest(plan, assemblyv1.SealInputs{
		SpecificationVersion: "vivy.assembly/v1",
		CompilerVersion:      "fixture-compiler",
		SDKVersion:           "fixture-sdk",
		CanonicalRecipe:      []byte(`{"apiVersion":"vivy.generation/v1","modules":["fixture/search"]}`),
	})
	if err != nil {
		t.Fatalf("seal fixture manifest: %v", err)
	}
	setEmbeddedManifest(t, generation.FrameEmbeddedManifest(raw))
}

func setEmbeddedManifest(t *testing.T, framed string) {
	t.Helper()
	previous := generation.EmbeddedManifestBase64
	generation.EmbeddedManifestBase64 = framed
	t.Cleanup(func() { generation.EmbeddedManifestBase64 = previous })
}

// writeConfig renders a minimal valid config.yaml inside dir and returns its
// absolute path.
func writeConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	doc := strings.Join([]string{
		"server:",
		`  addr: "127.0.0.1:0"`,
		"storage:",
		`  backend: sqlite`,
		"  data_dir: " + strconv.Quote(dir),
		"  sqlite:",
		"    path: " + strconv.Quote(filepath.Join(dir, "vivy.db")),
		"providers:",
		"  active: deepseek",
		"runtime:",
		"  stream_buffer: 8",
		"  max_event_payload_bytes: 65536",
		"  workspace_root: " + strconv.Quote(filepath.Join(dir, "workspace")),
		"  cron:",
		"    enabled: false",
		"tools:",
		"  enabled: [write_note]",
		"governance:",
		"  profile: default",
		"  profiles:",
		"    default:",
		"      default: allow",
		"logging:",
		"  stdout: false",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// providerEnv satisfies the composition path that resolves the configured
// provider; no model traffic is driven by these tests.
func providerEnv(t *testing.T) {
	t.Helper()
	runtime.SetEngineVersionOverride("v0.9.13")
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("DEEPSEEK_API_KEY", "host-boundary-test-key")
	t.Setenv("VIVY_PROVIDER", "deepseek")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unused", http.StatusTeapot)
	}))
	t.Setenv("VIVY_API_BASE", srv.URL)
	t.Cleanup(srv.Close)
}

func openSealed(t *testing.T) (*host.Host, string) {
	t.Helper()
	sealedFixtureGeneration(t)
	providerEnv(t)
	dir := t.TempDir()
	h, err := host.Open(context.Background(), host.Options{
		ConfigPath:  writeConfig(t, dir),
		WithoutEars: true,
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = h.Close(context.Background()) })
	return h, dir
}

func hostError(t *testing.T, err error) *host.Error {
	t.Helper()
	var herr *host.Error
	if !errors.As(err, &herr) {
		t.Fatalf("error %T %v is not *host.Error", err, err)
	}
	return herr
}

func TestOpenSealedAndTrustedCall(t *testing.T) {
	h, dir := openSealed(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	raw, err := h.Call(ctx, "initialize", nil)
	if err != nil {
		t.Fatalf("Call(initialize): %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode initialize: %v", err)
	}
	if result["protocol_version"] != "vivy.rpc.v1" {
		t.Fatalf("protocol_version = %v", result["protocol_version"])
	}
	// W3-5: embedded mode initializes the runtime's redacted, rotated
	// logging exactly once under the config's log directory.
	logs, err := filepath.Glob(filepath.Join(dir, "logs", "vivy.log.*"))
	if err != nil || len(logs) == 0 {
		t.Fatalf("no rotated log file under %s: %v", dir, err)
	}
	blob, err := os.ReadFile(logs[0])
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(blob)), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line is not redacted JSON: %q", line)
		}
	}
	if !strings.Contains(string(blob), "vivy host opened") {
		t.Fatalf("log file does not record the open milestone: %s", logs[0])
	}
}

func TestOpenRejectsMissingGeneration(t *testing.T) {
	setEmbeddedManifest(t, "")
	providerEnv(t)
	dir := t.TempDir()
	_, err := host.Open(context.Background(), host.Options{ConfigPath: writeConfig(t, dir), WithoutEars: true})
	herr := hostError(t, err)
	if herr.Kind != "incompatible_generation" || herr.Code != -32087 {
		t.Fatalf("kind/code = %s/%d, want incompatible_generation/-32087", herr.Kind, herr.Code)
	}
}

func TestOpenRejectsCorruptGeneration(t *testing.T) {
	setEmbeddedManifest(t, generation.FrameEmbeddedManifest([]byte(`{"generationId":`)))
	providerEnv(t)
	dir := t.TempDir()
	_, err := host.Open(context.Background(), host.Options{ConfigPath: writeConfig(t, dir), WithoutEars: true})
	herr := hostError(t, err)
	if herr.Kind != "incompatible_generation" {
		t.Fatalf("kind = %s, want incompatible_generation", herr.Kind)
	}
}

func TestOpenRejectsInvalidConfigInput(t *testing.T) {
	sealedFixtureGeneration(t)
	providerEnv(t)
	cases := map[string]string{
		"empty":       "",
		"relative":    "config.yaml",
		"nonexistent": filepath.Join(t.TempDir(), "absent.yaml"),
	}
	for name, path := range cases {
		_, err := host.Open(context.Background(), host.Options{ConfigPath: path, WithoutEars: true})
		herr := hostError(t, err)
		if herr.Kind != "invalid_input" || herr.Code != -32602 {
			t.Fatalf("%s: kind/code = %s/%d, want invalid_input/-32602", name, herr.Kind, herr.Code)
		}
	}
}

func TestFailedOpenReleasesOwnership(t *testing.T) {
	sealedFixtureGeneration(t)
	providerEnv(t)
	// Composition failure past config load: data_dir names an existing FILE,
	// so app.New cannot create the data root and Open must unwind.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	bad := filepath.Join(dir, "config.yaml")
	doc := strings.Join([]string{
		"server:",
		`  addr: "127.0.0.1:0"`,
		"storage:",
		`  backend: sqlite`,
		"  data_dir: " + strconv.Quote(blocker),
		"  sqlite:",
		"    path: " + strconv.Quote(filepath.Join(blocker, "vivy.db")),
		"providers:",
		"  active: deepseek",
		"runtime:",
		"  workspace_root: " + strconv.Quote(filepath.Join(dir, "workspace")),
		"  cron:",
		"    enabled: false",
		"tools:",
		"  enabled: [write_note]",
		"governance:",
		"  profile: default",
		"",
	}, "\n")
	if err := os.WriteFile(bad, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := host.Open(context.Background(), host.Options{ConfigPath: bad, WithoutEars: true}); err == nil {
		t.Fatal("Open against a file data_dir succeeded")
	}
	// The failed startup must not hold the single-owner claim: a valid open
	// immediately afterwards has to succeed.
	goodDir := t.TempDir()
	h, err := host.Open(context.Background(), host.Options{ConfigPath: writeConfig(t, goodDir), WithoutEars: true})
	if err != nil {
		t.Fatalf("Open after failed startup: %v", err)
	}
	defer func() { _ = h.Close(context.Background()) }()
}

func TestSecondLiveOwnerRejected(t *testing.T) {
	h, _ := openSealed(t)
	dir := t.TempDir()
	_, err := host.Open(context.Background(), host.Options{ConfigPath: writeConfig(t, dir), WithoutEars: true})
	herr := hostError(t, err)
	if herr.Kind != "already_initialized" || herr.Code != -32082 {
		t.Fatalf("kind/code = %s/%d, want already_initialized/-32082", herr.Kind, herr.Code)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := h.Call(ctx, "initialize", nil); err != nil {
		t.Fatalf("first owner Call after second Open: %v", err)
	}
}

func TestPostCloseRejectsCallAndNext(t *testing.T) {
	h, _ := openSealed(t)
	if err := h.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := h.Call(context.Background(), "initialize", nil); err == nil {
		t.Fatal("Call after Close succeeded")
	} else {
		herr := hostError(t, err)
		if herr.Kind != "closed" || herr.Code != -32081 {
			t.Fatalf("Call kind/code = %s/%d, want closed/-32081", herr.Kind, herr.Code)
		}
	}
	if _, err := h.Next(context.Background(), 0); err == nil {
		t.Fatal("Next after Close succeeded")
	} else {
		herr := hostError(t, err)
		if herr.Kind != "closed" {
			t.Fatalf("Next kind = %s, want closed", herr.Kind)
		}
	}
	if err := h.Close(context.Background()); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestCallEnforcesEncodedRequestBound(t *testing.T) {
	h, _ := openSealed(t)
	// One byte over the 4 MiB encoded-request bound must fail before dispatch.
	big := `{"blob":"` + strings.Repeat("x", 4<<20) + `"}`
	_, err := h.Call(context.Background(), "initialize", json.RawMessage(big))
	herr := hostError(t, err)
	if herr.Kind != "invalid_input" || herr.Code != -32602 {
		t.Fatalf("kind/code = %s/%d, want invalid_input/-32602", herr.Kind, herr.Code)
	}
}

func TestCallPreservesRPCErrorIdentity(t *testing.T) {
	h, _ := openSealed(t)
	_, err := h.Call(context.Background(), "no/such/method", nil)
	herr := hostError(t, err)
	if herr.Kind != "rpc" || herr.Code != -32601 {
		t.Fatalf("kind/code = %s/%d, want rpc/-32601", herr.Kind, herr.Code)
	}
}

func TestCallHonorsCallerCancellation(t *testing.T) {
	h, _ := openSealed(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.Call(ctx, "session/list", nil)
	herr := hostError(t, err)
	if herr.Kind != "cancelled" && herr.Kind != "timeout" {
		t.Fatalf("kind = %s, want cancelled", herr.Kind)
	}
}

func TestNextRejectsInvalidLimit(t *testing.T) {
	h, _ := openSealed(t)
	for _, limit := range []int{-1, 501, 1 << 20} {
		_, err := h.Next(context.Background(), limit)
		herr := hostError(t, err)
		if herr.Kind != "invalid_input" {
			t.Fatalf("limit %d: kind = %s, want invalid_input", limit, herr.Kind)
		}
	}
}

func TestNextHasSingleReader(t *testing.T) {
	h, _ := openSealed(t)
	ctx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := h.Next(ctx, 0); first <- err }()
	// Give the first reader a moment to claim the pump, then a concurrent
	// second Next must be rejected rather than queue silently.
	time.Sleep(100 * time.Millisecond)
	_, err := h.Next(context.Background(), 0)
	herr := hostError(t, err)
	if herr.Kind != "invalid_input" && herr.Kind != "not_ready" {
		t.Fatalf("concurrent Next kind = %s, want invalid_input/not_ready", herr.Kind)
	}
	cancel()
	if err := <-first; err == nil {
		t.Fatal("cancelled Next returned nil error")
	}
}

func TestConcurrentCallNextCloseRace(t *testing.T) {
	h, _ := openSealed(t)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, _ = h.Call(context.Background(), "session/list", nil)
			}
		}()
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			_, _ = h.Next(ctx, 0)
		}()
	}
	wg.Wait()
	if err := h.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
