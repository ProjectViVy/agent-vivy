package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/tools"
)

func mustPatch(t *testing.T, content, oldString, newString string, replaceAll bool) (string, int) {
	t.Helper()
	got, n, err := applyStringPatch(content, oldString, newString, replaceAll)
	if err != nil {
		t.Fatalf("applyStringPatch: %v", err)
	}
	return got, n
}

func TestApplyStringPatchExactBeatsTolerant(t *testing.T) {
	// The exact substring exists; the tolerant matcher must not run, so the
	// trailing-whitespace line is preserved verbatim.
	content := "keep trailing \nvalue"
	got, n := mustPatch(t, content, "value", "VALUE", false)
	if n != 1 || got != "keep trailing \nVALUE" {
		t.Fatalf("exact patch = %q/%d", got, n)
	}
}

func TestApplyStringPatchToleratesTrailingWhitespace(t *testing.T) {
	content := "func one() {\n\treturn 1 \n}\n"
	got, n := mustPatch(t, content, "func one() {\n\treturn 1\n}", "func one() {\n\treturn 2\n}", false)
	if n != 1 || got != "func one() {\n\treturn 2\n}\n" {
		t.Fatalf("trailing-whitespace patch = %q/%d", got, n)
	}
}

func TestApplyStringPatchRestoresFileIndentation(t *testing.T) {
	// The model guessed two spaces; the file uses four. The replacement
	// keeps the file's indentation because the model did not reindent.
	content := "func main() {\n    run()\n}\n"
	got, n := mustPatch(t, content, "func main() {\n  run()\n}", "func main() {\n  halt()\n}", false)
	if n != 1 || got != "func main() {\n    halt()\n}\n" {
		t.Fatalf("indent restoration = %q/%d", got, n)
	}
}

func TestApplyStringPatchKeepsDeliberateReindent(t *testing.T) {
	// The model changes the replacement's indentation relative to
	// old_string; its own leading whitespace wins.
	content := "block:\n    old()\nend\n"
	got, n := mustPatch(t, content, "block:\n    old()\n", "block:\n        new()\n", false)
	if n != 1 || got != "block:\n        new()\nend\n" {
		t.Fatalf("deliberate reindent = %q/%d", got, n)
	}
}

func TestApplyStringPatchPreservesCRLFLineEndings(t *testing.T) {
	content := "first\r\nsecond\r\nthird\r\n"
	got, n := mustPatch(t, content, "first\nsecond\nthird\n", "first\nupdated\nthird\n", false)
	if n != 1 || got != "first\r\nupdated\r\nthird\r\n" {
		t.Fatalf("CRLF patch = %q/%d", got, n)
	}
}

func TestApplyStringPatchVerbatimWhenLineCountsDiffer(t *testing.T) {
	content := "a\nb\nc\n"
	got, n := mustPatch(t, content, "a\nb\n", "x\ny\nz\n", false)
	if n != 1 || got != "x\ny\nz\nc\n" {
		t.Fatalf("line-count-differing patch = %q/%d", got, n)
	}
}

func TestApplyStringPatchReplaceAllTolerant(t *testing.T) {
	content := "if x:\n    go()\nif x:\n    go()\n"
	got, n := mustPatch(t, content, "if x:\n  go()\n", "if x:\n  stop()\n", true)
	if n != 2 || got != "if x:\n    stop()\nif x:\n    stop()\n" {
		t.Fatalf("flexible replace_all = %q/%d", got, n)
	}
}

func TestApplyStringPatchAmbiguousFlexibleMatch(t *testing.T) {
	content := "list:\n    item\nmid\nlist:\n    item\n"
	_, _, err := applyStringPatch(content, "list:\n  item", "x", false)
	if err == nil || !strings.Contains(err.Error(), "whitespace-insensitive") {
		t.Fatalf("ambiguous flexible error = %v", err)
	}
}

func TestApplyStringPatchNotFound(t *testing.T) {
	_, _, err := applyStringPatch("alpha\nbeta\n", "alpha\ngamma\n", "x", false)
	if err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("not-found error = %v", err)
	}
}

func TestApplyStringPatchRejectsEmptyOldString(t *testing.T) {
	_, _, err := applyStringPatch("content", "", "x", false)
	if err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("empty old_string error = %v", err)
	}
}

func TestApplyStringPatchExactAmbiguityUnchanged(t *testing.T) {
	_, _, err := applyStringPatch("same\nsame\n", "same", "x", false)
	if err == nil || !strings.Contains(err.Error(), "matched 2 times") {
		t.Fatalf("exact ambiguity error = %v", err)
	}
}

func TestBackendPatchFileWhitespaceTolerance(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	ctx := context.Background()
	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "src/code.go", Content: "func main() {\n    run()\n}\n", CreateParents: true}); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	result, err := backend.PatchFile(ctx, runID, tools.FilePatchRequest{
		Path:      "src/code.go",
		OldString: "func main() {\n  run()\n}",
		NewString: "func main() {\n  halt()\n}",
	})
	if err != nil {
		t.Fatalf("tolerant patch: %v", err)
	}
	if !result.Changed || result.Diff == "" {
		t.Fatalf("patch result = %+v, want changed with diff", result)
	}
	data, err := os.ReadFile(filepath.Join(workspace.Path, "src", "code.go"))
	if err != nil {
		t.Fatalf("read patched file: %v", err)
	}
	if string(data) != "func main() {\n    halt()\n}\n" {
		t.Fatalf("patched content = %q, want file indentation restored", data)
	}
}

func TestBackendMultiPatchFileAppliesAtomically(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	ctx := context.Background()
	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "notes.txt", Content: "a\nb\nc\n", CreateParents: true}); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	result, err := backend.MultiPatchFile(ctx, runID, tools.FileMultiEditRequest{
		Path: "notes.txt",
		Edits: []tools.FileMultiEditItem{
			{OldString: "a", NewString: "x"},
			{OldString: "c", NewString: "y"},
		},
	})
	if err != nil {
		t.Fatalf("multiedit: %v", err)
	}
	// Unified diff rows: "-a"/"+x" and "-c"/"+y" with unchanged "b" as context.
	if !result.Changed || !strings.Contains(result.Diff, "-a\n+x\n") || !strings.Contains(result.Diff, "-c\n+y\n") {
		t.Fatalf("multiedit result = %+v, want combined diff", result)
	}
	data, err := os.ReadFile(filepath.Join(workspace.Path, "notes.txt"))
	if err != nil {
		t.Fatalf("read multiedited file: %v", err)
	}
	if string(data) != "x\nb\ny\n" {
		t.Fatalf("multiedited content = %q", data)
	}
}

func TestBackendMultiPatchFileFailsClosed(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	ctx := context.Background()
	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "notes.txt", Content: "a\nb\nc\n", CreateParents: true}); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	_, err := backend.MultiPatchFile(ctx, runID, tools.FileMultiEditRequest{
		Path: "notes.txt",
		Edits: []tools.FileMultiEditItem{
			{OldString: "a", NewString: "x"},
			{OldString: "missing", NewString: "y"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "edit 2") {
		t.Fatalf("failing multiedit error = %v, want edit 2 failure", err)
	}
	data, err := os.ReadFile(filepath.Join(workspace.Path, "notes.txt"))
	if err != nil {
		t.Fatalf("read unchanged file: %v", err)
	}
	if string(data) != "a\nb\nc\n" {
		t.Fatalf("failed multiedit left content = %q, want untouched", data)
	}
}

func TestBackendMultiPatchFileTolerantEdit(t *testing.T) {
	backend, workspace, runID := newFilesystemTestBackend(t)
	ctx := context.Background()
	if _, err := backend.WriteFile(ctx, runID, tools.FileWriteRequest{Path: "code.py", Content: "def f():\n    return 1\n", CreateParents: true}); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	if _, err := backend.MultiPatchFile(ctx, runID, tools.FileMultiEditRequest{
		Path: "code.py",
		Edits: []tools.FileMultiEditItem{
			{OldString: "def f():\n  return 1\n", NewString: "def f():\n  return 2\n"},
		},
	}); err != nil {
		t.Fatalf("tolerant multiedit: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(workspace.Path, "code.py"))
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if string(data) != "def f():\n    return 2\n" {
		t.Fatalf("tolerant multiedit content = %q", data)
	}
}

func TestBackendMultiPatchFileRejectsEmptyEdits(t *testing.T) {
	backend, _, runID := newFilesystemTestBackend(t)
	_, err := backend.MultiPatchFile(context.Background(), runID, tools.FileMultiEditRequest{Path: "a.txt"})
	if err == nil || !strings.Contains(err.Error(), "at least one edit") {
		t.Fatalf("empty edits error = %v", err)
	}
}
