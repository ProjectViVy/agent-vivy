package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/tools"
)

func newMarketplaceTestService(t *testing.T, handler http.HandlerFunc) (*MarketplaceService, *EinoSkillBackend, *httptest.Server) {
	t.Helper()
	backend, _, _ := openSkillTestBackend(t)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	service, err := NewMarketplaceService(backend, server.URL)
	if err != nil {
		t.Fatalf("new marketplace service: %v", err)
	}
	return service, backend, server
}

const marketplaceSkillDoc = "---\nname: demo-skill\ndescription: A marketplace skill\n---\n\nUse this carefully.\n"

func TestMarketplaceSearchMapsDirectoryRows(t *testing.T) {
	service, _, _ := newMarketplaceTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" || r.URL.Query().Get("q") != "commit" || r.URL.Query().Get("limit") != "20" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"skills": []map[string]any{
			{"id": "juliusbrussee/caveman/caveman-commit", "name": "caveman-commit", "source": "juliusbrussee/caveman", "installs": 311998},
		}})
	})
	skills, err := service.SearchMarketplace(context.Background(), "commit", 0)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(skills) != 1 || skills[0].ID != "juliusbrussee/caveman/caveman-commit" || skills[0].Installs != 311998 {
		t.Fatalf("skills = %+v", skills)
	}
}

func TestMarketplaceSearchClampsLimitAndRejectsShortQuery(t *testing.T) {
	var seenLimit string
	service, _, _ := newMarketplaceTestService(t, func(w http.ResponseWriter, r *http.Request) {
		seenLimit = r.URL.Query().Get("limit")
		_, _ = w.Write([]byte(`{"skills":[]}`))
	})
	if _, err := service.SearchMarketplace(context.Background(), "x", 0); err == nil || !strings.Contains(err.Error(), "at least 2 characters") {
		t.Fatalf("short query error = %v", err)
	}
	if _, err := service.SearchMarketplace(context.Background(), "commit", 9999); err != nil {
		t.Fatalf("search: %v", err)
	}
	if seenLimit != "50" {
		t.Fatalf("limit = %q, want clamped 50", seenLimit)
	}
}

func TestMarketplaceInstallWritesLoaderValidSkill(t *testing.T) {
	service, backend, _ := newMarketplaceTestService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/download/vercel-labs/skills/demo-skill" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"files": []map[string]string{
				{"path": "SKILL.md", "contents": marketplaceSkillDoc},
				{"path": "references/guide.md", "contents": "guide body"},
				{"path": "README.md", "contents": "not hosted by vivy"},
			},
			"hash": "abc123",
		})
	})
	result, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill", "")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if result.Skill.Name != "demo-skill" || result.Skill.Enabled != true || result.Skill.Content == "" {
		t.Fatalf("installed skill = %+v", result.Skill)
	}
	if len(result.SkippedFiles) != 1 || result.SkippedFiles[0] != "README.md" {
		t.Fatalf("skipped = %+v", result.SkippedFiles)
	}
	if len(result.Skill.SupportingFiles) != 1 || result.Skill.SupportingFiles[0] != "references/guide.md" {
		t.Fatalf("supporting = %+v", result.Skill.SupportingFiles)
	}
	if _, err := os.Stat(filepath.Join(backend.root, "demo-skill", "references", "guide.md")); err != nil {
		t.Fatalf("installed file missing: %v", err)
	}
	items, err := backend.ListSkills(context.Background(), "")
	if err != nil || len(items) != 1 || items[0].Name != "demo-skill" {
		t.Fatalf("ListSkills after install = %+v, err %v", items, err)
	}
	// Reinstall defaults to create: the second attempt is a conflict.
	if _, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill", ""); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("reinstall error = %v", err)
	}
}

func TestMarketplaceInstallRejectsInvalidSnapshots(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		files   []map[string]string
		wantErr string
	}{
		{"name mismatch", "owner/repo/other-slug", []map[string]string{{"path": "SKILL.md", "contents": marketplaceSkillDoc}}, "does not match slug"},
		{"missing frontmatter name", "owner/repo/demo-skill", []map[string]string{{"path": "SKILL.md", "contents": "---\ndescription: no name\n---\nbody\n"}}, "must declare a name"},
		{"no root skill", "owner/repo/demo-skill", []map[string]string{{"path": "references/x.md", "contents": "x"}}, "no root SKILL.md"},
		{"binary head", "owner/repo/demo-skill", []map[string]string{{"path": "SKILL.md", "contents": "\x00\x01binary"}}, "not UTF-8"},
		{"traversal path", "owner/repo/demo-skill", []map[string]string{{"path": "SKILL.md", "contents": marketplaceSkillDoc}, {"path": "../escape.md", "contents": "x"}}, "clean relative path"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service, _, _ := newMarketplaceTestService(t, func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{"files": testCase.files, "hash": "h"})
			})
			if _, err := service.InstallMarketplace(context.Background(), testCase.id, ""); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
				t.Fatalf("install error = %v, want %q", err, testCase.wantErr)
			}
		})
	}
}

func TestMarketplaceInstallRejectsIDsVivyCannotHost(t *testing.T) {
	service, _, _ := newMarketplaceTestService(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream must not be called for invalid ids")
	})
	for _, id := range []string{"owner/repo", "owner/repo/slug/extra", "/repo/slug", "owner/repo/with.dot", "../../etc/passwd"} {
		if _, err := service.InstallMarketplace(context.Background(), id, ""); err == nil {
			t.Fatalf("id %q must be rejected", id)
		}
	}
}

func TestMarketplaceUpstreamErrorCarriesDetail(t *testing.T) {
	service, _, _ := newMarketplaceTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Query must be at least 2 characters"}`))
	})
	_, err := service.SearchMarketplace(context.Background(), "valid-query", 0)
	var upstream *MarketplaceUpstreamError
	if !errors.As(err, &upstream) {
		t.Fatalf("want MarketplaceUpstreamError, got %v", err)
	}
	if upstream.Status != http.StatusBadRequest || upstream.Detail != "Query must be at least 2 characters" {
		t.Fatalf("upstream = %+v", upstream)
	}
}

func TestMarketplaceServiceRejectsInvalidBaseURL(t *testing.T) {
	backend, _, _ := openSkillTestBackend(t)
	if _, err := NewMarketplaceService(backend, "not-a-url"); err == nil {
		t.Fatal("want invalid base url error")
	}
	if _, err := NewMarketplaceService(nil, ""); err == nil {
		t.Fatal("want missing backend error")
	}
}

func TestMarketplaceEnvOverrideWins(t *testing.T) {
	t.Setenv(MarketplaceBaseURLOverride, "http://127.0.0.1:9/override")
	backend, _, _ := openSkillTestBackend(t)
	service, err := NewMarketplaceService(backend, "")
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	if service.baseURL != "http://127.0.0.1:9/override" {
		t.Fatalf("baseURL = %q", service.baseURL)
	}
}

func TestFeaturedSnapshotParses(t *testing.T) {
	featured, err := featuredSnapshot()
	if err != nil {
		t.Fatalf("featured snapshot: %v", err)
	}
	if featured.GeneratedAt == "" || featured.Source == "" || len(featured.Skills) == 0 {
		t.Fatalf("featured = %+v", featured)
	}
	for _, skill := range featured.Skills {
		if _, _, _, err := parseMarketplaceSkillID(skill.ID); err != nil {
			t.Fatalf("bad featured id %q: %v", skill.ID, err)
		}
		if skill.Name == "" {
			t.Fatalf("featured skill %+v has no name", skill)
		}
	}
}

func TestMarketplaceFeaturedImplementsControlSurface(t *testing.T) {
	var _ tools.SkillsMarketplace = (*MarketplaceService)(nil)
}

func marketplaceSnapshotHandler(version int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/download/vercel-labs/skills/demo-skill" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		doc := fmt.Sprintf("---\nname: demo-skill\ndescription: Marketplace skill v%d\n---\n\nBody v%d.\n", version, version)
		files := []map[string]string{{"path": "SKILL.md", "contents": doc}}
		if version == 1 {
			files = append(files, map[string]string{"path": "references/guide.md", "contents": "guide v1"})
		} else {
			files = append(files, map[string]string{"path": "references/extra.md", "contents": "extra v2"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"files": files, "hash": fmt.Sprintf("hash-v%d", version)})
	}
}

func readSkillFile(t *testing.T, backend *EinoSkillBackend, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{backend.root, "demo-skill"}, parts...)...))
	if err != nil {
		t.Fatalf("read %v: %v", parts, err)
	}
	return string(data)
}

func TestMarketplaceUpgradeMirrorsSnapshot(t *testing.T) {
	version := 1
	service, backend, _ := newMarketplaceTestService(t, func(w http.ResponseWriter, r *http.Request) {
		marketplaceSnapshotHandler(version)(w, r)
	})
	created, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill", "")
	if err != nil || created.Outcome != "created" {
		t.Fatalf("create outcome = %q err %v", created.Outcome, err)
	}
	// Hand-placed extras: a hosted file the snapshot drops and a root file
	// outside Vivy's hosted set that must survive the upgrade.
	if err := os.WriteFile(filepath.Join(backend.root, "demo-skill", "references", "user-notes.md"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backend.root, "demo-skill", "NOTES.md"), []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	version = 2
	upgraded, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill", "upgrade")
	if err != nil || upgraded.Outcome != "upgraded" {
		t.Fatalf("upgrade outcome = %q err %v", upgraded.Outcome, err)
	}
	if readSkillFile(t, backend, "SKILL.md") != "---\nname: demo-skill\ndescription: Marketplace skill v2\n---\n\nBody v2.\n" {
		t.Fatalf("SKILL.md not swapped: %q", readSkillFile(t, backend, "SKILL.md"))
	}
	if readSkillFile(t, backend, "references", "extra.md") != "extra v2" {
		t.Fatal("added snapshot file missing")
	}
	if _, err := os.Stat(filepath.Join(backend.root, "demo-skill", "references", "guide.md")); !os.IsNotExist(err) {
		t.Fatalf("dropped snapshot file still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backend.root, "demo-skill", "references", "user-notes.md")); !os.IsNotExist(err) {
		t.Fatal("stale hosted file survived the upgrade")
	}
	if readSkillFile(t, backend, "NOTES.md") != "keep me" {
		t.Fatal("root extra file must survive an upgrade")
	}
	var manifest struct {
		MarketplaceID string `json:"marketplace_id"`
		SnapshotHash  string `json:"snapshot_hash"`
		InstalledAt   string `json:"installed_at"`
		UpgradedAt    string `json:"upgraded_at"`
	}
	data, err := os.ReadFile(filepath.Join(backend.root, "demo-skill", ".vivy-skill.json"))
	if err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.MarketplaceID != "vercel-labs/skills/demo-skill" || manifest.SnapshotHash != "hash-v2" ||
		manifest.InstalledAt == "" || manifest.UpgradedAt == "" {
		t.Fatalf("manifest after upgrade = %+v", manifest)
	}
	// Create mode still refuses to touch the existing skill.
	if _, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill", ""); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("create-over-existing error = %v", err)
	}
	items, err := backend.ListSkills(context.Background(), "")
	if err != nil || len(items) != 1 || items[0].Name != "demo-skill" {
		t.Fatalf("ListSkills after upgrade = %+v, err %v", items, err)
	}
}

func TestMarketplaceUpgradeUpToDateWritesNothing(t *testing.T) {
	service, backend, _ := newMarketplaceTestService(t, marketplaceSnapshotHandler(1))
	if _, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill", ""); err != nil {
		t.Fatalf("install: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(backend.root, "demo-skill", ".vivy-skill.json"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill", "upgrade")
	if err != nil || result.Outcome != "up_to_date" {
		t.Fatalf("outcome = %q err %v", result.Outcome, err)
	}
	after, err := os.ReadFile(filepath.Join(backend.root, "demo-skill", ".vivy-skill.json"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("up_to_date must leave the manifest untouched (equal=%v err=%v)", bytes.Equal(before, after), err)
	}
	if _, err := os.Stat(filepath.Join(backend.root, "demo-skill", "references", "guide.md")); err != nil {
		t.Fatalf("content must be preserved: %v", err)
	}
}

func TestMarketplaceUpgradeGuards(t *testing.T) {
	// Unknown install mode.
	service, backend, _ := newMarketplaceTestService(t, marketplaceSnapshotHandler(1))
	if _, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill", "replace"); err == nil || !strings.Contains(err.Error(), "unknown install mode") {
		t.Fatalf("mode error = %v", err)
	}
	// Hand-placed skill has no manifest: upgrade refuses.
	if err := os.MkdirAll(filepath.Join(backend.root, "demo-skill"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backend.root, "demo-skill", "SKILL.md"), []byte(marketplaceSkillDoc), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill", "upgrade"); err == nil || !strings.Contains(err.Error(), "not marketplace-managed") {
		t.Fatalf("unmanaged upgrade error = %v", err)
	}
	// Manifest from another origin: id mismatch refuses.
	manifest := `{"marketplace_id":"other/skills/demo-skill","installed_at":"2026-01-01T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(backend.root, "demo-skill", ".vivy-skill.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill", "upgrade"); err == nil || !strings.Contains(err.Error(), "was installed from") {
		t.Fatalf("id mismatch error = %v", err)
	}
}

func TestMarketplaceCheckUpdateStatuses(t *testing.T) {
	version := 1
	service, backend, _ := newMarketplaceTestService(t, func(w http.ResponseWriter, r *http.Request) {
		marketplaceSnapshotHandler(version)(w, r)
	})
	ghost, err := service.CheckMarketplaceUpdate(context.Background(), "ghost-skill")
	if err != nil || ghost.Status != tools.MarketplaceUpdateNotInstalled {
		t.Fatalf("not_installed check = %+v err %v", ghost, err)
	}
	if _, err := service.CheckMarketplaceUpdate(context.Background(), "BAD NAME"); err == nil {
		t.Fatal("invalid name must be rejected")
	}
	if err := os.MkdirAll(filepath.Join(backend.root, "demo-skill"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backend.root, "demo-skill", "SKILL.md"), []byte(marketplaceSkillDoc), 0o600); err != nil {
		t.Fatal(err)
	}
	unmanaged, err := service.CheckMarketplaceUpdate(context.Background(), "demo-skill")
	if err != nil || unmanaged.Status != tools.MarketplaceUpdateUnmanaged {
		t.Fatalf("unmanaged check = %+v err %v", unmanaged, err)
	}
	if _, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill", ""); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("install over hand-placed skill error = %v", err)
	}
	if err := os.RemoveAll(filepath.Join(backend.root, "demo-skill")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill", ""); err != nil {
		t.Fatalf("install: %v", err)
	}
	upToDate, err := service.CheckMarketplaceUpdate(context.Background(), "demo-skill")
	if err != nil || upToDate.Status != tools.MarketplaceUpdateUpToDate || upToDate.SnapshotHash != "hash-v1" ||
		upToDate.MarketplaceID != "vercel-labs/skills/demo-skill" {
		t.Fatalf("up_to_date check = %+v err %v", upToDate, err)
	}
	version = 2
	available, err := service.CheckMarketplaceUpdate(context.Background(), "demo-skill")
	if err != nil || available.Status != tools.MarketplaceUpdateUpgradeAvailable {
		t.Fatalf("upgrade_available check = %+v err %v", available, err)
	}
}
