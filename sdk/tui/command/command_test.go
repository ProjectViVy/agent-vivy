package command

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	corei18n "agent-vivy/internal/i18n"
	tuii18n "agent-vivy/sdk/tui/i18n"
)

var _ func(tuii18n.Translator) Registry = DefaultRegistry

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
		{`summarize @"docs/design notes.md" now`, "summarize  now", []string{"docs/design notes.md"}},
		{`inspect @'docs/Vivy\'s notes.md'`, "inspect ", []string{"docs/Vivy's notes.md"}},
		{`inspect @docs/design\ notes.md`, "inspect ", []string{"docs/design notes.md"}},
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
	if _, err := Parse(`inspect @"unterminated`); err == nil {
		t.Fatal("unterminated file path quote accepted")
	}
	if _, err := Parse(`inspect @"one.md"suffix`); err == nil {
		t.Fatal("quoted file path with adjacent suffix accepted")
	}
	for _, path := range []string{"README.md", "docs/design notes.md", `docs/a\"b.md`, `dir/a\\b.md`, "@mention.md", "'quote.md"} {
		got, err := Parse("inspect " + FormatFileReference(path))
		if err != nil || !reflect.DeepEqual(got.ContextPaths, []string{path}) {
			t.Fatalf("FormatFileReference(%q) round trip = %+v, %v", path, got, err)
		}
	}
	unicodeSpace := "docs/design\u2003notes.md"
	if formatted := FormatFileReference(unicodeSpace); !strings.HasPrefix(formatted, `@"`) {
		t.Fatalf("Unicode-space path was not quoted: %q", formatted)
	}
	windows, err := Parse(`inspect @src\main.go`)
	if err != nil || !reflect.DeepEqual(windows.ContextPaths, []string{"src/main.go"}) {
		t.Fatalf("Windows separator parse = %+v, %v", windows, err)
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
	r := DefaultRegistry(tuii18n.New(corei18n.English))
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
	if !strings.Contains(r.Help(), "/permission") || !strings.Contains(r.Help(), "/thinking [auto|on|off]") || !strings.Contains(r.Help(), "/model [filter]") || !strings.Contains(r.Help(), "/queue clear") || !strings.Contains(r.Help(), "/stats [period]") {
		t.Fatalf("help missing builtins:\n%s", r.Help())
	}
}

func TestRegistryValidatesAdvancedCommandArguments(t *testing.T) {
	r := DefaultRegistry(tuii18n.New(corei18n.English))
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

func TestHelpHidesUnavailableShellCapability(t *testing.T) {
	r := DefaultRegistry(tuii18n.New(corei18n.English))
	if strings.Contains(r.HelpFor(false), "!<script>") {
		t.Fatal("help advertised shell without an initialized capability")
	}
	if !strings.Contains(r.HelpFor(true), "!<script>") {
		t.Fatal("help hid shell despite shell.start capability")
	}
}

func TestDefaultRegistryLocalizesDescriptionsAndHelp(t *testing.T) {
	tests := []struct {
		name               string
		locale             corei18n.Locale
		descriptions       map[string]string
		commandsHeading    string
		prefixesHeading    string
		shellDescription   string
		fileDescription    string
		literalDescription string
		aliases            string
	}{
		{
			name:               "English",
			locale:             corei18n.English,
			descriptions:       map[string]string{"help": "View commands", "mcp": "View MCP servers and read-only resources", "quit": "Leave terminal"},
			commandsHeading:    "Commands",
			prefixesHeading:    "Input prefixes",
			shellDescription:   "Run a governed foreground shell command in the workspace",
			fileDescription:    "Attach project file context",
			literalDescription: "Send literal markers",
			aliases:            "(aliases: /?, /commands)",
		},
		{
			name:               "Chinese",
			locale:             corei18n.Chinese,
			descriptions:       map[string]string{"help": "查看命令", "mcp": "查看 MCP 服务与只读资源", "quit": "离开终端"},
			commandsHeading:    "命令",
			prefixesHeading:    "输入前缀",
			shellDescription:   "在工作区执行受治理的前台 shell 命令",
			fileDescription:    "附加项目文件上下文",
			literalDescription: "发送字面量标记",
			aliases:            "（别名：/?, /commands）",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := DefaultRegistry(tuii18n.New(tt.locale))
			for name, description := range tt.descriptions {
				spec, ok := r.Lookup(name)
				if !ok || spec.Description != description {
					t.Errorf("%s spec = %+v, %t; want description %q", name, spec, ok, description)
				}
			}
			help := r.HelpFor(true)
			for _, want := range []string{tt.commandsHeading, tt.prefixesHeading, tt.shellDescription, tt.fileDescription, tt.literalDescription, tt.aliases} {
				if !strings.Contains(help, want) {
					t.Errorf("localized help missing %q:\n%s", want, help)
				}
			}
		})
	}
}

func TestDefaultRegistryMapsAllLocalizedDescriptionKeys(t *testing.T) {
	type commandDescription struct {
		name    string
		english string
		chinese string
	}
	commands := []commandDescription{
		{name: "help", english: "View commands", chinese: "查看命令"},
		{name: "status", english: "View current run status", chinese: "当前运行状态"},
		{name: "sessions", english: "Open session list", chinese: "打开会话列表"},
		{name: "model", english: "Switch model", chinese: "切换模型"},
		{name: "new", english: "Create a session", chinese: "新建会话"},
		{name: "session", english: "Switch to a session", chinese: "切换到指定会话"},
		{name: "rename", english: "Rename current session", chinese: "重命名当前会话"},
		{name: "delete", english: "Delete a session", chinese: "删除会话"},
		{name: "cancel", english: "Cancel current run", chinese: "取消当前运行"},
		{name: "queue", english: "Clear queued turns", chinese: "清空排队回合"},
		{name: "permission", english: "Set permission preset", chinese: "设置权限档"},
		{name: "thinking", english: "Set thinking mode", chinese: "设置思考档"},
		{name: "image", english: "Attach a project image", chinese: "附加项目图片"},
		{name: "compact", english: "Compact current context", chinese: "压缩当前上下文"},
		{name: "fork", english: "Fork session at a message", chinese: "在消息处分叉会话"},
		{name: "rewind", english: "Rewind session view", chinese: "回退会话视图"},
		{name: "todos", english: "View todos", chinese: "查看待办"},
		{name: "stats", english: "View usage statistics", chinese: "查看用量统计"},
		{name: "skills", english: "View installed skills", chinese: "查看已安装技能"},
		{name: "mcp", english: "View MCP servers and read-only resources", chinese: "查看 MCP 服务与只读资源"},
		{name: "files", english: "View governed workspace", chinese: "查看受治理工作区"},
		{name: "tools", english: "View tool catalog", chinese: "查看工具目录"},
		{name: "quit", english: "Leave terminal", chinese: "离开终端"},
	}
	locales := []struct {
		name   string
		locale corei18n.Locale
		want   func(command commandDescription) string
	}{
		{name: "English", locale: corei18n.English, want: func(command commandDescription) string { return command.english }},
		{name: "Chinese", locale: corei18n.Chinese, want: func(command commandDescription) string { return command.chinese }},
	}
	for _, tt := range locales {
		t.Run(tt.name, func(t *testing.T) {
			translator := tuii18n.New(tt.locale)
			registry := DefaultRegistry(translator)
			if got := len(registry.Specs()); got != len(commands) {
				t.Fatalf("registry has %d commands, want %d", got, len(commands))
			}
			for _, command := range commands {
				want := tt.want(command)
				key := "vivy.tui.command." + command.name + ".description"
				if got := translator.T(key, nil); got != want {
					t.Errorf("T(%q) = %q, want %q", key, got, want)
				}
				spec, ok := registry.Lookup(command.name)
				if !ok || spec.Description != want {
					t.Errorf("Lookup(%q) = %+v, %t; want description %q", command.name, spec, ok, want)
				}
			}
		})
	}
}

func TestRegistryExtendPreservesLocalization(t *testing.T) {
	r := DefaultRegistry(tuii18n.New(corei18n.Chinese))
	extended, err := r.Extend(Spec{Name: "review", Description: "Review changes"})
	if err != nil {
		t.Fatal(err)
	}
	help := extended.Help()
	if !strings.HasPrefix(help, "命令\n") || !strings.Contains(help, "/review") {
		t.Fatalf("extended help lost catalog or command:\n%s", help)
	}
	if _, ok := r.Lookup("review"); ok {
		t.Fatal("Extend mutated the original registry")
	}
}

func TestFormatResultLabelsScopesAndSkippedCompaction(t *testing.T) {
	if got := FormatResult("compact", []byte(`{"before_tokens":0,"after_tokens":0,"folded_messages":0,"skipped":false}`)); !strings.Contains(got, "not-needed") || strings.Contains(got, "executed") {
		t.Fatalf("zero compaction result = %q", got)
	}
	if got := FormatResult("stats", []byte(`{"period":"1d","scope":"chat_runs","total":{"cost_known":false}}`)); !strings.Contains(got, "chat-run token usage") || !strings.Contains(got, "title/manual-compaction calls excluded") {
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
