package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const maxRenderedResultRunes = 32000

// FormatJSON renders a control-plane result for a command overlay or a line
// renderer. The JSON-RPC transport remains the source of truth; this helper
// only applies deterministic presentation and never invents fields such as
// cost, model, or connection status.
func FormatJSON(raw []byte) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ""
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return truncate(string(trimmed))
	}
	pretty, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return truncate(string(trimmed))
	}
	return truncate(string(pretty))
}

// FormatResult adds the command's data-scope label to a JSON-RPC snapshot.
// Labels are deliberately conservative: catalog/configured/probe are not
// presented as mounted/connected/current state. Compact's zero-value success
// is also normalized to an explicit not-needed outcome because the RPC
// contract uses a nil error for disabled or below-threshold compaction.
func FormatResult(name string, raw []byte) string {
	formatted := FormatJSON(raw)
	if formatted == "" {
		return formatted
	}
	var value map[string]any
	_ = json.Unmarshal(bytes.TrimSpace(raw), &value)
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "compact":
		var result struct {
			Before int  `json:"before_tokens"`
			After  int  `json:"after_tokens"`
			Folded int  `json:"folded_messages"`
			Skip   bool `json:"skipped"`
		}
		if json.Unmarshal(bytes.TrimSpace(raw), &result) == nil && (result.Skip || (result.Before == 0 && result.After == 0 && result.Folded == 0)) {
			return "compaction skipped (not-needed; no changes made)\n" + formatted
		}
	case "skills":
		if _, ok := value["skills"]; ok {
			return "skills catalog (catalog data; not session-mounted status)\n" + formatted
		}
		return "skill detail (catalog data)\n" + formatted
	case "mcp":
		if _, ok := value["servers"]; ok {
			return "configured MCP servers (configuration data; not live connection status)\n" + formatted
		}
		return "MCP server probe (configured server; probe status only)\n" + formatted
	case "stats":
		period, _ := value["period"].(string)
		if period != "" {
			return fmt.Sprintf("token usage aggregate (%s; no current-session cost)\n%s", period, formatted)
		}
		return "token usage aggregate (no current-session cost)\n" + formatted
	case "tools":
		return "tool catalog (registered/configured surface)\n" + formatted
	case "files":
		return "governed run workspace (run-scoped read-only view)\n" + formatted
	case "todos":
		return "active-session todos\n" + formatted
	}
	return formatted
}

func truncate(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= maxRenderedResultRunes {
		return string(runes)
	}
	return string(runes[:maxRenderedResultRunes-1]) + "…"
}
