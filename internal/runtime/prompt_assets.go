package runtime

import (
	"embed"
	"io/fs"
	"strings"
)

// Runtime-owned Markdown is embedded with the runtime so the optional mask
// module never becomes a prompt dependency. User supplied mask Markdown is
// inserted later as bounded JSON data, never parsed as a template.
//
//go:embed prompts/*.md
var promptAssets embed.FS

func promptAsset(name string) string {
	value, err := fs.ReadFile(promptAssets, "prompts/"+name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(value))
}
