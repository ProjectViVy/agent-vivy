package runtime

import (
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
)

// preambleSpecs mirrors the V0 shipped tool set: one read-only
// auto-execute tool and one effectful approval-gated tool.
func preambleSpecs() []domain.ToolSpec {
	return []domain.ToolSpec{
		{Name: "echo_info", Description: "Echoes the given text back.", Readonly: true},
		{Name: "write_note", Description: "Saves a note to the notebook.", Readonly: false},
	}
}

func TestComposeRunPreambleShape(t *testing.T) {
	now := time.Date(2026, 8, 9, 15, 4, 0, 0, time.UTC)
	got := composeRunPreamble(now, preambleSpecs(), "")

	if !strings.HasPrefix(got, preamblePersona) {
		t.Fatalf("preamble must lead with the persona, got %q", got)
	}
	if !strings.Contains(got, "Today's date: 2026-08-09.") {
		t.Fatalf("preamble missing the formatted date: %q", got)
	}
	if !strings.Contains(got, "- echo_info: Echoes the given text back. (read-only; runs automatically)") {
		t.Fatalf("preamble missing the readonly tool line: %q", got)
	}
	if !strings.Contains(got, "- write_note: Saves a note to the notebook. (makes changes; requires the user's approval before running)") {
		t.Fatalf("preamble missing the effectful tool line: %q", got)
	}
	// Tool lines keep the resolved order.
	if strings.Index(got, "echo_info") > strings.Index(got, "write_note") {
		t.Fatalf("preamble must list tools in resolved order: %q", got)
	}
	if strings.Contains(got, "Recent notes") {
		t.Fatalf("empty digest must not open the notes section: %q", got)
	}
}

func TestComposeRunPreambleNoTools(t *testing.T) {
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	got := composeRunPreamble(now, nil, "")
	if !strings.Contains(got, "No tools are available in this session.") {
		t.Fatalf("preamble missing the no-tools wording: %q", got)
	}
}

func TestComposeRunPreambleNotesDigest(t *testing.T) {
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	got := composeRunPreamble(now, preambleSpecs(), "- 2026-01-02: code word is bluebird")
	if !strings.Contains(got, "Recent notes from the user's notebook:\n- 2026-01-02: code word is bluebird") {
		t.Fatalf("preamble missing the notes digest section: %q", got)
	}
}

// Fixed inputs must yield byte-identical output: the feed is a product
// surface, not a source of run-to-run drift (FR-3 spirit).
func TestComposeRunPreambleDeterministic(t *testing.T) {
	now := time.Date(2026, 8, 9, 15, 4, 0, 0, time.UTC)
	if a, b := composeRunPreamble(now, preambleSpecs(), ""), composeRunPreamble(now, preambleSpecs(), ""); a != b {
		t.Fatalf("preamble not deterministic:\n%q\n%q", a, b)
	}
}
