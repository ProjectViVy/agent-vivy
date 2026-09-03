// Package command contains the protocol-independent command language used by
// both first-party terminal faces. It only classifies input; command effects
// remain owned by the TUI surface driver.
package command

import (
	"fmt"
	"strings"
	"unicode"
)

// Kind describes the result of parsing one input line.
type Kind uint8

const (
	// Plain is ordinary user text. The Text field is byte-for-byte unchanged.
	Plain Kind = iota
	// Command is a slash command parsed into Invocation.
	Command
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

// Result is the classification of one editor line. A plain result has a nil
// Invocation; a command result has the parsed invocation and an empty Text.
type Result struct {
	Kind       Kind
	Text       string
	Invocation *Invocation
}

// IsCommand reports whether the result is a slash command.
func (r Result) IsCommand() bool { return r.Kind == Command && r.Invocation != nil }

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
	if !strings.HasPrefix(trimmed, "/") {
		return Result{Kind: Plain, Text: input}, nil
	}
	if strings.HasPrefix(trimmed, "//") {
		prefix := input[:len(input)-len(trimmed)]
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
	ordered []Spec
	byName  map[string]Spec
}

// NewRegistry builds a deterministic registry from the supplied specs. Empty
// names and duplicate names/aliases are rejected so a face cannot silently
// route one command to two handlers.
func NewRegistry(specs ...Spec) (Registry, error) {
	r := Registry{ordered: make([]Spec, 0, len(specs)), byName: make(map[string]Spec)}
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

// Help returns the stable built-in help text for a registry.
func (r Registry) Help() string {
	var b strings.Builder
	b.WriteString("commands\n")
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
			aliases = " (" + strings.Join(parts, ", ") + ")"
		}
		fmt.Fprintf(&b, "  %-22s %s%s\n", usage, spec.Description, aliases)
	}
	return b.String()
}

// DefaultRegistry is the command catalog shared by the built-in and packed
// code faces. Effects are dispatched by sdk/tui/view; this package stays
// unaware of RPC or Bubble Tea.
func DefaultRegistry() Registry {
	r, err := NewRegistry(
		Spec{Name: "help", Aliases: []string{"?", "commands"}, Usage: "/help", Description: "show commands"},
		Spec{Name: "status", Usage: "/status", Description: "show active run status"},
		Spec{Name: "sessions", Usage: "/sessions", Description: "open the sessions picker"},
		Spec{Name: "new", Usage: "/new [title]", Description: "create a session"},
		Spec{Name: "session", Usage: "/session <id>", Description: "switch to a session"},
		Spec{Name: "rename", Usage: "/rename <title>", Description: "rename the active session"},
		Spec{Name: "delete", Usage: "/delete [id]", Description: "delete a session after confirmation"},
		Spec{Name: "cancel", Usage: "/cancel", Description: "cancel the active run"},
		Spec{Name: "queue", Usage: "/queue clear", Description: "clear queued turns"},
		Spec{Name: "permission", Usage: "/permission [preset]", Description: "set or cycle permission"},
		Spec{Name: "quit", Aliases: []string{"exit", "q"}, Usage: "/quit", Description: "leave the TUI"},
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
