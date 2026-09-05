package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func newDownloadTestBackend(t *testing.T, maxBytes int64) (*DownloadBackend, string, domain.RunID) {
	t.Helper()
	manager, err := NewWorkspaceManager(filepath.Join(t.TempDir(), "workspaces"))
	if err != nil {
		t.Fatalf("new workspace manager: %v", err)
	}
	runID := domain.RunID("run_download_test")
	workspace, err := manager.Ensure(context.Background(), runID)
	if err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	sandbox, err := NewSandboxManager(
		domain.SandboxModeDangerFullAccess,
		workspace.Path,
		nil,
		&domain.NetworkPolicy{DenyPrivateIPs: false},
	)
	if err != nil {
		t.Fatalf("new sandbox manager: %v", err)
	}
	files := NewEinoFilesystemBackend(manager, sandbox)
	downloads := NewDownloadBackend(files, sandbox)
	downloads.allowLoopbackForTest()
	downloads.maxBytes = maxBytes
	return downloads, workspace.Path, runID
}

func TestDownloadStreamsIntoWorkspace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("hello download payload"))
	}))
	defer server.Close()
	backend, workspace, runID := newDownloadTestBackend(t, 1<<20)

	result, err := backend.Download(context.Background(), runID, tools.DownloadRequest{URL: server.URL, Path: "docs/file.bin"})
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if result.Path != "docs/file.bin" || result.Bytes != int64(len("hello download payload")) || result.Overwritten {
		t.Fatalf("unexpected result: %+v", result)
	}
	sum := sha256.Sum256([]byte("hello download payload"))
	if result.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha mismatch: %q", result.SHA256)
	}
	data, err := os.ReadFile(filepath.Join(workspace, "docs", "file.bin"))
	if err != nil || string(data) != "hello download payload" {
		t.Fatalf("workspace payload wrong: %q err=%v", data, err)
	}
}

func TestDownloadOverwritesAndChecksPrecondition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("new bytes"))
	}))
	defer server.Close()
	backend, workspace, runID := newDownloadTestBackend(t, 1<<20)
	target := filepath.Join(workspace, "existing.txt")
	if err := os.WriteFile(target, []byte("old bytes"), 0o600); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}
	ctx := context.Background()

	result, err := backend.Download(ctx, runID, tools.DownloadRequest{URL: server.URL, Path: "existing.txt"})
	if err != nil {
		t.Fatalf("download overwrite: %v", err)
	}
	if !result.Overwritten {
		t.Fatalf("expected overwritten flag: %+v", result)
	}

	stale := tools.WithProposalPrecondition(ctx, "deadbeef")
	if _, err := backend.Download(stale, runID, tools.DownloadRequest{URL: server.URL, Path: "existing.txt"}); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("expected stale proposal refusal, got %v", err)
	}
}

func TestDownloadRejectsOversizeAndBadURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 64)))
	}))
	defer server.Close()
	backend, workspace, runID := newDownloadTestBackend(t, 16)

	if _, err := backend.Download(context.Background(), runID, tools.DownloadRequest{URL: server.URL, Path: "big.bin"}); err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("expected cap refusal, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "big.bin")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed download must not leave a target: %v", err)
	}

	if _, err := backend.Download(context.Background(), runID, tools.DownloadRequest{URL: server.URL + "/?api_key=x", Path: "cred.bin"}); err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("expected credential refusal, got %v", err)
	}

	// Without the loopback test seam the public-only dialer refuses localhost.
	plain := NewDownloadBackend(backend.files, backend.sandbox)
	loopback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer loopback.Close()
	if _, err := plain.Download(context.Background(), runID, tools.DownloadRequest{URL: loopback.URL, Path: "local.bin"}); err == nil || !strings.Contains(err.Error(), "private or local") {
		t.Fatalf("expected private-address refusal, got %v", err)
	}
}

func TestDownloadNon2xxIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	backend, _, runID := newDownloadTestBackend(t, 1<<20)
	if _, err := backend.Download(context.Background(), runID, tools.DownloadRequest{URL: server.URL, Path: "denied.bin"}); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected status error, got %v", err)
	}
}

func TestPrepareDownloadProposal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	defer server.Close()
	backend, workspace, runID := newDownloadTestBackend(t, 1<<20)
	target := filepath.Join(workspace, "existing.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatalf("seed existing file: %v", err)
	}

	proposal, err := backend.PrepareDownload(context.Background(), runID, tools.DownloadRequest{URL: server.URL, Path: "new/one.bin"})
	if err != nil {
		t.Fatalf("prepare fresh target: %v", err)
	}
	if proposal.Action != tools.DownloadName || proposal.Target != "new/one.bin" {
		t.Fatalf("unexpected proposal: %+v", proposal)
	}
	if len(proposal.RiskFindings) != 1 || !strings.Contains(proposal.RiskFindings[0], "workspace") {
		t.Fatalf("unexpected risk findings: %+v", proposal.RiskFindings)
	}

	proposal, err = backend.PrepareDownload(context.Background(), runID, tools.DownloadRequest{URL: server.URL, Path: "existing.txt"})
	if err != nil {
		t.Fatalf("prepare existing target: %v", err)
	}
	foundOverwrite := false
	for _, warning := range proposal.RiskFindings {
		foundOverwrite = foundOverwrite || strings.Contains(warning, "overwrites")
	}
	if !foundOverwrite {
		t.Fatalf("expected overwrite warning: %+v", proposal.RiskFindings)
	}

	if _, err := backend.PrepareDownload(context.Background(), runID, tools.DownloadRequest{URL: "https://example.com/?password=x", Path: "x.bin"}); err == nil {
		t.Fatal("expected credential URL refusal in proposal")
	}
}

// Sandbox validation must run after parents exist: the symlink walk needs
// real directories under confined modes (workspace-write, not danger).
func TestDownloadCreatesParentsUnderConfinedSandbox(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("nested"))
	}))
	defer server.Close()

	manager, err := NewWorkspaceManager(filepath.Join(t.TempDir(), "workspaces"))
	if err != nil {
		t.Fatalf("new workspace manager: %v", err)
	}
	runID := domain.RunID("run_download_confined")
	workspace, err := manager.Ensure(context.Background(), runID)
	if err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	sandbox, err := NewSandboxManager(
		domain.SandboxModeWorkspaceWrite,
		workspace.Path,
		nil,
		&domain.NetworkPolicy{DenyPrivateIPs: false},
	)
	if err != nil {
		t.Fatalf("new sandbox manager: %v", err)
	}
	backend := NewDownloadBackend(NewEinoFilesystemBackend(manager, sandbox), sandbox)
	backend.allowLoopbackForTest()

	result, err := backend.Download(context.Background(), runID, tools.DownloadRequest{URL: server.URL, Path: "deep/dir/file.txt"})
	if err != nil {
		t.Fatalf("download under workspace-write: %v", err)
	}
	if result.Bytes != int64(len("nested")) || result.Path != "deep/dir/file.txt" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
