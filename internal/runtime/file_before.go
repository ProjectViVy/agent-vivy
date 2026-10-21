package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

const fileBeforeFormat = "vivy.file-before/v1"

type fileBeforeState struct {
	exists  bool
	content []byte
}
type fileBeforeEnvelope struct {
	Format  string `json:"format"`
	Path    string `json:"path"`
	Exists  *bool  `json:"exists"`
	Content []byte `json:"content"`
}

type fileBeforeBindingError struct{ cause error }

func (e *fileBeforeBindingError) Error() string {
	return "filesystem: source_binding_unavailable: re-present the file for review"
}
func (e *fileBeforeBindingError) Unwrap() error { return e.cause }

// File before-state is private ProviderData inside D03's approval envelope.
// Reads use the existing root-bound descriptor, limits and identity checks.
func captureFileBefore(ctx context.Context, root, path string, limit int64) (fileBeforeState, error) {
	file, err := openWorkspaceFile(root, displayPath(root, path), true)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fileBeforeState{}, nil
		}
		return fileBeforeState{}, fmt.Errorf("filesystem: read before-state: %w", err)
	}
	defer file.close()
	if file.info.Size() > limit {
		return fileBeforeState{}, fmt.Errorf("filesystem: before-state exceeds %d bytes", limit)
	}
	content, err := io.ReadAll(io.LimitReader(ctxReader{ctx: ctx, r: file.file}, limit+1))
	if err != nil {
		return fileBeforeState{}, fmt.Errorf("filesystem: read before-state: %w", err)
	}
	if int64(len(content)) > limit {
		return fileBeforeState{}, fmt.Errorf("filesystem: before-state exceeds %d bytes", limit)
	}
	if err := file.recheckIdentity(file.root.Name()); err != nil {
		return fileBeforeState{}, fmt.Errorf("filesystem: read before-state: %w", err)
	}
	after, err := file.file.Stat()
	if err != nil {
		return fileBeforeState{}, fmt.Errorf("filesystem: stat before-state: %w", err)
	}
	if after.Size() != file.info.Size() || !after.ModTime().Equal(file.info.ModTime()) {
		return fileBeforeState{}, fmt.Errorf("filesystem: %w", errSecureChanged)
	}
	return fileBeforeState{exists: true, content: content}, nil
}

func encodeFileBefore(path string, before fileBeforeState) (json.RawMessage, error) {
	exists := before.exists
	data, err := json.Marshal(fileBeforeEnvelope{Format: fileBeforeFormat, Path: path, Exists: &exists, Content: before.content})
	if err != nil {
		return nil, fmt.Errorf("filesystem: encode private before-state: %w", err)
	}
	return data, nil
}

func checkReviewedFileBefore(ctx context.Context, path string, current fileBeforeState, limit int64) error {
	data := tools.ProposalDataFromContext(ctx)
	if !tools.HasProposalData(ctx) {
		return nil
	}
	if int64(len(data)) > 2*limit+8192 {
		return errors.New("filesystem: source_binding_unavailable: private before-state exceeds limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var before fileBeforeEnvelope
	if err := decoder.Decode(&before); err != nil {
		return &fileBeforeBindingError{cause: err}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return &fileBeforeBindingError{cause: err}
		}
		return errors.New("filesystem: source_binding_unavailable: re-present the file for review")
	}
	if before.Format != fileBeforeFormat || before.Path != path || before.Exists == nil || (*before.Exists && before.Content == nil) || int64(len(before.Content)) > limit || (!*before.Exists && len(before.Content) != 0) {
		return errors.New("filesystem: source_binding_unavailable: re-present the file for review")
	}
	if *before.Exists != current.exists || !bytes.Equal(before.Content, current.content) {
		return errors.New("filesystem: proposal stale: target changed after human review")
	}
	return nil
}

func (b *EinoFilesystemBackend) recheckFileBefore(ctx context.Context, runID domain.RunID, path string, before fileBeforeState, limit int64) error {
	root, absolute, err := b.resolve(ctx, runID, path, true)
	if err != nil {
		return err
	}
	if b.sandbox != nil {
		if err := b.sandbox.ValidatePathWithinRoot(root, absolute, FileOpWrite, sandboxMode(ctx)); err != nil {
			return fmt.Errorf("sandbox: %w", err)
		}
	}
	current, err := captureFileBefore(ctx, root, absolute, limit)
	if err != nil {
		return err
	}
	if before.exists != current.exists || !bytes.Equal(before.content, current.content) {
		return errors.New("filesystem: proposal stale: target changed before write")
	}
	return nil
}
