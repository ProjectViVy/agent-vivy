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
	if !strings.Contains(r.Help(), "/permission") || !strings.Contains(r.Help(), "/queue clear") {
		t.Fatalf("help missing builtins:\n%s", r.Help())
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
