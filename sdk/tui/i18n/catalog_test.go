package i18n

import (
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	corei18n "agent-vivy/internal/i18n"
)

var placeholderPattern = regexp.MustCompile(`\{\{([[:word:]]+)\}\}`)

func TestCatalogsHaveIdenticalKeysAndPlaceholders(t *testing.T) {
	if !reflect.DeepEqual(sortedKeys(englishCatalog), sortedKeys(chineseCatalog)) {
		t.Fatalf("catalog keys differ:\nEnglish: %q\nChinese: %q", sortedKeys(englishCatalog), sortedKeys(chineseCatalog))
	}
	for key, english := range englishCatalog {
		if !strings.HasPrefix(key, "vivy.tui.") {
			t.Errorf("catalog key %q is outside the stable vivy.tui namespace", key)
		}
		chinese := chineseCatalog[key]
		if !reflect.DeepEqual(placeholders(english), placeholders(chinese)) {
			t.Errorf("placeholder mismatch for %q: English %q, Chinese %q", key, placeholders(english), placeholders(chinese))
		}
	}
}

func TestTranslatorFallsBackToEnglishThenKey(t *testing.T) {
	translator := New(corei18n.Locale("unsupported"))
	if got := translator.T("vivy.tui.help.heading.commands", nil); got != "Commands" {
		t.Fatalf("English fallback = %q, want Commands", got)
	}
	if got := translator.T("vivy.tui.missing", nil); got != "vivy.tui.missing" {
		t.Fatalf("diagnostic fallback = %q, want key", got)
	}
}

func TestTranslatorInterpolatesNamedArguments(t *testing.T) {
	translator := New(corei18n.Chinese)
	if translator.Locale() != corei18n.Chinese {
		t.Fatalf("Locale() = %q, want zh", translator.Locale())
	}
	if got := translator.T("vivy.tui.help.aliases", map[string]any{"aliases": "/?, /commands"}); got != "（别名：/?, /commands）" {
		t.Fatalf("interpolation = %q", got)
	}
	if got := translator.T("vivy.tui.help.aliases", nil); got != "（别名：{{aliases}}）" {
		t.Fatalf("missing argument changed placeholder: %q", got)
	}
}

func sortedKeys(catalog map[string]string) []string {
	keys := make([]string, 0, len(catalog))
	for key := range catalog {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func placeholders(message string) []string {
	matches := placeholderPattern.FindAllStringSubmatch(message, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}
	sort.Strings(out)
	return out
}
