package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// Root aliases like "./" are valid workspace-relative inputs. They used to be
// rejected because filepath.Clean collapses them to "." while the raw value is
// not exactly ".". That rejection made a model's harmless retry fail the run.
func TestSafeWorkspacePathAcceptsWorkspaceRootAliases(t *testing.T) {
	root := t.TempDir()
	for _, value := range []string{".", "./", "./.", "foo/..", "foo/../"} {
		got, err := safeWorkspacePath(root, value, false)
		if err != nil {
			t.Fatalf("safeWorkspacePath(%q) = %v, want the workspace root", value, err)
		}
		if filepath.Clean(got) != filepath.Clean(root) {
			t.Fatalf("safeWorkspacePath(%q) = %q, want %q", value, got, root)
		}
	}
}

// Bad path input is a model-correctable argument error, not a backend failure
// that should terminate the run.
func TestSafeWorkspacePathRejectsBadPathsAsArguments(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	for _, value := range []string{"../outside.txt", outside, "keys.txt", ".env", "secrets/token"} {
		_, err := safeWorkspacePath(root, value, false)
		var argErr *tools.ArgError
		if !errors.As(err, &argErr) {
			t.Fatalf("safeWorkspacePath(%q) err = %v, want *tools.ArgError", value, err)
		}
		if argErr.Field != "path" {
			t.Fatalf("safeWorkspacePath(%q) field = %q, want path", value, argErr.Field)
		}
	}
}

func TestEinoFilesystemBackendListDirEmptyAndRootAliases(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	ctx := context.Background()

	empty, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: "."})
	if err != nil {
		t.Fatalf("list empty workspace: %v", err)
	}
	if empty.Entries == nil {
		t.Fatal("empty directory entries = nil, want a non-nil empty list")
	}
	encoded, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("marshal empty listing: %v", err)
	}
	if !strings.Contains(string(encoded), `"entries":[]`) {
		t.Fatalf("empty listing JSON = %s, want entries:[]", encoded)
	}

	if err := os.WriteFile(filepath.Join(workspace.Path, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	for _, alias := range []string{"./", "./.", "sub/.."} {
		listed, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: alias})
		if err != nil {
			t.Fatalf("list %q: %v", alias, err)
		}
		if len(listed.Entries) != 1 || listed.Entries[0].Path != "a.txt" {
			t.Fatalf("list %q entries = %+v, want the workspace root listing", alias, listed.Entries)
		}
	}
}

// Path-shape errors from the real backend must land in the existing
// model-correctable allowlist, so Eino's ToolsNode does not turn them into a
// NodeRunError that fails the whole run.
func TestEinoFilesystemBackendPathErrorsAreModelCorrectable(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(workspace.Path, "a.txt"), []byte("a"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	spec := domain.ToolSpec{Name: tools.ListDirName, Readonly: true}
	cases := []struct {
		name string
		path string
	}{
		{name: "escape", path: "../outside"},
		{name: "absolute", path: filepath.Join(t.TempDir(), "outside.txt")},
		{name: "protected", path: "keys.txt"},
		{name: "component is a file", path: "a.txt/child"},
		{name: "target is a file", path: "a.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: tc.path})
			var argErr *tools.ArgError
			if !errors.As(err, &argErr) {
				t.Fatalf("ListDir(%q) err = %v, want *tools.ArgError", tc.path, err)
			}
			failure, ok := classifyToolFailure(ctx, spec, err)
			if !ok {
				t.Fatalf("ListDir(%q) stayed run-fatal: %v", tc.path, err)
			}
			if failure.Status != toolFailureStatusRecoverable || failure.Reason != toolFailureReasonInvalidArguments {
				t.Fatalf("ListDir(%q) classification = %+v, want recoverable/invalid_arguments", tc.path, failure)
			}
		})
	}
}

// A genuinely missing read target keeps its os.ErrNotExist chain so agentsmd
// can skip an absent AGENTS.md and readonly tools still classify not_found.
func TestEinoFilesystemBackendMissingPathKeepsNotExistChain(t *testing.T) {
	backend, _, runID := newFilesystemTestBackend(t)
	_, err := backend.ReadFile(context.Background(), runID, tools.FileReadRequest{Path: "missing.txt"})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing read err = %v, want os.ErrNotExist chain", err)
	}
}
