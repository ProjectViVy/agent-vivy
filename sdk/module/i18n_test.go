package module

import (
	"strings"
	"testing"
)

func TestDescriptorLocalizationRequirement(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Descriptor)
		wantError string
	}{
		{name: "backend only may omit catalog"},
		{
			name: "ui requires catalog",
			mutate: func(descriptor *Descriptor) {
				descriptor.Provides = []PortRef{{Port: "std/ui-extension@v1", ID: "example.search-ui"}}
			},
			wantError: "i18n is required for UI Module example/search-tools",
		},
		{
			name: "ui accepts canonical catalog",
			mutate: func(descriptor *Descriptor) {
				descriptor.Provides = []PortRef{{Port: "std/ui-root@v1", ID: "example.search-ui"}}
				descriptor.I18N = &I18N{Catalog: "i18n/catalog.json", DefaultLocale: "en", Locales: []string{"en", "zh", "ja"}}
			},
		},
		{
			name: "catalog stays source relative",
			mutate: func(descriptor *Descriptor) {
				descriptor.I18N = &I18N{Catalog: "../catalog.json", DefaultLocale: "en", Locales: []string{"en"}}
			},
			wantError: "i18n.catalog must be a source-confined relative path",
		},
		{
			name: "default is english",
			mutate: func(descriptor *Descriptor) {
				descriptor.I18N = &I18N{Catalog: "i18n/catalog.json", DefaultLocale: "zh", Locales: []string{"en", "zh"}}
			},
			wantError: "i18n.default_locale must be en",
		},
		{
			name: "packaged locales include english",
			mutate: func(descriptor *Descriptor) {
				descriptor.I18N = &I18N{Catalog: "i18n/catalog.json", DefaultLocale: "en", Locales: []string{"zh"}}
			},
			wantError: "i18n.locales must include en",
		},
		{
			name: "normalized locales are unique",
			mutate: func(descriptor *Descriptor) {
				descriptor.I18N = &I18N{Catalog: "i18n/catalog.json", DefaultLocale: "en", Locales: []string{"en", "ZH", "zh"}}
			},
			wantError: "duplicate i18n locale zh",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			descriptor := validDescriptor()
			if test.mutate != nil {
				test.mutate(&descriptor)
			}
			err := descriptor.Validate()
			if test.wantError == "" && err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
				t.Fatalf("Validate() error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}
