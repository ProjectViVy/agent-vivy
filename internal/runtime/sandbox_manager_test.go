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
