package view

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// OSC 8 hyperlinks (VCP-G3): bare URLs in rendered transcript lines become
// clickable in supporting terminals; terminals that ignore the sequence show
// the plain text. Clipboard copy (OSC 52) already exists as /copy — see
// copyLastAssistant in tree.go; the copy_last action routes there.

// bareURLPattern matches URLs in already-rendered lines. ANSI styling may sit
// inside the visible text, so the URI is re-derived by stripping escapes from
// the matched run rather than taking it verbatim.
var bareURLPattern = regexp.MustCompile(`https?://[^\s\x1b<>"']+`)

func linkifyOSC8(line string) string {
	if !strings.Contains(line, "http") {
		return line
	}
	return bareURLPattern.ReplaceAllStringFunc(line, func(match string) string {
		display := match
		// A trailing close paren/period/comma nearly always belongs to the
		// prose around the link, not the URL itself.
		trim := len(display)
		for trim > 0 && strings.ContainsRune(").,;", rune(display[trim-1])) {
			trim--
		}
		uri := ansi.Strip(display[:trim])
		if uri == "" || !strings.HasPrefix(uri, "http") {
			return match
		}
		return "\x1b]8;;" + uri + "\x07" + display + "\x1b]8;;\x07"
	})
}
