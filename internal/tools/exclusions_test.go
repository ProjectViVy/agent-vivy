package tools

import (
	"strings"
	"testing"
)

func TestBrowserUseNamesAreExcludedFromProductionRegistry(t *testing.T) {
	for _, name := range []string{"browser_use", "browseruse", "playwright", "puppeteer"} {
		if !IsBrowserUseName(name) {
			t.Errorf("%q was not classified as browser automation", name)
		}
	}
	if IsBrowserUseName("network_search") {
		t.Fatal("network search must remain available")
	}
	registry := NewRegistry(NewEchoInfo())
	if _, err := registry.Resolve([]string{"browser_use"}); err == nil || !strings.Contains(err.Error(), "excluded") {
		t.Fatalf("resolve error = %v", err)
	}
}
