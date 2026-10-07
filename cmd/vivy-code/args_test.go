package main

import (
	"testing"
)

func parse(t *testing.T, args ...string) Args {
	t.Helper()
	a := parseArgs(args)
	if len(a.Errors) > 0 {
		t.Fatalf("unexpected parse errors for %v: %v", args, a.Errors)
	}
	return a
}

func TestParseArgsDefaults(t *testing.T) {
	a := parse(t)
	if a.Options.Mode != "" {
		t.Fatalf("Mode = %q, want empty (text)", a.Options.Mode)
	}
	if a.Help || a.Version {
		t.Fatal("help/version set on empty args")
	}
}

func TestParseArgsMode(t *testing.T) {
	for _, m := range []string{"text", "json", "rpc", "print"} {
		if got := parse(t, "--mode", m).Options.Mode; got != m {
			t.Fatalf("Mode = %q, want %q", got, m)
		}
	}
	a := parseArgs([]string{"--mode", "bogus"})
	if len(a.Errors) == 0 {
		t.Fatal("invalid --mode accepted")
	}
}

func TestParseArgsPrint(t *testing.T) {
	a := parse(t, "-p", "hello world")
	if a.Options.Mode != "print" || a.Options.Prompt != "hello world" {
		t.Fatalf("got Mode=%q Prompt=%q", a.Options.Mode, a.Options.Prompt)
	}
	// Bare -p without a value is still valid print mode.
	a = parse(t, "--print")
	if a.Options.Mode != "print" || a.Options.Prompt != "" {
		t.Fatalf("got Mode=%q Prompt=%q", a.Options.Mode, a.Options.Prompt)
	}
	// A flag after -p is not swallowed as the prompt.
	a = parse(t, "-p", "--verbose")
	if a.Options.Prompt != "" || !a.Options.Verbose {
		t.Fatalf("got Prompt=%q Verbose=%v", a.Options.Prompt, a.Options.Verbose)
	}
	// -p does not clobber an explicit headless mode (pi parity).
	a = parse(t, "--mode", "json", "-p", "hi")
	if a.Options.Mode != "json" || a.Options.Prompt != "hi" {
		t.Fatalf("got Mode=%q Prompt=%q", a.Options.Mode, a.Options.Prompt)
	}
	a = parse(t, "-p", "--mode", "rpc")
	if a.Options.Mode != "rpc" {
		t.Fatalf("got Mode=%q", a.Options.Mode)
	}
}

func TestParseArgsSessionSelectors(t *testing.T) {
	a := parse(t, "-c")
	if !a.Options.ContinueNewest {
		t.Fatal("ContinueNewest unset")
	}
	a = parse(t, "-r", "--session", "s.jsonl", "--session-id", "sid", "--fork", "m123", "--session-dir", "/tmp/x", "--name", "n", "--no-session")
	o := a.Options
	if !o.Resume || o.Session != "s.jsonl" || o.SessionID != "sid" || o.Fork != "m123" || o.SessionDir != "/tmp/x" || o.Name != "n" || !o.NoSession {
		t.Fatalf("bad options: %+v", o)
	}
}

func TestParseArgsModelSurface(t *testing.T) {
	a := parse(t, "--provider", "anthropic", "--model", "claude-sonnet-4.5", "--api-key", "k", "--thinking", "high", "--models", "a,b,c", "--system-prompt", "S", "--append-system-prompt", "x", "--append-system-prompt", "y")
	o := a.Options
	if o.Provider != "anthropic" || o.Model != "claude-sonnet-4.5" || o.APIKey != "k" || o.Thinking != "high" || o.SystemPrompt != "S" {
		t.Fatalf("bad options: %+v", o)
	}
	if len(o.Models) != 3 || len(o.AppendSystemPrompt) != 2 {
		t.Fatalf("bad slices: %+v", o)
	}
	// Invalid thinking level is a warning, not an error (pi parity).
	a = parseArgs([]string{"--thinking", "bogus"})
	if len(a.Errors) != 0 || len(a.Warnings) == 0 || a.Options.Thinking != "" {
		t.Fatalf("bad diagnostics: %+v", a)
	}
	// --list-models with and without a pattern.
	if !parse(t, "--list-models").Options.ListModels {
		t.Fatal("ListModels unset")
	}
	if got := parse(t, "--list-models", "claude").Options.ListModelsPattern; got != "claude" {
		t.Fatalf("ListModelsPattern = %q", got)
	}
}

func TestParseArgsToolSurface(t *testing.T) {
	a := parse(t, "-t", "read_file,patch", "-xt", "execute", "-nt", "-nbt", "--no-mcp")
	o := a.Options
	if len(o.Tools) != 2 || len(o.ExcludeTools) != 1 || !o.NoTools || !o.NoBuiltinTools || !o.NoMCP {
		t.Fatalf("bad options: %+v", o)
	}
}

func TestParseArgsContextSurface(t *testing.T) {
	a := parse(t, "--skill", "s1", "--prompt-template", "pt", "--theme", "th", "--use-theme", "dark", "-ns", "-np", "--no-themes", "-nc")
	o := a.Options
	if len(o.Skills) != 1 || len(o.PromptTemplates) != 1 || len(o.Themes) != 1 || o.UseTheme != "dark" {
		t.Fatalf("bad options: %+v", o)
	}
	if !o.NoSkills || !o.NoPromptTemplates || !o.NoThemes || !o.NoContextFiles {
		t.Fatalf("bad no-* flags: %+v", o)
	}
}

func TestParseArgsPolicyAndMisc(t *testing.T) {
	a := parse(t, "-a")
	if a.Options.Approve == nil || !*a.Options.Approve {
		t.Fatal("Approve not true")
	}
	a = parse(t, "-na")
	if a.Options.Approve == nil || *a.Options.Approve {
		t.Fatal("Approve not false")
	}
	if parse(t).Options.Approve != nil {
		t.Fatal("Approve set by default")
	}
	a = parse(t, "--export", "out.html", "--tui-mode", "fullscreen", "--offline", "--verbose", "--debug-tools")
	o := a.Options
	if o.Export != "out.html" || o.TUIMode != "fullscreen" || !o.Offline || !o.Verbose || !o.DebugToolOutput {
		t.Fatalf("bad options: %+v", o)
	}
}

func TestParseArgsPositionalAndFiles(t *testing.T) {
	a := parse(t, "fix", "the", "bug", "@main.go", "@README.md")
	if a.Options.Prompt != "fix the bug" {
		t.Fatalf("Prompt = %q", a.Options.Prompt)
	}
	if len(a.Options.Files) != 2 || a.Options.Files[0] != "main.go" {
		t.Fatalf("Files = %v", a.Options.Files)
	}
	// Everything after -- is literal.
	a = parse(t, "--", "fix", "--verbose", "@x.txt")
	if a.Options.Prompt != "fix --verbose" || len(a.Options.Files) != 1 || a.Options.Verbose {
		t.Fatalf("post-- parse: %+v", a.Options)
	}
}

func TestParseArgsUnknownFlagIsError(t *testing.T) {
	a := parseArgs([]string{"--bogus-flag"})
	if len(a.Errors) == 0 {
		t.Fatal("unknown flag accepted")
	}
	// Missing values are errors too.
	a = parseArgs([]string{"--model"})
	if len(a.Errors) == 0 {
		t.Fatal("missing --model value accepted")
	}
}
