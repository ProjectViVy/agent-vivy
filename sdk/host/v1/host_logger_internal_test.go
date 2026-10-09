package host

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/internal/embedded"
	"agent-vivy/sdk/generation"
	assemblyv1 "agent-vivy/sdk/internal/assembly"
	"agent-vivy/sdk/module"
)

func TestHostFailedOpenClosesOwnedSinkAndReleasesSlot(t *testing.T) {
	setHostTestManifest(t)
	dir := t.TempDir()
	configPath := writeHostTestConfig(t, dir)
	previous := slog.New(slog.NewTextHandler(os.Stderr, nil))
	oldDefault := slog.Default()
	slog.SetDefault(previous)
	t.Cleanup(func() { slog.SetDefault(oldDefault) })

	originalOpen := openEmbeddedRuntime
	t.Cleanup(func() { openEmbeddedRuntime = originalOpen })
	startupErr := errors.New("injected composition failure")
	var calls int
	var retained []*slog.Logger
	var newerLogger *slog.Logger
	openEmbeddedRuntime = func(context.Context, config.Config, embedded.Options) (*embedded.Host, error) {
		calls++
		logger := slog.Default()
		retained = append(retained, logger)
		logger.Info("injected post-logging composition failure")
		if calls == 2 {
			newerLogger = slog.New(slog.NewTextHandler(os.Stderr, nil))
			slog.SetDefault(newerLogger)
		}
		return nil, startupErr
	}

	if _, err := Open(context.Background(), Options{ConfigPath: configPath, WithoutEars: true}); err == nil {
		t.Fatal("Open unexpectedly succeeded")
	}
	if slog.Default() != previous {
		t.Fatal("failed Open did not restore the previous default logger")
	}
	if _, err := Open(context.Background(), Options{ConfigPath: configPath, WithoutEars: true}); err == nil {
		t.Fatal("second injected Open unexpectedly succeeded")
	}
	if calls != 2 {
		t.Fatalf("embedded startup calls=%d, want 2; failed Open retained the owner slot", calls)
	}
	if slog.Default() != newerLogger {
		t.Fatal("failed Open overwrote a newer process logger")
	}
	// The second callback replaces the process default while the host logger
	// is installed; cleanup must preserve that newer logger.
	if _, err := os.Stat(filepath.Join(dir, "logs")); err != nil {
		t.Fatalf("logging directory missing: %v", err)
	}
	paths, err := filepath.Glob(filepath.Join(dir, "logs", "vivy.log.*"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("log files=%v err=%v, want one", paths, err)
	}
	info, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	retained[0].Info("late write after failed Open")
	infoAfter, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if infoAfter.Size() != info.Size() {
		t.Fatalf("failed Open retained logger changed closed sink: %d -> %d", info.Size(), infoAfter.Size())
	}
}

func setHostTestManifest(t *testing.T) {
	t.Helper()
	descriptor := module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: "fixture/logger", Version: "1.0.0"},
		Source:     module.Source{Ref: "git:fixture/logger@0123456", SHA256: strings.Repeat("ab", 32)},
		Provides:   []module.PortRef{{Port: "std/tool@v1", ID: "fixture.logger"}},
		Lifecycle:  module.Lifecycle{Scope: module.ScopeGeneration},
	}
	plan := assemblyv1.AssemblyPlan{Modules: []assemblyv1.ResolvedModule{{Descriptor: descriptor, Trust: assemblyv1.TrustT2}}, LifecycleOrder: []string{"fixture/logger"}}
	_, raw, err := assemblyv1.SealManifest(plan, assemblyv1.SealInputs{
		SpecificationVersion: "vivy.assembly/v1", CompilerVersion: "fixture-compiler", SDKVersion: "fixture-sdk",
		CanonicalRecipe: []byte(`{"apiVersion":"vivy.generation/v1","modules":["fixture/logger"]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	previous := generation.EmbeddedManifestBase64
	generation.EmbeddedManifestBase64 = generation.FrameEmbeddedManifest(raw)
	t.Cleanup(func() { generation.EmbeddedManifestBase64 = previous })
}

func writeHostTestConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	doc := strings.Join([]string{
		"server:", `  addr: "127.0.0.1:0"`, "storage:", `  backend: sqlite`,
		"  data_dir: " + strconv.Quote(dir), "  sqlite:", `    path: "` + filepath.Join(dir, "vivy.db") + `"`,
		"providers:", `  active: deepseek`, "runtime:", `  stream_buffer: 8`,
		`  max_event_payload_bytes: 65536`, `  workspace_root: "` + filepath.Join(dir, "workspace") + `"`,
		"  cron:", `    enabled: false`, "tools:", "  enabled: [write_note]", "governance:",
		"  profile: default", "  profiles:", "    default:", "      default: allow", "logging:", "  stdout: false", "",
	}, "\n")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
