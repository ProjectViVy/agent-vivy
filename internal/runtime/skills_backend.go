package runtime

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	einoskill "github.com/cloudwego/eino/adk/middlewares/skill"
	"gopkg.in/yaml.v3"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
	"agent-vivy/internal/tools"
)

const (
	maxSkillBytes        = 512 << 10
	maxSkillFiles        = 128
	maxSkillPathBytes    = 512
	maxSkillPreviewBytes = 32 << 10
)

var _ einoskill.Backend = (*EinoSkillBackend)(nil)
var _ tools.SkillOperations = (*EinoSkillBackend)(nil)

// EinoSkillBackend is a trusted-root, read-only Eino Skill backend plus the
// Vivy-owned staged mutation surface. Skill content is always returned as
// untrusted data; it is never executed by this package.
type EinoSkillBackend struct {
	root      string
	revisions storage.SkillRevisionStore
}

func NewEinoSkillBackend(root string, revisions storage.SkillRevisionStore) (*EinoSkillBackend, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("skills: root must not be empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("skills: resolve root: %w", err)
	}
	return &EinoSkillBackend{root: filepath.Clean(abs), revisions: revisions}, nil
}

func (b *EinoSkillBackend) List(ctx context.Context) ([]einoskill.FrontMatter, error) {
	items, err := b.loadSkills(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]einoskill.FrontMatter, 0, len(items))
	for _, item := range items {
		out = append(out, item.front)
	}
	return out, nil
}

func (b *EinoSkillBackend) Get(ctx context.Context, name string) (einoskill.Skill, error) {
	item, err := b.loadSkill(ctx, name)
	if err != nil {
		return einoskill.Skill{}, err
	}
	return einoskill.Skill{FrontMatter: item.front, Content: item.content, BaseDirectory: item.dir}, nil
}

func (b *EinoSkillBackend) ListSkills(ctx context.Context, _ domain.RunID) ([]tools.SkillSummary, error) {
	items, err := b.loadSkills(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]tools.SkillSummary, 0, len(items))
	for _, item := range items {
		out = append(out, b.summary(item))
	}
	return out, nil
}

func (b *EinoSkillBackend) ViewSkill(ctx context.Context, _ domain.RunID, name, relativePath string) (tools.SkillView, error) {
	item, err := b.loadSkill(ctx, name)
	if err != nil {
		return tools.SkillView{}, err
	}
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return tools.SkillView{SkillSummary: b.summary(item), Content: item.content, RelativePath: "SKILL.md", SupportingFiles: b.supportingFiles(item.dir)}, nil
	}
	path, rel, err := b.resolveSkillPath(item.name, relativePath, false)
	if err != nil {
		return tools.SkillView{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return tools.SkillView{}, fmt.Errorf("skills: read %s: %w", rel, err)
	}
	if len(data) > maxSkillBytes || isBinary(data) {
		return tools.SkillView{}, fmt.Errorf("skills: supporting file is binary or too large")
	}
	warnings := scanSkillText(string(data))
	view := tools.SkillView{SkillSummary: b.summary(item), Content: string(data), RelativePath: rel}
	view.Warnings = warnings
	return view, nil
}

func (b *EinoSkillBackend) ManageSkill(ctx context.Context, runID domain.RunID, req tools.SkillManageRequest) (tools.SkillManageResult, error) {
	proposal, err := b.PrepareSkillProposal(ctx, runID, req)
	if err != nil {
		return tools.SkillManageResult{}, err
	}
	var ref struct {
		RevisionID string `json:"revision_id"`
	}
	if err := json.Unmarshal(proposal.Data, &ref); err != nil || ref.RevisionID == "" {
		return tools.SkillManageResult{}, errors.New("skills: staged revision reference is invalid")
	}
	return b.ApplySkillRevision(ctx, runID, ref.RevisionID)
}

func (b *EinoSkillBackend) PrepareSkillProposal(ctx context.Context, runID domain.RunID, req tools.SkillManageRequest) (domain.ToolProposal, error) {
	if b.revisions == nil {
		return domain.ToolProposal{}, errors.New("skills: revision store not wired")
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	req.SkillName = strings.TrimSpace(req.SkillName)
	if err := validSkillName(req.SkillName); err != nil {
		return domain.ToolProposal{}, err
	}
	if req.Action == "rollback" {
		return b.prepareRollbackProposal(ctx, runID, req)
	}
	if len(req.Path) > maxSkillPathBytes {
		return domain.ToolProposal{}, errors.New("skills: path is too long")
	}
	current, target, targetRel, baseHash, err := b.skillMutationState(ctx, req)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	after, warnings, err := skillMutationContent(req, current, target != "")
	if err != nil {
		return domain.ToolProposal{}, err
	}
	if len(after) > maxSkillBytes {
		return domain.ToolProposal{}, fmt.Errorf("skills: content exceeds %d bytes", maxSkillBytes)
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return domain.ToolProposal{}, fmt.Errorf("skills: encode mutation: %w", err)
	}
	revisionID := newSkillRevisionID(payload)
	contentHash := sha256Hex(after)
	preview := boundedSkillPreview(targetRel, string(current), string(after), req.Action)
	if len(preview) > maxSkillPreviewBytes {
		preview = preview[:safeUTF8Prefix(preview, maxSkillPreviewBytes)] + "\n[preview truncated]\n"
	}
	warnings = append(warnings, scanSkillText(string(after))...)
	warningsJSON, _ := json.Marshal(warnings)
	revision := domain.SkillRevision{
		ID: revisionID, RunID: runID, SkillName: req.SkillName, Action: req.Action, TargetPath: targetRel,
		Payload: payload, BeforePayload: current, BaseHash: baseHash, ContentHash: contentHash, Preview: preview,
		WarningsJSON: warningsJSON, Status: domain.SkillRevisionPending, CreatedAt: time.Now().UnixMilli(),
	}
	if err := b.revisions.CreateSkillRevision(ctx, revision); err != nil {
		return domain.ToolProposal{}, err
	}
	data, _ := json.Marshal(struct {
		RevisionID string `json:"revision_id"`
	}{RevisionID: revisionID})
	return domain.ToolProposal{Action: "skill_" + req.Action, Target: targetRel, PreconditionHash: baseHash,
		Preview: preview, RiskFindings: warnings, Data: data}, nil
}

func (b *EinoSkillBackend) ApplySkillRevision(ctx context.Context, runID domain.RunID, revisionID string) (tools.SkillManageResult, error) {
	if b.revisions == nil {
		return tools.SkillManageResult{}, errors.New("skills: revision store not wired")
	}
	revision, err := b.revisions.GetSkillRevision(ctx, revisionID)
	if err != nil {
		return tools.SkillManageResult{}, err
	}
	if revision.Status != domain.SkillRevisionPending {
		return tools.SkillManageResult{}, fmt.Errorf("skills: revision %s is not pending", revisionID)
	}
	if revision.RunID != "" && runID != "" && revision.RunID != runID {
		return tools.SkillManageResult{}, errors.New("skills: revision run binding mismatch")
	}
	var req tools.SkillManageRequest
	if err := json.Unmarshal(revision.Payload, &req); err != nil {
		return tools.SkillManageResult{}, fmt.Errorf("skills: decode revision %s: %w", revisionID, err)
	}
	if strings.ToLower(strings.TrimSpace(req.Action)) == "rollback" {
		source, _, target, targetRel, baseHash, after, err := b.rollbackMutationState(ctx, runID, req)
		if err != nil {
			return tools.SkillManageResult{}, err
		}
		if baseHash != revision.BaseHash {
			return tools.SkillManageResult{}, errors.New("skills: rollback target changed after human review")
		}
		if err := b.applyRollbackMutation(source, target, targetRel, after); err != nil {
			_ = b.revisions.SetSkillRevisionStatus(context.Background(), revisionID, domain.SkillRevisionFailed, time.Now().UnixMilli())
			return tools.SkillManageResult{}, err
		}
		if err := b.revisions.SetSkillRevisionStatus(ctx, revisionID, domain.SkillRevisionApplied, time.Now().UnixMilli()); err != nil {
			return tools.SkillManageResult{}, err
		}
		return tools.SkillManageResult{RevisionID: revisionID, Status: string(domain.SkillRevisionApplied), SkillName: req.SkillName,
			Action: req.Action, TargetPath: targetRel, Hash: sha256Hex(after), Warnings: []string{"rollback applied"}}, nil
	}
	current, target, targetRel, baseHash, err := b.skillMutationState(ctx, req)
	if err != nil {
		return tools.SkillManageResult{}, err
	}
	if baseHash != revision.BaseHash {
		return tools.SkillManageResult{}, errors.New("skills: target changed after human review")
	}
	after, warnings, err := skillMutationContent(req, current, target != "")
	if err != nil {
		return tools.SkillManageResult{}, err
	}
	if err := b.applyMutation(req, target, targetRel, after, revisionID); err != nil {
		_ = b.revisions.SetSkillRevisionStatus(context.Background(), revisionID, domain.SkillRevisionFailed, time.Now().UnixMilli())
		return tools.SkillManageResult{}, err
	}
	if err := b.revisions.SetSkillRevisionStatus(ctx, revisionID, domain.SkillRevisionApplied, time.Now().UnixMilli()); err != nil {
		return tools.SkillManageResult{}, err
	}
	return tools.SkillManageResult{RevisionID: revisionID, Status: string(domain.SkillRevisionApplied), SkillName: req.SkillName,
		Action: req.Action, TargetPath: targetRel, Hash: sha256Hex(after), Warnings: warnings}, nil
}

func (b *EinoSkillBackend) prepareRollbackProposal(ctx context.Context, runID domain.RunID, req tools.SkillManageRequest) (domain.ToolProposal, error) {
	source, current, _, targetRel, baseHash, after, err := b.rollbackMutationState(ctx, runID, req)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return domain.ToolProposal{}, err
	}
	preview := boundedSkillPreview(targetRel, string(current), string(after), "rollback")
	revisionID := newSkillRevisionID(payload)
	warnings := []string{fmt.Sprintf("rolls back applied revision %s", source.ID)}
	warnings = append(warnings, scanSkillText(string(after))...)
	warningsJSON, _ := json.Marshal(warnings)
	revision := domain.SkillRevision{ID: revisionID, RunID: runID, SkillName: req.SkillName, Action: "rollback", TargetPath: targetRel, Payload: payload, BeforePayload: current, BaseHash: baseHash, ContentHash: sha256Hex(after), Preview: preview, WarningsJSON: warningsJSON, Status: domain.SkillRevisionPending, CreatedAt: time.Now().UnixMilli()}
	if err := b.revisions.CreateSkillRevision(ctx, revision); err != nil {
		return domain.ToolProposal{}, err
	}
	data, _ := json.Marshal(map[string]string{"revision_id": revisionID})
	return domain.ToolProposal{Action: "skill_rollback", Target: targetRel, PreconditionHash: baseHash, Preview: preview, RiskFindings: warnings, Data: data}, nil
}

type loadedSkill struct {
	front    einoskill.FrontMatter
	content  string
	dir      string
	name     string
	hash     string
	warnings []string
}

func (b *EinoSkillBackend) loadSkills(ctx context.Context) ([]loadedSkill, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(b.root, 0o700); err != nil {
		return nil, fmt.Errorf("skills: create root: %w", err)
	}
	entries, err := os.ReadDir(b.root)
	if err != nil {
		return nil, fmt.Errorf("skills: list root: %w", err)
	}
	out := make([]loadedSkill, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		item, err := b.loadSkill(ctx, entry.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (b *EinoSkillBackend) loadSkill(ctx context.Context, name string) (loadedSkill, error) {
	if err := validSkillName(name); err != nil {
		return loadedSkill{}, err
	}
	dir, err := b.skillDir(name)
	if err != nil {
		return loadedSkill{}, err
	}
	path := filepath.Join(dir, "SKILL.md")
	data, err := readTrustedFile(path)
	if err != nil {
		return loadedSkill{}, fmt.Errorf("skills: read %s: %w", name, err)
	}
	front, content, err := parseSkillDocument(data)
	if err != nil {
		return loadedSkill{}, fmt.Errorf("skills: parse %s: %w", name, err)
	}
	if front.Name == "" {
		front.Name = name
	}
	if front.Name != name {
		return loadedSkill{}, fmt.Errorf("skills: frontmatter name %q does not match directory %q", front.Name, name)
	}
	hash := sha256Hex(data)
	return loadedSkill{front: front, content: content, dir: dir, name: name, hash: hash, warnings: scanSkillText(string(data))}, nil
}

func (b *EinoSkillBackend) summary(item loadedSkill) tools.SkillSummary {
	return tools.SkillSummary{Name: item.front.Name, Description: item.front.Description, Context: string(item.front.Context),
		Agent: item.front.Agent, Model: item.front.Model, Hash: item.hash, Warnings: append([]string(nil), item.warnings...)}
}

func (b *EinoSkillBackend) skillDir(name string) (string, error) {
	path := filepath.Join(b.root, name)
	if err := validateUnderRoot(b.root, path); err != nil {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("skills: skill %q not found", name)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("skills: skill %q is not a directory", name)
	}
	return path, nil
}

func (b *EinoSkillBackend) resolveSkillPath(name, relative string, allowMissing bool) (string, string, error) {
	dir, err := b.skillDir(name)
	if err != nil {
		if !allowMissing {
			return "", "", err
		}
		dir = filepath.Join(b.root, name)
	}
	relative = filepath.Clean(filepath.FromSlash(strings.TrimSpace(relative)))
	if relative == "." || filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" || strings.ContainsRune(relative, '\x00') {
		return "", "", errors.New("skills: supporting path must be relative")
	}
	relativeSlash := filepath.ToSlash(relative)
	parts := strings.Split(relativeSlash, "/")
	if relativeSlash != "SKILL.md" && (len(parts) == 0 || parts[0] != "references" && parts[0] != "templates" && parts[0] != "scripts" && parts[0] != "assets") {
		return "", "", errors.New("skills: supporting path must be under references, templates, scripts, or assets")
	}
	path := filepath.Join(dir, relative)
	if err := validateUnderRoot(dir, path); err != nil {
		return "", "", err
	}
	if !allowMissing {
		if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
			return "", "", errors.New("skills: supporting file is unavailable")
		}
	}
	return path, filepath.ToSlash(filepath.Join(name, relative)), nil
}

func (b *EinoSkillBackend) skillMutationState(ctx context.Context, req tools.SkillManageRequest) ([]byte, string, string, string, error) {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "delete" || action == "create" {
		dir := filepath.Join(b.root, req.SkillName)
		if err := validateUnderRoot(b.root, dir); err != nil {
			return nil, "", "", "", err
		}
		if action == "create" {
			if _, err := os.Lstat(dir); err == nil {
				return nil, "", "", "", errors.New("skills: skill already exists")
			} else if !errors.Is(err, os.ErrNotExist) {
				return nil, "", "", "", err
			}
			return nil, filepath.Join(dir, "SKILL.md"), filepath.ToSlash(filepath.Join(req.SkillName, "SKILL.md")), "", nil
		}
		current, err := readTrustedFile(filepath.Join(dir, "SKILL.md"))
		if err != nil {
			return nil, "", "", "", err
		}
		return current, dir, filepath.ToSlash(req.SkillName) + "/", sha256Hex(current), nil
	}
	if _, err := b.skillDir(req.SkillName); err != nil {
		return nil, "", "", "", err
	}
	rel := req.Path
	if action == "edit" || action == "patch" {
		rel = "SKILL.md"
	}
	if rel == "" {
		return nil, "", "", "", errors.New("skills: supporting path is required")
	}
	path, targetRel, err := b.resolveSkillPath(req.SkillName, rel, action == "write_file")
	if err != nil {
		return nil, "", "", "", err
	}
	current, err := readTrustedFile(path)
	if errors.Is(err, os.ErrNotExist) && action == "write_file" {
		current = nil
	} else if err != nil {
		return nil, "", "", "", err
	}
	return current, path, targetRel, sha256Hex(current), nil
}

func skillMutationContent(req tools.SkillManageRequest, current []byte, targetExists bool) ([]byte, []string, error) {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	warnings := []string{}
	switch action {
	case "create", "edit", "write_file":
		if req.Content == "" {
			return nil, nil, errors.New("skills: content is required")
		}
		if action == "create" {
			if _, _, err := parseSkillDocument([]byte(req.Content)); err != nil {
				return nil, nil, fmt.Errorf("skills: invalid SKILL.md: %w", err)
			}
		}
		if action != "create" && !targetExists {
			return nil, nil, errors.New("skills: target file does not exist")
		}
		return []byte(req.Content), warnings, nil
	case "patch":
		if req.OldString == "" || req.OldString == req.NewString {
			return nil, nil, errors.New("skills: patch source and replacement are invalid")
		}
		count := strings.Count(string(current), req.OldString)
		if count == 0 {
			return nil, nil, errors.New("skills: patch source was not found")
		}
		if !req.ReplaceAll && count != 1 {
			return nil, nil, fmt.Errorf("skills: patch source matched %d times", count)
		}
		if req.ReplaceAll {
			warnings = append(warnings, fmt.Sprintf("replaces %d occurrences", count))
		}
		return []byte(strings.Replace(string(current), req.OldString, req.NewString, -1)), warnings, nil
	case "remove_file":
		if !targetExists {
			return nil, nil, errors.New("skills: target file does not exist")
		}
		return nil, warnings, nil
	case "delete":
		warnings = append(warnings, "deletes the entire Skill directory")
		return nil, warnings, nil
	default:
		return nil, nil, fmt.Errorf("skills: unsupported action %q", action)
	}
}

func (b *EinoSkillBackend) applyMutation(req tools.SkillManageRequest, target, targetRel string, after []byte, revisionID string) error {
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "delete" {
		trash := filepath.Join(b.root, ".vivy-trash")
		if err := os.MkdirAll(trash, 0o700); err != nil {
			return err
		}
		return os.Rename(target, filepath.Join(trash, revisionID))
	}
	if action == "remove_file" {
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("skills: remove %s: %w", targetRel, err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return fmt.Errorf("skills: create parent: %w", err)
	}
	if err := atomicWrite(target, after, fileMode(target)); err != nil {
		return fmt.Errorf("skills: apply %s: %w", targetRel, err)
	}
	return nil
}

func (b *EinoSkillBackend) supportingFiles(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || len(out) >= maxSkillFiles {
			return err
		}
		if path == dir {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) > 0 && (parts[0] == "references" || parts[0] == "templates" || parts[0] == "scripts" || parts[0] == "assets") {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

func parseSkillDocument(data []byte) (einoskill.FrontMatter, string, error) {
	text := strings.TrimSpace(string(data))
	if !strings.HasPrefix(text, "---") {
		return einoskill.FrontMatter{}, "", errors.New("SKILL.md must start with YAML frontmatter")
	}
	rest := text[3:]
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return einoskill.FrontMatter{}, "", errors.New("SKILL.md frontmatter is not closed")
	}
	var front einoskill.FrontMatter
	if err := yaml.Unmarshal([]byte(strings.TrimSpace(rest[:idx])), &front); err != nil {
		return einoskill.FrontMatter{}, "", fmt.Errorf("decode frontmatter: %w", err)
	}
	if strings.TrimSpace(front.Description) == "" {
		return einoskill.FrontMatter{}, "", errors.New("frontmatter description is required")
	}
	content := strings.TrimSpace(rest[idx+4:])
	return front, content, nil
}

func readTrustedFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("skills: symlink or non-regular file denied")
	}
	if info.Size() > maxSkillBytes {
		return nil, errors.New("skills: file is too large")
	}
	return os.ReadFile(path)
}

func validateUnderRoot(root, path string) error {
	rel, err := filepath.Rel(root, filepath.Clean(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errors.New("skills: path escapes trusted root")
	}
	return nil
}

func validSkillName(name string) error {
	if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return errors.New("skills: invalid skill name")
	}
	for _, r := range name {
		if r == '-' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		return errors.New("skills: invalid skill name")
	}
	return nil
}

func scanSkillText(text string) []string {
	lower := strings.ToLower(text)
	var warnings []string
	for _, marker := range []struct{ needle, warning string }{
		{"ignore previous instructions", "contains an instruction-override phrase"},
		{"system message", "contains system-message-like text"},
		{"curl ", "contains a network command reference"},
		{"powershell", "contains a process command reference"},
		{"api_key", "mentions a credential-like field"},
		{"secret", "mentions secret-like content"},
	} {
		if strings.Contains(lower, marker.needle) {
			warnings = append(warnings, marker.warning)
		}
	}
	return warnings
}

func boundedSkillPreview(path, old, new, action string) string {
	if action == "delete" {
		return fmt.Sprintf("delete Skill %s", path)
	}
	return boundedDiff(path, old, new)
}

func newSkillRevisionID(payload []byte) string {
	seed := append([]byte(time.Now().UTC().String()), payload...)
	var random [8]byte
	_, _ = rand.Read(random[:])
	seed = append(seed, random[:]...)
	hash := sha256.Sum256(seed)
	return "skrev_" + hex.EncodeToString(hash[:10])
}
func (b *EinoSkillBackend) rollbackMutationState(ctx context.Context, runID domain.RunID, req tools.SkillManageRequest) (domain.SkillRevision, []byte, string, string, string, []byte, error) {
	if b.revisions == nil || strings.TrimSpace(req.RevisionID) == "" {
		return domain.SkillRevision{}, nil, "", "", "", nil, errors.New("skills: revision_id is required for rollback")
	}
	source, err := b.revisions.GetSkillRevision(ctx, strings.TrimSpace(req.RevisionID))
	if err != nil {
		return domain.SkillRevision{}, nil, "", "", "", nil, err
	}
	if source.Status != domain.SkillRevisionApplied {
		return domain.SkillRevision{}, nil, "", "", "", nil, errors.New("skills: only an applied revision can be rolled back")
	}
	if source.RunID != "" && runID != "" && source.RunID != runID {
		return domain.SkillRevision{}, nil, "", "", "", nil, errors.New("skills: rollback run binding mismatch")
	}
	if req.SkillName != "" && req.SkillName != source.SkillName {
		return domain.SkillRevision{}, nil, "", "", "", nil, errors.New("skills: rollback skill name mismatch")
	}
	if source.Action == "delete" {
		target := filepath.Join(b.root, source.SkillName)
		if err := validateUnderRoot(b.root, target); err != nil {
			return domain.SkillRevision{}, nil, "", "", "", nil, err
		}
		if _, err := os.Lstat(target); err == nil {
			return domain.SkillRevision{}, nil, "", "", "", nil, errors.New("skills: deleted Skill target already exists")
		} else if !errors.Is(err, os.ErrNotExist) {
			return domain.SkillRevision{}, nil, "", "", "", nil, err
		}
		return source, nil, target, filepath.ToSlash(source.SkillName) + "/", "", source.BeforePayload, nil
	}
	rel := strings.TrimPrefix(filepath.ToSlash(source.TargetPath), filepath.ToSlash(source.SkillName)+"/")
	target, targetRel, err := b.resolveSkillPath(source.SkillName, rel, true)
	if err != nil {
		return domain.SkillRevision{}, nil, "", "", "", nil, err
	}
	var current []byte
	if data, readErr := readTrustedFile(target); readErr == nil {
		current = data
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return domain.SkillRevision{}, nil, "", "", "", nil, readErr
	}
	if source.Action != "create" && source.BeforePayload == nil {
		return domain.SkillRevision{}, nil, "", "", "", nil, errors.New("skills: revision has no rollback payload")
	}
	return source, current, target, targetRel, sha256Hex(current), source.BeforePayload, nil
}
func (b *EinoSkillBackend) applyRollbackMutation(source domain.SkillRevision, target, targetRel string, after []byte) error {
	if source.Action == "delete" {
		trash := filepath.Join(b.root, ".vivy-trash", source.ID)
		if err := os.Rename(trash, target); err != nil {
			return fmt.Errorf("skills: restore %s: %w", targetRel, err)
		}
		return nil
	}
	if source.Action == "create" && len(after) == 0 {
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("skills: remove created %s: %w", targetRel, err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	return atomicWrite(target, after, fileMode(target))
}
