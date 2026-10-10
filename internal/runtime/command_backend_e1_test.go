package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func newE1Backend(t *testing.T, opts CommandBackendOptions) (*CommandBackend, *WorkspaceManager) {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available on this host")
	}
	root := t.TempDir()
	manager, err := NewWorkspaceManager(root)
	if err != nil {
		t.Fatal(err)
	}
	sandbox, err := NewSandboxManager(domain.SandboxModeDangerFullAccess, root, []string{"bash", "echo"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewCommandBackend(manager, sandbox, []string{"bash", "echo"}, 30*time.Second, opts), manager
}

func TestCommandEnvInjection(t *testing.T) {
	backend, _ := newE1Backend(t, CommandBackendOptions{})
	ctx := tools.WithSessionID(context.Background(), "sess-env")
	ctx = domain.WithRunLabels(ctx, domain.RunLabels{Provider: "prov-x", Model: "model-y"})
	ctx = domain.WithThinkingMode(ctx, domain.ThinkingModeOn)
	result, err := backend.Execute(ctx, "run-env", tools.CommandRequest{
		Command: "bash",
		Args:    []string{"-c", `printf "%s|%s|%s|%s|%s" "$VIVY_SESSION_ID" "$VIVY_RUN_ID" "$VIVY_PROVIDER" "$VIVY_MODEL" "$VIVY_THINKING"`},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if want := "sess-env|run-env|prov-x|model-y|on"; strings.TrimSpace(result.Stdout) != want {
		t.Fatalf("env = %q, want %q", result.Stdout, want)
	}
	// Non-allowlisted request env stays rejected on the commandline path.
	if _, err := backend.Execute(ctx, "run-env", tools.CommandRequest{
		Command: "echo", Args: []string{"x"}, Env: map[string]string{"API_KEY": "x"},
	}); err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("env override error = %v, want allowlist rejection", err)
	}
}

func TestShellPrefixBashAndCommandline(t *testing.T) {
	backend, _ := newE1Backend(t, CommandBackendOptions{ShellPrefix: "printf 'PRE\\n';"})
	ctx := context.Background()

	bash, err := backend.Execute(ctx, "run-prefix-bash", tools.CommandRequest{Command: "bash", Args: []string{"-c", "echo body"}})
	if err != nil {
		t.Fatalf("bash execute: %v", err)
	}
	if !strings.HasPrefix(bash.Stdout, "PRE\n") || !strings.Contains(bash.Stdout, "body") {
		t.Fatalf("prefixed bash stdout = %q", bash.Stdout)
	}

	wrapped, err := backend.Execute(ctx, "run-prefix-cli", tools.CommandRequest{
		Command: "echo", Args: []string{"a b", "c"}, ApplyShellPrefix: true,
	})
	if err != nil {
		t.Fatalf("commandline execute: %v", err)
	}
	if wrapped.Stdout != "PRE\na b c\n" {
		t.Fatalf("prefixed commandline stdout = %q, argv must survive verbatim", wrapped.Stdout)
	}

	// The plain execute path never picks up the prefix.
	plain, err := backend.Execute(ctx, "run-prefix-exec", tools.CommandRequest{Command: "echo", Args: []string{"hi"}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if plain.Stdout != "hi\n" {
		t.Fatalf("unprefixed stdout = %q", plain.Stdout)
	}
}

func TestCommandOutputSpillsToWorkspace(t *testing.T) {
	testCommandOutputSpillsToWorkspace(t, context.Background())
}

func testCommandOutputSpillsToWorkspace(t *testing.T, ctx context.Context) {
	t.Helper()
	backend, manager := newE1Backend(t, CommandBackendOptions{})
	result, err := backend.Execute(ctx, "run-spill", tools.CommandRequest{
		Command: "bash", Args: []string{"-c", `printf '%200000s' x`},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.StdoutSpillPath == "" || result.StdoutTotalBytes != 200000 || !result.StdoutTrunc {
		t.Fatalf("result = %+v", result)
	}
	workspace, err := manager.Ensure(context.Background(), "run-spill")
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(workspace.Path, ".vivy", "tool-output")
	if filepath.Dir(result.StdoutSpillPath) != wantDir {
		t.Fatalf("spill dir = %q, want %q", result.StdoutSpillPath, wantDir)
	}
	data, err := os.ReadFile(result.StdoutSpillPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 200000 {
		t.Fatalf("spill file = %d bytes, want 200000", len(data))
	}
	if len(result.Stdout) >= 200000 {
		t.Fatalf("inline stdout = %d bytes, want bounded tail", len(result.Stdout))
	}
}

// Windows uses the embedded interpreter; exercise its actual spill path on
// every platform so fixture scripts cannot silently depend on OS Bash printf.
func TestCommandOutputSpillsToWorkspaceEmbedded(t *testing.T) {
	testCommandOutputSpillsToWorkspace(t, withDirectShell(context.Background()))
}
