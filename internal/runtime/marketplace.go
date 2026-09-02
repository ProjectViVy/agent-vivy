package runtime

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"agent-vivy/internal/tools"
)

const (
	// DefaultMarketplaceBaseURL is the public skills.sh directory that powers
	// the `npx skills` CLI. Search and file snapshots are unauthenticated.
	DefaultMarketplaceBaseURL = "https://skills.sh"
	// MarketplaceBaseURLOverride is the optional environment override for the
	// marketplace base URL (operators, tests, air-gapped mirrors).
	MarketplaceBaseURLOverride = "VIVY_SKILLS_MARKETPLACE_URL"

	marketplaceSearchDefaultLimit = 20
	marketplaceSearchMaxLimit     = 50
	marketplaceRequestTimeout     = 30 * time.Second
	marketplaceUserAgent          = "agent-vivy-marketplace/1.0"
	// maxMarketplaceTotalBytes caps the whole snapshot on top of the
	// per-file maxSkillBytes and per-skill maxSkillFiles limits the loader
	// enforces anyway.
	maxMarketplaceTotalBytes = 5 << 20
)

var _ tools.SkillsMarketplace = (*MarketplaceService)(nil)

// MarketplaceUpstreamError reports a failed skills.sh call so the control
// plane can map it to a dedicated RPC code instead of a generic 500.
type MarketplaceUpstreamError struct {
	Op     string
	Status int
	Detail string
}

func (e *MarketplaceUpstreamError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("marketplace: %s failed with upstream status %d", e.Op, e.Status)
	}
	return fmt.Sprintf("marketplace: %s failed with upstream status %d: %s", e.Op, e.Status, e.Detail)
}

// MarketplaceService adapts the skills.sh directory onto the local
// skills_root. Skill text from the directory is untrusted data: every
// installed file passes the same loader validation, size caps, and
// prompt-injection scanning as hand-placed skills.
type MarketplaceService struct {
	backend *EinoSkillBackend
	baseURL string
	client  *http.Client
}

func NewMarketplaceService(backend *EinoSkillBackend, baseURL string) (*MarketplaceService, error) {
	if backend == nil {
		return nil, errors.New("marketplace: skills backend is required")
	}
	base := strings.TrimSpace(baseURL)
	if base == "" {
		if override := strings.TrimSpace(os.Getenv(MarketplaceBaseURLOverride)); override != "" {
			base = override
		} else {
			base = DefaultMarketplaceBaseURL
		}
	}
	base = strings.TrimRight(base, "/")
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("marketplace: base url %q must be an absolute http(s) URL", baseURL)
	}
	return &MarketplaceService{backend: backend, baseURL: base, client: &http.Client{Timeout: marketplaceRequestTimeout}}, nil
}

func (s *MarketplaceService) SearchMarketplace(ctx context.Context, query string, limit int) ([]tools.MarketplaceSkill, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return nil, errors.New("marketplace: query must be at least 2 characters")
	}
	if limit <= 0 {
		limit = marketplaceSearchDefaultLimit
	}
	limit = min(marketplaceSearchMaxLimit, max(1, limit))
	endpoint := fmt.Sprintf("%s/api/search?q=%s&limit=%d", s.baseURL, url.QueryEscape(query), limit)
	body, err := s.get(ctx, "search", endpoint)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Skills []tools.MarketplaceSkill `json:"skills"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("marketplace: search returned invalid JSON: %w", err)
	}
	if payload.Skills == nil {
		payload.Skills = []tools.MarketplaceSkill{}
	}
	return payload.Skills, nil
}

func (s *MarketplaceService) FeaturedMarketplace(context.Context) (tools.MarketplaceFeatured, error) {
	return featuredSnapshot()
}

func (s *MarketplaceService) InstallMarketplace(ctx context.Context, id string, mode tools.MarketplaceInstallMode) (tools.MarketplaceInstallResult, error) {
	id = strings.TrimSpace(id)
	if mode == "" {
		mode = tools.MarketplaceInstallCreate
	}
	if mode != tools.MarketplaceInstallCreate && mode != tools.MarketplaceInstallUpgrade {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("marketplace: unknown install mode %q (want create or upgrade)", mode)
	}
	slug, snapshotHash, files, err := s.downloadSnapshot(ctx, id)
	if err != nil {
		return tools.MarketplaceInstallResult{}, err
	}
	return s.installSnapshot(ctx, id, slug, snapshotHash, files, mode)
}

// downloadSnapshot resolves owner/repo/slug, rejects slugs Vivy cannot host,
// fetches the skills.sh file snapshot, and validates its JSON shape.
func (s *MarketplaceService) downloadSnapshot(ctx context.Context, id string) (string, string, []snapshotFile, error) {
	owner, repo, slug, err := parseMarketplaceSkillID(id)
	if err != nil {
		return "", "", nil, err
	}
	// Fail fast on slugs Vivy cannot host before spending a download.
	if err := validSkillName(slug); err != nil {
		return "", "", nil, fmt.Errorf("marketplace: installable slug is not valid for Vivy: %w", err)
	}
	encodedOwner, err := encodeMarketplaceSegment(owner)
	if err != nil {
		return "", "", nil, err
	}
	encodedRepo, err := encodeMarketplaceSegment(repo)
	if err != nil {
		return "", "", nil, err
	}
	encodedSlug, err := encodeMarketplaceSegment(slug)
	if err != nil {
		return "", "", nil, err
	}
	endpoint := fmt.Sprintf("%s/api/download/%s/%s/%s", s.baseURL, encodedOwner, encodedRepo, encodedSlug)
	body, err := s.get(ctx, "download", endpoint)
	if err != nil {
		return "", "", nil, err
	}
	var snapshot struct {
		Files []struct {
			Path     string `json:"path"`
			Contents string `json:"contents"`
		} `json:"files"`
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return "", "", nil, fmt.Errorf("marketplace: download returned invalid JSON: %w", err)
	}
	files := make([]snapshotFile, 0, len(snapshot.Files))
	for _, file := range snapshot.Files {
		files = append(files, snapshotFile{path: file.Path, contents: file.Contents})
	}
	return slug, snapshot.Hash, files, nil
}

type snapshotFile struct {
	path     string
	contents string
}

// marketplaceManifestName is the provenance marker written next to SKILL.md.
// The skill loader only reads SKILL.md and the four hosted directories, so
// the manifest never enters skill content or warnings.
const marketplaceManifestName = ".vivy-skill.json"

type marketplaceManifest struct {
	MarketplaceID string `json:"marketplace_id"`
	SnapshotHash  string `json:"snapshot_hash,omitempty"`
	InstalledAt   string `json:"installed_at"`
	UpgradedAt    string `json:"upgraded_at,omitempty"`
}

func readMarketplaceManifest(dir string) (marketplaceManifest, []byte, error) {
	raw, err := os.ReadFile(filepath.Join(dir, marketplaceManifestName))
	if err != nil {
		return marketplaceManifest{}, nil, err
	}
	var manifest marketplaceManifest
	if json.Unmarshal(raw, &manifest) != nil || strings.TrimSpace(manifest.MarketplaceID) == "" {
		return manifest, raw, errors.New("marketplace: manifest is invalid")
	}
	return manifest, raw, nil
}

func writeMarketplaceManifest(dir string, manifest marketplaceManifest) error {
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(dir, marketplaceManifestName), append(encoded, '\n'), 0o600)
}

// installSnapshot validates the whole snapshot before touching disk, then
// creates the skill directory or, in upgrade mode, swaps the hosted file set
// of a marketplace-managed skill in place. Every guard mirrors the disk
// loader (validSkillName, frontmatter name match, supporting-directory
// restriction, size caps, binary rejection) so an installed package is
// loadable on the very next turn without any engine restart.
func (s *MarketplaceService) installSnapshot(ctx context.Context, id, slug, snapshotHash string, files []snapshotFile, mode tools.MarketplaceInstallMode) (tools.MarketplaceInstallResult, error) {
	hosted, skipped, err := validateSnapshot(slug, files)
	if err != nil {
		return tools.MarketplaceInstallResult{}, err
	}
	dir := filepath.Join(s.backend.root, slug)
	if err := validateUnderRoot(s.backend.root, dir); err != nil {
		return tools.MarketplaceInstallResult{}, err
	}
	_, statErr := os.Lstat(dir)
	if errors.Is(statErr, os.ErrNotExist) {
		return s.createSkill(ctx, id, slug, snapshotHash, hosted, skipped)
	}
	if statErr != nil {
		return tools.MarketplaceInstallResult{}, statErr
	}
	if mode != tools.MarketplaceInstallUpgrade {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("skills: skill %q already exists", slug)
	}
	return s.upgradeSkill(ctx, id, slug, snapshotHash, hosted)
}

// validateSnapshot applies the full loader-mirroring guard set to the raw
// snapshot and returns the hosted file set (SKILL.md plus the four
// supporting directories) as path→contents, along with the skipped paths.
func validateSnapshot(slug string, files []snapshotFile) (map[string]string, []string, error) {
	if len(files) == 0 {
		return nil, nil, errors.New("marketplace: snapshot has no files")
	}
	if len(files) > maxSkillFiles {
		return nil, nil, fmt.Errorf("marketplace: snapshot exceeds %d files", maxSkillFiles)
	}
	hosted := make(map[string]string, len(files))
	var skipped []string
	var head []byte
	total := 0
	sawHead := false
	for _, file := range files {
		total += len(file.contents)
		if total > maxMarketplaceTotalBytes {
			return nil, nil, errors.New("marketplace: snapshot exceeds the total size limit")
		}
		path, err := normalizeSnapshotPath(file.path)
		if err != nil {
			return nil, nil, err
		}
		if path == "SKILL.md" {
			if sawHead {
				return nil, nil, errors.New("marketplace: snapshot has duplicate SKILL.md files")
			}
			sawHead = true
			head = []byte(file.contents)
			hosted[path] = file.contents
			continue
		}
		first := strings.SplitN(path, "/", 2)[0]
		if first != "references" && first != "templates" && first != "scripts" && first != "assets" {
			skipped = append(skipped, path)
			continue
		}
		if len(file.contents) > maxSkillBytes {
			return nil, nil, fmt.Errorf("marketplace: %s exceeds the size limit", file.path)
		}
		if isBinary([]byte(file.contents)) {
			return nil, nil, fmt.Errorf("marketplace: %s is not UTF-8 text", file.path)
		}
		hosted[path] = file.contents
	}
	if !sawHead {
		return nil, nil, errors.New("marketplace: snapshot has no root SKILL.md")
	}
	if isBinary(head) {
		return nil, nil, errors.New("marketplace: SKILL.md is not UTF-8 text")
	}
	front, _, _, err := parseSkillDocument(head)
	if err != nil {
		return nil, nil, fmt.Errorf("marketplace: snapshot SKILL.md is invalid: %w", err)
	}
	if strings.TrimSpace(front.Name) == "" {
		return nil, nil, errors.New("marketplace: snapshot SKILL.md must declare a name matching the install slug")
	}
	if front.Name != slug {
		return nil, nil, fmt.Errorf("marketplace: snapshot name %q does not match slug %q", front.Name, slug)
	}
	return hosted, skipped, nil
}

func (s *MarketplaceService) createSkill(ctx context.Context, id, slug, snapshotHash string, hosted map[string]string, skipped []string) (tools.MarketplaceInstallResult, error) {
	dir := filepath.Join(s.backend.root, slug)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("marketplace: create skill directory: %w", err)
	}
	rollback := func(cause error) (tools.MarketplaceInstallResult, error) {
		_ = os.RemoveAll(dir)
		return tools.MarketplaceInstallResult{}, cause
	}
	manifest := marketplaceManifest{MarketplaceID: id, SnapshotHash: snapshotHash, InstalledAt: time.Now().UTC().Format(time.RFC3339)}
	if err := writeMarketplaceManifest(dir, manifest); err != nil {
		return rollback(fmt.Errorf("marketplace: write manifest: %w", err))
	}
	for path, contents := range hosted {
		if err := ctx.Err(); err != nil {
			return rollback(err)
		}
		target := filepath.Join(dir, filepath.FromSlash(path))
		if err := validateUnderRoot(dir, target); err != nil {
			return rollback(err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return rollback(fmt.Errorf("marketplace: create %s parent: %w", path, err))
		}
		if err := atomicWrite(target, []byte(contents), 0o600); err != nil {
			return rollback(fmt.Errorf("marketplace: write %s: %w", path, err))
		}
	}
	view, err := s.viewFor(ctx, slug)
	if err != nil {
		return rollback(err)
	}
	return tools.MarketplaceInstallResult{Skill: view, Outcome: tools.MarketplaceOutcomeCreated, SkippedFiles: skipped}, nil
}

// upgradeSkill replaces the hosted file set of an existing marketplace-managed
// skill. The previous hosted files and manifest are held in memory as the
// rollback state; writes go through atomicWrite so a crash mid-upgrade leaves
// a loadable skill (old or new SKILL.md), never a broken one.
func (s *MarketplaceService) upgradeSkill(ctx context.Context, id, slug, snapshotHash string, hosted map[string]string) (tools.MarketplaceInstallResult, error) {
	dir := filepath.Join(s.backend.root, slug)
	manifest, rawManifest, err := readMarketplaceManifest(dir)
	if err != nil {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("skills: skill %q is not marketplace-managed; remove it and reinstall to upgrade", slug)
	}
	if manifest.MarketplaceID != id {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("skills: skill %q was installed from %s, not %s", slug, manifest.MarketplaceID, id)
	}
	installed, err := readHostedState(dir)
	if err != nil {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("marketplace: read installed skill: %w", err)
	}
	if hostedSetsEqual(installed, hosted) {
		view, viewErr := s.viewFor(ctx, slug)
		if viewErr != nil {
			return tools.MarketplaceInstallResult{}, viewErr
		}
		return tools.MarketplaceInstallResult{Skill: view, Outcome: tools.MarketplaceUpdateUpToDate}, nil
	}
	updated := marketplaceManifest{MarketplaceID: id, SnapshotHash: snapshotHash, InstalledAt: manifest.InstalledAt, UpgradedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := applyHostedState(dir, hosted); err != nil {
		restoreHostedState(dir, installed, rawManifest)
		return tools.MarketplaceInstallResult{}, err
	}
	if err := writeMarketplaceManifest(dir, updated); err != nil {
		restoreHostedState(dir, installed, rawManifest)
		return tools.MarketplaceInstallResult{}, fmt.Errorf("marketplace: write manifest: %w", err)
	}
	view, err := s.viewFor(ctx, slug)
	if err != nil {
		restoreHostedState(dir, installed, rawManifest)
		return tools.MarketplaceInstallResult{}, err
	}
	return tools.MarketplaceInstallResult{Skill: view, Outcome: tools.MarketplaceOutcomeUpgraded}, nil
}

// readHostedState captures the installed hosted file set (SKILL.md plus the
// four supporting directories) for equality checks and rollback. Unreadable
// walk entries are skipped; an unreadable hosted file fails the capture so
// callers never upgrade on top of state they could not back up.
func readHostedState(dir string) (map[string][]byte, error) {
	head, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{"SKILL.md": head}
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || path == dir {
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
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		slash := filepath.ToSlash(rel)
		first := strings.SplitN(slash, "/", 2)[0]
		if first != "references" && first != "templates" && first != "scripts" && first != "assets" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		out[slash] = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func hostedSetsEqual(installed map[string][]byte, snapshot map[string]string) bool {
	if len(installed) != len(snapshot) {
		return false
	}
	for path, want := range snapshot {
		got, ok := installed[path]
		if !ok || !bytes.Equal(got, []byte(want)) {
			return false
		}
	}
	return true
}

// applyHostedState overwrites/creates the snapshot's hosted files and deletes
// installed hosted files the snapshot no longer contains. Non-hosted extras
// (anything outside SKILL.md and the four roots) are left untouched.
func applyHostedState(dir string, hosted map[string]string) error {
	for path, contents := range hosted {
		target := filepath.Join(dir, filepath.FromSlash(path))
		if err := validateUnderRoot(dir, target); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return fmt.Errorf("marketplace: create %s parent: %w", path, err)
		}
		if err := atomicWrite(target, []byte(contents), 0o600); err != nil {
			return fmt.Errorf("marketplace: write %s: %w", path, err)
		}
	}
	extra, err := readHostedState(dir)
	if err != nil {
		return fmt.Errorf("marketplace: scan installed skill: %w", err)
	}
	for path := range extra {
		if _, keep := hosted[path]; keep {
			continue
		}
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(path))); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("marketplace: remove stale %s: %w", path, err)
		}
	}
	for _, root := range []string{"references", "templates", "scripts", "assets"} {
		_ = os.Remove(filepath.Join(dir, root)) // succeeds only when empty
	}
	return nil
}

// restoreHostedState puts the pre-upgrade hosted set back after a failed
// upgrade: files the snapshot added are removed, previous contents are
// rewritten, and the previous manifest bytes are restored.
func restoreHostedState(dir string, state map[string][]byte, rawManifest []byte) {
	extra, err := readHostedState(dir)
	if err == nil {
		for path := range extra {
			if _, keep := state[path]; keep {
				continue
			}
			_ = os.Remove(filepath.Join(dir, filepath.FromSlash(path)))
		}
	}
	for path, data := range state {
		target := filepath.Join(dir, filepath.FromSlash(path))
		_ = os.MkdirAll(filepath.Dir(target), 0o700)
		_ = atomicWrite(target, data, 0o600)
	}
	if len(rawManifest) > 0 {
		_ = atomicWrite(filepath.Join(dir, marketplaceManifestName), rawManifest, 0o600)
	}
	for _, root := range []string{"references", "templates", "scripts", "assets"} {
		_ = os.Remove(filepath.Join(dir, root))
	}
}

func (s *MarketplaceService) viewFor(ctx context.Context, slug string) (tools.SkillView, error) {
	item, err := s.backend.loadSkill(ctx, slug)
	if err != nil {
		return tools.SkillView{}, err
	}
	return tools.SkillView{SkillSummary: s.backend.summary(item), Content: item.content,
		RelativePath: slug + "/SKILL.md", SupportingFiles: s.backend.supportingFiles(item.dir)}, nil
}

// CheckMarketplaceUpdate compares one installed skill against the
// marketplace snapshot named in its provenance manifest. Local content is
// the version source: equality is byte-level over the hosted file set.
func (s *MarketplaceService) CheckMarketplaceUpdate(ctx context.Context, name string) (tools.MarketplaceUpdateCheck, error) {
	name = strings.TrimSpace(name)
	if err := validSkillName(name); err != nil {
		return tools.MarketplaceUpdateCheck{}, fmt.Errorf("skills: %w", err)
	}
	check := tools.MarketplaceUpdateCheck{Name: name}
	dir := filepath.Join(s.backend.root, name)
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		check.Status = tools.MarketplaceUpdateNotInstalled
		return check, nil
	} else if err != nil {
		return check, err
	}
	manifest, _, err := readMarketplaceManifest(dir)
	if err != nil {
		check.Status = tools.MarketplaceUpdateUnmanaged
		return check, nil
	}
	check.MarketplaceID = manifest.MarketplaceID
	slug, snapshotHash, files, err := s.downloadSnapshot(ctx, manifest.MarketplaceID)
	if err != nil {
		return check, err
	}
	check.SnapshotHash = snapshotHash
	hosted, _, err := validateSnapshot(slug, files)
	if err != nil {
		return check, err
	}
	installed, err := readHostedState(dir)
	if err != nil {
		return check, fmt.Errorf("marketplace: read installed skill: %w", err)
	}
	if hostedSetsEqual(installed, hosted) {
		check.Status = tools.MarketplaceUpdateUpToDate
	} else {
		check.Status = tools.MarketplaceUpdateUpgradeAvailable
	}
	return check, nil
}

// normalizeSnapshotPath accepts clean relative slash paths only. Unlike the
// ZIP ingestion path there is no shared archive root to strip: the skills.sh
// snapshot is already flat.
func normalizeSnapshotPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.ContainsRune(path, '\x00') || strings.ContainsRune(path, '\\') ||
		filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", fmt.Errorf("marketplace: snapshot path %q is not a clean relative path", path)
	}
	clean := filepath.ToSlash(path)
	if strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("marketplace: snapshot path %q is not a clean relative path", path)
	}
	for _, part := range strings.Split(clean, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("marketplace: snapshot path %q is not a clean relative path", path)
		}
	}
	return clean, nil
}

func parseMarketplaceSkillID(id string) (string, string, string, error) {
	parts := strings.Split(id, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("marketplace: skill id must look like owner/repo/slug, got %q", id)
	}
	return parts[0], parts[1], parts[2], nil
}

func encodeMarketplaceSegment(segment string) (string, error) {
	if segment == "" || segment == "." || segment == ".." {
		return "", fmt.Errorf("marketplace: invalid path segment %q", segment)
	}
	for _, r := range segment {
		if r == '-' || r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		return "", fmt.Errorf("marketplace: invalid path segment %q", segment)
	}
	return segment, nil
}

func (s *MarketplaceService) get(ctx context.Context, op, endpoint string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("marketplace: build %s request: %w", op, err)
	}
	request.Header.Set("User-Agent", marketplaceUserAgent)
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("marketplace: %s request failed: %w", op, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxMarketplaceTotalBytes*2))
	if err != nil {
		return nil, fmt.Errorf("marketplace: %s response read failed: %w", op, err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, &MarketplaceUpstreamError{Op: op, Status: response.StatusCode, Detail: upstreamErrorDetail(body)}
	}
	return body, nil
}

func upstreamErrorDetail(body []byte) string {
	var parsed struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		if parsed.Error != "" {
			return parsed.Error
		}
		if parsed.Message != "" {
			return parsed.Message
		}
	}
	detail := strings.TrimSpace(string(body))
	if len(detail) > 200 {
		detail = detail[:safeUTF8Prefix(detail, 200)]
	}
	return detail
}

// The featured snapshot is committed YAML embedded at build time so the
// marketplace tab works with no runtime network access. Regenerate with
// scripts/fetch_marketplace_featured.py.
//
//go:embed marketplace_featured.yaml
var marketplaceFeaturedYAML string

var (
	featuredOnce sync.Once
	featuredData tools.MarketplaceFeatured
	featuredErr  error
)

func featuredSnapshot() (tools.MarketplaceFeatured, error) {
	featuredOnce.Do(func() {
		var snapshot struct {
			GeneratedAt string `yaml:"generated_at"`
			Source      string `yaml:"source"`
			Metric      string `yaml:"metric"`
			Skills      []struct {
				ID       string `yaml:"id"`
				Name     string `yaml:"name"`
				Source   string `yaml:"source"`
				Installs uint64 `yaml:"installs"`
			} `yaml:"skills"`
		}
		if err := yaml.Unmarshal([]byte(marketplaceFeaturedYAML), &snapshot); err != nil {
			featuredErr = fmt.Errorf("marketplace: embedded featured snapshot is invalid: %w", err)
			return
		}
		if len(snapshot.Skills) == 0 {
			featuredErr = errors.New("marketplace: embedded featured snapshot is empty")
			return
		}
		skills := make([]tools.MarketplaceSkill, 0, len(snapshot.Skills))
		for _, skill := range snapshot.Skills {
			skills = append(skills, tools.MarketplaceSkill{ID: skill.ID, Name: skill.Name, Source: skill.Source, Installs: skill.Installs})
		}
		featuredData = tools.MarketplaceFeatured{GeneratedAt: snapshot.GeneratedAt, Source: snapshot.Source, Metric: snapshot.Metric, Skills: skills}
	})
	return featuredData, featuredErr
}
