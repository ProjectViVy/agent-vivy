package module

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var localePattern = regexp.MustCompile(`^[a-z]{2,3}(?:-[a-z0-9]{2,8})*$`)

type I18N struct {
	Catalog       string   `json:"catalog" yaml:"catalog"`
	DefaultLocale string   `json:"default_locale" yaml:"default_locale"`
	Locales       []string `json:"locales" yaml:"locales"`
}

func (catalog I18N) Validate() error {
	if catalog.Catalog == "" || !filepath.IsLocal(catalog.Catalog) || filepath.Clean(catalog.Catalog) == "." {
		return fmt.Errorf("i18n.catalog must be a source-confined relative path: %q", catalog.Catalog)
	}
	if filepath.Ext(catalog.Catalog) != ".json" {
		return fmt.Errorf("i18n.catalog must name one JSON file: %q", catalog.Catalog)
	}
	if NormalizeLocale(catalog.DefaultLocale) != "en" {
		return fmt.Errorf("i18n.default_locale must be en")
	}
	seen := make(map[string]struct{}, len(catalog.Locales))
	for _, raw := range catalog.Locales {
		locale := NormalizeLocale(raw)
		if !localePattern.MatchString(locale) {
			return fmt.Errorf("invalid i18n locale %q", raw)
		}
		if _, exists := seen[locale]; exists {
			return fmt.Errorf("duplicate i18n locale %s", locale)
		}
		seen[locale] = struct{}{}
	}
	if _, exists := seen["en"]; !exists {
		return fmt.Errorf("i18n.locales must include en")
	}
	return nil
}

func NormalizeLocale(locale string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(locale), "_", "-"))
}

// Normalize returns a copy with canonical locale spelling and set order.
func (catalog I18N) Normalize() I18N {
	normalized := catalog
	normalized.DefaultLocale = NormalizeLocale(catalog.DefaultLocale)
	normalized.Locales = make([]string, len(catalog.Locales))
	for index, locale := range catalog.Locales {
		normalized.Locales[index] = NormalizeLocale(locale)
	}
	sort.Strings(normalized.Locales)
	return normalized
}
