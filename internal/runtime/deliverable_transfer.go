package runtime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"agent-vivy/internal/domain"
)

// transferIdleTTL bounds a live download handle; a client that goes quiet
// loses the snapshot and re-authorizes a fresh one on its next read.
const transferIdleTTL = 60 * time.Second

// secureOpenError carries a safe public reason for one refused file access.
// Internal causes stay off the wire.
type secureOpenError struct {
	public string
	cause  error
}

func (e *secureOpenError) Error() string { return e.public }
func (e *secureOpenError) Unwrap() error { return e.cause }

var (
	errSecureOutside   = errors.New("runtime: path escapes workspace root")
	errSecureSymlink   = errors.New("runtime: symlink paths are not allowed")
	errSecureIrregular = errors.New("runtime: path is not a regular file")
	errSecureSensitive = errors.New("runtime: path is sensitive")
	errSecureChanged   = errors.New("runtime: file changed while reading")
)

// secureFile is one validated, root-bound regular-file handle. Callers read
// and hash through this descriptor only; reopening by name is never trusted.
type secureFile struct {
	file  *os.File
	info  os.FileInfo
	clean string // normalized workspace-relative path, forward slashes
	real  string // canonical absolute path inside the real root
	root  *os.Root
}

// openWorkspaceFileSecure resolves rel against a trusted workspace root with
// the shared safe-open pattern: path normalization, sensitive-path policy,
// pre-flight link checks, an os.Root-bound open and a regular-type check on
// the opened descriptor. A name swapped between the checks is either refused
// by the root handle or caught by the post-read identity recheck.
func openWorkspaceFileSecure(root, rel string) (*secureFile, error) {
	clean, err := workspaceRelPath(rel)
	if err != nil {
		return nil, &secureOpenError{public: "invalid workspace-relative path", cause: err}
	}
	if SensitiveWorkspacePath(clean) {
		return nil, &secureOpenError{public: "path is sensitive", cause: errSecureSensitive}
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, &secureOpenError{public: "workspace is unavailable", cause: err}
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, &secureOpenError{public: "workspace is unavailable", cause: err}
	}
	rootReal, err = filepath.Abs(rootReal)
	if err != nil {
		return nil, &secureOpenError{public: "workspace is unavailable", cause: err}
	}
	info, err := os.Stat(rootReal)
	if err != nil {
		return nil, &secureOpenError{public: "workspace is unavailable", cause: err}
	}
	if !info.IsDir() {
		return nil, &secureOpenError{public: "workspace is unavailable", cause: errSecureOutside}
	}
	rootHandle, err := os.OpenRoot(rootReal)
	if err != nil {
		return nil, &secureOpenError{public: "workspace is unavailable", cause: err}
	}
	sf := &secureFile{clean: clean, root: rootHandle}
	if err := sf.preflight(rootAbs, rootReal); err != nil {
		_ = rootHandle.Close()
		return nil, err
	}
	return sf, nil
}

// preflight rejects links before the root-bound open. The workspace contract
// is link-free, so any resolved path that differs from the lexical join or
// escapes the real root is refused outright.
func (s *secureFile) preflight(rootAbs, rootReal string) error {
	native := filepath.FromSlash(s.clean)
	// Reject FIFOs, sockets and devices before any open: opening a FIFO for
	// reading blocks until a writer appears.
	lstat, err := s.root.Lstat(native)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &secureOpenError{public: "file is missing", cause: err}
		}
		return &secureOpenError{public: "file cannot be opened", cause: err}
	}
	if !lstat.Mode().IsRegular() {
		return &secureOpenError{public: "path is not a regular file", cause: errSecureIrregular}
	}
	candidateReal, err := filepath.EvalSymlinks(filepath.Join(rootAbs, native))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &secureOpenError{public: "file is missing", cause: err}
		}
		return &secureOpenError{public: "file cannot be opened", cause: err}
	}
	candidateReal, err = filepath.Abs(candidateReal)
	if err != nil || !pathWithinRoot(rootReal, candidateReal) {
		if err == nil {
			err = errSecureOutside
		}
		return &secureOpenError{public: "path escapes the workspace", cause: errSecureOutside}
	}
	canonicalRel, err := filepath.Rel(rootReal, candidateReal)
	if err != nil {
		return &secureOpenError{public: "file cannot be opened", cause: err}
	}
	if SensitiveWorkspacePath(canonicalRel) {
		return &secureOpenError{public: "path is sensitive", cause: errSecureSensitive}
	}
	lexicalReal, err := filepath.Abs(filepath.Join(rootReal, native))
	if err != nil || !sameFilePath(lexicalReal, candidateReal) {
		return &secureOpenError{public: "symlink paths are not allowed", cause: errSecureSymlink}
	}
	file, err := s.root.Open(native)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &secureOpenError{public: "file is missing", cause: err}
		}
		return &secureOpenError{public: "file cannot be opened", cause: err}
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return &secureOpenError{public: "file cannot be opened", cause: err}
	}
	if !openedInfo.Mode().IsRegular() {
		_ = file.Close()
		return &secureOpenError{public: "path is not a regular file", cause: errSecureIrregular}
	}
	s.file = file
	s.info = openedInfo
	s.real = lexicalReal
	return nil
}

// recheckIdentity verifies the named entry still resolves to the opened
// descriptor after a read. A concurrent replacement — same name, other inode
// or a link — is detected instead of trusted.
func (s *secureFile) recheckIdentity(rootReal string) error {
	native := filepath.FromSlash(s.clean)
	postReal, postErr := filepath.EvalSymlinks(filepath.Join(rootReal, native))
	postInfo, statErr := os.Stat(filepath.Join(rootReal, native))
	if postErr != nil || statErr != nil {
		return &secureOpenError{public: "file changed while reading", cause: errSecureChanged}
	}
	postReal, absErr := filepath.Abs(postReal)
	postRel, relErr := filepath.Rel(rootReal, postReal)
	if absErr != nil || relErr != nil || !sameFilePath(s.real, postReal) || SensitiveWorkspacePath(postRel) || !os.SameFile(s.info, postInfo) {
		return &secureOpenError{public: "file changed while reading", cause: errSecureChanged}
	}
	return nil
}

func (s *secureFile) close() {
	if s.file != nil {
		_ = s.file.Close()
	}
	if s.root != nil {
		_ = s.root.Close()
	}
}

// ctxReader aborts a long copy when the caller's context is cancelled.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (r ctxReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func pathWithinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func sameFilePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

// deliveryTransfer is one verified download snapshot bound to its owner,
// item and digest. Only the last served window may be replayed; every other
// offset must advance strictly.
type deliveryTransfer struct {
	id          string
	owner       domain.SessionID
	itemID      string
	digest      string
	snapshot    string
	size        int64
	servedStart int64
	servedLen   int64
	hasServed   bool
	expiresAt   time.Time
}

// transferManager owns short-lived verified snapshots in one dedicated
// scratch directory. Handles are opaque, owner-scoped and replaced wholesale
// on every new first-read: a connection never accumulates two live copies.
type transferManager struct {
	dir     string
	ttl     time.Duration
	now     func() time.Time
	maxRead int64

	mu      sync.Mutex
	byID    map[string]*deliveryTransfer
	byOwner map[domain.SessionID]string
}

func newTransferManager(dir string, now func() time.Time) (*transferManager, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, errors.New("runtime: transfer scratch must not be empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("runtime: resolve transfer scratch: %w", err)
	}
	if now == nil {
		now = time.Now
	}
	m := &transferManager{dir: abs, ttl: transferIdleTTL, now: now, byID: map[string]*deliveryTransfer{}, byOwner: map[domain.SessionID]string{}}
	if err := m.purge(); err != nil {
		return nil, err
	}
	return m, nil
}

// purge wipes abandoned snapshots on startup. It removes only the contents
// of the dedicated scratch directory, never workspace files.
func (m *transferManager) purge() error {
	if err := os.MkdirAll(m.dir, 0o700); err != nil {
		return fmt.Errorf("runtime: prepare transfer scratch: %w", err)
	}
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return fmt.Errorf("runtime: list transfer scratch: %w", err)
	}
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(m.dir, entry.Name())); err != nil {
			return fmt.Errorf("runtime: purge transfer scratch: %w", err)
		}
	}
	return nil
}

// create hashes the complete bounded file while copying it into a scratch
// snapshot, then creates the owner-bound handle. A hash that does not match
// the presented digest deletes the temp and reports changed.
func (m *transferManager) create(ctx context.Context, sf *secureFile, rootReal string, owner domain.SessionID, itemID, digest string, maxBytes int64) (*deliveryTransfer, error) {
	bound := maxBytes
	if m.maxRead > 0 && (bound <= 0 || m.maxRead < bound) {
		bound = m.maxRead
	}
	if bound > 0 && sf.info.Size() > bound {
		return nil, &secureOpenError{public: "file exceeds the readable limit", cause: errSecureIrregular}
	}
	tmp, err := os.CreateTemp(m.dir, "snap-*")
	if err != nil {
		return nil, fmt.Errorf("runtime: create transfer snapshot: %w", err)
	}
	tmpPath := tmp.Name()
	remove := func() {
		_ = os.Remove(tmpPath)
	}
	hash := sha256.New()
	// bound+1 detects an oversize read; a zero bound hashes the stat size.
	limit := bound
	if limit <= 0 {
		limit = sf.info.Size()
	}
	written, err := io.CopyN(tmp, io.TeeReader(ctxReader{ctx: ctx, r: sf.file}, hash), limit+1)
	closeErr := tmp.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		remove()
		return nil, fmt.Errorf("runtime: snapshot transfer bytes: %w", err)
	}
	if closeErr != nil {
		remove()
		return nil, fmt.Errorf("runtime: write transfer snapshot: %w", closeErr)
	}
	if bound > 0 && written > bound {
		remove()
		return nil, &secureOpenError{public: "file exceeds the readable limit", cause: errSecureIrregular}
	}
	if err := ctx.Err(); err != nil {
		remove()
		return nil, err
	}
	if err := sf.recheckIdentity(rootReal); err != nil {
		remove()
		return nil, err
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	if sum != digest {
		remove()
		return nil, &secureOpenError{public: "file changed since presentation", cause: errSecureChanged}
	}
	id, err := newTransferID()
	if err != nil {
		remove()
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if previous, ok := m.byOwner[owner]; ok {
		m.dropLocked(previous)
	}
	tr := &deliveryTransfer{
		id:        id,
		owner:     owner,
		itemID:    itemID,
		digest:    digest,
		snapshot:  tmpPath,
		size:      written,
		expiresAt: m.now().Add(m.ttl),
	}
	m.byID[id] = tr
	m.byOwner[owner] = id
	return tr, nil
}

// lookup validates a handle for a subsequent chunk: it must exist, be fresh,
// belong to the caller and pin the same item and digest.
func (m *transferManager) lookup(owner domain.SessionID, transferID, itemID, digest string) (*deliveryTransfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	tr, ok := m.byID[transferID]
	if !ok {
		return nil, &secureOpenError{public: "transfer is not active", cause: fs.ErrNotExist}
	}
	if tr.owner != owner {
		return nil, &secureOpenError{public: "transfer belongs to another session", cause: errSecureOutside}
	}
	if tr.itemID != itemID || tr.digest != digest {
		return nil, &secureOpenError{public: "transfer does not match this item", cause: errSecureIrregular}
	}
	tr.expiresAt = m.now().Add(m.ttl)
	return tr, nil
}

// close removes one handle and its snapshot. Removing an unknown or foreign
// handle is refused, never silently ignored.
func (m *transferManager) close(owner domain.SessionID, transferID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked()
	tr, ok := m.byID[transferID]
	if !ok {
		return &secureOpenError{public: "transfer is not active", cause: fs.ErrNotExist}
	}
	if tr.owner != owner {
		return &secureOpenError{public: "transfer belongs to another session", cause: errSecureOutside}
	}
	m.dropLocked(transferID)
	return nil
}

// closeSession drops every handle of one session; deletion must not leave a
// live copy reachable after the owning session is gone.
func (m *transferManager) closeSession(owner domain.SessionID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.byOwner[owner]; ok {
		m.dropLocked(id)
	}
}

func (m *transferManager) expireLocked() {
	now := m.now()
	for id, tr := range m.byID {
		if !tr.expiresAt.After(now) {
			m.dropLocked(id)
		}
	}
}

func (m *transferManager) dropLocked(id string) {
	tr, ok := m.byID[id]
	if !ok {
		return
	}
	delete(m.byID, id)
	if m.byOwner[tr.owner] == id {
		delete(m.byOwner, tr.owner)
	}
	_ = os.Remove(tr.snapshot)
}

// newTransferID mints an opaque, unguessable handle; it is not a capability
// URL and carries no authority without the owner check.
func newTransferID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("runtime: mint transfer id: %w", err)
	}
	return "xfr_" + hex.EncodeToString(raw[:]), nil
}
