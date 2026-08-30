package runtime

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func (s *MarketplaceService) InstallMarketplace(ctx context.Context, id string) (tools.MarketplaceInstallResult, error) {
	id = strings.TrimSpace(id)
	owner, repo, slug, err := parseMarketplaceSkillID(id)
	if err != nil {
		return tools.MarketplaceInstallResult{}, err
	}
	// Fail fast on slugs Vivy cannot host before spending a download.
	if err := validSkillName(slug); err != nil {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("marketplace: installable slug is not valid for Vivy: %w", err)
	}
	encodedOwner, err := encodeMarketplaceSegment(owner)
	if err != nil {
		return tools.MarketplaceInstallResult{}, err
	}
	encodedRepo, err := encodeMarketplaceSegment(repo)
	if err != nil {
		return tools.MarketplaceInstallResult{}, err
	}
	encodedSlug, err := encodeMarketplaceSegment(slug)
	if err != nil {
		return tools.MarketplaceInstallResult{}, err
	}
	endpoint := fmt.Sprintf("%s/api/download/%s/%s/%s", s.baseURL, encodedOwner, encodedRepo, encodedSlug)
	body, err := s.get(ctx, "download", endpoint)
	if err != nil {
		return tools.MarketplaceInstallResult{}, err
	}
	var snapshot struct {
		Files []struct {
			Path     string `json:"path"`
			Contents string `json:"contents"`
		} `json:"files"`
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("marketplace: download returned invalid JSON: %w", err)
	}
	files := make([]snapshotFile, 0, len(snapshot.Files))
	for _, file := range snapshot.Files {
		files = append(files, snapshotFile{path: file.Path, contents: file.Contents})
	}
	return s.installSnapshot(ctx, slug, files)
}

type snapshotFile struct {
	path     string
	contents string
}

// installSnapshot performs the create-only install. Every guard mirrors the
// disk loader (validSkillName, frontmatter name match, supporting-directory
// restriction, size caps, binary rejection) so an installed package is
// loadable on the very next turn without any engine restart.
func (s *MarketplaceService) installSnapshot(ctx context.Context, slug string, files []snapshotFile) (tools.MarketplaceInstallResult, error) {
	if len(files) == 0 {
		return tools.MarketplaceInstallResult{}, errors.New("marketplace: snapshot has no files")
	}
	if len(files) > maxSkillFiles {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("marketplace: snapshot exceeds %d files", maxSkillFiles)
	}
	var head []byte
	var supporting []snapshotFile
	var skipped []string
	total := 0
	sawHead := false
	for _, file := range files {
		total += len(file.contents)
		if total > maxMarketplaceTotalBytes {
			return tools.MarketplaceInstallResult{}, errors.New("marketplace: snapshot exceeds the total size limit")
		}
		path, err := normalizeSnapshotPath(file.path)
		if err != nil {
			return tools.MarketplaceInstallResult{}, err
		}
		if path == "SKILL.md" {
			if sawHead {
				return tools.MarketplaceInstallResult{}, errors.New("marketplace: snapshot has duplicate SKILL.md files")
			}
			sawHead = true
			head = []byte(file.contents)
			continue
		}
		first := strings.SplitN(path, "/", 2)[0]
		if first != "references" && first != "templates" && first != "scripts" && first != "assets" {
			skipped = append(skipped, path)
			continue
		}
		supporting = append(supporting, snapshotFile{path: path, contents: file.contents})
	}
	if !sawHead {
		return tools.MarketplaceInstallResult{}, errors.New("marketplace: snapshot has no root SKILL.md")
	}
	if isBinary(head) {
		return tools.MarketplaceInstallResult{}, errors.New("marketplace: SKILL.md is not UTF-8 text")
	}
	for _, file := range supporting {
		if len(file.contents) > maxSkillBytes {
			return tools.MarketplaceInstallResult{}, fmt.Errorf("marketplace: %s exceeds the size limit", file.path)
		}
		if isBinary([]byte(file.contents)) {
			return tools.MarketplaceInstallResult{}, fmt.Errorf("marketplace: %s is not UTF-8 text", file.path)
		}
	}
	front, _, _, err := parseSkillDocument(head)
	if err != nil {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("marketplace: snapshot SKILL.md is invalid: %w", err)
	}
	if strings.TrimSpace(front.Name) == "" {
		return tools.MarketplaceInstallResult{}, errors.New("marketplace: snapshot SKILL.md must declare a name matching the install slug")
	}
	if front.Name != slug {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("marketplace: snapshot name %q does not match slug %q", front.Name, slug)
	}

	dir := filepath.Join(s.backend.root, slug)
	if err := validateUnderRoot(s.backend.root, dir); err != nil {
		return tools.MarketplaceInstallResult{}, err
	}
	if _, err := os.Lstat(dir); err == nil {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("skills: skill %q already exists", slug)
	} else if !errors.Is(err, os.ErrNotExist) {
		return tools.MarketplaceInstallResult{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tools.MarketplaceInstallResult{}, fmt.Errorf("marketplace: create skill directory: %w", err)
	}
	rollback := func(cause error) (tools.MarketplaceInstallResult, error) {
		_ = os.RemoveAll(dir)
		return tools.MarketplaceInstallResult{}, cause
	}
	if err := atomicWrite(filepath.Join(dir, "SKILL.md"), head, 0o600); err != nil {
		return rollback(fmt.Errorf("marketplace: write SKILL.md: %w", err))
	}
	for _, file := range supporting {
		if err := ctx.Err(); err != nil {
			return rollback(err)
		}
		target := filepath.Join(dir, filepath.FromSlash(file.path))
		if err := validateUnderRoot(dir, target); err != nil {
			return rollback(err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return rollback(fmt.Errorf("marketplace: create %s parent: %w", file.path, err))
		}
		if err := atomicWrite(target, []byte(file.contents), 0o600); err != nil {
			return rollback(fmt.Errorf("marketplace: write %s: %w", file.path, err))
		}
	}
	item, err := s.backend.loadSkill(ctx, slug)
	if err != nil {
		return rollback(err)
	}
	view := tools.SkillView{SkillSummary: s.backend.summary(item), Content: item.content,
		RelativePath: slug + "/SKILL.md", SupportingFiles: s.backend.supportingFiles(item.dir)}
	return tools.MarketplaceInstallResult{Skill: view, SkippedFiles: skipped, Warnings: item.warnings}, nil
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
