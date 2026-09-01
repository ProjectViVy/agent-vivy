package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
)

func TestSandboxManagerPerCallMode(t *testing.T) {
	root := t.TempDir()
	mgr, err := NewSandboxManager(domain.SandboxModeWorkspaceWrite, root, []string{"go"}, &domain.NetworkPolicy{DenyPrivateIPs: true})
	if err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, "note.md")
	if err := os.WriteFile(inside, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.ValidatePathWithMode(inside, FileOpWrite, domain.SandboxModeReadOnly); !errors.Is(err, ErrSandboxDenied) {
		t.Fatalf("read-only write = %v", err)
	}
	if err := mgr.ValidatePathWithMode(inside, FileOpWrite, domain.SandboxModeWorkspaceWrite); err != nil {
		t.Fatalf("workspace write = %v", err)
	}
	if err := mgr.ConfineCommandWithMode("go", nil, domain.SandboxModeReadOnly); !errors.Is(err, ErrSandboxDenied) {
		t.Fatalf("read-only command = %v", err)
	}
	if err := mgr.ConfineCommandWithMode("python", nil, domain.SandboxModeDangerFullAccess); err != nil {
		t.Fatalf("trusted command = %v", err)
	}
	if err := mgr.ConfineCommandWithMode("python", nil, domain.SandboxModeWorkspaceWrite); !errors.Is(err, ErrSandboxDenied) {
		t.Fatalf("smart unknown command = %v", err)
	}
}

func TestSandboxManagerLiveNetworkPolicy(t *testing.T) {
	root := t.TempDir()
	mgr, err := NewSandboxManager(domain.SandboxModeWorkspaceWrite, root, nil, &domain.NetworkPolicy{
		DenyPrivateIPs: true, AllowedDomains: []string{"example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.CheckNetwork("https://other.test/"); !errors.Is(err, ErrSandboxDenied) {
		t.Fatalf("restricted domain = %v", err)
	}
	mgr.SetNetworkPolicy(domain.NetworkPolicy{DenyPrivateIPs: false, AllowedDomains: nil})
	if err := mgr.CheckNetwork("https://other.test/"); err != nil {
		t.Fatalf("open network = %v", err)
	}
}

func TestIsDangerousCommandRootDeletion(t *testing.T) {
	dangerous := []struct {
		cmd  string
		args []string
	}{
		{"rm", []string{"-rf", "/"}},
		{"rm", []string{"-fr", "/"}},
		{"rm", []string{"-Rf", "/"}},
		{"rm", []string{"-r", "-f", "/"}},
		{"rm", []string{"--recursive", "--force", "/"}},
		{"rm", []string{"-rf", "/*"}},
		{"rm", []string{"-rf", "."}},
		{"rm", []string{"-rf", ".."}},
		{"rm", []string{"-rf", "~"}},
		{"rm", []string{"-rf", "C:\\"}},
		{"rm", []string{"-rf", "C:\\*"}},
		{"rm", []string{"-rf", "--no-preserve-root", "/tmp/x"}},
		{"rm", []string{"-rf", `"` + `/` + `"`}},
		{"rm", []string{"-r", "/"}},
		{"rm", []string{"-rf", "/ "}},
		{"RM", []string{"-RF", "/"}},
		{"del", []string{"/f", "/s", "/q", "*"}},
		{"del", []string{"/s", "C:\\"}},
		{"del", []string{"/s", "*"}},
		{"rd", []string{"/s", "/q", "."}},
		{"rmdir", []string{"/s", "c:/"}},
		{"format", nil},
		{"diskpart", nil},
	}
	for _, tc := range dangerous {
		if !isDangerousCommand(tc.cmd, tc.args) {
			t.Errorf("%s %v = allowed, want denied", tc.cmd, tc.args)
		}
		if err := (&SandboxManager{workspaceRoot: t.TempDir()}).ConfineCommandWithMode(tc.cmd, tc.args, domain.SandboxModeDangerFullAccess); !errors.Is(err, ErrSandboxDenied) {
			t.Errorf("ConfineCommandWithMode(%s %v, danger) = %v, want ErrSandboxDenied", tc.cmd, tc.args, err)
		}
	}
}

func TestIsDangerousCommandAllowsWorkbenchDeletes(t *testing.T) {
	allowed := []struct {
		cmd  string
		args []string
	}{
		{"rm", []string{"-rf", "build"}},
		{"rm", []string{"-rf", "node_modules/pkg"}},
		{"rm", []string{"-f", "file.txt"}},
		{"rm", []string{"file.txt"}},
		{"rm", []string{"-r", "dist"}},
		{"rm", []string{"--recursive", "tmp"}},
		{"rm", []string{"-rf", "c:/temp/x"}},
		{"del", []string{"/f", "/q", "file.txt"}},
		{"del", []string{"/s", "build"}},
		{"rd", []string{"/s", "/q", "dist"}},
		{"go", []string{"test", "./..."}},
	}
	for _, tc := range allowed {
		if isDangerousCommand(tc.cmd, tc.args) {
			t.Errorf("%s %v = denied, want allowed", tc.cmd, tc.args)
		}
	}
}
