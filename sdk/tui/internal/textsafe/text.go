// Package textsafe owns the shared sanitization of untrusted TUI display text.
// It must not be used to rewrite protocol errors or conversation data.
package textsafe

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

const maxMultilineRunes = 64 * 1024

// Inline applies the established inline display semantics: remove terminal
// sequences and unsafe controls, expand tabs, and flatten normalized newlines.
func Inline(text string) string {
	return strings.TrimSpace(strings.ReplaceAll(Multiline(text), "\n", " "))
}

// Multiline sanitizes display copy while retaining line breaks and readable
// text. Strip complete terminal sequences before applying the existing bound.
func Multiline(text string) string {
	text = strings.ReplaceAll(strings.ReplaceAll(ansi.Strip(text), "\r\n", "\n"), "\r", "\n")
	clean := make([]rune, 0, min(len([]rune(text)), maxMultilineRunes))
	for _, r := range text {
		if r == '\n' {
			clean = append(clean, r)
		} else if r == '\t' {
			clean = append(clean, ' ', ' ', ' ', ' ')
		} else if !unicode.IsControl(r) && !IsBidiControl(r) {
			clean = append(clean, r)
		}
		if len(clean) >= maxMultilineRunes {
			break
		}
	}
	return string(clean)
}

// IsBidiControl identifies the directional controls excluded by TUI display
// sanitizers without excluding ordinary Unicode text.
func IsBidiControl(r rune) bool {
	return r == '\u061c' || r == '\u200e' || r == '\u200f' || (r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069')
}
