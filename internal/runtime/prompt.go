package runtime

import (
	"fmt"
	"strings"
	"time"

	"agent-vivy/internal/domain"
)

// preamblePersona leads every run's context block; the static agent
// Instruction carries the same line at the engine level, and the preamble
// adds the per-run facts the Instruction cannot (date, tool set, notes).
const preamblePersona = "You are Vivy, a precise personal assistant running locally on the user's machine."

// composeRunPreamble assembles the per-run context block that leads the
// history feed (MA-2): persona + current date + tool guidance generated
// from the resolved ToolSpecs, plus an optional notes digest (filled by
// MA-3; empty for now). Pure text, deterministic for fixed inputs, and
// deliberately free of any engine import or environment access — the
// secret boundary (D-010, E3) holds by construction.
func composeRunPreamble(now time.Time, specs []domain.ToolSpec, notesDigest string) string {
	var b strings.Builder
	b.WriteString(preamblePersona)
	fmt.Fprintf(&b, "\nToday's date: %s.", now.Format("2006-01-02"))
	if len(specs) == 0 {
		b.WriteString("\nNo tools are available in this session.")
	} else {
		b.WriteString("\nAvailable tools:\n")
		for _, s := range specs {
			mode := "makes changes; requires the user's approval before running"
			if s.Readonly {
				mode = "read-only; runs automatically"
			}
			fmt.Fprintf(&b, "- %s: %s (%s)\n", s.Name, s.Description, mode)
		}
	}
	if notesDigest != "" {
		b.WriteString("\nRecent notes from the user's notebook:\n")
		b.WriteString(notesDigest)
	}
	return b.String()
}
