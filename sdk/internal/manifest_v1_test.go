package sdk

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validDescriptorYAML = `apiVersion: vivy.module/v1
module:
  id: fixture/minimal
  version: 1.0.0
source:
  ref: git:fixture/minimal@0123456
  sha256: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
provides: []
requires: []
requestedGrants: []
lifecycle:
  scope: generation
`

func TestLoadDescriptorRejectsEveryLegacyFixture(t *testing.T) {
	dir := t.TempDir()
	legacy := "vivy.plugin/" + "v0"
	if err := os.WriteFile(filepath.Join(dir, "vivy-plugin.json"), []byte(`{"apiVersion":"`+legacy+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := loadDescriptor(dir)
	if err == nil || !strings.Contains(err.Error(), "unsupported apiVersion "+legacy) {
		t.Fatalf("loadDescriptor() error = %v, want hard v0 rejection", err)
	}
}

func TestLoadDescriptorRejectsDuplicateAndUnknownFields(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "duplicate key",
			source:  validDescriptorYAML + "apiVersion: vivy.module/v1\n",
			wantErr: "duplicate",
		},
		{
			name:    "unknown semantic field",
			source:  validDescriptorYAML + "trust: internal\n",
			wantErr: "field trust not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "vivy-module.yaml"), []byte(test.source), 0o600); err != nil {
				t.Fatal(err)
			}
			_, _, err := loadDescriptor(dir)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(test.wantErr)) {
				t.Fatalf("loadDescriptor() error = %v, want substring %q", err, test.wantErr)
			}
		})
	}
}

func TestLoadDescriptorCanonicalizesSemanticInput(t *testing.T) {
	sources := []string{
		validDescriptorYAML,
		`lifecycle: {scope: generation}
requestedGrants: []
requires: []
provides: []
source: {sha256: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef, ref: "git:fixture/minimal@0123456"}
module: {version: 1.0.0, id: fixture/minimal}
apiVersion: vivy.module/v1
`,
	}

	var canonical []byte
	for index, source := range sources {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "vivy-module.yaml"), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
		descriptor, got, err := loadDescriptor(dir)
		if err != nil {
			t.Fatalf("loadDescriptor(%d) error = %v", index, err)
		}
		if descriptor.Module.ID != "fixture/minimal" {
			t.Fatalf("descriptor module id = %q", descriptor.Module.ID)
		}
		if index == 0 {
			canonical = got
			continue
		}
		if !bytes.Equal(got, canonical) {
			t.Fatalf("canonical bytes differ\nfirst: %s\nother: %s", canonical, got)
		}
	}
}

func TestLoadDescriptorCanonicalizesPackagedLocales(t *testing.T) {
	source := strings.Replace(validDescriptorYAML, "lifecycle:\n", "i18n:\n  catalog: i18n/catalog.json\n  default_locale: en\n  locales: [ZH, en, ja]\nlifecycle:\n", 1)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "vivy-module.yaml"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	descriptor, canonical, err := loadDescriptor(dir)
	if err != nil {
		t.Fatalf("loadDescriptor() error = %v", err)
	}
	if got := strings.Join(descriptor.I18N.Locales, ","); got != "en,ja,zh" {
		t.Fatalf("normalized locales = %q, want en,ja,zh", got)
	}
	if !bytes.Contains(canonical, []byte(`"locales":["en","ja","zh"]`)) {
		t.Fatalf("canonical bytes do not contain normalized locales: %s", canonical)
	}
}
