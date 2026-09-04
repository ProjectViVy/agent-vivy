package postgres

import (
	"context"
	"fmt"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

var _ storage.ModifiedFileStore = (*Backend)(nil)

// ListModifiedFiles returns bounded per-path summaries ordered by the newest
// retained file-version timestamp. Bodies are read only long enough to count
// line changes and never leave this storage projection.
func (b *Backend) ListModifiedFiles(ctx context.Context, sessionID domain.SessionID, limit int) ([]storage.ModifiedFile, error) {
	if limit <= 0 {
		return []storage.ModifiedFile{}, nil
	}
	if limit > storage.ModifiedFileMax {
		limit = storage.ModifiedFileMax
	}
	pathsRows, err := b.db.QueryContext(ctx, `
		SELECT path FROM file_versions
		WHERE session_id = ?
		GROUP BY path
		ORDER BY MAX(created_at) DESC, path
		LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: list modified file paths %s: %w", sessionID, err)
	}
	defer func() { _ = pathsRows.Close() }()
	var paths []string
	for pathsRows.Next() {
		var path string
		if err := pathsRows.Scan(&path); err != nil {
			return nil, fmt.Errorf("storage: scan modified file path: %w", err)
		}
		paths = append(paths, path)
	}
	if err := pathsRows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate modified file paths: %w", err)
	}
	if err := pathsRows.Close(); err != nil {
		return nil, fmt.Errorf("storage: close modified file paths: %w", err)
	}

	out := make([]storage.ModifiedFile, 0, len(paths))
	for _, path := range paths {
		var oldest storage.FileVersionRow
		err := b.db.QueryRowContext(ctx, `
			SELECT path, version, content, created_at
			FROM file_versions
			WHERE session_id = ? AND path = ?
			ORDER BY version ASC
			LIMIT 1`, sessionID, path).Scan(&oldest.Path, &oldest.Version, &oldest.Content, &oldest.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("storage: read oldest modified file version %s: %w", path, err)
		}
		var newest storage.FileVersionRow
		err = b.db.QueryRowContext(ctx, `
			SELECT path, version, content, created_at
			FROM file_versions
			WHERE session_id = ? AND path = ?
			ORDER BY version DESC
			LIMIT 1`, sessionID, path).Scan(&newest.Path, &newest.Version, &newest.Content, &newest.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("storage: read newest modified file version %s: %w", path, err)
		}
		versions := []storage.FileVersionRow{oldest}
		if newest.Version != oldest.Version {
			versions = append(versions, newest)
		}
		if summary := storage.BuildModifiedFileSummaries(versions); summary.Path != "" && (summary.Diff.Additions > 0 || summary.Diff.Deletions > 0) {
			out = append(out, summary)
		}
	}
	return out, nil
}
