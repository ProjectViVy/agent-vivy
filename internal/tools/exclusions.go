package tools

import "strings"

// IsBrowserUseName is the single production guardrail for browser automation.
// Browser Use is intentionally outside Vivy scope, including dynamically
// discovered MCP names; ordinary API-backed text search remains allowed.
func IsBrowserUseName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.NewReplacer("_", "", "-", "", " ", "").Replace(normalized)
	return strings.Contains(normalized, "browser") || strings.Contains(normalized, "playwright") || strings.Contains(normalized, "puppeteer")
}
