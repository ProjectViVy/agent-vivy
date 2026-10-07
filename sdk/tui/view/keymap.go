package view

import (
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"
)

// Keymap maps named actions to chords. Chords use bubbletea's KeyMsg.String
// vocabulary ("ctrl+p", "alt+enter", "shift+tab", "alt+up", "enter", "esc",
// "pgup", or a literal rune like "q"/"G"). An action may hold several chords;
// the first declared action wins a chord shared with another action.
type Keymap struct {
	order    []string
	byAction map[string][]string
	byChord  map[string]string
}

// keyBinding pairs an action with its default chords. Declaration order is
// the conflict-resolution order: the earlier action keeps a shared chord.
var defaultKeyBindings = []struct {
	action string
	chords []string
}{
	{"quit", []string{"ctrl+c"}},
	{"palette", []string{"ctrl+p", "shift+h", "H", "/"}},
	{"shortcuts", []string{"ctrl+x"}},
	{"sessions", []string{"ctrl+s"}},
	{"new_session", []string{"ctrl+n"}},
	{"model_picker", []string{"ctrl+l"}},
	{"model_cycle", []string{"alt+p"}},
	{"permission_cycle", []string{"ctrl+y"}},
	{"thinking_cycle", []string{"ctrl+t"}},
	{"tools_toggle", []string{"ctrl+o"}},
	{"reasoning_toggle", []string{"ctrl+r"}},
	{"mode_cycle", []string{"shift+tab"}},
	{"sidebar_focus", []string{"ctrl+right"}},
	{"dequeue", []string{"alt+up"}},
	{"follow_up", []string{"alt+enter", "ctrl+q"}},
	{"send", []string{"enter"}},
	{"newline", []string{"ctrl+j"}},
	{"cancel", []string{"esc"}},
	{"fast_quit", []string{"q"}},
	{"jump_bottom", []string{"G"}},
	{"page_up", []string{"pgup"}},
	{"page_down", []string{"pgdown"}},
	{"top", []string{"home"}},
	{"bottom", []string{"end"}},
	{"search", []string{"ctrl+f"}},
	{"prompt_prev", []string{"ctrl+up"}},
	{"prompt_next", []string{"ctrl+down"}},
	{"copy_last", []string{"alt+c"}},
	{"external_editor", []string{"ctrl+e"}},
}

// DefaultKeymap returns the built-in bindings.
func DefaultKeymap() *Keymap {
	k := &Keymap{byAction: map[string][]string{}}
	for _, b := range defaultKeyBindings {
		k.order = append(k.order, b.action)
		k.byAction[b.action] = append([]string(nil), b.chords...)
	}
	k.reindex(nil)
	return k
}

// LoadKeymap reads a `action: chord | [chords]` YAML override file and
// returns the effective map plus warnings. A missing file yields the
// defaults with no warning; every other failure mode degrades to defaults
// or partial overrides with a warning — never an error.
func LoadKeymap(path string) (*Keymap, []string) {
	k := DefaultKeymap()
	if path == "" {
		return k, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return k, nil
		}
		return k, []string{fmt.Sprintf("keybindings %s: %v; using defaults", path, err)}
	}
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return k, []string{fmt.Sprintf("keybindings %s: invalid YAML: %v; using defaults", path, err)}
	}
	var warns []string
	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, action := range names {
		if _, ok := k.byAction[action]; !ok {
			warns = append(warns, fmt.Sprintf("keybindings %s: unknown action %q ignored", path, action))
			continue
		}
		chords, err := decodeChords(raw[action])
		if err != nil {
			warns = append(warns, fmt.Sprintf("keybindings %s: action %q: %v", path, action, err))
			continue
		}
		k.byAction[action] = chords
	}
	k.reindex(&warns)
	return k, warns
}

func decodeChords(node yaml.Node) ([]string, error) {
	var chords []string
	switch node.Kind {
	case yaml.ScalarNode:
		chords = []string{node.Value}
	case yaml.SequenceNode:
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("chord list must be strings")
			}
			chords = append(chords, item.Value)
		}
	default:
		return nil, fmt.Errorf("want a chord string or list of strings")
	}
	out := make([]string, 0, len(chords))
	for _, c := range chords {
		c = normalizeChord(c)
		if c == "" {
			return nil, fmt.Errorf("empty chord")
		}
		out = append(out, c)
	}
	return out, nil
}

// normalizeChord lowercases modifier words and named keys. A bare single
// rune keeps its case ("G" ≠ "g"); a modified rune is case-insensitive so
// "alt+P" and "alt+p" are the same chord, matching bubbletea String().
func normalizeChord(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "+")
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if i == len(parts)-1 && len(p) == 1 && len(parts) == 1 {
			continue
		}
		parts[i] = strings.ToLower(p)
	}
	return strings.Join(parts, "+")
}

// keyChord canonicalizes an incoming key message for map lookup.
func keyChord(msg tea.KeyMsg) string {
	return normalizeChord(msg.String())
}

// Action reports which action owns the chord, or "". A nil map (tests that
// skip New) behaves as if nothing is bound.
func (k *Keymap) Action(chord string) string {
	if k == nil {
		return ""
	}
	return k.byChord[chord]
}

// Chords lists the effective chords for an action in declaration order.
func (k *Keymap) Chords(action string) []string {
	if k == nil {
		return nil
	}
	return append([]string(nil), k.byAction[action]...)
}

// Lines renders the effective binding table for /hotkeys.
func (k *Keymap) Lines() []string {
	if k == nil {
		return nil
	}
	lines := make([]string, 0, len(k.order))
	for _, action := range k.order {
		chords := k.byAction[action]
		if len(chords) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("%-18s %s", action, strings.Join(chords, ", ")))
	}
	return lines
}

// reindex rebuilds the chord→action index in declaration order; a chord
// claimed by an earlier action wins and the later binding warns + drops.
func (k *Keymap) reindex(warns *[]string) {
	k.byChord = map[string]string{}
	for _, action := range k.order {
		kept := k.byAction[action][:0]
		for _, chord := range k.byAction[action] {
			if owner, taken := k.byChord[chord]; taken && owner != action {
				if warns != nil {
					*warns = append(*warns, fmt.Sprintf("chord %q already bound to %s; %s loses it", chord, owner, action))
				}
				continue
			}
			k.byChord[chord] = action
			kept = append(kept, chord)
		}
		k.byAction[action] = kept
	}
}
