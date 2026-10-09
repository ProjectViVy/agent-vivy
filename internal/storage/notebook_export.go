package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"agent-vivy/internal/notebookcontract"
)

// ExportNotebookEntry writes one entry revision as a markdown artifact into a
// fresh output directory. The output path must be new: an existing directory
// with content is refused rather than overwritten, and identifier segments
// are sanitized so trusted IDs can never traverse the filesystem.
func ExportNotebookEntry(ctx context.Context, store NotebookStore, scope notebookcontract.ScopeID, entryID, revisionID, outDir string) (string, error) {
	if store == nil {
		return "", fmt.Errorf("notebook export: no storage engine")
	}
	if scope == "" || entryID == "" || outDir == "" {
		return "", &notebookcontract.Error{Code: notebookcontract.CodeInvalidRequest, Message: "export requires --scope, --entry and --output"}
	}
	view, err := store.GetEntry(ctx, scope, notebookcontract.GetEntryRequest{EntryID: entryID, RevisionID: revisionID})
	if err != nil {
		return "", err
	}
	info, err := os.Stat(outDir)
	switch {
	case err == nil && !info.IsDir():
		return "", fmt.Errorf("notebook export: output path %q is not a directory", outDir)
	case err == nil:
		entries, rerr := os.ReadDir(outDir)
		if rerr != nil {
			return "", rerr
		}
		if len(entries) != 0 {
			return "", fmt.Errorf("notebook export: output directory %q is not empty (refusing to overwrite)", outDir)
		}
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(outDir, 0o700); err != nil {
			return "", fmt.Errorf("notebook export: create output directory: %w", err)
		}
	default:
		return "", err
	}
	name := safeExportName(view.Entry.Title)
	if name == "" {
		name = "entry"
	}
	target := filepath.Join(outDir, fmt.Sprintf("%s-%s.md", name, safeExportName(view.Revision.ID)))
	header := fmt.Sprintf("---\nentry: %s\nrevision: %s\nscope: %s\norigin: %s\n---\n\n# %s\n\n",
		view.Entry.ID, view.Revision.ID, scope, view.Revision.Origin, view.Entry.Title)
	if err := os.WriteFile(target, []byte(header+view.Revision.Markdown), 0o600); err != nil {
		return "", fmt.Errorf("notebook export: write artifact: %w", err)
	}
	return target, nil
}

var exportNamePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// safeExportName strips path separators and traversal so user-controlled IDs
// or titles cannot escape the output directory.
func safeExportName(s string) string {
	s = exportNamePattern.ReplaceAllString(s, "-")
	for len(s) > 0 && (s[0] == '-' || s[0] == '.' || s[0] == '_') {
		s = s[1:]
	}
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}
