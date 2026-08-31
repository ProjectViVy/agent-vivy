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
	static := composeStaticInstruction()
	got := composeRunPreamble(now, "", preambleSpecs(), domain.FaceWeb)

	if !strings.HasPrefix(static, preamblePersona) {
		t.Fatalf("static instruction must lead with the persona, got %q", static)
	}
	if !strings.Contains(got, "Today's date: 2026-08-09.") {
		t.Fatalf("preamble missing the formatted date: %q", got)
	}
	if !strings.Contains(got, "- echo_info: Echoes the given text back. (read-only; runs automatically)") {
		t.Fatalf("run preamble missing the readonly tool line: %q", got)
	}
	if !strings.Contains(got, "- write_note: Saves a note to the notebook. (makes changes; requires the user's approval before running)") {
		t.Fatalf("run preamble missing the effectful tool line: %q", got)
	}
	// Tool lines keep the resolved order.
	if strings.Index(got, "echo_info") > strings.Index(got, "write_note") {
		t.Fatalf("run preamble must list tools in resolved order: %q", got)
	}
	if strings.Contains(static, "echo_info") || strings.Contains(static, "write_note") {
		t.Fatalf("static instruction must not contain request-scoped tools: %q", static)
	}
	if strings.Contains(got, "Recent notes") {
		t.Fatalf("empty digest must not open the notes section: %q", got)
	}
}

func TestComposeRunPreambleNoTools(t *testing.T) {
	got := composeRunPreamble(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), "", nil, domain.FaceWeb)
	if !strings.Contains(got, "No tools are enabled for this request; answer without tool calls.") {
		t.Fatalf("run preamble missing the no-tools wording: %q", got)
	}
}

func TestComposeRunPreambleNotesDigest(t *testing.T) {
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	got := composeRunPreamble(now, "- 2026-01-02: code word is bluebird", nil, domain.FaceWeb)
	if !strings.Contains(got, "Recent notes from the user's notebook:\n- 2026-01-02: code word is bluebird") {
		t.Fatalf("preamble missing the notes digest section: %q", got)
	}
}

// Fixed inputs must yield byte-identical output: the feed is a product
// surface, not a source of run-to-run drift (FR-3 spirit).
func TestComposeRunPreambleDeterministic(t *testing.T) {
	now := time.Date(2026, 8, 9, 15, 4, 0, 0, time.UTC)
	if a, b := composeRunPreamble(now, "", nil, domain.FaceWeb), composeRunPreamble(now, "", nil, domain.FaceWeb); a != b {
		t.Fatalf("preamble not deterministic:\n%q\n%q", a, b)
	}
}

// The code face adds its coder framing to the preamble; the web face must
// not (the framing is face-scoped, not ambient).
func TestComposeRunPreambleFaceFraming(t *testing.T) {
	now := time.Date(2026, 8, 9, 15, 4, 0, 0, time.UTC)
	web := composeRunPreamble(now, "", preambleSpecs(), domain.FaceWeb)
	if strings.Contains(web, "Code mode is active") {
		t.Fatalf("web preamble must not carry the code framing: %q", web)
	}
	code := composeRunPreamble(now, "", preambleSpecs(), domain.FaceCode)
	if !strings.Contains(code, "Code mode is active") {
		t.Fatalf("code preamble missing the code framing: %q", code)
	}
	if !strings.Contains(code, "Never commit, push, or otherwise operate version control unless the user explicitly asks.") {
		t.Fatalf("code preamble missing the no-unrequested-commits rule: %q", code)
	}
	if !strings.Contains(code, "path:line") {
		t.Fatalf("code preamble missing the path:line reference rule: %q", code)
	}
	// The framing follows the date and leads the tool manifest.
	if strings.Index(code, "Today's date") > strings.Index(code, "Code mode is active") || strings.Index(code, "Code mode is active") > strings.Index(code, "- echo_info") {
		t.Fatalf("code framing must sit between the date and the tool manifest: %q", code)
	}
}

func TestStaticInstructionDoesNotContainRunFacts(t *testing.T) {
	static := composeStaticInstruction()
	if strings.Contains(static, "Today's date") || strings.Contains(static, "Recent notes") {
		t.Fatalf("static instruction contains dynamic run facts: %q", static)
	}
}
