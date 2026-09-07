package view

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"agent-vivy/sdk/tui/surface"
)

func streamTestEntry(t *testing.T, id string) *streamEntry {
	t.Helper()
	t.Cleanup(func() { delete(streamEntries, streamEntryKey{id: id}) })
	entry, ok := streamEntries[streamEntryKey{id: id}]
	if !ok {
		t.Fatal("stream entry missing after render")
	}
	return entry
}

func TestStreamMarkdownPromotesSafeBoundaryAcrossFlushes(t *testing.T) {
	id := "stream-promote"
	first := "# Heading\n\nFirst paragraph"
	if _, err := streamMarkdownRender(id, first, 40, false); err != nil {
		t.Fatal(err)
	}
	entry := streamTestEntry(t, id)
	if entry.stablePrefix != "# Heading\n\n" {
		t.Fatalf("seeded prefix = %q", entry.stablePrefix)
	}

	// A flush with no new blank line must keep the cache and render only the
	// trailing delta.
	if _, err := streamMarkdownRender(id, first+" extended", 40, false); err != nil {
		t.Fatal(err)
	}
	if entry.stablePrefix != "# Heading\n\n" {
		t.Fatalf("boundaryless flush moved prefix = %q", entry.stablePrefix)
	}

	second := first + "\n\nSecond paragraph arrives"
	out, err := streamMarkdownRender(id, second, 40, false)
	if err != nil {
		t.Fatal(err)
	}
	if entry.stablePrefix != second[:strings.Index(second, "\n\nSecond")+2] {
		t.Fatalf("flush did not promote the new boundary: prefix=%q", entry.stablePrefix)
	}
	if entry.baseFenceCount != 0 || !strings.HasPrefix(second, entry.stablePrefix) {
		t.Fatalf("promoted state inconsistent: %+v", entry)
	}
	stripped := ansi.Strip(out)
	for _, want := range []string{"Heading", "First paragraph", "Second paragraph arrives"} {
		if !strings.Contains(stripped, want) {
			t.Fatalf("streaming output lost %q:\n%s", want, stripped)
		}
	}
	if strings.Contains(out, "\n\n\n") {
		t.Fatalf("glued output produced doubled blank lines:\n%q", out)
	}
}

func TestStreamMarkdownNeverCutsInsideOpenFence(t *testing.T) {
	id := "stream-fence"
	content := "```\ncode line 1\n\ncode line 2\n"
	if _, err := streamMarkdownRender(id, content, 40, false); err != nil {
		t.Fatal(err)
	}
	entry := streamTestEntry(t, id)
	if entry.stablePrefix != "" {
		t.Fatalf("open fence still seeded a prefix: %q", entry.stablePrefix)
	}

	// A closed fence followed by prose is a safe boundary again.
	closed := "```go\nfmt.Println()\n```\n\nAfter the fence"
	if _, err := streamMarkdownRender(id, closed, 40, false); err != nil {
		t.Fatal(err)
	}
	if entry.stablePrefix != "```go\nfmt.Println()\n```\n\n" {
		t.Fatalf("closed fence boundary = %q", entry.stablePrefix)
	}
}

func TestStreamMarkdownResetsOnWidthChangeAndRewrite(t *testing.T) {
	id := "stream-reset"
	if _, err := streamMarkdownRender(id, "one\n\ntwo", 40, false); err != nil {
		t.Fatal(err)
	}
	entry := streamTestEntry(t, id)
	if entry.width != 40 || entry.stablePrefix == "" {
		t.Fatalf("initial entry = %+v", entry)
	}

	// Rewritten content that does not extend the prefix drops the cache.
	if _, err := streamMarkdownRender(id, "different document\n\nbody", 40, false); err != nil {
		t.Fatal(err)
	}
	if entry.stablePrefix != "different document\n\n" {
		t.Fatalf("rewritten content kept the old prefix: %q", entry.stablePrefix)
	}

	// A width change resets the entry for the new wrap width.
	if _, err := streamMarkdownRender(id, "different document\n\nbody again", 60, false); err != nil {
		t.Fatal(err)
	}
	if entry.width != 60 {
		t.Fatalf("width change did not reset: %+v", entry)
	}
	if entry.stablePrefix != "different document\n\n" {
		t.Fatalf("reseeded prefix after width change = %q", entry.stablePrefix)
	}
}

func TestFindSafeMarkdownBoundaryHazards(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    int
	}{
		{"paragraph split", "one\n\ntwo", len("one\n\n")},
		{"open fence blank line", "```\ncode\n\nmore\n", -1},
		{"setext follows", "Title\n\n===", -1},
		{"loose list continuation", "- item one\n\n  continued\n", -1},
		{"link reference", "[label]: https://example.com\n\nprose", -1},
		{"html block", "<div>\n\nprose", -1},
		{"table row", "| a | b |\n\nprose", -1},
	} {
		if got := findSafeMarkdownBoundary(tc.content); got != tc.want {
			t.Fatalf("%s: boundary = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestStreamingMessageRendersThroughCache(t *testing.T) {
	id := "stream-message"
	t.Cleanup(func() { delete(streamEntries, streamEntryKey{id: id}) })
	lines := renderMessage(surface.Message{
		ID: id, Role: surface.RoleAssistant, Content: "# Live\n\nStreaming body", Streaming: true,
	}, 80, DefaultPalette())
	stripped := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(stripped, "Live") || !strings.Contains(stripped, "Streaming body") {
		t.Fatalf("streaming message lost markdown content:\n%s", stripped)
	}
	if entry := streamEntries[streamEntryKey{id: id}]; entry == nil {
		t.Fatal("streaming message did not use the streaming cache")
	}
}
