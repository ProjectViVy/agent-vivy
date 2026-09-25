package runtime

import (
	"fmt"
	"strings"
	"time"

	"agent-vivy/internal/domain"
)

// preamblePersona leads every run's context block; the static agent
// Instruction carries the same line at the engine level, and the preamble
// adds the per-run facts the Instruction cannot (date and notes).
const preamblePersona = "You are Vivy, a precise personal assistant running locally on the user's machine."

var planGuidanceText = promptAsset("plan.md")

// composeStaticInstruction assembles the cache-stable instruction prefix.
// It must not contain dates, session history, notes, or per-run tool
// manifests.
func composeStaticInstruction() string {
	return strings.Join([]string{promptAsset("persona-default.md"), promptAsset("runtime.md"), promptAsset("configuration.md")}, "\n\n")
}

// faceCodePreamble frames the code face in the per-run preamble. It
// carries the ratified coder rule-set blueprint (never operate version
// control unasked; path:line references); it states no tool claims — the
// base tool surface stays mainline-shared (D1).
const faceCodePreamble = "Code mode is active: work directly on the files in this run's workspace. Never commit, push, or otherwise operate version control unless the user explicitly asks. When pointing at code, reference locations as path:line when the location is known."

// composeRunPreamble assembles the dynamic run context that follows the
// cache-stable Engine instruction. It contains only per-run facts, the
// existence of an enabled tool surface, and the existing bounded Notes
// digest; it does not introduce a new memory source or repeat the tool
// catalog in the prompt.
func composeRunPreamble(now time.Time, notesDigest string, hasEnabledTools bool, face domain.Face, collaboration ...domain.CollaborationMode) string {
	var b strings.Builder
	softPlan := len(collaboration) > 0 && collaboration[0] == domain.CollaborationModePlan
	dateText := strings.ReplaceAll(promptAsset("dynamic-context.md"), "{{date}}", now.Format("2006-01-02"))
	if dateText == "" {
		fmt.Fprintf(&b, "Today's date: %s.", now.Format("2006-01-02"))
	} else {
		b.WriteString(dateText)
	}
	if face == domain.FaceCode {
		b.WriteString("\n" + promptAsset("code-mode.md"))
	}
	if softPlan {
		b.WriteString("\n" + planGuidanceText)
	}
	if !hasEnabledTools {
		// Defensive: an empty active set is a legal configuration
		// (chat-only mode via tools.enabled), not a routing outcome.
		b.WriteString("\nNo tools are enabled for this request; answer without tool calls.")
	}
	if notesDigest != "" {
		b.WriteString("\nRecent notes from the user's notebook:\n")
		b.WriteString(notesDigest)
	}
	return b.String()
}

// Digest bounds keep the preamble's notes section small regardless of
// notebook size (NFR: bounded); compaction strategies stay deferred.
const (
	digestNoteLimit = 5
	digestLineLimit = 80
)

// formatNotesDigest renders the most recent notes as bounded one-line
// entries for the preamble (MA-3). Input is expected newest-first (the
// NoteStore contract); only the first digestNoteLimit entries appear and
// each entry's first content line is truncated to digestLineLimit runes.
// The id stays visible so the model can address read_note.
func formatNotesDigest(notes []domain.Note) string {
	if len(notes) == 0 {
		return ""
	}
	shown := notes
	if len(shown) > digestNoteLimit {
		shown = shown[:digestNoteLimit]
	}
	var b strings.Builder
	for _, n := range shown {
		line := n.Content
		if i := strings.IndexByte(line, '\n'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if runes := []rune(line); len(runes) > digestLineLimit {
			line = string(runes[:digestLineLimit]) + "..."
		}
		fmt.Fprintf(&b, "- %s (%s): %s\n", n.ID,
			time.UnixMilli(n.CreatedAt).UTC().Format("2006-01-02"), line)
	}
	return strings.TrimRight(b.String(), "\n")
}

// Nudge reminder templates (NUDGE-DESIGN §7). The reminder names no tool
// arguments, diagnostics, or results — it carries only the repetition
// count and the fixed guidance. The refusal variant names the decision
// source and forbids bypass; the failure variant warns that a failed
// call may already have caused effects.
const (
	nudgeFailureTemplate = "Runtime reminder: this unsuccessful tool call has repeated %d times. Inspect the previous result, correct the arguments or choose another permitted approach. If blocked, report the blocker. A failed call may already have caused effects; inspect state before repeating a mutation."
	nudgeRefusalTemplate = "Runtime reminder: this refused call has repeated %d times. Respect the policy or user decision. Do not bypass it through another tool. Continue only within existing authorization, or report the blocker."
)

// renderNudge builds the fixed reminder for a sealed notice (§7). The
// template is selected by the sealed failure's status, not by text
// matching on the refusal reason.
func renderNudge(n nudgeNotice) string {
	if n.Status == toolFailureStatusRefused {
		return fmt.Sprintf(nudgeRefusalTemplate, n.Count)
	}
	return fmt.Sprintf(nudgeFailureTemplate, n.Count)
}
