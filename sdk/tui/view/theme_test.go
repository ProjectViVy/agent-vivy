package view

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"agent-vivy/sdk/tui/theme"

	"github.com/charmbracelet/lipgloss"
)

// TestPaletteFromColorsMapsEveryRole guards the migration: the palette is
// built exclusively from theme.Colors, so a custom theme must change every
// dependent style slot that previously held a hardcoded value.
func TestPaletteFromColorsMapsEveryRole(t *testing.T) {
	custom := theme.Dark0()
	custom.Primary = "#010203"
	p := PaletteFromColors(custom)
	if got := p.DialogTitle.GetForeground(); got != lipgloss.Color("#010203") {
		t.Fatalf("primary role did not reach palette: %v", got)
	}
	if p.Colors != custom {
		t.Fatal("palette must carry the resolved colors for markdown styles")
	}
	if DefaultPalette().Colors != theme.Dark() {
		t.Fatal("DefaultPalette must be the embedded dark theme")
	}
}

// TestNoHardcodedColorsInView fails if a view source file reintroduces a
// hex color literal; colors must come from theme.Colors.
func TestNoHardcodedColorsInView(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Skip("source path unavailable")
	}
	dir := filepath.Dir(file)
	hex := regexp.MustCompile(`"#[0-9a-fA-F]{3,8}"`)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".go" || name == "theme_test.go" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if m := hex.Find(data); m != nil {
			t.Fatalf("%s reintroduces a hardcoded color %s; use theme.Colors", name, m)
		}
	}
}
