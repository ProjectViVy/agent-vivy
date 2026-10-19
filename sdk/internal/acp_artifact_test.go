package sdk

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"
	"gopkg.in/yaml.v3"
)

// The ACP artifact tests bind the real Recipe to the real plugin source: the
// selection test always runs, while the packed-binary peer script is opt-in
// because it needs one full `go build` of cmd/vivy.

func readACPRecipe(t *testing.T, repoRoot string) assemblyv1.Recipe {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot, "recipes", "acp.vivy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(raw)))
	decoder.KnownFields(true)
	var recipe assemblyv1.Recipe
	if err := decoder.Decode(&recipe); err != nil {
		t.Fatalf("parse recipes/acp.vivy.yml: %v", err)
	}
	return recipe
}

func TestACPRecipeSelection(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	recipe := readACPRecipe(t, repoRoot)

	pin, ok := recipe.Sources["projectvivy/acp"]
	if !ok {
		t.Fatalf("recipe has no projectvivy/acp source pin: %v", recipe.Sources)
	}
	if pin.Ref != "repo:plugins/acp" {
		t.Fatalf("source pin ref = %q, want repo:plugins/acp", pin.Ref)
	}
	if recipe.Exclusive["std/face@v1"] != "projectvivy/acp" {
		t.Fatalf("exclusive face = %q", recipe.Exclusive["std/face@v1"])
	}
	if len(recipe.GrantApprovals) != 1 || recipe.GrantApprovals[0].Module != "projectvivy/acp" ||
		recipe.GrantApprovals[0].Name != module.GrantRPCClient {
		t.Fatalf("grant approvals = %+v, want only rpc.client for projectvivy/acp", recipe.GrantApprovals)
	}

	// One digest home: the plugin manifest, the Recipe pin and the actual
	// tree must agree, and the packer's masked tree hash must be stable.
	pluginDir := filepath.Join(repoRoot, "plugins", "acp")
	manifestRaw, err := os.ReadFile(filepath.Join(pluginDir, "vivy-module.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Source struct {
			Ref    string `yaml:"ref"`
			SHA256 string `yaml:"sha256"`
		} `yaml:"source"`
	}
	if err := yaml.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Source.Ref != pin.Ref || manifest.Source.SHA256 != pin.SHA256 {
		t.Fatalf("plugin manifest %s/%s != recipe pin %s/%s",
			manifest.Source.Ref, manifest.Source.SHA256, pin.Ref, pin.SHA256)
	}
	treeHash, err := assemblyv1.HashSourceTree(pluginDir, pin.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if treeHash != pin.SHA256 {
		t.Fatalf("plugin tree hash drifted: tree=%s pinned=%s (run `go run ./sdk/internal/cmd/source-hash plugins/acp %s`)",
			treeHash, pin.SHA256, pin.SHA256)
	}

	snapshotRoot, buildSources, err := snapshotSourceDirs(repoRoot, []string{pluginDir})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(snapshotRoot)
	plan, runtimeSource := compileACPPlan(t, repoRoot, recipe, buildSources)
	if !selectedFaceClaimsProtocolMode(&recipe, plan) {
		t.Fatal("the ACP recipe does not claim protocol mode")
	}
	var found bool
	for _, resolved := range plan.Modules {
		if resolved.Descriptor.Module.ID != "projectvivy/acp" {
			continue
		}
		found = true
		if resolved.Trust != assemblyv1.TrustT2 {
			t.Fatalf("projectvivy/acp trust = %v, want T2", resolved.Trust)
		}
		var provides bool
		for _, p := range resolved.Descriptor.Provides {
			if p.Port == "std/face@v1" && p.ID == "projectvivy.acp" {
				provides = true
			}
		}
		if !provides {
			t.Fatalf("projectvivy/acp does not provide std/face@v1: %+v", resolved.Descriptor.Provides)
		}
		var granted bool
		for _, g := range resolved.EffectiveGrants {
			if g.Name == module.GrantRPCClient {
				granted = true
			}
		}
		if !granted {
			t.Fatalf("projectvivy/acp effective grants = %+v, want rpc.client", resolved.EffectiveGrants)
		}
	}
	if !found {
		t.Fatal("compiled plan lacks projectvivy/acp")
	}
	if !strings.Contains(string(runtimeSource), "acp.NewProvider()") {
		t.Fatalf("generated assembly does not construct the ACP provider")
	}

	// The real recipe must stage the pack-only TUI stub, so the artifact's
	// process surface is the protocol and not the terminal face.
	overlayFile, err := stagePackOverlay(repoRoot, t.TempDir(), &recipe, plan, runtimeSource)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(overlayFile)
	if err != nil {
		t.Fatal(err)
	}
	var overlay struct {
		Replace map[string]string `json:"Replace"`
	}
	if err := json.Unmarshal(raw, &overlay); err != nil {
		t.Fatal(err)
	}
	if _, ok := overlay.Replace[filepath.Join(repoRoot, "cmd", "vivy", "tui.go")]; !ok {
		t.Fatalf("ACP overlay lacks the tui.go replacement: %v", overlay.Replace)
	}

	// No other Recipe may select the ACP face implicitly.
	recipes, err := filepath.Glob(filepath.Join(repoRoot, "recipes", "*.vivy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range recipes {
		if filepath.Base(path) == "acp.vivy.yml" {
			continue
		}
		rawRecipe, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var other assemblyv1.Recipe
		if err := yaml.Unmarshal(rawRecipe, &other); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if other.Exclusive["std/face@v1"] == "projectvivy/acp" {
			t.Fatalf("%s selects the ACP face", filepath.Base(path))
		}
	}
}

// acpPeerScript is a minimal newline-delimited JSON-RPC client for the packed
// artifact. It never assumes a model is configured: every reply is evidence
// about the protocol surface, including governed errors.
type acpPeerScript struct {
	t   *testing.T
	in  io.WriteCloser
	out *json.Decoder
}

func (p *acpPeerScript) send(id int, method string, params any) {
	p.t.Helper()
	frame, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		p.t.Fatal(err)
	}
	if _, err := p.in.Write(append(frame, '\n')); err != nil {
		p.t.Fatalf("write %s: %v", method, err)
	}
}

func (p *acpPeerScript) notify(method string, params any) {
	p.t.Helper()
	frame, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if err != nil {
		p.t.Fatal(err)
	}
	if _, err := p.in.Write(append(frame, '\n')); err != nil {
		p.t.Fatalf("write %s: %v", method, err)
	}
}

// await reads frames until the reply for id arrives, skipping notifications.
func (p *acpPeerScript) await(id int) map[string]any {
	p.t.Helper()
	deadline := time.After(30 * time.Second)
	for {
		type result struct {
			frame map[string]any
			err   error
		}
		ch := make(chan result, 1)
		go func() {
			var frame map[string]any
			err := p.out.Decode(&frame)
			ch <- result{frame: frame, err: err}
		}()
		select {
		case r := <-ch:
			if r.err != nil {
				p.t.Fatalf("read frame %d: %v", id, r.err)
			}
			if got, _ := r.frame["id"].(float64); int(got) == id {
				return r.frame
			}
		case <-deadline:
			p.t.Fatalf("no reply for id %d within 30s", id)
		}
	}
}

func TestACPPackedArtifactScriptedPeer(t *testing.T) {
	if os.Getenv("VIVY_ACP_ARTIFACT_E2E") != "1" {
		t.Skip("set VIVY_ACP_ARTIFACT_E2E=1 to pack and drive the ACP artifact")
	}
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(t.TempDir(), "artifact")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	artifact, err := Pack(ctx, packOptions{
		Recipe:  filepath.Join(repoRoot, "recipes", "acp.vivy.yml"),
		Output:  outputDir,
		Sources: []string{filepath.Join(repoRoot, "plugins", "acp")},
	})
	if err != nil {
		t.Fatalf("pack ACP artifact: %v", err)
	}
	inspected, err := InspectArtifact(artifact.Directory)
	if err != nil {
		t.Fatalf("inspect packed artifact: %v", err)
	}
	var acpModule *assemblyv1.ManifestModule
	for i := range inspected.Manifest.Modules {
		if inspected.Manifest.Modules[i].ID == "projectvivy/acp" {
			acpModule = &inspected.Manifest.Modules[i]
		}
	}
	if acpModule == nil {
		t.Fatalf("artifact manifest lacks projectvivy/acp: %+v", inspected.Manifest.Modules)
	}
	if acpModule.Trust != assemblyv1.TrustT2 {
		t.Fatalf("artifact projectvivy/acp trust = %v, want T2", acpModule.Trust)
	}

	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	if err := os.WriteFile(configPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(home, "workspace")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(inspected.Binary)
	cmd.Env = append(os.Environ(),
		"VIVY_USER_HOME="+home,
		"VIVY_CONFIG="+configPath,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	peer := &acpPeerScript{t: t, in: stdin, out: json.NewDecoder(bufio.NewReader(stdout))}

	peer.send(1, "initialize", map[string]any{
		"protocolVersion":    1,
		"clientCapabilities": map[string]any{},
	})
	init := peer.await(1)
	if init["error"] != nil {
		t.Fatalf("initialize error: %v\nstderr: %s", init["error"], stderr.String())
	}
	initResult, _ := init["result"].(map[string]any)
	if v, _ := initResult["protocolVersion"].(float64); int(v) != 1 {
		t.Fatalf("initialize result = %v", initResult)
	}
	info, _ := initResult["agentInfo"].(map[string]any)
	if info["name"] != "projectvivy/acp" {
		t.Fatalf("agentInfo = %v", info)
	}

	peer.send(2, "session/new", map[string]any{"cwd": workspace, "mcpServers": []any{}})
	newSession := peer.await(2)
	if newSession["error"] != nil {
		t.Fatalf("session/new error: %v\nstderr: %s", newSession["error"], stderr.String())
	}
	newResult, _ := newSession["result"].(map[string]any)
	sessionID, _ := newResult["sessionId"].(string)
	if sessionID == "" {
		t.Fatalf("session/new result lacks a sessionId: %v", newResult)
	}

	// A prompt without a configured model must produce one governed error,
	// not a crash or a silent success.
	peer.send(3, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    []any{map[string]any{"type": "text", "text": "hello"}},
	})
	prompt := peer.await(3)
	if prompt["error"] == nil {
		t.Fatalf("prompt without a configured model did not fail: %v", prompt["result"])
	}
	t.Logf("governed prompt error: %v", prompt["error"])
	peer.notify("session/cancel", map[string]any{"sessionId": sessionID})

	// EOF is the normal Face exit path.
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("artifact exit: %v\nstderr: %s", err, stderr.String())
		}
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("artifact did not exit after stdin EOF\nstderr: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "") && strings.Contains(stderr.String(), "panic") {
		t.Fatalf("artifact panicked: %s", stderr.String())
	}
	fmt.Fprintf(os.Stderr, "ACP artifact: %s\n", inspected.Binary)
}
