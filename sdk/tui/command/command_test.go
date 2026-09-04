package command

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestParsePlainTextPreservesInput(t *testing.T) {
	for _, input := range []string{"hello  world", "  leading", "emoji 🙂 中文", "a\\b"} {
		got, err := Parse(input)
		if err != nil {
			t.Fatalf("Parse(%q): %v", input, err)
		}
		if got.Kind != Plain || got.Text != input || got.Invocation != nil {
			t.Fatalf("Parse(%q) = %+v, want unchanged plain text", input, got)
		}
	}
}

func TestParseDoubleSlashEscapesOneSlash(t *testing.T) {
	for _, tc := range []struct {
		input, want string
	}{
		{"//hello", "/hello"},
		{"///hello", "//hello"},
		{"  //你好 🙂", "  /你好 🙂"},
		{"//", "/"},
	} {
		got, err := Parse(tc.input)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.input, err)
		}
		if got.Kind != Plain || got.Text != tc.want {
			t.Fatalf("Parse(%q) = %+v, want plain %q", tc.input, got, tc.want)
		}
	}
}

func TestParseDoubleBangAndAtEscapeOneMarker(t *testing.T) {
	for _, tc := range []struct {
		input, want string
	}{
		{"!!echo", "!echo"},
		{"  !!你好 🙂", "  !你好 🙂"},
		{"@@file.txt", "@file.txt"},
		{"@@你好", "@你好"},
	} {
		got, err := Parse(tc.input)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.input, err)
		}
		if got.Kind != Plain || got.Text != tc.want || got.IsUnavailable() {
			t.Fatalf("Parse(%q) = %+v, want plain %q", tc.input, got, tc.want)
		}
	}
}

func TestParseShellAndFileInputs(t *testing.T) {
	shell, err := Parse("  !echo hi && printf ok")
	if err != nil || !shell.IsShell() || shell.Shell.Script != "echo hi && printf ok" || shell.Shell.Raw != "  !echo hi && printf ok" {
		t.Fatalf("shell parse = %+v, %v", shell, err)
	}
	for _, tc := range []struct {
		input string
		text  string
		paths []string
	}{
		{"@README.md", "", []string{"README.md"}},
		{"summarize @README.md please", "summarize  please", []string{"README.md"}},
		{"@a.md @b.go inspect", "  inspect", []string{"a.md", "b.go"}},
		{"email a@b.test", "email a@b.test", nil},
		{"@@literal", "@literal", nil},
	} {
		got, err := Parse(tc.input)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.input, err)
		}
		if len(tc.paths) == 0 {
			if got.Kind != Plain || got.Text != tc.text || len(got.ContextPaths) != 0 {
				t.Fatalf("Parse(%q) = %+v, want plain %q", tc.input, got, tc.text)
			}
			continue
		}
		if !got.IsFile() || got.Text != tc.text || !reflect.DeepEqual(got.ContextPaths, tc.paths) {
			t.Fatalf("Parse(%q) = %+v, want file text=%q paths=%q", tc.input, got, tc.text, tc.paths)
		}
	}
	if _, err := Parse("!"); err == nil {
		t.Fatal("empty shell script accepted")
	}
	if _, err := Parse("@"); err == nil {
		t.Fatal("empty file path accepted")
	}
}

func TestParseCommandsQuotesEscapesAndUnicode(t *testing.T) {
	got, err := Parse(`/new "你好 世界" 'from \'Vivy\'' emoji\ 🙂`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != Command || got.Invocation == nil {
		t.Fatalf("not a command: %+v", got)
	}
	want := []string{"new", "你好 世界", "from 'Vivy'", "emoji 🙂"}
	if got.Invocation.Name != want[0] || !reflect.DeepEqual(got.Invocation.Args, want[1:]) {
		t.Fatalf("invocation = %+v, want name=%q args=%q", got.Invocation, want[0], want[1:])
	}
	if got.Invocation.Raw != `/new "你好 世界" 'from \'Vivy\'' emoji\ 🙂` {
		t.Fatalf("raw input changed: %q", got.Invocation.Raw)
	}
}

func TestParseSyntaxErrors(t *testing.T) {
	for _, input := range []string{"/", `/new "unterminated`, `/new 'unterminated`, `/new trailing\`} {
		_, err := Parse(input)
		var syntax *SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("Parse(%q) error = %v, want SyntaxError", input, err)
		}
		if !strings.Contains(err.Error(), "invalid command syntax") {
			t.Fatalf("Parse(%q) error = %v, missing syntax prefix", input, err)
		}
	}
}

func TestRegistryResolvesAliasesAndRejectsUnknownLocally(t *testing.T) {
	r := DefaultRegistry()
	for _, name := range []string{"/help", "/?", "/commands", "/HELP", "/q", "/exit"} {
		got, err := r.Parse(name)
		if err != nil || !got.IsCommand() {
			t.Fatalf("registry Parse(%q) = %+v, %v", name, got, err)
		}
	}
	_, err := r.Parse(`/does-not-exist "🙂"`)
	var unknown *UnknownCommandError
	if !errors.As(err, &unknown) || unknown.Name != "does-not-exist" {
		t.Fatalf("unknown error = %T %v, want UnknownCommandError", err, err)
	}
	if !strings.Contains(r.Help(), "/permission") || !strings.Contains(r.Help(), "/thinking [auto|on|off]") || !strings.Contains(r.Help(), "/queue clear") || !strings.Contains(r.Help(), "/stats [period]") {
		t.Fatalf("help missing builtins:\n%s", r.Help())
	}
}

func TestRegistryValidatesAdvancedCommandArguments(t *testing.T) {
	r := DefaultRegistry()
	for _, input := range []string{"/thinking", "/thinking on", "/image photo.png", "/image remove 1", "/image clear", "/compact", "/fork msg-1", "/fork msg-1 \"new title\"", "/rewind msg-1", "/tasks", "/stats 1w", "/skills writer", "/mcp docs", "/mcp resources docs", "/mcp read docs \"docs://guide\"", "/files run-1 path.txt", "/tools"} {
		parsed, err := r.Parse(input)
		if err != nil {
			t.Fatalf("Parse(%q): %v", input, err)
		}
		if err := r.Validate(parsed.Invocation); err != nil {
			t.Fatalf("Validate(%q): %v", input, err)
		}
	}
	for _, input := range []string{"/thinking max", "/thinking on extra", "/image", "/image remove", "/image remove 0", "/image clear now", "/compact now", "/fork", "/rewind", "/stats 2h", "/mcp resources", "/mcp resources docs extra", "/mcp read docs", "/mcp read docs \"\"", "/mcp read docs uri extra", "/tools extra", "/files a b c"} {
		parsed, err := r.Parse(input)
		if err != nil {
			t.Fatalf("Parse(%q): %v", input, err)
		}
		if err := r.Validate(parsed.Invocation); err == nil {
			t.Fatalf("Validate(%q) accepted invalid arguments", input)
		}
	}
}

func TestFormatResultLabelsScopesAndSkippedCompaction(t *testing.T) {
	if got := FormatResult("compact", []byte(`{"before_tokens":0,"after_tokens":0,"folded_messages":0,"skipped":false}`)); !strings.Contains(got, "not-needed") || strings.Contains(got, "executed") {
		t.Fatalf("zero compaction result = %q", got)
	}
	if got := FormatResult("stats", []byte(`{"period":"1d","total":{"cost_known":false}}`)); !strings.Contains(got, "token usage aggregate") || !strings.Contains(got, "no current-session cost") {
		t.Fatalf("stats scope label = %q", got)
	}
	if got := FormatResult("mcp", []byte(`{"servers":[]}`)); !strings.Contains(got, "configured MCP") || strings.Contains(got, "connected") {
		t.Fatalf("mcp scope label = %q", got)
	}
	if got := FormatResult("mcp", []byte(`{"server":"docs","resources":[{"uri":"docs://guide"}],"untrusted":true}`)); !strings.Contains(got, "untrusted") || strings.Contains(got, "mounted") == false {
		t.Fatalf("mcp resources scope label = %q", got)
	}
	if got := FormatResult("mcp", []byte(`{"server":"docs","uri":"docs://guide","contents":[{"text":"hello"}],"untrusted":true}`)); !strings.Contains(got, "untrusted") || !strings.Contains(got, "read-only") {
		t.Fatalf("mcp read scope label = %q", got)
	}
}

func TestRegistryRejectsDuplicateNamesAndAliases(t *testing.T) {
	if _, err := NewRegistry(Spec{Name: "one"}, Spec{Name: "ONE"}); err == nil {
		t.Fatal("duplicate canonical names accepted")
	}
	if _, err := NewRegistry(Spec{Name: "one", Aliases: []string{"x"}}, Spec{Name: "two", Aliases: []string{"X"}}); err == nil {
		t.Fatal("duplicate aliases accepted")
	}
}
