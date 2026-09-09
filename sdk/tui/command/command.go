// Package command contains the protocol-independent command language used by
// both first-party terminal faces. It only classifies input; command effects
// remain owned by the TUI surface driver.
package command

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	corei18n "agent-vivy/internal/i18n"
	tuii18n "agent-vivy/sdk/tui/i18n"
)

// Kind describes the result of parsing one input line.
type Kind uint8

const (
	// Plain is ordinary user text. The Text field is byte-for-byte unchanged.
	Plain Kind = iota
	// Command is a slash command parsed into Invocation.
	Command
	// Unavailable is a locally recognized command prefix whose effect is not
	// wired in this face. It must never be sent to the model. Text preserves
	// the original input so the editor can keep the draft for correction.
	Unavailable
	// Shell is a bang-prefixed shell script. The script is sent to the
	// server-owned shell/start seam; a face must never execute it locally.
	Shell
	// File is a prompt containing one or more project-relative @file
	// references. Text is the prompt with the reference markers removed and
	// ContextPaths contains the original path tokens. The server resolves
	// those paths again at turn/start time.
	File
)

// Invocation is one syntactically valid slash command.
type Invocation struct {
	// Name is the command name without the leading slash. Registry lookup is
	// case-insensitive, but Parse preserves the spelling for diagnostics.
	Name string
	Args []string
	// Raw is the original input, including leading whitespace and the slash.
	Raw string
}

// ShellInvocation is the parsed form of a !script line. Script deliberately
// preserves every byte after the leading marker (including intentional
// whitespace); the server is responsible for parsing, policy and execution.
type ShellInvocation struct {
	Script string
	Raw    string
}

// Result is the classification of one editor line. A plain result has a nil
// Invocation; a command result has the parsed invocation and an empty Text.
type Result struct {
	Kind       Kind
	Text       string
	Invocation *Invocation
	Shell      *ShellInvocation
	// ContextPaths are project-relative paths extracted from @file markers.
	// They are hints only: the control plane validates and re-resolves them
	// immediately before starting a run.
	ContextPaths      []string
	UnavailableReason string
}

// IsCommand reports whether the result is a slash command.
func (r Result) IsCommand() bool { return r.Kind == Command && r.Invocation != nil }

// IsUnavailable reports a recognized local-only prefix without an available
// control-plane implementation. Callers must render the reason locally and
// must not forward Text to the model or execute it on the host.
func (r Result) IsUnavailable() bool { return r.Kind == Unavailable }

// IsShell reports a parsed !script input.
func (r Result) IsShell() bool { return r.Kind == Shell && r.Shell != nil }

// IsFile reports a prompt containing one or more @file references.
func (r Result) IsFile() bool { return r.Kind == File && len(r.ContextPaths) > 0 }

// FilePaths returns a defensive copy of project-context paths.
func (r Result) FilePaths() []string { return append([]string(nil), r.ContextPaths...) }

// SyntaxError identifies a malformed command line. Offset is a rune offset,
// which keeps diagnostics useful for Unicode input.
type SyntaxError struct {
	Offset  int
	Message string
}

func (e *SyntaxError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return "invalid command syntax"
	}
	return fmt.Sprintf("invalid command syntax at %d: %s", e.Offset, e.Message)
}

// UnknownCommandError is returned by Registry.Parse when a syntactically
// valid slash command is not registered. Callers must show it locally and
// must not forward the original line as model text.
type UnknownCommandError struct{ Name string }

func (e *UnknownCommandError) Error() string {
	if e == nil || e.Name == "" {
		return "unknown command"
	}
	return fmt.Sprintf("unknown command /%s", e.Name)
}

// Parse classifies input and parses slash-command quoting. Ordinary text is
// returned exactly as supplied. A second leading slash escapes the command
// marker, so //hello becomes the literal model text /hello.
func Parse(input string) (Result, error) {
	trimmed := strings.TrimLeftFunc(input, unicode.IsSpace)
	prefix := input[:len(input)-len(trimmed)]
	if !strings.HasPrefix(trimmed, "/") {
		// Double prefixes escape one marker. Shell and project-context effects
		// are represented as data here; execution and filesystem access remain
		// owned by the control plane.
		if strings.HasPrefix(trimmed, "!!") {
			return Result{Kind: Plain, Text: prefix + trimmed[1:]}, nil
		}
		if strings.HasPrefix(trimmed, "@@") {
			return Result{Kind: Plain, Text: prefix + trimmed[1:]}, nil
		}
		if strings.HasPrefix(trimmed, "!") {
			script := trimmed[1:]
			if strings.TrimSpace(script) == "" {
				return Result{}, &SyntaxError{Offset: 1, Message: "shell script is required"}
			}
			return Result{Kind: Shell, Text: input, Shell: &ShellInvocation{Script: script, Raw: input}}, nil
		}
		return parseFileReferences(input)
	}
	if strings.HasPrefix(trimmed, "//") {
		return Result{Kind: Plain, Text: prefix + trimmed[1:]}, nil
	}

	runes := []rune(trimmed)
	words, err := tokenize(runes[1:])
	if err != nil {
		return Result{}, err
	}
	if len(words) == 0 || words[0] == "" {
		return Result{}, &SyntaxError{Offset: 1, Message: "command name is required"}
	}
	args := append([]string(nil), words[1:]...)
	return Result{
		Kind: Command,
		Invocation: &Invocation{
			Name: words[0],
			Args: args,
			Raw:  input,
		},
	}, nil
}

// parseFileReferences extracts project-relative @path tokens from ordinary
// text. A marker is recognized only at the beginning of the trimmed input or
// immediately after Unicode whitespace; this prevents email addresses and
// ordinary prose from becoming filesystem requests. @@ escapes one marker.
// Quoted references (for example @"docs/design notes.md") and backslash
// escapes make every server-listed project path representable in the editor.
// The returned Text keeps all non-reference text except for removed markers.
// The control plane remains authoritative and resolves ContextPaths again at
// turn/start time.
func parseFileReferences(input string) (Result, error) {
	runes := []rune(input)
	if len(runes) == 0 {
		return Result{Kind: Plain, Text: input}, nil
	}
	var out []rune
	paths := make([]string, 0, 1)
	for i := 0; i < len(runes); {
		if runes[i] != '@' || (i > 0 && !unicode.IsSpace(runes[i-1])) {
			out = append(out, runes[i])
			i++
			continue
		}
		if i+1 < len(runes) && runes[i+1] == '@' {
			out = append(out, '@')
			i += 2
			continue
		}
		path, end, err := parseFileReferenceToken(runes, i+1)
		if err != nil {
			return Result{}, err
		}
		if path == "" {
			return Result{}, &SyntaxError{Offset: i, Message: "file path is required after @"}
		}
		if end < len(runes) && !unicode.IsSpace(runes[end]) {
			return Result{}, &SyntaxError{Offset: end, Message: "file reference must end before text"}
		}
		paths = append(paths, path)
		i = end
	}
	if len(paths) == 0 {
		return Result{Kind: Plain, Text: input}, nil
	}
	return Result{Kind: File, Text: string(out), ContextPaths: paths}, nil
}

// FormatFileReference returns one parser-safe @ reference. Simple paths stay
// compact; paths containing whitespace, quotes, or backslashes use the same
// double-quoted escape grammar accepted by Parse.
func FormatFileReference(path string) string {
	needsQuote := strings.HasPrefix(path, "@") || strings.HasPrefix(path, "'")
	for _, r := range path {
		if unicode.IsSpace(r) || r == '\\' || r == '"' {
			needsQuote = true
			break
		}
	}
	if !needsQuote {
		return "@" + path
	}
	escaped := strings.ReplaceAll(path, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return "@\"" + escaped + "\""
}

func parseFileReferenceToken(input []rune, start int) (string, int, error) {
	if start >= len(input) || unicode.IsSpace(input[start]) {
		return "", start, nil
	}
	var path []rune
	quote := rune(0)
	i := start
	if input[i] == '\'' || input[i] == '"' {
		quote = input[i]
		i++
	}
	for i < len(input) {
		r := input[i]
		if quote != 0 {
			if r == quote {
				return string(path), i + 1, nil
			}
		} else if unicode.IsSpace(r) {
			return string(path), i, nil
		}
		if r == '\\' {
			if i+1 >= len(input) {
				return "", i, &SyntaxError{Offset: i, Message: "trailing escape in file path"}
			}
			next := input[i+1]
			if unicode.IsSpace(next) || next == '\\' || next == '\'' || next == '"' {
				path = append(path, next)
				i += 2
				continue
			}
			// The control plane canonicalizes Windows separators to '/'. Make
			// hand-entered Windows paths behave the same as listed candidates.
			path = append(path, '/')
			i++
			continue
		}
		path = append(path, r)
		i++
	}
	if quote != 0 {
		return "", i, &SyntaxError{Offset: i, Message: "unterminated file path quote"}
	}
	return string(path), i, nil
}

// ParseLine is an explicit alias for callers whose input is line-oriented.
func ParseLine(input string) (Result, error) { return Parse(input) }

// Spec describes a registered command. Name is the canonical spelling;
// Aliases are alternate names without a slash.
type Spec struct {
	Name        string
	Aliases     []string
	Usage       string
	Description string
}

// Registry resolves command names and owns the shared built-in catalog.
type Registry struct {
	ordered    []Spec
	byName     map[string]Spec
	translator tuii18n.Translator
}

// NewRegistry builds a deterministic registry from the supplied specs. Empty
// names and duplicate names/aliases are rejected so a face cannot silently
// route one command to two handlers.
func NewRegistry(specs ...Spec) (Registry, error) {
	return newRegistry(tuii18n.New(corei18n.English), specs...)
}

func newRegistry(translator tuii18n.Translator, specs ...Spec) (Registry, error) {
	r := Registry{
		ordered:    make([]Spec, 0, len(specs)),
		byName:     make(map[string]Spec),
		translator: translator,
	}
	for _, spec := range specs {
		name := normalizeName(spec.Name)
		if name == "" {
			return Registry{}, fmt.Errorf("command: empty command name")
		}
		if _, exists := r.byName[name]; exists {
			return Registry{}, fmt.Errorf("command: duplicate command /%s", name)
		}
		copySpec := spec
		copySpec.Name = name
		copySpec.Aliases = normalizeAliases(spec.Aliases)
		if err := r.addName(name, copySpec); err != nil {
			return Registry{}, err
		}
		for _, alias := range copySpec.Aliases {
			if err := r.addName(alias, copySpec); err != nil {
				return Registry{}, err
			}
		}
		r.ordered = append(r.ordered, copySpec)
	}
	return r, nil
}

// Extend returns a new registry with specs appended while preserving the
// receiver's translator. The receiver and its catalog remain unchanged.
func (r Registry) Extend(specs ...Spec) (Registry, error) {
	all := r.Specs()
	all = append(all, specs...)
	return newRegistry(r.translator, all...)
}

func (r *Registry) addName(name string, spec Spec) error {
	if _, exists := r.byName[name]; exists {
		return fmt.Errorf("command: duplicate command /%s", name)
	}
	r.byName[name] = spec
	return nil
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "/")))
}

func normalizeAliases(aliases []string) []string {
	out := make([]string, 0, len(aliases))
	seen := make(map[string]struct{}, len(aliases))
	for _, alias := range aliases {
		name := normalizeName(alias)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// Lookup resolves a canonical name or alias case-insensitively.
func (r Registry) Lookup(name string) (Spec, bool) {
	spec, ok := r.byName[normalizeName(name)]
	return spec, ok
}

// Specs returns a defensive copy in declaration order (aliases included in
// each spec, but not as separate rows).
func (r Registry) Specs() []Spec {
	out := make([]Spec, len(r.ordered))
	for i, spec := range r.ordered {
		out[i] = spec
		out[i].Aliases = append([]string(nil), spec.Aliases...)
	}
	return out
}

// Parse parses input and resolves its command against this registry.
func (r Registry) Parse(input string) (Result, error) {
	result, err := Parse(input)
	if err != nil || !result.IsCommand() {
		return result, err
	}
	if _, ok := r.Lookup(result.Invocation.Name); !ok {
		return Result{}, &UnknownCommandError{Name: result.Invocation.Name}
	}
	return result, nil
}

// Validate checks the argument contract for a registered command. Parsing is
// deliberately separate so editors can preserve a syntactically valid draft
// while still rejecting a bad invocation before any RPC is sent.
func (r Registry) Validate(invocation *Invocation) error {
	if invocation == nil {
		return &SyntaxError{Offset: 1, Message: "command name is required"}
	}
	spec, ok := r.Lookup(invocation.Name)
	if !ok {
		return &UnknownCommandError{Name: invocation.Name}
	}
	args := invocation.Args
	usage := func() error {
		return fmt.Errorf("%s", r.translator.T("vivy.tui.error.usage", map[string]any{"usage": spec.Usage}))
	}
	count := func(min, max int) error {
		if len(args) < min || (max >= 0 && len(args) > max) {
			return usage()
		}
		return nil
	}
	switch spec.Name {
	case "help", "status", "sessions", "cancel", "compact", "todos", "tools", "quit":
		return count(0, 0)
	case "model":
		return count(0, -1)
	case "mcp":
		if len(args) == 0 {
			return nil
		}
		if strings.TrimSpace(args[0]) == "" {
			return usage()
		}
		switch strings.ToLower(strings.TrimSpace(args[0])) {
		case "resources":
			if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
				return fmt.Errorf("%s", r.translator.T("vivy.tui.error.usage", map[string]any{"usage": "/mcp resources <server>"}))
			}
		case "read":
			if len(args) != 3 || strings.TrimSpace(args[1]) == "" || strings.TrimSpace(args[2]) == "" {
				return fmt.Errorf("%s", r.translator.T("vivy.tui.error.usage", map[string]any{"usage": "/mcp read <server> <uri>"}))
			}
		default:
			return count(1, 1)
		}
		return nil
	case "new":
		return count(0, -1)
	case "session":
		return count(1, 1)
	case "rename":
		if len(args) == 0 || strings.TrimSpace(strings.Join(args, " ")) == "" {
			return usage()
		}
		return nil
	case "delete":
		return count(0, 1)
	case "queue":
		if len(args) != 1 || !strings.EqualFold(args[0], "clear") {
			return usage()
		}
		return nil
	case "permission":
		if err := count(0, 1); err != nil {
			return err
		}
		if len(args) == 1 {
			switch strings.ToLower(strings.TrimSpace(args[0])) {
			case "cautious", "smart", "trusted":
			default:
				return fmt.Errorf("%s", r.translator.T("vivy.tui.error.permission", nil))
			}
		}
		return nil
	case "thinking":
		if err := count(0, 1); err != nil {
			return err
		}
		if len(args) == 1 {
			switch strings.ToLower(strings.TrimSpace(args[0])) {
			case "auto", "on", "off":
			default:
				return fmt.Errorf("%s", r.translator.T("vivy.tui.error.thinking", nil))
			}
		}
		return nil
	case "image":
		if len(args) == 1 && strings.TrimSpace(args[0]) != "" &&
			!strings.EqualFold(strings.TrimSpace(args[0]), "remove") &&
			!strings.EqualFold(strings.TrimSpace(args[0]), "clear") {
			return nil
		}
		if len(args) == 2 && strings.EqualFold(strings.TrimSpace(args[0]), "remove") {
			index, err := strconv.Atoi(strings.TrimSpace(args[1]))
			if err == nil && index > 0 {
				return nil
			}
			return fmt.Errorf("%s", r.translator.T("vivy.tui.live.imageIndex", nil))
		}
		if len(args) == 1 && strings.EqualFold(strings.TrimSpace(args[0]), "clear") {
			return nil
		}
		return fmt.Errorf("%s", r.translator.T("vivy.tui.error.usage", map[string]any{"usage": "/image <relative-path> | /image remove <index> | /image clear"}))
	case "fork":
		return count(1, 2)
	case "rewind":
		return count(1, 1)
	case "stats":
		if err := count(0, 1); err != nil {
			return err
		}
		if len(args) == 1 {
			switch strings.ToLower(strings.TrimSpace(args[0])) {
			case "1d", "3d", "1w", "1m", "6m", "1y":
			default:
				return fmt.Errorf("%s", r.translator.T("vivy.tui.live.statsPeriod", nil))
			}
		}
		return nil
	case "skills":
		return count(0, 1)
	case "files":
		return count(0, 2)
	default:
		return nil
	}
}

// Help returns the stable built-in help text for a registry.
func (r Registry) Help() string {
	return r.HelpFor(true)
}

// HelpFor renders input prefixes supported by the initialized control plane.
// Slash commands remain stable; unavailable server capabilities fail closed.
func (r Registry) HelpFor(shellSupported bool) string {
	var b strings.Builder
	b.WriteString(r.translator.T("vivy.tui.help.heading.commands", nil))
	b.WriteByte('\n')
	for _, spec := range r.ordered {
		usage := spec.Usage
		if usage == "" {
			usage = "/" + spec.Name
		}
		aliases := ""
		if len(spec.Aliases) > 0 {
			parts := make([]string, 0, len(spec.Aliases))
			for _, alias := range spec.Aliases {
				parts = append(parts, "/"+alias)
			}
			aliases = " " + r.translator.T("vivy.tui.help.aliases", map[string]any{"aliases": strings.Join(parts, ", ")})
		}
		fmt.Fprintf(&b, "  %-22s %s%s\n", usage, spec.Description, aliases)
	}
	b.WriteByte('\n')
	b.WriteString(r.translator.T("vivy.tui.help.heading.inputPrefixes", nil))
	b.WriteByte('\n')
	if shellSupported {
		fmt.Fprintf(&b, "  %-22s %s\n", "!<script>", r.translator.T("vivy.tui.help.prefix.shell.description", nil))
	}
	fmt.Fprintf(&b, "  %-22s %s\n", r.translator.T("vivy.tui.help.prefix.file.usage", nil), r.translator.T("vivy.tui.help.prefix.file.description", nil))
	fmt.Fprintf(&b, "  %-22s %s\n", "!! / @@", r.translator.T("vivy.tui.help.prefix.literal.description", nil))
	return b.String()
}

// DefaultRegistry is the command catalog shared by the built-in and packed
// code faces. Effects are dispatched by sdk/tui/view; this package stays
// unaware of RPC or Bubble Tea.
func DefaultRegistry(translator tuii18n.Translator) Registry {
	description := func(name string) string {
		return translator.T("vivy.tui.command."+name+".description", nil)
	}
	r, err := newRegistry(translator,
		Spec{Name: "help", Aliases: []string{"?", "commands"}, Usage: "/help", Description: description("help")},
		Spec{Name: "status", Usage: "/status", Description: description("status")},
		Spec{Name: "sessions", Usage: "/sessions", Description: description("sessions")},
		Spec{Name: "model", Usage: "/model [filter]", Description: description("model")},
		Spec{Name: "new", Usage: "/new [title]", Description: description("new")},
		Spec{Name: "session", Usage: "/session <id>", Description: description("session")},
		Spec{Name: "rename", Usage: "/rename <title>", Description: description("rename")},
		Spec{Name: "delete", Usage: "/delete [id]", Description: description("delete")},
		Spec{Name: "cancel", Usage: "/cancel", Description: description("cancel")},
		Spec{Name: "queue", Usage: "/queue clear", Description: description("queue")},
		Spec{Name: "permission", Usage: "/permission [preset]", Description: description("permission")},
		Spec{Name: "thinking", Usage: "/thinking [auto|on|off]", Description: description("thinking")},
		Spec{Name: "image", Aliases: []string{"attach"}, Usage: "/image <relative-path>", Description: description("image")},
		Spec{Name: "compact", Usage: "/compact", Description: description("compact")},
		Spec{Name: "fork", Usage: "/fork <message_id> [title]", Description: description("fork")},
		Spec{Name: "rewind", Usage: "/rewind <message_id>", Description: description("rewind")},
		Spec{Name: "todos", Aliases: []string{"tasks"}, Usage: "/todos", Description: description("todos")},
		Spec{Name: "stats", Usage: "/stats [period]", Description: description("stats")},
		Spec{Name: "skills", Usage: "/skills [name]", Description: description("skills")},
		Spec{Name: "mcp", Usage: "/mcp [server|resources <server>|read <server> <uri>]", Description: description("mcp")},
		Spec{Name: "files", Usage: "/files [run_id [path]]", Description: description("files")},
		Spec{Name: "tools", Usage: "/tools", Description: description("tools")},
		Spec{Name: "quit", Aliases: []string{"exit", "q"}, Usage: "/quit", Description: description("quit")},
	)
	if err != nil {
		// The literal catalog above is package-owned and validated by tests. A
		// zero registry is safer than a panic if it is edited incorrectly.
		return Registry{}
	}
	return r
}

func tokenize(input []rune) ([]string, error) {
	var words []string
	var current []rune
	var quote rune
	started := false
	for i := 0; i < len(input); i++ {
		r := input[i]
		if quote != 0 {
			if r == quote {
				quote = 0
				started = true
				continue
			}
			if r == '\\' {
				if i+1 >= len(input) {
					return nil, &SyntaxError{Offset: i + 1, Message: "trailing escape"}
				}
				i++
				current = append(current, input[i])
				started = true
				continue
			}
			current = append(current, r)
			started = true
			continue
		}
		switch {
		case r == '\\':
			if i+1 >= len(input) {
				return nil, &SyntaxError{Offset: i + 1, Message: "trailing escape"}
			}
			i++
			current = append(current, input[i])
			started = true
		case r == '\'' || r == '"':
			quote = r
			started = true
		case unicode.IsSpace(r):
			if started {
				words = append(words, string(current))
				current = current[:0]
				started = false
			}
		default:
			current = append(current, r)
			started = true
		}
	}
	if quote != 0 {
		return nil, &SyntaxError{Offset: len(input), Message: "unterminated quote"}
	}
	if started {
		words = append(words, string(current))
	}
	return words, nil
}
