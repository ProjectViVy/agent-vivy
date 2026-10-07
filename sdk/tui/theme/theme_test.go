package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveEmbeddedDefaults(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"dark", "light"} {
		colors, warns := Resolve(dir, name)
		if len(warns) != 0 {
			t.Fatalf("%s: unexpected warnings %v", name, warns)
		}
		if colors.Primary == "" || colors.Fg == "" || colors.CodeBg == "" {
			t.Fatalf("%s: embedded theme missing roles: %+v", name, colors)
		}
		if colors.Name != name {
			t.Fatalf("%s: Name = %q", name, colors.Name)
		}
	}
	if Dark() == Light() {
		t.Fatal("dark and light embedded themes must differ")
	}
}

func TestResolveUserFileOverridesEmbedded(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "dark", `{"name":"mine","colors":{"primary":"#FF0000"}}`)
	colors, warns := Resolve(dir, "dark")
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings %v", warns)
	}
	if colors.Primary != "#ff0000" {
		t.Fatalf("user file did not win: %q", colors.Primary)
	}
	// Partial themes fill the remaining roles from the dark default.
	if colors.Fg != Dark0().Fg {
		t.Fatalf("partial theme did not inherit fg: %q", colors.Fg)
	}
	if colors.Name != "mine" {
		t.Fatalf("name = %q", colors.Name)
	}
}

func TestResolveCustomNamedTheme(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "solar", `{"colors":{"primary":"#B58900","fg":"#657b83"}}`)
	colors, warns := Resolve(dir, "solar")
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings %v", warns)
	}
	if colors.Primary != "#b58900" {
		t.Fatalf("primary = %q", colors.Primary)
	}
	if colors.Name != "solar" {
		t.Fatalf("name = %q", colors.Name)
	}
}

func TestResolveUnknownNameFallsBackToDark(t *testing.T) {
	colors, warns := Resolve(t.TempDir(), "does-not-exist")
	if colors != Dark() {
		t.Fatal("unknown theme must fall back to dark")
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "does-not-exist") {
		t.Fatalf("want a visible warning, got %v", warns)
	}
}

func TestResolveInvalidUserFileWarnsAndFallsBack(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "broken", `{"colors":{"primary":"red"}}`)
	colors, warns := Resolve(dir, "broken")
	if colors != Dark() {
		t.Fatal("invalid theme must fall back to dark")
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "broken") {
		t.Fatalf("want a visible warning, got %v", warns)
	}
	// An invalid file named after an embedded theme keeps its warning and
	// falls back to that embedded theme, not dark.
	writeTheme(t, dir, "light", `not json`)
	colors, warns = Resolve(dir, "light")
	if colors != Light() {
		t.Fatal("invalid override of embedded theme must fall back to embedded")
	}
	if len(warns) != 1 {
		t.Fatalf("want a visible warning, got %v", warns)
	}
}

func TestParseNormalizesShortHexAndWarnsOnUnknownKeys(t *testing.T) {
	colors, warns, err := parse([]byte(`{"name":"x","colors":{"primary":"#F00"},"bogus":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if colors.Primary != "#ff0000" {
		t.Fatalf("short hex not expanded: %q", colors.Primary)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "bogus") {
		t.Fatalf("unknown key must warn, got %v", warns)
	}
}

func TestParseRejectsBadHexAndBadJSON(t *testing.T) {
	for _, doc := range []string{
		`{"colors":{"primary":"#12"}}`,
		`{"colors":{"primary":"0xFF0000"}}`,
		`{"colors":{"primary":"#GGGGGG"}}`,
		`{`,
	} {
		if _, _, err := parse([]byte(doc)); err == nil {
			t.Fatalf("%s must be rejected", doc)
		}
	}
}

func TestColorsIDDistinguishesThemes(t *testing.T) {
	if Dark().ID() != "dark" || Light().ID() != "light" {
		t.Fatalf("named themes must ID by name: %q %q", Dark().ID(), Light().ID())
	}
	custom := Dark0()
	custom.Name = ""
	if custom.ID() == Dark().ID() {
		t.Fatal("nameless colors must ID by value, not collide with dark")
	}
}

func TestDetectLightCOLORFGBGFallback(t *testing.T) {
	if _, ok := queryOSC11(0); ok {
		t.Skip("controlling terminal answered OSC 11; fallback untestable here")
	}
	for _, tc := range []struct {
		env   string
		light bool
	}{
		{"0;15", true}, {"15;7", true}, {"0;8", true},
		{"15;0", false}, {"7;0", false}, {"", false}, {"garbage", false},
	} {
		t.Setenv("COLORFGBG", tc.env)
		if got := DetectLight(); got != tc.light {
			t.Fatalf("COLORFGBG=%q: DetectLight = %v, want %v", tc.env, got, tc.light)
		}
	}
}

func TestParseOSC11(t *testing.T) {
	r, g, b, ok := parseOSC11("\x1b]11;rgb:ffff/ffff/ffff\a")
	if !ok || r != 1 || g != 1 || b != 1 {
		t.Fatalf("white reply parsed %v %v %v ok=%v", r, g, b, ok)
	}
	if _, _, _, ok := parseOSC11("\x1b]11;rgb:ff/ff/ff\a"); !ok {
		t.Fatal("8-bit component reply must parse")
	}
	if _, _, _, ok := parseOSC11("noise"); ok {
		t.Fatal("garbage must not parse")
	}
}

func TestDefaultDir(t *testing.T) {
	t.Setenv("VIVY_USER_HOME", "/tmp/vivy-home-test")
	if got := DefaultDir(); got != filepath.Join("/tmp/vivy-home-test", "themes") {
		t.Fatalf("DefaultDir = %q", got)
	}
	t.Setenv("VIVY_USER_HOME", "")
	if home, err := os.UserHomeDir(); err == nil {
		if got := DefaultDir(); got != filepath.Join(home, ".vivy", "themes") {
			t.Fatalf("DefaultDir = %q", got)
		}
	}
}

func writeTheme(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
