package view

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDefaultKeymapCoversGlobalChords(t *testing.T) {
	k := DefaultKeymap()
	for action, chord := range map[string]string{
		"send":         "enter",
		"follow_up":    "alt+enter",
		"dequeue":      "alt+up",
		"cancel":       "esc",
		"palette":      "ctrl+p",
		"model_picker": "ctrl+l",
		"model_cycle":  "alt+p",
		"fast_quit":    "q",
		"jump_bottom":  "G",
	} {
		if got := k.Action(chord); got != action {
			t.Fatalf("chord %q → %q, want %q", chord, got, action)
		}
	}
	// Every declared default must survive indexing (no internal conflicts).
	for _, action := range k.order {
		if len(k.Chords(action)) == 0 {
			t.Fatalf("default action %q lost all chords", action)
		}
	}
}

func TestNormalizeChordCaseRules(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Ctrl+P", "ctrl+p"}, {"ALT+ENTER", "alt+enter"},
		{"alt+P", "alt+p"}, // modified rune is case-insensitive
		{"G", "G"}, {"g", "g"}, {" / ", "/"},
		{"Shift+Tab", "shift+tab"}, {"", ""},
	} {
		if got := normalizeChord(tc.in); got != tc.want {
			t.Fatalf("normalizeChord(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLoadKeymapMissingFileIsDefaults(t *testing.T) {
	k, warns := LoadKeymap(filepath.Join(t.TempDir(), "nope.yaml"))
	if len(warns) != 0 {
		t.Fatalf("missing file must not warn, got %v", warns)
	}
	if k.Action("ctrl+p") != "palette" {
		t.Fatal("defaults must apply when the file is missing")
	}
}

func TestLoadKeymapOverrideRemapsAction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")
	if err := os.WriteFile(path, []byte("palette: f1\npage_down: ctrl+d\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	k, warns := LoadKeymap(path)
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings %v", warns)
	}
	if k.Action("f1") != "palette" {
		t.Fatal("override did not bind f1 to palette")
	}
	if k.Action("ctrl+p") != "" {
		t.Fatal("override must replace the action's default chords")
	}
	if k.Action("ctrl+d") != "page_down" || k.Action("pgdown") != "" {
		t.Fatal("list/scalar override did not replace page_down chords")
	}
}

func TestLoadKeymapListValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")
	os.WriteFile(path, []byte("cancel: [esc, ctrl+g]\n"), 0o600)
	k, _ := LoadKeymap(path)
	if k.Action("esc") != "cancel" || k.Action("ctrl+g") != "cancel" {
		t.Fatal("list form must bind every chord")
	}
}

func TestLoadKeymapUnknownActionWarns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")
	os.WriteFile(path, []byte("not_an_action: f9\n"), 0o600)
	k, warns := LoadKeymap(path)
	if len(warns) != 1 || !strings.Contains(warns[0], "not_an_action") {
		t.Fatalf("want unknown-action warning, got %v", warns)
	}
	if k.Action("f9") != "" {
		t.Fatal("unknown action must not bind")
	}
}

func TestLoadKeymapChordConflictFirstWins(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")
	// palette is declared before sessions; claiming ctrl+s twice makes
	// sessions lose the chord deterministically.
	os.WriteFile(path, []byte("palette: ctrl+s\n"), 0o600)
	k, warns := LoadKeymap(path)
	if k.Action("ctrl+s") != "palette" {
		t.Fatal("earlier action must win a shared chord")
	}
	if len(k.Chords("sessions")) != 0 {
		t.Fatal("losing action must drop the contested chord")
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "ctrl+s") {
		t.Fatalf("want conflict warning, got %v", warns)
	}
}

func TestLoadKeymapInvalidYAMLFallsBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")
	os.WriteFile(path, []byte("::: not yaml"), 0o600)
	k, warns := LoadKeymap(path)
	if k.Action("ctrl+p") != "palette" {
		t.Fatal("invalid file must keep defaults")
	}
	if len(warns) != 1 {
		t.Fatalf("want a warning, got %v", warns)
	}
}

func TestModelKeysNilKeymapIsInert(t *testing.T) {
	var m Model // zero value, as in render-only tests
	if m.keys.Action("ctrl+c") != "" {
		t.Fatal("nil keymap must not claim chords")
	}
}

func TestKeyChordMatchesBubbleteaStrings(t *testing.T) {
	cases := []struct {
		msg   tea.KeyMsg
		chord string
	}{
		{tea.KeyMsg{Type: tea.KeyCtrlP}, "ctrl+p"},
		{tea.KeyMsg{Type: tea.KeyEnter, Alt: true}, "alt+enter"},
		{tea.KeyMsg{Type: tea.KeyUp, Alt: true}, "alt+up"},
		{tea.KeyMsg{Type: tea.KeyShiftTab}, "shift+tab"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}, Alt: true}, "alt+p"},
	}
	k := DefaultKeymap()
	for _, tc := range cases {
		if k.Action(keyChord(tc.msg)) == "" {
			t.Fatalf("%v produced chord %q, bound to nothing", tc.msg, keyChord(tc.msg))
		}
	}
}
