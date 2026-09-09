package command

import (
	"bytes"
	"encoding/json"
	"strings"

	corei18n "agent-vivy/internal/i18n"
	tuii18n "agent-vivy/sdk/tui/i18n"
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
	return FormatResultWithTranslator(tuii18n.New(corei18n.English), name, raw)
}

// FormatResultWithTranslator localizes only client-owned scope labels. JSON
// fields, backend strings, and command identifiers remain protocol data.
func FormatResultWithTranslator(translator tuii18n.Translator, name string, raw []byte) string {
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
			return translator.T("vivy.tui.result.compactSkipped", nil) + "\n" + formatted
		}
	case "skills":
		if _, ok := value["skills"]; ok {
			return translator.T("vivy.tui.result.skillsCatalog", nil) + "\n" + formatted
		}
		return translator.T("vivy.tui.result.skillDetail", nil) + "\n" + formatted
	case "mcp":
		if _, ok := value["resources"]; ok {
			return translator.T("vivy.tui.result.mcpResources", nil) + "\n" + formatted
		}
		if _, ok := value["contents"]; ok {
			return translator.T("vivy.tui.result.mcpRead", nil) + "\n" + formatted
		}
		if _, ok := value["servers"]; ok {
			return translator.T("vivy.tui.result.mcpServers", nil) + "\n" + formatted
		}
		return translator.T("vivy.tui.result.mcpProbe", nil) + "\n" + formatted
	case "stats":
		period, _ := value["period"].(string)
		if period != "" {
			return translator.T("vivy.tui.result.statsPeriod", map[string]any{"period": period}) + "\n" + formatted
		}
		return translator.T("vivy.tui.result.stats", nil) + "\n" + formatted
	case "tools":
		return translator.T("vivy.tui.result.tools", nil) + "\n" + formatted
	case "files":
		return translator.T("vivy.tui.result.files", nil) + "\n" + formatted
	case "todos":
		return translator.T("vivy.tui.result.todos", nil) + "\n" + formatted
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
