package runtime

import (
	"strings"
	"testing"
	"time"

	"agent-vivy/internal/domain"
)

func TestComposeRunPreambleShape(t *testing.T) {
	now := time.Date(2026, 8, 9, 15, 4, 0, 0, time.UTC)
	static := composeStaticInstruction()
	got := composeRunPreamble(now, "", true, domain.FaceWeb)

	if !strings.HasPrefix(static, preamblePersona) {
		t.Fatalf("static instruction must lead with the persona, got %q", static)
	}
	if !strings.Contains(got, "Today's date: 2026-08-09.") {
		t.Fatalf("preamble missing the formatted date: %q", got)
	}
	if strings.Contains(got, "Tools available") || strings.Contains(got, "echo_info") || strings.Contains(got, "write_note") || strings.Contains(got, "No tools are enabled") {
		t.Fatalf("run preamble must not repeat the full tool catalog: %q", got)
	}
	if strings.Contains(static, "echo_info") || strings.Contains(static, "write_note") {
		t.Fatalf("static instruction must not contain request-scoped tools: %q", static)
	}
	if strings.Contains(got, "Recent notes") {
		t.Fatalf("empty digest must not open the notes section: %q", got)
	}
}

func TestComposeRunPreambleNoTools(t *testing.T) {
	got := composeRunPreamble(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), "", false, domain.FaceWeb)
	if !strings.Contains(got, "No tools are enabled for this request; answer without tool calls.") {
		t.Fatalf("run preamble missing the no-tools wording: %q", got)
	}
}

func TestComposeRunPreambleNotesDigest(t *testing.T) {
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	got := composeRunPreamble(now, "- 2026-01-02: code word is bluebird", false, domain.FaceWeb)
	if !strings.Contains(got, "Recent notes from the user's notebook:\n- 2026-01-02: code word is bluebird") {
		t.Fatalf("preamble missing the notes digest section: %q", got)
	}
}

// Fixed inputs must yield byte-identical output: the feed is a product
// surface, not a source of run-to-run drift (FR-3 spirit).
func TestComposeRunPreambleDeterministic(t *testing.T) {
	now := time.Date(2026, 8, 9, 15, 4, 0, 0, time.UTC)
	if a, b := composeRunPreamble(now, "", false, domain.FaceWeb), composeRunPreamble(now, "", false, domain.FaceWeb); a != b {
		t.Fatalf("preamble not deterministic:\n%q\n%q", a, b)
	}
}

// The code face adds its coder framing to the preamble; the web face must
// not (the framing is face-scoped, not ambient).
func TestComposeRunPreambleFaceFraming(t *testing.T) {
	now := time.Date(2026, 8, 9, 15, 4, 0, 0, time.UTC)
	web := composeRunPreamble(now, "", true, domain.FaceWeb)
	if strings.Contains(web, "Code mode is active") {
		t.Fatalf("web preamble must not carry the code framing: %q", web)
	}
	code := composeRunPreamble(now, "", true, domain.FaceCode)
	if !strings.Contains(code, "Code mode is active") {
		t.Fatalf("code preamble missing the code framing: %q", code)
	}
	if !strings.Contains(code, "Never commit, push, or otherwise operate version control unless the user explicitly asks.") {
		t.Fatalf("code preamble missing the no-unrequested-commits rule: %q", code)
	}
	if !strings.Contains(code, "path:line") {
		t.Fatalf("code preamble missing the path:line reference rule: %q", code)
	}
	if strings.Index(code, "Today's date") > strings.Index(code, "Code mode is active") {
		t.Fatalf("code framing must follow the date: %q", code)
	}
}

func TestStaticInstructionDoesNotContainRunFacts(t *testing.T) {
	static := composeStaticInstruction()
	if strings.Contains(static, "Today's date") || strings.Contains(static, "Recent notes") {
		t.Fatalf("static instruction contains dynamic run facts: %q", static)
	}
	if strings.Contains(static, "tools listed in the current run context") || !strings.Contains(static, "Use only tools exposed by the runtime") {
		t.Fatalf("static instruction has stale tool-list wording: %q", static)
	}
}
