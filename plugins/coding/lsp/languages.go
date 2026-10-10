package lsp

import (
	"path/filepath"
	"strings"
)

// language is one language-server command this plugin knows how to drive.
type language struct {
	Name    string
	Exts    []string
	Command string
	Args    []string
}

// languages are the default servers, matched by file extension. All are
// plain PATH names; servers must be installed by the user, and a missing
// one fails the tool call with the spawn error.
var languages = []language{
	{Name: "go", Exts: []string{".go"}, Command: "gopls", Args: []string{"serve"}},
	{Name: "typescript", Exts: []string{".ts", ".tsx"}, Command: "typescript-language-server", Args: []string{"--stdio"}},
	{Name: "javascript", Exts: []string{".js", ".jsx", ".mjs", ".cjs"}, Command: "typescript-language-server", Args: []string{"--stdio"}},
	{Name: "python", Exts: []string{".py"}, Command: "pyright-langserver", Args: []string{"--stdio"}},
	{Name: "rust", Exts: []string{".rs"}, Command: "rust-analyzer"},
}

func languageFor(p string) (language, bool) {
	ext := strings.ToLower(filepath.Ext(p))
	for _, lang := range languages {
		for _, have := range lang.Exts {
			if have == ext {
				return lang, true
			}
		}
	}
	return language{}, false
}
