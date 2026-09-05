package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
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
	files    *EinoFilesystemBackend
	maxBytes int64
}

var _ tools.DownloadOperations = (*DownloadBackend)(nil)

func NewDownloadBackend(files *EinoFilesystemBackend, sandbox *SandboxManager) *DownloadBackend {
	return &DownloadBackend{
		client:   &http.Client{Transport: newPublicHTTPTransport(), Timeout: defaultDownloadTime},
		sandbox:  sandbox,
		files:    files,
		maxBytes: maxDownloadBytes,
	}
}

// allowLoopbackForTest lets httptest servers on 127.0.0.1 exercise the
// pipeline in tests; the production constructor never enables it.
func (b *DownloadBackend) allowLoopbackForTest() {
	b.client.Transport = &http.Transport{
		Proxy:             http.ProxyFromEnvironment,
		DialContext:       safeDialContext,
		ForceAttemptHTTP2: true,
	}
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
	var old []byte
	if existing, readErr := os.ReadFile(path); readErr == nil {
		old = existing
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return tools.DownloadResult{}, fmt.Errorf("download: read existing %s: %w", displayPath(root, path), readErr)
	}
	if expected := tools.ProposalPreconditionFromContext(ctx); expected != "" && sha256Hex(old) != expected {
		return tools.DownloadResult{}, errors.New("download: proposal stale: target changed after human review")
	}

	client := *b.client
	client.Timeout = clampFetchTimeout(input.TimeoutSeconds, defaultDownloadTime, minDownloadTimeout, maxDownloadTimeout)
	client.CheckRedirect = publicRedirectCheck()
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
		if err := b.sandbox.ValidatePathWithMode(path, FileOpWrite, sandboxMode(ctx)); err != nil {
			return tools.DownloadResult{}, fmt.Errorf("sandbox: %w", err)
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".vivy-download-*")
	if err != nil {
		return tools.DownloadResult{}, fmt.Errorf("download: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	sink := &hashingWriter{w: tmp, hash: sha256.New()}
	_, copyErr := io.Copy(sink, io.LimitReader(resp.Body, b.maxBytes+1))
	closeErr := tmp.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr == nil && sink.n > b.maxBytes {
		copyErr = fmt.Errorf("download: response exceeds %d byte limit", b.maxBytes)
	}
	if copyErr == nil {
		copyErr = renameInto(tmpName, path, fileMode(path))
	}
	if copyErr != nil {
		_ = os.Remove(tmpName)
		return tools.DownloadResult{}, fmt.Errorf("download: %w", copyErr)
	}
	return tools.DownloadResult{
		Path:        displayPath(root, path),
		Bytes:       sink.n,
		SHA256:      hex.EncodeToString(sink.hash.Sum(nil)),
		ContentType: resp.Header.Get("Content-Type"),
		Overwritten: len(old) > 0,
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
	warnings := []string{"writes remote content into the workspace"}
	if info, statErr := os.Stat(path); statErr == nil && info.Size() > 0 {
		warnings = append(warnings, "overwrites existing file")
	}
	data, _ := json.Marshal(req)
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

// hashingWriter hashes and counts everything that reached the disk.
type hashingWriter struct {
	w    io.Writer
	hash hash.Hash
	n    int64
}

func (h *hashingWriter) Write(p []byte) (int, error) {
	n, err := h.w.Write(p)
	h.hash.Write(p[:n])
	h.n += int64(n)
	return n, err
}

// renameInto mirrors atomicWrite's durable tail for content that was already
// streamed to a temp file.
func renameInto(tmpName, path string, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o600
	}
	if err := os.Chmod(tmpName, mode.Perm()); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		return os.Rename(tmpName, path)
	}
	return nil
}
