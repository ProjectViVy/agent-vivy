package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	backend := NewEinoCommandBackend(manager, sandbox, []string{"go"})
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
	backend := NewEinoCommandBackend(manager, sandbox, []string{"go"})
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

func TestBoundedCommandOutputKeepsLimit(t *testing.T) {
	var output boundedCommandOutput
	output.limit = 4
	_, _ = output.Write([]byte("abcdef"))
	if output.String() != "abcd" || !output.truncated {
		t.Fatalf("output=%q truncated=%v", output.String(), output.truncated)
	}
}
