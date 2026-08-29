package runtime

import (
	"context"
	"encoding/json"
	"errors"
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
	result, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill")
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
	// Reinstall is create-only: the second attempt is a conflict.
	if _, err := service.InstallMarketplace(context.Background(), "vercel-labs/skills/demo-skill"); err == nil || !strings.Contains(err.Error(), "already exists") {
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
			if _, err := service.InstallMarketplace(context.Background(), testCase.id); err == nil || !strings.Contains(err.Error(), testCase.wantErr) {
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
		if _, err := service.InstallMarketplace(context.Background(), id); err == nil {
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
