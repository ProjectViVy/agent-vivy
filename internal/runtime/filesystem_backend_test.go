package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk/filesystem"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

func newFilesystemTestBackend(t *testing.T) (*EinoFilesystemBackend, Workspace, domain.RunID) {
	t.Helper()
	manager, err := NewWorkspaceManager(filepath.Join(t.TempDir(), "workspaces"))
	if err != nil {
		t.Fatalf("new workspace manager: %v", err)
	}
	runID := domain.RunID("run_filesystem_test")
	workspace, err := manager.Ensure(context.Background(), runID)
	if err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	// Create a permissive sandbox for tests (danger_full_access to avoid blocking test operations)
	sandbox, err := NewSandboxManager(
		domain.SandboxModeDangerFullAccess,
		workspace.Path,
		[]string{"go", "git", "rg"},
		nil,
	)
	if err != nil {
		t.Fatalf("new sandbox manager: %v", err)
	}
	return NewEinoFilesystemBackend(manager, sandbox), workspace, runID
}

func TestEinoFilesystemBackendWriteFileConfinedFreshNestedDir(t *testing.T) {
	manager, err := NewWorkspaceManager(filepath.Join(t.TempDir(), "workspaces"))
	if err != nil {
		t.Fatal(err)
	}
	runID := domain.RunID("run_confined_write")
	workspace, err := manager.Ensure(context.Background(), runID)
	if err != nil {
		t.Fatal(err)
	}
	sandbox, err := NewSandboxManager(domain.SandboxModeWorkspaceWrite, workspace.Path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	backend := NewEinoFilesystemBackend(manager, sandbox)
	ctx := context.Background()

	// WEB-2 regression: parents do not exist yet, so sandbox validation must
	// run after MkdirAll (download.go order), not before it.
	write, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{
		Path: "a/b/c/new.txt", Content: "fresh", CreateParents: true,
	})
	if err != nil {
		t.Fatalf("confined write to fresh nested dir = %v, want success", err)
	}
	if !write.Changed || write.Path != "a/b/c/new.txt" {
		t.Fatalf("unexpected write result: %+v", write)
	}
	data, err := os.ReadFile(filepath.Join(workspace.Path, "a", "b", "c", "new.txt"))
	if err != nil || string(data) != "fresh" {
		t.Fatalf("read back = %q, %v", data, err)
	}

	readOnly, err := NewSandboxManager(domain.SandboxModeReadOnly, workspace.Path, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	denied := NewEinoFilesystemBackend(manager, readOnly)
	if _, err := denied.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "other.txt", Content: "x", CreateParents: true}); !errors.Is(err, ErrSandboxDenied) {
		t.Fatalf("read-only write = %v, want ErrSandboxDenied", err)
	}
}

func TestEinoFilesystemBackendReadWriteAndPatch(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	ctx := context.Background()

	write, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{
		Path: "src/example.txt", Content: "one\ntwo\nthree", CreateParents: true,
	})
	if err != nil {
		t.Fatalf("write file: %v", err)
	}
	if !write.Changed || write.Path != "src/example.txt" || write.Bytes != len("one\ntwo\nthree") {
		t.Fatalf("unexpected write result: %+v", write)
	}
	hash := sha256.Sum256([]byte("one\ntwo\nthree"))
	if write.SHA256 != hex.EncodeToString(hash[:]) {
		t.Fatalf("write hash = %q", write.SHA256)
	}

	read, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "src/example.txt", StartLine: 2, EndLine: 2})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if read.Content != "two" || read.StartLine != 2 || read.EndLine != 2 || read.TotalLines != 3 {
		t.Fatalf("unexpected read result: %+v", read)
	}

	patch, err := backend.PatchFile(ctx, runID, tools.FilePatchRequest{
		Path: "src/example.txt", OldString: "two", NewString: "TWO",
	})
	if err != nil {
		t.Fatalf("patch file: %v", err)
	}
	// Real unified diff (go-udiff): context lines carry the ' ' prefix, so
	// the changed pair renders as "-two"/"+TWO" between context rows.
	if !patch.Changed || !strings.Contains(patch.Diff, " one\n-two\n+TWO\n three") {
		t.Fatalf("unexpected patch result: %+v", patch)
	}

	data, err := os.ReadFile(filepath.Join(workspace.Path, "src", "example.txt"))
	if err != nil {
		t.Fatalf("read patched fixture: %v", err)
	}
	if string(data) != "one\nTWO\nthree" {
		t.Fatalf("patched content = %q", data)
	}

	unchanged, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{
		Path: "src/example.txt", Content: "one\nTWO\nthree", CreateParents: true,
	})
	if err != nil {
		t.Fatalf("idempotent write: %v", err)
	}
	if unchanged.Changed {
		t.Fatalf("same content should not be marked changed: %+v", unchanged)
	}
}

func TestEinoFilesystemBackendSearchAndEinoMethods(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	for name, content := range map[string]string{
		"a.txt":        "Needle here\nother line",
		"nested/b.go":  "needle again\nno match",
		"nested/c.bin": "\x00needle",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(workspace.Path, name)), 0o700); err != nil {
			t.Fatalf("mkdir fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(workspace.Path, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	search, err := backend.SearchFiles(context.Background(), runID, tools.FileSearchRequest{
		Query: "needle", Glob: "*.txt", MaxResults: 10, CaseSensitive: false,
	})
	if err != nil {
		t.Fatalf("search files: %v", err)
	}
	if len(search.Matches) != 1 || search.Matches[0].Path != "a.txt" || search.Matches[0].Line != 1 {
		t.Fatalf("unexpected search result: %+v", search)
	}

	ctx := tools.WithRunID(context.Background(), runID)
	listed, err := backend.LsInfo(ctx, &filesystem.LsInfoRequest{Path: "nested"})
	if err != nil {
		t.Fatalf("list info: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("list count = %d, want 2", len(listed))
	}
	globbed, err := backend.GlobInfo(ctx, &filesystem.GlobInfoRequest{Path: ".", Pattern: "*.txt"})
	if err != nil {
		t.Fatalf("glob info: %v", err)
	}
	if len(globbed) != 1 || globbed[0].Path != "a.txt" {
		t.Fatalf("unexpected glob result: %+v", globbed)
	}
	grep, err := backend.GrepRaw(ctx, &filesystem.GrepRequest{Path: ".", Pattern: "(?i)needle"})
	if err != nil {
		t.Fatalf("grep raw: %v", err)
	}
	if len(grep) != 2 {
		t.Fatalf("grep count = %d, want 2", len(grep))
	}
}

func TestEinoFilesystemBackendListDir(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	for name, content := range map[string]string{
		"a.txt":               "top",
		"nested/b.go":         "b",
		"nested/deep/c.txt":   "c",
		"node_modules/pkg.js": "x",
		".git/config":         "g",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(workspace.Path, filepath.FromSlash(name))), 0o700); err != nil {
			t.Fatalf("mkdir fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(workspace.Path, filepath.FromSlash(name)), []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	ctx := context.Background()

	flat, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: "."})
	if err != nil {
		t.Fatalf("list flat: %v", err)
	}
	if len(flat.Entries) != 4 || flat.Truncated {
		t.Fatalf("flat entries = %+v", flat.Entries)
	}
	byPath := make(map[string]tools.DirEntry, len(flat.Entries))
	for _, entry := range flat.Entries {
		byPath[entry.Path] = entry
	}
	if entry := byPath["a.txt"]; entry.IsDir || entry.Size != int64(len("top")) || entry.ModifiedAt == "" {
		t.Fatalf("file entry = %+v", entry)
	}
	for _, dir := range []string{".git", "nested", "node_modules"} {
		if entry := byPath[dir]; !entry.IsDir {
			t.Fatalf("dir entry %s = %+v", dir, entry)
		}
	}

	nested, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: "nested"})
	if err != nil {
		t.Fatalf("list nested: %v", err)
	}
	if len(nested.Entries) != 2 || nested.Entries[0].Path != "nested/b.go" || nested.Entries[1].Path != "nested/deep" {
		t.Fatalf("nested entries = %+v", nested.Entries)
	}

	shallow, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: ".", Recursive: true, Depth: 1})
	if err != nil {
		t.Fatalf("list shallow: %v", err)
	}
	if len(shallow.Entries) != 4 {
		t.Fatalf("depth-1 entries = %+v", shallow.Entries)
	}

	recursive, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: ".", Recursive: true})
	if err != nil {
		t.Fatalf("list recursive: %v", err)
	}
	want := []string{".git", "a.txt", "nested", "nested/b.go", "nested/deep", "nested/deep/c.txt", "node_modules"}
	if len(recursive.Entries) != len(want) || recursive.Truncated {
		t.Fatalf("recursive entries = %+v", recursive.Entries)
	}
	for i, path := range want {
		if recursive.Entries[i].Path != path {
			t.Fatalf("recursive entry %d = %s, want %s", i, recursive.Entries[i].Path, path)
		}
	}

	depthTwo, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: ".", Recursive: true, Depth: 2})
	if err != nil {
		t.Fatalf("list depth two: %v", err)
	}
	for _, entry := range depthTwo.Entries {
		if entry.Path == "nested/deep/c.txt" {
			t.Fatal("depth 2 should not reach nested/deep/c.txt")
		}
	}

	capped, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: ".", Recursive: true, MaxEntries: 3})
	if err != nil {
		t.Fatalf("list capped: %v", err)
	}
	if len(capped.Entries) != 3 || !capped.Truncated {
		t.Fatalf("capped entries = %+v (truncated=%v)", capped.Entries, capped.Truncated)
	}

	if _, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: "a.txt"}); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("file target error = %v", err)
	}
	if _, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: "../outside"}); err == nil {
		t.Fatal("path escape should be rejected")
	}
}

func TestEinoFilesystemBackendRejectsUnsafePathsAndPatchAmbiguity(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	outside := filepath.Join(t.TempDir(), "outside.txt")
	unsafe := []string{"../outside.txt", outside, "keys.txt", ".env", "secrets/token"}
	for _, path := range unsafe {
		if _, err := backend.WriteFile(context.Background(), runID, tools.FileWriteRequest{Path: path, Content: "blocked", CreateParents: true}); err == nil {
			t.Errorf("path %q was accepted", path)
		}
	}

	if err := os.WriteFile(filepath.Join(workspace.Path, "repeat.txt"), []byte("repeat\nrepeat"), 0o600); err != nil {
		t.Fatalf("write repeat fixture: %v", err)
	}
	if _, err := backend.PatchFile(context.Background(), runID, tools.FilePatchRequest{
		Path: "repeat.txt", OldString: "repeat", NewString: "changed",
	}); err == nil || !strings.Contains(err.Error(), "matched 2 times") {
		t.Fatalf("ambiguous patch error = %v", err)
	}
	if _, err := backend.PatchFile(context.Background(), runID, tools.FilePatchRequest{
		Path: "repeat.txt", OldString: "repeat", NewString: "changed", ReplaceAll: true,
	}); err != nil {
		t.Fatalf("replace all patch: %v", err)
	}

	link := filepath.Join(workspace.Path, "link.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable on this Windows account: %v", err)
	}
	if _, err := backend.ReadFile(context.Background(), runID, tools.FileReadRequest{Path: "link.txt"}); err == nil {
		t.Fatal("symlink read should be rejected")
	}
}

func TestEinoFilesystemBackendHonorsCancellation(t *testing.T) {
	backend, _, runID := newFilesystemTestBackend(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "missing.txt"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("read error = %v, want context cancellation", err)
	}
}

// A model may name the workspace root as ".", "./", ".//" or ""; every form
// must resolve to the root instead of failing the run. Regression for
// run_8d376d0f457fa5e5: list_dir {"path":"./"} returned
// "filesystem: invalid path" and Eino's ToolsNode turned that into a fatal
// [NodeRunError].
func TestEinoFilesystemBackendAcceptsWorkspaceRootSpellings(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	if err := os.WriteFile(filepath.Join(workspace.Path, "root.txt"), []byte("top"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	ctx := context.Background()

	for _, path := range []string{"", ".", "./", ".//", "./."} {
		listed, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: path})
		if err != nil {
			t.Fatalf("list_dir %q = %v, want the workspace root", path, err)
		}
		if listed.Path != "." || len(listed.Entries) != 1 || listed.Entries[0].Path != "root.txt" {
			t.Fatalf("list_dir %q = %+v, want the root entry", path, listed)
		}
	}
	read, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "./root.txt"})
	if err != nil || read.Content != "top" {
		t.Fatalf("read_file ./root.txt = %+v, %v", read, err)
	}
}

// A path defect is the model's own argument mistake: it must surface as a
// structured *tools.ArgError so the runtime's §5 soft conversion keeps the
// run alive with a model-visible diagnostic (NUDGE-DESIGN §5), never as a
// run-fatal engine error.
func TestEinoFilesystemBackendPathDefectsAreArgumentErrors(t *testing.T) {
	backend, _, runID := newFilesystemTestBackend(t)
	ctx := context.Background()

	cases := []string{"/etc", "/", `\Windows`, "../outside", "a\x00b", "root.txt/nested"}
	if filepath.VolumeName(`C:\Windows`) != "" {
		// Drive-absolute paths only exist on Windows.
		cases = append(cases, `C:\Windows`)
	}
	if err := os.WriteFile(filepath.Join(mustWorkspace(t, backend, runID), "root.txt"), []byte("file"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			_, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: path})
			assertPathArgError(t, err)
		})
	}

	// The list target itself may be a file rather than a directory.
	_, err := backend.ListDir(ctx, runID, tools.DirListRequest{Path: "root.txt"})
	assertPathArgError(t, err)

	// Search/glob/grep share the same resolve seam.
	if _, err := backend.SearchFiles(ctx, runID, tools.FileSearchRequest{Query: "x", Path: "/etc"}); err == nil {
		t.Fatal("search with a rooted path was accepted")
	} else {
		assertPathArgError(t, err)
	}
}

func mustWorkspace(t *testing.T, backend *EinoFilesystemBackend, runID domain.RunID) string {
	t.Helper()
	workspace, err := backend.manager.Ensure(context.Background(), runID)
	if err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	return workspace.Path
}

func assertPathArgError(t *testing.T, err error) {
	t.Helper()
	var argErr *tools.ArgError
	if !errors.As(err, &argErr) || argErr.Field != "path" {
		t.Fatalf("error = %v (%T), want *tools.ArgError on field %q", err, err, "path")
	}
	// The same error must classify as recoverable for a readonly tool, the
	// way the runtime adapter treats it at the invocation seam.
	failure, ok := classifyToolFailure(context.Background(), domain.ToolSpec{Name: tools.ListDirName, Readonly: true}, err)
	if !ok || failure.Status != toolFailureStatusRecoverable || failure.Reason != toolFailureReasonInvalidArguments {
		t.Fatalf("classification = %+v (ok=%v), want recoverable/invalid_arguments", failure, ok)
	}
}

// Every remaining argument defect in the filesystem family carries the
// offending field, so §5 recovers it as recoverable/invalid_arguments instead
// of letting Eino's ToolsNode fail the whole run. Regression coverage for the
// same class as run_8d376d0f457fa5e5 (a model mistake must not kill the run),
// now including patch/read/write limits that used to be bare errors.
func TestEinoFilesystemBackendArgumentDefectsAreTypedArgumentErrors(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	ctx := context.Background()
	runCtx := tools.WithRunID(ctx, runID)
	// A tiny cap keeps the over-limit cases cheap to express.
	backend.maxFileBytes = 24

	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(workspace.Path, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	write("notes.txt", "alpha\nbeta\ngamma\ndelta\n")
	write("twice.txt", "dup\ndup\n")
	write("binary.bin", "\x00\x01ab")
	write("shot.png", strings.Repeat("p", 25))

	big := strings.Repeat("x", 25)
	cases := []struct {
		name  string
		field string
		call  func() error
	}{
		{"read reversed line range", "end_line", func() error {
			_, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "notes.txt", StartLine: 4, EndLine: 2})
			return err
		}}, {"read image over the read cap", "path", func() error {
			_, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "shot.png"})
			return err
		}},
		{"search empty query", "query", func() error {
			_, err := backend.SearchFiles(ctx, runID, tools.FileSearchRequest{Query: "", Path: "."})
			return err
		}},
		{"write over the content cap", "content", func() error {
			_, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "big.txt", Content: big})
			return err
		}},
		{"prepare write over the content cap", "content", func() error {
			_, err := backend.PrepareWriteFile(ctx, runID, tools.FileWriteRequest{Path: "big.txt", Content: big})
			return err
		}},
		{"patch empty old_string", "old_string", func() error {
			_, err := backend.PatchFile(ctx, runID, tools.FilePatchRequest{Path: "notes.txt", OldString: "", NewString: "x"})
			return err
		}},
		{"patch identical strings", "new_string", func() error {
			_, err := backend.PatchFile(ctx, runID, tools.FilePatchRequest{Path: "notes.txt", OldString: "alpha", NewString: "alpha"})
			return err
		}},
		{"patch unmatched old_string", "old_string", func() error {
			_, err := backend.PatchFile(ctx, runID, tools.FilePatchRequest{Path: "notes.txt", OldString: "epsilon", NewString: "x"})
			return err
		}},
		{"patch ambiguous old_string", "replace_all", func() error {
			_, err := backend.PatchFile(ctx, runID, tools.FilePatchRequest{Path: "twice.txt", OldString: "dup", NewString: "x"})
			return err
		}},
		{"patch binary target", "path", func() error {
			_, err := backend.PatchFile(ctx, runID, tools.FilePatchRequest{Path: "binary.bin", OldString: "ab", NewString: "x"})
			return err
		}},
		{"patch missing target", "path", func() error {
			_, err := backend.PatchFile(ctx, runID, tools.FilePatchRequest{Path: "absent.txt", OldString: "ab", NewString: "x"})
			return err
		}},
		{"prepare patch missing target", "path", func() error {
			_, err := backend.PreparePatchFile(ctx, runID, tools.FilePatchRequest{Path: "absent.txt", OldString: "ab", NewString: "x"})
			return err
		}},
		{"prepare patch unmatched old_string", "old_string", func() error {
			_, err := backend.PreparePatchFile(ctx, runID, tools.FilePatchRequest{Path: "notes.txt", OldString: "epsilon", NewString: "x"})
			return err
		}},
		{"prepare patch binary target", "path", func() error {
			_, err := backend.PreparePatchFile(ctx, runID, tools.FilePatchRequest{Path: "binary.bin", OldString: "ab", NewString: "x"})
			return err
		}},
		{"multiedit without edits", "edits", func() error {
			_, err := backend.MultiPatchFile(ctx, runID, tools.FileMultiEditRequest{Path: "notes.txt"})
			return err
		}},
		{"multiedit binary target", "path", func() error {
			_, err := backend.MultiPatchFile(ctx, runID, tools.FileMultiEditRequest{Path: "binary.bin", Edits: []tools.FileMultiEditItem{{OldString: "ab", NewString: "x"}}})
			return err
		}},
		{"prepare multiedit without edits", "edits", func() error {
			_, err := backend.PrepareMultiPatchFile(ctx, runID, tools.FileMultiEditRequest{Path: "notes.txt"})
			return err
		}},
		{"multiedit missing target", "path", func() error {
			_, err := backend.MultiPatchFile(ctx, runID, tools.FileMultiEditRequest{Path: "absent.txt", Edits: []tools.FileMultiEditItem{{OldString: "ab", NewString: "x"}}})
			return err
		}},
		{"prepare multiedit missing target", "path", func() error {
			_, err := backend.PrepareMultiPatchFile(ctx, runID, tools.FileMultiEditRequest{Path: "absent.txt", Edits: []tools.FileMultiEditItem{{OldString: "ab", NewString: "x"}}})
			return err
		}},
		{"glob empty pattern", "pattern", func() error {
			_, err := backend.Glob(runCtx, tools.GlobRequest{Pattern: ""})
			return err
		}},
		{"glob escaping pattern", "pattern", func() error {
			_, err := backend.Glob(runCtx, tools.GlobRequest{Pattern: "../outside/*"})
			return err
		}},
		{"grep invalid pattern", "pattern", func() error {
			_, err := backend.Grep(runCtx, tools.GrepRequest{Pattern: "([a-z"})
			return err
		}},
		{"eino glob empty pattern", "pattern", func() error {
			_, err := backend.GlobInfo(runCtx, &filesystem.GlobInfoRequest{Pattern: ""})
			return err
		}},
		{"eino grep invalid pattern", "pattern", func() error {
			_, err := backend.GrepRaw(runCtx, &filesystem.GrepRequest{Pattern: "([a-z"})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			var argErr *tools.ArgError
			if !errors.As(err, &argErr) || argErr.Field != tc.field {
				t.Fatalf("error = %v (%T), want *tools.ArgError on field %q", err, err, tc.field)
			}
			failure, ok := classifyToolFailure(ctx, domain.ToolSpec{Name: tools.PatchName}, err)
			if !ok || failure.Status != toolFailureStatusRecoverable || failure.Reason != toolFailureReasonInvalidArguments {
				t.Fatalf("classification = %+v (ok=%v), want recoverable/invalid_arguments", failure, ok)
			}
		})
	}
}

// A boundary denial is a governance decision, not a run failure: the
// filesystem backend must report the typed sentinel and the adapter must turn
// it into a per-call refusal (refused/policy_denied/not_executed) so the model
// is told the call did not run.
func TestEinoFilesystemBackendProtectedPathDenialIsTypedRefusal(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(workspace.Path, ".env"), []byte("SECRET=1"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	_, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: ".env"})
	if !errors.Is(err, ErrProtectedPathDenied) {
		t.Fatalf("protected read error = %v, want ErrProtectedPathDenied", err)
	}
	assertBoundaryDenialRefusal(t, err, tools.ReadFileName, true)
}

func TestEinoFilesystemBackendSymlinkDenialIsTypedRefusal(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	ctx := context.Background()
	target := filepath.Join(workspace.Path, "real.txt")
	if err := os.WriteFile(target, []byte("real"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(workspace.Path, "link.txt")); err != nil {
		t.Skipf("symlink unavailable on this Windows account: %v", err)
	}
	_, err := backend.ReadFile(ctx, runID, tools.FileReadRequest{Path: "link.txt"})
	if !errors.Is(err, ErrSymlinkPathDenied) {
		t.Fatalf("symlink read error = %v, want ErrSymlinkPathDenied", err)
	}
	assertBoundaryDenialRefusal(t, err, tools.ReadFileName, true)
}

// Every governance denial the filesystem boundary can raise must resolve to a
// refusal through the shared adapter seam, on a model-driven leg or not.
func TestBoundaryDenialSentinelsAreRefusals(t *testing.T) {
	for _, sentinel := range []error{ErrProtectedPathDenied, ErrSymlinkPathDenied, ErrSandboxDenied} {
		t.Run(sentinel.Error(), func(t *testing.T) {
			assertBoundaryDenialRefusal(t, fmt.Errorf("filesystem: denied: %w", sentinel), tools.ReadFileName, false)
		})
	}
}

func assertBoundaryDenialRefusal(t *testing.T, err error, toolName string, modelLeg bool) {
	t.Helper()
	// A refusal defaults to policy_denied/refused/not_executed: the call never
	// reached the tool, so its effects are known to be absent.
	refusal, ok := asToolRefusal(err)
	if !ok {
		t.Fatalf("error = %v (%T), want a tool refusal", err, err)
	}
	if refusal.classification != "" {
		t.Fatalf("refusal classification = %q, want the policy_denied default", refusal.classification)
	}
	failure := refusalFailure(refusal.classification, refusal.reason)
	if failure.Status != toolFailureStatusRefused || failure.Reason != toolFailureReasonPolicyDenied || failure.Effects != toolEffectsNotExecuted {
		t.Fatalf("refusal record = %+v, want refused/policy_denied/not_executed", failure)
	}
	// A refusal is never soft-converted into an invalid_arguments result.
	if soft, softOK := classifyToolFailure(context.Background(), domain.ToolSpec{Name: toolName, Readonly: modelLeg}, err); softOK {
		t.Fatalf("boundary denial was classified as a recoverable failure: %+v", soft)
	}
}
