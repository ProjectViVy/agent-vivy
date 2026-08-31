package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	einofs "github.com/cloudwego/eino/adk/filesystem"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/tools"
)

const (
	defaultFilesystemMaxBytes = 1 << 20
	defaultSearchMaxResults   = 100
	defaultSearchMaxBytes     = 128 << 10
	defaultListDepth          = 4
	maxListDepthLimit         = 10
	maxListEntriesLimit       = 200
	maxDiffBytes              = 32 << 10
)

var (
	errSearchLimit = errors.New("filesystem search limit reached")
	errListLimit   = errors.New("filesystem list limit reached")
)

// EinoFilesystemBackend is the Vivy filesystem boundary. It implements both
// Eino's filesystem.Backend and the Eino-independent tools.FileOperations
// contract so direct Vivy ToolSpecs and Eino middleware share one policy-safe
// implementation.
type EinoFilesystemBackend struct {
	manager        *WorkspaceManager
	sandbox        *SandboxManager
	maxFileBytes   int
	maxResults     int
	maxSearchBytes int
	maxListDepth   int
	maxListEntries int
	rgPath         string
}

var (
	_ einofs.Backend             = (*EinoFilesystemBackend)(nil)
	_ tools.FileOperations       = (*EinoFilesystemBackend)(nil)
	_ tools.GrepOperations       = (*EinoFilesystemBackend)(nil)
	_ tools.MultiPatchOperations = (*EinoFilesystemBackend)(nil)
)

// NewEinoFilesystemBackend binds file operations to the existing per-run
// workspace allocator. A nil manager is accepted for unit construction but
// every operation fails closed until a manager is supplied. The sandbox
// parameter controls permission boundaries (D-021).
func NewEinoFilesystemBackend(manager *WorkspaceManager, sandbox *SandboxManager) *EinoFilesystemBackend {
	// ripgrep is an optional accelerator: probes fail silently and every
	// Grep path keeps a pure-Go fallback.
	rgPath, _ := exec.LookPath("rg")
	return &EinoFilesystemBackend{
		manager: manager, sandbox: sandbox, maxFileBytes: defaultFilesystemMaxBytes,
		maxResults: defaultSearchMaxResults, maxSearchBytes: defaultSearchMaxBytes,
		maxListDepth: maxListDepthLimit, maxListEntries: maxListEntriesLimit,
		rgPath: rgPath,
	}
}

// ListDir implements tools.FileOperations with a bounded directory listing.
// Recursive walks list ignored directories (.git, node_modules, ...) but
// never traverse them, mirroring the search walk policy.
func (b *EinoFilesystemBackend) ListDir(ctx context.Context, runID domain.RunID, req tools.DirListRequest) (tools.DirListResult, error) {
	if b.sandbox != nil {
		root, _, err := b.resolve(ctx, runID, req.Path, false)
		if err == nil {
			fullPath := filepath.Join(root, req.Path)
			if err := b.sandbox.ValidatePathWithMode(fullPath, FileOpRead, sandboxMode(ctx)); err != nil {
				return tools.DirListResult{}, fmt.Errorf("sandbox: %w", err)
			}
		}
	}
	root, path, err := b.resolve(ctx, runID, req.Path, false)
	if err != nil {
		return tools.DirListResult{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return tools.DirListResult{}, fmt.Errorf("filesystem: list %s: %w", displayPath(root, path), err)
	}
	if !info.IsDir() {
		return tools.DirListResult{}, fmt.Errorf("filesystem: list target %s is not a directory", displayPath(root, path))
	}
	maxEntries := req.MaxEntries
	if maxEntries <= 0 || maxEntries > b.maxListEntries {
		maxEntries = b.maxListEntries
	}
	result := tools.DirListResult{Path: displayPath(root, path)}
	appendEntry := func(entryPath string, entry fs.DirEntry) error {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return nil
		}
		if len(result.Entries) >= maxEntries {
			result.Truncated = true
			return errListLimit
		}
		result.Entries = append(result.Entries, tools.DirEntry{
			Path: displayPath(root, entryPath), IsDir: entryInfo.IsDir(), Size: entryInfo.Size(),
			ModifiedAt: entryInfo.ModTime().UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
		return nil
	}
	if !req.Recursive {
		entries, err := os.ReadDir(path)
		if err != nil {
			return tools.DirListResult{}, fmt.Errorf("filesystem: list %s: %w", displayPath(root, path), err)
		}
		for _, entry := range entries {
			if err := appendEntry(filepath.Join(path, entry.Name()), entry); err != nil {
				break
			}
		}
		return result, nil
	}
	depth := req.Depth
	if depth <= 0 {
		depth = defaultListDepth
	}
	if depth > b.maxListDepth {
		depth = b.maxListDepth
	}
	walkErr := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if current == path {
			return nil
		}
		if err := appendEntry(current, entry); err != nil {
			return err
		}
		if entry.IsDir() && (entryDepth(path, current) >= depth || isIgnoredDirectory(entry.Name())) {
			return filepath.SkipDir
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errListLimit) {
		return tools.DirListResult{}, fmt.Errorf("filesystem: list: %w", walkErr)
	}
	return result, nil
}

// ReadFile implements tools.FileOperations.
func (b *EinoFilesystemBackend) ReadFile(ctx context.Context, runID domain.RunID, req tools.FileReadRequest) (tools.FileReadResult, error) {
	// Sandbox validation: check read permission
	if b.sandbox != nil {
		root, _, err := b.resolve(ctx, runID, req.Path, false)
		if err == nil {
			fullPath := filepath.Join(root, req.Path)
			if err := b.sandbox.ValidatePathWithMode(fullPath, FileOpRead, sandboxMode(ctx)); err != nil {
				return tools.FileReadResult{}, fmt.Errorf("sandbox: %w", err)
			}
		}
	}

	root, path, err := b.resolve(ctx, runID, req.Path, false)
	if err != nil {
		return tools.FileReadResult{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return tools.FileReadResult{}, fmt.Errorf("filesystem: read %s: %w", displayPath(root, path), err)
	}
	result := tools.FileReadResult{Path: displayPath(root, path), Bytes: len(data)}
	if len(data) > b.maxFileBytes {
		data = data[:b.maxFileBytes]
		result.Truncated = true
	}
	if isBinary(data) {
		result.Binary = true
		return result, nil
	}
	text := string(data)
	lines := strings.Split(text, "\n")
	result.TotalLines = len(lines)
	start := req.StartLine
	if start <= 0 {
		start = 1
	}
	end := req.EndLine
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		result.StartLine, result.EndLine = start, start-1
		return result, nil
	}
	if end < start {
		return tools.FileReadResult{}, fmt.Errorf("filesystem: end_line must be >= start_line")
	}
	result.StartLine, result.EndLine = start, end
	result.Content = strings.Join(lines[start-1:end], "\n")
	if req.MaxBytes > 0 && len(result.Content) > req.MaxBytes {
		result.Content = result.Content[:safeUTF8Prefix(result.Content, req.MaxBytes)]
		result.Truncated = true
	}
	return result, nil
}

// SearchFiles implements tools.FileOperations with bounded literal search.
func (b *EinoFilesystemBackend) SearchFiles(ctx context.Context, runID domain.RunID, req tools.FileSearchRequest) (tools.FileSearchResult, error) {
	if err := b.validateSandboxPath(ctx, runID, req.Path, FileOpRead, false); err != nil {
		return tools.FileSearchResult{}, err
	}
	root, base, err := b.resolve(ctx, runID, req.Path, false)
	if err != nil {
		return tools.FileSearchResult{}, err
	}
	query := req.Query
	if query == "" {
		return tools.FileSearchResult{}, fmt.Errorf("filesystem: search query must not be empty")
	}
	maxResults := req.MaxResults
	if maxResults <= 0 || maxResults > b.maxResults {
		maxResults = b.maxResults
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 || maxBytes > b.maxSearchBytes {
		maxBytes = b.maxSearchBytes
	}
	needle := query
	if !req.CaseSensitive {
		needle = strings.ToLower(needle)
	}
	result := tools.FileSearchResult{}
	walkErr := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if path != base && isIgnoredDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel := displayPath(root, path)
		if req.Glob != "" && !matchesGlob(req.Glob, rel, entry.Name()) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		result.FilesScanned++
		if len(data) > b.maxFileBytes || isBinary(data) {
			return nil
		}
		for lineNo, line := range strings.Split(string(data), "\n") {
			candidate := line
			if !req.CaseSensitive {
				candidate = strings.ToLower(candidate)
			}
			if !strings.Contains(candidate, needle) {
				continue
			}
			match := tools.FileMatch{Path: rel, Line: lineNo + 1, Text: trimResultLine(line)}
			if len(result.Matches) >= maxResults || resultBytes(result)+len(match.Text)+len(match.Path) > maxBytes {
				result.Truncated = true
				return errSearchLimit
			}
			result.Matches = append(result.Matches, match)
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errSearchLimit) {
		return tools.FileSearchResult{}, fmt.Errorf("filesystem: search: %w", walkErr)
	}
	return result, nil
}

// WriteFile implements tools.FileOperations. The runtime adapter assumes the
// caller has already passed policy/HITL; it still revalidates the target here.
func (b *EinoFilesystemBackend) WriteFile(ctx context.Context, runID domain.RunID, req tools.FileWriteRequest) (tools.FileMutationResult, error) {
	// Sandbox validation: check write permission
	if b.sandbox != nil {
		root, _, err := b.resolve(ctx, runID, req.Path, true)
		if err == nil {
			fullPath := filepath.Join(root, req.Path)
			if err := b.sandbox.ValidatePathWithMode(fullPath, FileOpWrite, sandboxMode(ctx)); err != nil {
				return tools.FileMutationResult{}, fmt.Errorf("sandbox: %w", err)
			}
		}
	}

	root, path, err := b.resolve(ctx, runID, req.Path, true)
	if err != nil {
		return tools.FileMutationResult{}, err
	}
	if len(req.Content) > b.maxFileBytes {
		return tools.FileMutationResult{}, fmt.Errorf("filesystem: content exceeds %d bytes", b.maxFileBytes)
	}
	var old []byte
	if existing, readErr := os.ReadFile(path); readErr == nil {
		old = existing
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return tools.FileMutationResult{}, fmt.Errorf("filesystem: read existing %s: %w", displayPath(root, path), readErr)
	}
	if expected := tools.ProposalPreconditionFromContext(ctx); expected != "" && sha256Hex(old) != expected {
		err := fmt.Errorf("filesystem: proposal stale: target changed after human review")
		return tools.FileMutationResult{}, err
	}
	if bytes.Equal(old, []byte(req.Content)) {
		return mutationResult(root, path, old, false), nil
	}
	if req.CreateParents {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return tools.FileMutationResult{}, fmt.Errorf("filesystem: create parent: %w", err)
		}
		if _, _, err := b.resolve(ctx, runID, req.Path, true); err != nil {
			return tools.FileMutationResult{}, err
		}
	}
	if err := atomicWrite(path, []byte(req.Content), fileMode(path)); err != nil {
		return tools.FileMutationResult{}, fmt.Errorf("filesystem: write %s: %w", displayPath(root, path), err)
	}
	result := mutationResult(root, path, []byte(req.Content), true)
	result.Diff = boundedDiff(displayPath(root, path), string(old), req.Content)
	return result, nil
}

// PatchFile implements exact unique replacement with a bounded
// whitespace-tolerant fallback (applyStringPatch).
func (b *EinoFilesystemBackend) PatchFile(ctx context.Context, runID domain.RunID, req tools.FilePatchRequest) (tools.FileMutationResult, error) {
	if req.OldString == "" {
		return tools.FileMutationResult{}, fmt.Errorf("filesystem: old_string must not be empty")
	}
	if req.OldString == req.NewString {
		return tools.FileMutationResult{}, fmt.Errorf("filesystem: old_string and new_string must differ")
	}
	if err := b.validateSandboxPath(ctx, runID, req.Path, FileOpWrite, false); err != nil {
		return tools.FileMutationResult{}, err
	}
	root, path, err := b.resolve(ctx, runID, req.Path, false)
	if err != nil {
		return tools.FileMutationResult{}, err
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return tools.FileMutationResult{}, fmt.Errorf("filesystem: read patch target: %w", err)
	}
	if isBinary(old) {
		return tools.FileMutationResult{}, fmt.Errorf("filesystem: patch target is binary")
	}
	if expected := tools.ProposalPreconditionFromContext(ctx); expected != "" && sha256Hex(old) != expected {
		err := fmt.Errorf("filesystem: proposal stale: target changed after human review")
		return tools.FileMutationResult{}, err
	}
	newContent, _, err := applyStringPatch(string(old), req.OldString, req.NewString, req.ReplaceAll)
	if err != nil {
		return tools.FileMutationResult{}, err
	}
	result, err := b.WriteFile(ctx, runID, tools.FileWriteRequest{Path: req.Path, Content: newContent})
	if err != nil {
		return tools.FileMutationResult{}, err
	}
	result.Diff = boundedDiff(displayPath(root, path), string(old), newContent)
	return result, nil
}

// PrepareWriteFile builds the review record without mutating the target.
func (b *EinoFilesystemBackend) PrepareWriteFile(ctx context.Context, runID domain.RunID, req tools.FileWriteRequest) (domain.ToolProposal, error) {
	root, path, err := b.resolve(ctx, runID, req.Path, true)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	if len(req.Content) > b.maxFileBytes {
		return domain.ToolProposal{}, fmt.Errorf("filesystem: content exceeds %d bytes", b.maxFileBytes)
	}
	old, err := readExistingFile(path)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	oldHash := sha256Hex(old)
	warnings := []string{}
	if len(old) == 0 {
		warnings = append(warnings, "creates a new file")
	} else {
		warnings = append(warnings, "replaces existing file content")
	}
	data, _ := json.Marshal(req)
	return domain.ToolProposal{
		Action: "write_file", Target: displayPath(root, path), PreconditionHash: oldHash,
		Preview: boundedDiff(displayPath(root, path), string(old), req.Content), RiskFindings: warnings, Data: data,
	}, nil
}

// PreparePatchFile validates the patch (with the same whitespace-tolerant
// fallback as PatchFile) and returns its diff without changing the target.
// The same application runs again in PatchFile after resume.
func (b *EinoFilesystemBackend) PreparePatchFile(ctx context.Context, runID domain.RunID, req tools.FilePatchRequest) (domain.ToolProposal, error) {
	if req.OldString == "" {
		return domain.ToolProposal{}, fmt.Errorf("filesystem: old_string must not be empty")
	}
	if req.OldString == req.NewString {
		return domain.ToolProposal{}, fmt.Errorf("filesystem: old_string and new_string must differ")
	}
	root, path, err := b.resolve(ctx, runID, req.Path, false)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return domain.ToolProposal{}, fmt.Errorf("filesystem: read patch target: %w", err)
	}
	if isBinary(old) {
		return domain.ToolProposal{}, fmt.Errorf("filesystem: patch target is binary")
	}
	newContent, occurrences, err := applyStringPatch(string(old), req.OldString, req.NewString, req.ReplaceAll)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	data, _ := json.Marshal(req)
	warnings := []string{}
	if req.ReplaceAll {
		warnings = append(warnings, fmt.Sprintf("replaces %d occurrences", occurrences))
	}
	return domain.ToolProposal{
		Action: "patch", Target: displayPath(root, path), PreconditionHash: sha256Hex(old),
		Preview: boundedDiff(displayPath(root, path), string(old), newContent), RiskFindings: warnings, Data: data,
	}, nil
}

// MultiPatchFile applies several string replacements to one file atomically:
// edits run in order against an in-memory buffer and the file is written
// once, so a failing edit leaves the target untouched.
func (b *EinoFilesystemBackend) MultiPatchFile(ctx context.Context, runID domain.RunID, req tools.FileMultiEditRequest) (tools.FileMutationResult, error) {
	if len(req.Edits) == 0 {
		return tools.FileMutationResult{}, fmt.Errorf("filesystem: multiedit requires at least one edit")
	}
	if err := b.validateSandboxPath(ctx, runID, req.Path, FileOpWrite, false); err != nil {
		return tools.FileMutationResult{}, err
	}
	root, path, err := b.resolve(ctx, runID, req.Path, false)
	if err != nil {
		return tools.FileMutationResult{}, err
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return tools.FileMutationResult{}, fmt.Errorf("filesystem: read multiedit target: %w", err)
	}
	if isBinary(old) {
		return tools.FileMutationResult{}, fmt.Errorf("filesystem: multiedit target is binary")
	}
	if expected := tools.ProposalPreconditionFromContext(ctx); expected != "" && sha256Hex(old) != expected {
		return tools.FileMutationResult{}, fmt.Errorf("filesystem: proposal stale: target changed after human review")
	}
	content := string(old)
	for i, item := range req.Edits {
		var err error
		content, _, err = applyStringPatch(content, item.OldString, item.NewString, item.ReplaceAll)
		if err != nil {
			return tools.FileMutationResult{}, fmt.Errorf("filesystem: edit %d: %w", i+1, err)
		}
	}
	result, err := b.WriteFile(ctx, runID, tools.FileWriteRequest{Path: req.Path, Content: content})
	if err != nil {
		return tools.FileMutationResult{}, err
	}
	result.Diff = boundedDiff(displayPath(root, path), string(old), content)
	return result, nil
}

// PrepareMultiPatchFile validates the whole edit list and returns the combined
// diff without changing the target.
func (b *EinoFilesystemBackend) PrepareMultiPatchFile(ctx context.Context, runID domain.RunID, req tools.FileMultiEditRequest) (domain.ToolProposal, error) {
	if len(req.Edits) == 0 {
		return domain.ToolProposal{}, fmt.Errorf("filesystem: multiedit requires at least one edit")
	}
	root, path, err := b.resolve(ctx, runID, req.Path, false)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return domain.ToolProposal{}, fmt.Errorf("filesystem: read multiedit target: %w", err)
	}
	if isBinary(old) {
		return domain.ToolProposal{}, fmt.Errorf("filesystem: multiedit target is binary")
	}
	content := string(old)
	for i, item := range req.Edits {
		var err error
		content, _, err = applyStringPatch(content, item.OldString, item.NewString, item.ReplaceAll)
		if err != nil {
			return domain.ToolProposal{}, fmt.Errorf("filesystem: edit %d: %w", i+1, err)
		}
	}
	data, _ := json.Marshal(req)
	return domain.ToolProposal{
		Action: "multiedit", Target: displayPath(root, path), PreconditionHash: sha256Hex(old),
		Preview:      boundedDiff(displayPath(root, path), string(old), content),
		RiskFindings: []string{fmt.Sprintf("applies %d edits in one write", len(req.Edits))}, Data: data,
	}, nil
}

// Read implements Eino's filesystem.Backend.
func (b *EinoFilesystemBackend) Read(ctx context.Context, req *einofs.ReadRequest) (*einofs.FileContent, error) {
	if req == nil {
		return nil, fmt.Errorf("filesystem: read request is nil")
	}
	endLine := 0
	if req.Limit > 0 {
		endLine = req.Offset + req.Limit - 1
	}
	result, err := b.ReadFile(ctx, filesystemRunID(ctx), tools.FileReadRequest{Path: req.FilePath, StartLine: req.Offset, EndLine: endLine})
	if err != nil {
		return nil, err
	}
	if result.Binary {
		return &einofs.FileContent{Content: "[binary file omitted]"}, nil
	}
	return &einofs.FileContent{Content: result.Content}, nil
}

func (b *EinoFilesystemBackend) Write(ctx context.Context, req *einofs.WriteRequest) error {
	if req == nil {
		return fmt.Errorf("filesystem: write request is nil")
	}
	_, err := b.WriteFile(ctx, filesystemRunID(ctx), tools.FileWriteRequest{Path: req.FilePath, Content: req.Content, CreateParents: true})
	return err
}

func (b *EinoFilesystemBackend) Edit(ctx context.Context, req *einofs.EditRequest) error {
	if req == nil {
		return fmt.Errorf("filesystem: edit request is nil")
	}
	_, err := b.PatchFile(ctx, filesystemRunID(ctx), tools.FilePatchRequest{Path: req.FilePath, OldString: req.OldString, NewString: req.NewString, ReplaceAll: req.ReplaceAll})
	return err
}

func (b *EinoFilesystemBackend) LsInfo(ctx context.Context, req *einofs.LsInfoRequest) ([]einofs.FileInfo, error) {
	pathValue := ""
	if req != nil {
		pathValue = req.Path
	}
	if err := b.validateSandboxPath(ctx, filesystemRunID(ctx), pathValue, FileOpRead, false); err != nil {
		return nil, err
	}
	root, path, err := b.resolve(ctx, filesystemRunID(ctx), pathValue, false)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("filesystem: list: %w", err)
	}
	out := make([]einofs.FileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out = append(out, einofs.FileInfo{Path: displayPath(root, filepath.Join(path, entry.Name())), IsDir: info.IsDir(), Size: info.Size(), ModifiedAt: info.ModTime().UTC().Format("2006-01-02T15:04:05Z07:00")})
	}
	return out, nil
}

func (b *EinoFilesystemBackend) GlobInfo(ctx context.Context, req *einofs.GlobInfoRequest) ([]einofs.FileInfo, error) {
	if req == nil || strings.TrimSpace(req.Pattern) == "" {
		return nil, fmt.Errorf("filesystem: glob pattern must not be empty")
	}
	if err := b.validateSandboxPath(ctx, filesystemRunID(ctx), req.Path, FileOpRead, false); err != nil {
		return nil, err
	}
	root, base, err := b.resolve(ctx, filesystemRunID(ctx), req.Path, false)
	if err != nil {
		return nil, err
	}
	out := make([]einofs.FileInfo, 0)
	err = filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel := displayPath(root, path)
		if matchesGlob(req.Pattern, rel, entry.Name()) {
			info, err := entry.Info()
			if err == nil {
				out = append(out, einofs.FileInfo{Path: rel, IsDir: info.IsDir(), Size: info.Size(), ModifiedAt: info.ModTime().UTC().Format("2006-01-02T15:04:05Z07:00")})
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("filesystem: glob: %w", err)
	}
	return out, nil
}

func (b *EinoFilesystemBackend) GrepRaw(ctx context.Context, req *einofs.GrepRequest) ([]einofs.GrepMatch, error) {
	if req == nil || req.Pattern == "" {
		return nil, fmt.Errorf("filesystem: grep pattern must not be empty")
	}
	pattern := req.Pattern
	if req.CaseInsensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("filesystem: invalid grep pattern: %w", err)
	}
	root, base, err := b.searchRoot(ctx, req.Path)
	if err != nil {
		return nil, err
	}
	res, err := b.goGrep(base, root, re, "")
	if err != nil {
		return nil, err
	}
	out := make([]einofs.GrepMatch, 0, len(res.Matches))
	for _, m := range res.Matches {
		out = append(out, einofs.GrepMatch{Path: m.Path, Line: m.Line, Content: m.Content})
	}
	return out, nil
}

func (b *EinoFilesystemBackend) validateSandboxPath(ctx context.Context, runID domain.RunID, value string, op FileOp, allowMissing bool) error {
	if b.sandbox == nil {
		return nil
	}
	root, _, err := b.resolve(ctx, runID, value, allowMissing)
	if err != nil {
		return nil
	}
	fullPath := filepath.Join(root, value)
	if err := b.sandbox.ValidatePathWithMode(fullPath, op, sandboxMode(ctx)); err != nil {
		return fmt.Errorf("sandbox: %w", err)
	}
	return nil
}

func (b *EinoFilesystemBackend) resolve(ctx context.Context, runID domain.RunID, value string, allowMissing bool) (string, string, error) {
	if b == nil || b.manager == nil {
		return "", "", fmt.Errorf("filesystem: workspace manager not wired")
	}
	workspace, err := b.manager.Ensure(ctx, runID)
	if err != nil {
		return "", "", err
	}
	path, err := safeWorkspacePath(workspace.Path, value, allowMissing)
	if err != nil {
		return "", "", err
	}
	return workspace.Path, path, nil
}

func filesystemRunID(ctx context.Context) domain.RunID {
	if runID := contextRunID(ctx); runID != "" {
		return runID
	}
	return tools.RunIDFromContext(ctx)
}

func safeWorkspacePath(root, value string, allowMissing bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "."
	}
	if strings.ContainsRune(value, '\x00') || filepath.IsAbs(filepath.FromSlash(value)) || filepath.VolumeName(filepath.FromSlash(value)) != "" || strings.HasPrefix(value, "\\\\") {
		return "", fmt.Errorf("filesystem: path must be workspace-relative")
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." && value != "." {
		return "", fmt.Errorf("filesystem: invalid path")
	}
	full := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("filesystem: path escapes workspace")
	}
	if protectedPath(rel) {
		return "", fmt.Errorf("filesystem: protected path denied")
	}
	parts := strings.Split(rel, string(filepath.Separator))
	current := root
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if allowMissing {
				break
			}
			// Wrap the sentinel so eino middlewares (agentsmd) can
			// errors.Is(err, os.ErrNotExist) this as "absent, skip".
			return "", fmt.Errorf("filesystem: path does not exist: %w", os.ErrNotExist)
		}
		if err != nil {
			return "", fmt.Errorf("filesystem: inspect path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("filesystem: symlink path denied")
		}
		if i < len(parts)-1 && !info.IsDir() {
			return "", fmt.Errorf("filesystem: path component is not a directory")
		}
	}
	return full, nil
}

func protectedPath(rel string) bool {
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		lower := strings.ToLower(part)
		if lower == ".env" || strings.HasPrefix(lower, ".env.") || lower == "keys.txt" || lower == "credentials" || lower == "secrets" || lower == ".ssh" || lower == "id_rsa" || lower == "id_ed25519" {
			return true
		}
	}
	return false
}

func atomicWrite(path string, content []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".vivy-write-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if mode == 0 {
		mode = 0o600
	}
	if err := tmp.Chmod(mode.Perm()); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
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

func fileMode(path string) os.FileMode {
	info, err := os.Stat(path)
	if err == nil {
		return info.Mode()
	}
	return 0o600
}

func mutationResult(root, path string, content []byte, changed bool) tools.FileMutationResult {
	return tools.FileMutationResult{Path: displayPath(root, path), Changed: changed, Bytes: len(content), SHA256: sha256Hex(content)}
}

func readExistingFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("filesystem: read existing file: %w", err)
	}
	if isBinary(data) {
		return nil, fmt.Errorf("filesystem: target file is binary")
	}
	return data, nil
}

func sha256Hex(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func boundedDiff(path, old, new string) string {
	diff := fmt.Sprintf("--- %s\n+++ %s\n@@\n-%s\n+%s\n", path, path, old, new)
	if len(diff) <= maxDiffBytes {
		return diff
	}
	return diff[:safeUTF8Prefix(diff, maxDiffBytes)] + "\n[diff truncated]\n"
}

func displayPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func isBinary(data []byte) bool {
	return bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data)
}

func safeUTF8Prefix(value string, max int) int {
	if max >= len(value) {
		return len(value)
	}
	for max > 0 && (value[max]&0xc0) == 0x80 {
		max--
	}
	return max
}

func trimResultLine(line string) string {
	line = strings.TrimSpace(line)
	if len(line) > 512 {
		line = line[:safeUTF8Prefix(line, 512)] + "..."
	}
	return line
}

func resultBytes(result tools.FileSearchResult) int {
	total := 0
	for _, match := range result.Matches {
		total += len(match.Path) + len(match.Text) + 24
	}
	return total
}

func isIgnoredDirectory(name string) bool {
	switch strings.ToLower(name) {
	case ".git", ".svn", "node_modules":
		return true
	default:
		return false
	}
}

// entryDepth counts separator-delimited levels below base; direct children
// of base are depth 1.
func entryDepth(base, path string) int {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return 0
	}
	return strings.Count(filepath.ToSlash(rel), "/") + 1
}

func matchesGlob(pattern, relative, base string) bool {
	return globPatternMatches(filepath.ToSlash(pattern), filepath.ToSlash(relative), base)
}
