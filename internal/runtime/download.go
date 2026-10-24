package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

// maxDownloadBytes caps one streamed download. Unlike the text tools it does
// not enter the model context, so the bound protects the workspace disk, not
// tokens.
const maxDownloadBytes = 100 << 20

// DownloadBackend streams a public URL into the run workspace. Path
// policy is delegated to the filesystem backend; the network gates are the
// same public-only surface as WebFetchBackend.
type DownloadBackend struct {
	client   *http.Client
	sandbox  *SandboxManager
	network  *webNetwork
	files    *EinoFilesystemBackend
	maxBytes int64
}

var _ tools.DownloadOperations = (*DownloadBackend)(nil)

func NewDownloadBackend(files *EinoFilesystemBackend, sandbox *SandboxManager) *DownloadBackend {
	return newDownloadBackend(files, sandbox, newWebNetworkFromEnv())
}

func newDownloadBackend(files *EinoFilesystemBackend, sandbox *SandboxManager, network *webNetwork) *DownloadBackend {
	return &DownloadBackend{
		client:   &http.Client{Transport: network.transport(), Timeout: defaultDownloadTime},
		sandbox:  sandbox,
		network:  network,
		files:    files,
		maxBytes: maxDownloadBytes,
	}
}

// allowLoopbackForTest lets httptest servers on 127.0.0.1 exercise the
// pipeline in tests; the production constructor never enables it.
func (b *DownloadBackend) allowLoopbackForTest() {
	b.network.allowLoopback = true
	b.client.Transport = b.network.testTransport()
}

// Download implements tools.DownloadOperations. The adapter assumes policy/
// HITL already ran; the target and network are revalidated here.
func (b *DownloadBackend) Download(ctx context.Context, runID domain.RunID, input tools.DownloadRequest) (tools.DownloadResult, error) {
	root, path, err := b.resolveTarget(ctx, runID, input.Path)
	if err != nil {
		return tools.DownloadResult{}, err
	}
	u, err := validatePublicURL(input.URL)
	if err != nil {
		return tools.DownloadResult{}, err
	}
	if b.sandbox != nil {
		if err := b.sandbox.CheckNetwork(u.String()); err != nil {
			return tools.DownloadResult{}, fmt.Errorf("sandbox: %w", err)
		}
	}
	if err := b.network.validateTarget(ctx, u); err != nil {
		return tools.DownloadResult{}, err
	}
	before, err := captureFileBefore(ctx, root, path, b.maxBytes)
	if err != nil {
		return tools.DownloadResult{}, err
	}
	if err := checkReviewedFileBefore(ctx, displayPath(root, path), before, b.maxBytes); err != nil {
		return tools.DownloadResult{}, err
	}

	client := *b.client
	client.Timeout = clampFetchTimeout(input.TimeoutSeconds, defaultDownloadTime, minDownloadTimeout, maxDownloadTimeout)
	client.CheckRedirect = b.network.redirectCheck()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return tools.DownloadResult{}, fmt.Errorf("download: build request: %w", err)
	}
	req.Header.Set("User-Agent", webFetchUserAgent)
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return tools.DownloadResult{}, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return tools.DownloadResult{}, fmt.Errorf("download: remote returned status %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return tools.DownloadResult{}, fmt.Errorf("download: create parent: %w", err)
	}
	// Sandbox validation runs only after the parents exist: the symlink walk
	// needs real directories, and resolve() has already bounded the path to
	// the workspace before anything is created.
	if b.sandbox != nil {
		if err := b.sandbox.ValidatePathWithinRoot(root, path, FileOpWrite, sandboxMode(ctx)); err != nil {
			return tools.DownloadResult{}, fmt.Errorf("sandbox: %w", err)
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".vivy-download-*")
	if err != nil {
		return tools.DownloadResult{}, fmt.Errorf("download: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	copied, copyErr := io.Copy(tmp, io.LimitReader(resp.Body, b.maxBytes+1))
	if copyErr == nil {
		copyErr = tmp.Sync()
	}
	closeErr := tmp.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr == nil && copied > b.maxBytes {
		copyErr = fmt.Errorf("download: response exceeds %d byte limit", b.maxBytes)
	}
	if copyErr == nil {
		copyErr = renameInto(tmpName, path, fileMode(path), func() error { return b.files.recheckFileBefore(ctx, runID, input.Path, before, b.maxBytes) })
	}
	if copyErr != nil {
		_ = os.Remove(tmpName)
		return tools.DownloadResult{}, fmt.Errorf("download: %w", copyErr)
	}
	return tools.DownloadResult{
		Path:        displayPath(root, path),
		Bytes:       copied,
		ContentType: resp.Header.Get("Content-Type"),
		Overwritten: before.exists,
	}, nil
}

// PrepareDownload builds the review record without touching the network or
// the target; the same checks run again in Download after approval.
func (b *DownloadBackend) PrepareDownload(ctx context.Context, runID domain.RunID, req tools.DownloadRequest) (domain.ToolProposal, error) {
	root, path, err := b.resolveTarget(ctx, runID, req.Path)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	if _, err := validatePublicURL(req.URL); err != nil {
		return domain.ToolProposal{}, err
	}
	before, err := captureFileBefore(ctx, root, path, b.maxBytes)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	warnings := []string{"writes remote content into the workspace"}
	if before.exists {
		warnings = append(warnings, "overwrites existing file")
	}
	data, err := encodeFileBefore(displayPath(root, path), before)
	if err != nil {
		return domain.ToolProposal{}, err
	}

	target := displayPath(root, path)
	return domain.ToolProposal{
		Action: tools.DownloadName, Target: target,
		Preview:      fmt.Sprintf("download %s\n→ %s", req.URL, target),
		RiskFindings: warnings, Data: data,
	}, nil
}

func (b *DownloadBackend) resolveTarget(ctx context.Context, runID domain.RunID, path string) (string, string, error) {
	if b.files == nil {
		return "", "", errors.New("download: filesystem backend not wired")
	}
	// Bounding only, no sandbox check here: proposals must not mutate the
	// tree, and execution-time validation needs the parents to exist (same
	// split as WriteFile/PrepareWriteFile).
	root, abs, err := b.files.resolve(ctx, runID, path, true)
	if err != nil {
		return "", "", err
	}
	return root, abs, nil
}

// renameInto mirrors atomicWrite's durable tail for content that was already
// streamed to a temp file.
func renameInto(tmpName, path string, mode os.FileMode, check func() error) error {
	if mode == 0 {
		mode = 0o600
	}
	if err := os.Chmod(tmpName, mode.Perm()); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
