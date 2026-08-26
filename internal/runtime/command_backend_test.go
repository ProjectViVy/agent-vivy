package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func TestEinoCommandBackendRunsInsideWorkspaceAndBuildsProposal(t *testing.T) {
	t.Setenv("PATH", `C:\Program Files\Go\bin;`+os.Getenv("PATH"))
	root := t.TempDir()
	manager, err := NewWorkspaceManager(root)
	if err != nil {
		t.Fatal(err)
	}
	// Create permissive sandbox for tests
	sandbox, err := NewSandboxManager(
		domain.SandboxModeDangerFullAccess,
		root,
		[]string{"go"},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	backend := NewEinoCommandBackend(manager, sandbox, []string{"go"}, 30*time.Second)
	result, err := backend.Execute(context.Background(), "run-command", tools.CommandRequest{Command: "go", Args: []string{"version"}, Env: map[string]string{"NO_COLOR": "1"}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.ExitCode != 0 || !strings.Contains(result.Stdout, "go version") || !result.Untrusted {
		t.Fatalf("result = %#v", result)
	}
	workspace, err := manager.Ensure(context.Background(), "run-command")
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := backend.PrepareCommand(context.Background(), "run-command", tools.CommandRequest{Command: "go", Args: []string{"version"}})
	if err != nil || !strings.Contains(proposal.Preview, "go version") || !strings.Contains(proposal.Target, filepath.Base(workspace.Path)) {
		t.Fatalf("proposal=%#v err=%v", proposal, err)
	}
}
func TestEinoCommandBackendRejectsShellEscapesOutsideCwdAndSecrets(t *testing.T) {
	manager, err := NewWorkspaceManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sandbox, err := NewSandboxManager(
		domain.SandboxModeDangerFullAccess,
		t.TempDir(),
		[]string{"go"},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	backend := NewEinoCommandBackend(manager, sandbox, []string{"go"}, 30*time.Second)
	cases := []struct {
		name    string
		request tools.CommandRequest
		want    string
	}{
		{name: "shell", request: tools.CommandRequest{Command: "go;whoami"}, want: "shell syntax"},
		{name: "outside cwd", request: tools.CommandRequest{Command: "go", Cwd: ".."}, want: "escapes"},
		{name: "secret env", request: tools.CommandRequest{Command: "go", Env: map[string]string{"API_KEY": "secret"}}, want: "not allowlisted"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, err := backend.Execute(context.Background(), "run-command", test.request); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v want=%q", err, test.want)
			}
		})
	}
}

func TestEinoCommandBackendTimeoutCeiling(t *testing.T) {
	manager, err := NewWorkspaceManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sandbox, err := NewSandboxManager(
		domain.SandboxModeDangerFullAccess,
		t.TempDir(),
		[]string{"go"},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name       string
		configured time.Duration
		requestMS  int
		want       time.Duration
	}{
		{name: "request above ceiling clamps", configured: 2 * time.Minute, requestMS: 10 * 60 * 1000, want: 2 * time.Minute},
		{name: "request below ceiling keeps request", configured: 2 * time.Minute, requestMS: 5000, want: 5 * time.Second},
		{name: "omitted request keeps per-request default", configured: 2 * time.Minute, requestMS: 0, want: 5 * time.Second},
		{name: "ceiling below per-request default clamps default", configured: 2 * time.Second, requestMS: 0, want: 2 * time.Second},
		{name: "non-positive ceiling falls back to 30s", configured: 0, requestMS: 60 * 1000, want: 30 * time.Second},
		{name: "ceiling above hard cap clamps to 10m", configured: time.Hour, requestMS: 2 * 60 * 60 * 1000, want: 10 * time.Minute},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			backend := NewEinoCommandBackend(manager, sandbox, []string{"go"}, test.configured)
			_, _, _, _, timeout, err := backend.validateRequest(context.Background(), "run-command", tools.CommandRequest{Command: "go", TimeoutMS: test.requestMS})
			if err != nil {
				t.Fatalf("validateRequest: %v", err)
			}
			if timeout != test.want {
				t.Fatalf("timeout = %v, want %v", timeout, test.want)
			}
		})
	}
}

func TestBoundedCommandOutputKeepsLimit(t *testing.T) {
	var output boundedCommandOutput
	output.limit = 4
	_, _ = output.Write([]byte("abcdef"))
	if output.String() != "abcd" || !output.truncated {
		t.Fatalf("output=%q truncated=%v", output.String(), output.truncated)
	}
}
