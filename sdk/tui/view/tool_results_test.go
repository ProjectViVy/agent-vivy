package view

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"agent-vivy/sdk/tui/surface"
)

func TestToolResultContentClassification(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want toolResultKind
	}{
		{"json object", `{"a":1}`, toolResultJSON},
		{"json array", `[1, 2]`, toolResultJSON},
		{"invalid json", `{"a":1`, toolResultPlain},
		{"unified diff", "--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new", toolResultDiff},
		{"git diff", "diff --git a/main.go b/main.go\n--- a/main.go\n+++ b/main.go\n@@ -1,2 +1,3 @@\n+import \"fmt\"", toolResultDiff},
		{"hunk only", "@@ -1 +1 @@\n-old\n+new", toolResultDiff},
		{"markdown list", "- item one\n- item two", toolResultMarkdown},
		{"markdown heading and list", "## Summary\n\n- item one\n- item two", toolResultMarkdown},
		{"markdown heading", "## Summary", toolResultMarkdown},
		{"plain output", "total 12\ndrwxr-xr-x src", toolResultPlain},
	} {
		kind, _ := toolResultContent(tc.body)
		if kind != tc.want {
			t.Fatalf("%s: kind = %d, want %d", tc.name, kind, tc.want)
		}
	}
}

func TestToolResultJSONIsReindentedBytePreserving(t *testing.T) {
	body := `{"id":12345678901234567890,"nested":{"x":true}}`
	kind, content := toolResultContent(body)
	if kind != toolResultJSON {
		t.Fatalf("kind = %d, want json", kind)
	}
	// json.Indent reformats layout only: the oversized integer must survive
	// without float64 round-tripping.
	if !strings.Contains(content, "12345678901234567890") {
		t.Fatalf("number was rewritten: %q", content)
	}
	if !strings.Contains(content, "\n  \"nested\"") {
		t.Fatalf("nested object was not indented: %q", content)
	}
}

func TestRenderToolBodyLinesRouting(t *testing.T) {
	p := DefaultPalette()

	plain := renderToolBodyLines("total 12\ndrwxr-xr-x src", 40, p)
	if strings.Contains(strings.Join(plain, "\n"), "\x1b[48;2;") {
		t.Fatalf("plain body picked up code background: %q", strings.Join(plain, "\n"))
	}

	markdown := renderToolBodyLines("## Summary\n\n- item one\n- item two", 40, p)
	joined := strings.Join(markdown, "\n")
	// Chroma emits 256-color foreground escapes; plain wrapText emits none.
	if !strings.Contains(joined, "\x1b[38;5;") {
		t.Fatalf("markdown body was not chroma-highlighted:\n%q", joined)
	}
	if stripped := ansi.Strip(joined); !strings.Contains(stripped, "item one") || !strings.Contains(stripped, "Summary") {
		t.Fatalf("markdown body lost content:\n%s", stripped)
	}

	diff := renderToolBodyLines("--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new", 40, p)
	if strings.Contains(strings.Join(diff, "\n"), "```") {
		t.Fatal("diff body was fenced instead of diff-colored")
	}
}

func TestToolCardRoutesMarkdownResultThroughChroma(t *testing.T) {
	card := &surface.ToolCard{
		ToolName: "notes",
		Status:   "done",
		Result:   "# Report\n\n- alpha\n- beta",
	}
	rendered := strings.Join((Model{}).renderToolWithOptions(card, 80, DefaultPalette(), false), "\n")
	if !strings.Contains(rendered, "\x1b[38;5;") {
		t.Fatalf("markdown tool result did not use the code path:\n%q", rendered)
	}
	if stripped := ansi.Strip(rendered); !strings.Contains(stripped, "alpha") || !strings.Contains(stripped, "Report") {
		t.Fatalf("tool card lost result content:\n%s", stripped)
	}
}
