package sdk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
)

const completeCatalogJSON = `{
  "apiVersion": "vivy.i18n/v1",
  "units": {
    "plugin.example/search-tools.results.count": {
      "description": "Number of search results",
      "placeholders": ["count"],
      "messages": {"en": "{{count}} results", "zh": "{{count}} 个结果"},
      "short": {"en": "{{count}} results", "zh": "{{count}} 项"}
    }
  }
}`

const catalogSourceHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func descriptorForCatalog(locales ...string) module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: "example/search-tools", Version: "1.0.0"},
		Source: module.Source{
			Ref:    "git:example/search-tools@0123456",
			SHA256: catalogSourceHash,
		},
		I18N:      &module.I18N{Catalog: "i18n/catalog.json", DefaultLocale: "en", Locales: locales},
		Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

func writeCatalog(t *testing.T, source string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "i18n")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "catalog.json"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLoadCatalogCanonicalDigestAndCompleteness(t *testing.T) {
	descriptor := descriptorForCatalog("en", "zh")
	first, err := loadCatalog(writeCatalog(t, completeCatalogJSON), descriptor)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	if first.State != catalogComplete {
		t.Fatalf("catalog state = %q, want %q", first.State, catalogComplete)
	}
	if first.Completeness["en"] != localeComplete || first.Completeness["zh"] != localeComplete {
		t.Fatalf("completeness = %#v", first.Completeness)
	}
	if len(first.Digest) != 64 {
		t.Fatalf("digest = %q, want lowercase SHA-256", first.Digest)
	}

	reordered := `{"units":{"plugin.example/search-tools.results.count":{"short":{"zh":"{{count}} 项","en":"{{count}} results"},"messages":{"zh":"{{count}} 个结果","en":"{{count}} results"},"placeholders":["count"],"description":"Number of search results"}},"apiVersion":"vivy.i18n/v1"}`
	second, err := loadCatalog(writeCatalog(t, reordered), descriptor)
	if err != nil {
		t.Fatalf("loadCatalog(reordered) error = %v", err)
	}
	if second.Digest != first.Digest || string(second.Canonical) != string(first.Canonical) {
		t.Fatalf("semantic formatting changed canonical identity\nfirst:  %s %s\nsecond: %s %s", first.Digest, first.Canonical, second.Digest, second.Canonical)
	}
}

func TestLoadCatalogAllowsVisibleIncompleteLocales(t *testing.T) {
	descriptor := descriptorForCatalog("en", "zh", "ja")
	source := strings.Replace(completeCatalogJSON, `, "zh": "{{count}} 个结果"`, "", 1)
	source = strings.Replace(source, `, "zh": "{{count}} 项"`, "", 1)

	compiled, err := loadCatalog(writeCatalog(t, source), descriptor)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	if compiled.State != catalogIncompleteLocale {
		t.Fatalf("catalog state = %q, want %q", compiled.State, catalogIncompleteLocale)
	}
	if compiled.Completeness["en"] != localeComplete || compiled.Completeness["zh"] != localeIncomplete || compiled.Completeness["ja"] != localeIncomplete {
		t.Fatalf("completeness = %#v", compiled.Completeness)
	}
}

func TestLoadCatalogRejectsInvalidSemanticInput(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "duplicate key",
			source:  strings.Replace(completeCatalogJSON, `"apiVersion": "vivy.i18n/v1",`, `"apiVersion": "vivy.i18n/v1", "apiVersion": "vivy.i18n/v1",`, 1),
			wantErr: "duplicate JSON key apiVersion",
		},
		{
			name:    "unknown field",
			source:  strings.Replace(completeCatalogJSON, `"description": "Number of search results",`, `"description": "Number of search results", "html": true,`, 1),
			wantErr: "unknown field",
		},
		{
			name:    "owner namespace",
			source:  strings.Replace(completeCatalogJSON, "plugin.example/search-tools", "vivy", 1),
			wantErr: "outside owner namespace plugin.example/search-tools.*",
		},
		{
			name:    "placeholder drift",
			source:  strings.Replace(completeCatalogJSON, "{{count}} 个结果", "{{total}} 个结果", 1),
			wantErr: "placeholder parity",
		},
		{
			name:    "placeholder syntax",
			source:  strings.Replace(completeCatalogJSON, "{{count}} 个结果", "{{ count }} 个结果", 1),
			wantErr: "invalid placeholder",
		},
		{
			name:    "missing english",
			source:  strings.Replace(completeCatalogJSON, `"en": "{{count}} results", `, "", 1),
			wantErr: "missing non-empty English message",
		},
		{
			name:    "message limit",
			source:  strings.Replace(completeCatalogJSON, "{{count}} results", "{{count}}"+strings.Repeat("x", maxCatalogMessageBytes), 1),
			wantErr: "exceeds 8192 bytes",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := loadCatalog(writeCatalog(t, test.source), descriptorForCatalog("en", "zh"))
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("loadCatalog() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestLoadCatalogRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(outside, []byte(completeCatalogJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "i18n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "i18n", "catalog.json")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	_, err := loadCatalog(root, descriptorForCatalog("en", "zh"))
	if err == nil || !strings.Contains(err.Error(), "escapes Module source") {
		t.Fatalf("loadCatalog() error = %v, want symlink escape rejection", err)
	}
}

func TestLoadCatalogNotApplicable(t *testing.T) {
	descriptor := descriptorForCatalog("en")
	descriptor.I18N = nil
	compiled, err := loadCatalog(t.TempDir(), descriptor)
	if err != nil {
		t.Fatalf("loadCatalog() error = %v", err)
	}
	if compiled.State != catalogNotApplicable || compiled.Digest != "" {
		t.Fatalf("catalog = %#v, want NOT_APPLICABLE without digest", compiled)
	}
}
