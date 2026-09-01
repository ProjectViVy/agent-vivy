package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/logging"
)

// ValidateArgsSafety blocks generic path/command hazards before a tool
// implementation can observe the arguments. Tools with no such fields are
// unaffected; domain-specific validation remains the tool's responsibility.
func ValidateArgsSafety(spec domain.ToolSpec, args json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(args, &fields); err != nil || fields == nil {
		return nil // ValidateArgs owns shape errors and runs before this guard.
	}
	for name, raw := range fields {
		field := strings.ToLower(strings.TrimSpace(name))
		if field != "path" && field != "filepath" && field != "file_path" && field != "command" && field != "cmd" {
			continue
		}
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			continue
		}
		if strings.ContainsRune(value, '\x00') {
			return &ArgError{Field: name, Reason: "contains a NUL byte"}
		}
		// The bash tool's whole contract is running shell syntax; its risk
		// is handled by the shell classifier and the approval tiers, not by
		// this shape-level guard.
		if (field == "command" || field == "cmd") && spec.Name != BashName {
			lower := strings.ToLower(value)
			for _, token := range []string{"&&", "||", ";", "|", "`", "$(", "powershell", "cmd.exe"} {
				if strings.Contains(lower, token) {
					return &ArgError{Field: name, Reason: fmt.Sprintf("contains blocked command syntax %q", token)}
				}
			}
		}
		if strings.Contains(value, "..\\") || strings.Contains(value, "../") || strings.HasPrefix(value, "\\\\") {
			return &ArgError{Field: name, Reason: "path traversal or UNC paths are not allowed"}
		}
	}
	return nil
}

// RedactSensitive removes common credential and email forms from tool data
// before the result enters model context or a durable tool.finished payload.
// The vocabulary is single-sourced in internal/logging so the handler-layer
// guard and this boundary redact with the same shapes and markers.
func RedactSensitive(text string) string {
	return logging.Redact(text)
}
