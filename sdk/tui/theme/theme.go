// Package theme resolves the TUI color palette: two embedded defaults
// (dark, light), operator files under <agent home>/themes/*.json, and a
// terminal-background auto-detect (OSC 11, COLORFGBG, else dark). Theme
// files declare a name and a role map; every role must be a #rgb or
// #rrggbb hex color. Resolution never fails: an unreadable or invalid
// file yields a warning and the dark fallback.
package theme

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed themes/*.json
var embedded embed.FS

// Colors carries the role map the view's Palette and markdown style read.
// Field names are the theme-file keys; Name identifies the theme for
// renderer caches and diagnostics.
type Colors struct {
	Name      string `json:"-"`
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
	Fg        string `json:"fg"`
	Muted     string `json:"muted"`
	Subtle    string `json:"subtle"`
	Success   string `json:"success"`
	Warn      string `json:"warn"`
	Danger    string `json:"danger"`
	User      string `json:"user"`
	OnPrimary string `json:"on_primary"`
	CodeBg    string `json:"code_bg"`
	String    string `json:"string"`
	Link      string `json:"link"`
}

// ID identifies the color set for renderer caches; the file name wins so a
// renamed-but-identical theme still shares its glamour renderers.
func (c Colors) ID() string {
	if c.Name != "" {
		return c.Name
	}
	return fmt.Sprintf("%x", c)
}

type file struct {
	Name   string `json:"name"`
	Colors Colors `json:"colors"`
}

var embeddedNames = map[string]string{
	"dark":  "themes/dark.json",
	"light": "themes/light.json",
}

var roleNames = map[string]bool{
	"primary": true, "secondary": true, "fg": true, "muted": true,
	"subtle": true, "success": true, "warn": true, "danger": true,
	"user": true, "on_primary": true, "code_bg": true, "string": true,
	"link": true,
}

func knownRole(name string) bool { return roleNames[name] }

// DefaultDir is the conventional operator theme directory when the caller
// has no agent home to anchor to: $VIVY_USER_HOME/themes or ~/.vivy/themes.
func DefaultDir() string {
	if v := strings.TrimSpace(os.Getenv("VIVY_USER_HOME")); v != "" {
		return filepath.Join(v, "themes")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".vivy", "themes")
	}
	return "themes"
}

// Dark returns the embedded dark palette (the historical default).
func Dark() Colors {
	c, err := parseEmbedded("dark")
	if err != nil {
		panic("theme: embedded dark theme is invalid: " + err.Error())
	}
	return c
}

// Light returns the embedded light palette.
func Light() Colors {
	c, err := parseEmbedded("light")
	if err != nil {
		panic("theme: embedded light theme is invalid: " + err.Error())
	}
	return c
}

// Resolve picks the effective palette for a theme name: "" or "auto"
// detects the terminal background, "light"/"dark" name the embedded
// defaults, and anything else names <dir>/<name>.json — a file that
// overrides an embedded theme of the same name when present. Every
// failure mode degrades to dark with a warning instead of an error.
func Resolve(dir, name string) (Colors, []string) {
	name = strings.TrimSpace(name)
	switch {
	case name == "" || name == "auto":
		if DetectLight() {
			return Light(), nil
		}
		return Dark(), nil
	case name == "dark", name == "light":
		c, warns, ok := loadUserFile(dir, name)
		if ok {
			return c, warns
		}
		if len(warns) != 0 {
			// A file overriding the embedded name exists but is invalid:
			// keep the warning and fall back to the embedded theme.
			return resolveEmbedded(name), warns
		}
		return resolveEmbedded(name), nil
	default:
		c, warns, ok := loadUserFile(dir, name)
		if ok {
			return c, warns
		}
		if embedded, err := parseEmbedded(name); err == nil {
			return embedded, warns
		}
		if len(warns) == 0 {
			warns = []string{fmt.Sprintf("theme %q not found in %s; using dark", name, dir)}
		}
		return Dark(), warns
	}
}

func resolveEmbedded(name string) Colors {
	if name == "light" {
		return Light()
	}
	return Dark()
}

// loadUserFile reads <dir>/<name>.json; the second result reports whether
// the returned colors should be used at all.
func loadUserFile(dir, name string) (Colors, []string, bool) {
	if dir == "" {
		return Colors{}, nil, false
	}
	path := filepath.Join(dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return Colors{}, nil, false
	}
	colors, warns, err := parse(data)
	if err != nil {
		return Colors{}, []string{fmt.Sprintf("theme %s: %v; using dark", path, err)}, false
	}
	if colors.Name == "" {
		colors.Name = name
	}
	return colors, warns, true
}

func parseEmbedded(name string) (Colors, error) {
	path, ok := embeddedNames[name]
	if !ok {
		return Colors{}, fmt.Errorf("unknown embedded theme %q", name)
	}
	data, err := embedded.ReadFile(path)
	if err != nil {
		return Colors{}, err
	}
	colors, warns, err := parse(data)
	if err != nil {
		return Colors{}, err
	}
	if len(warns) != 0 {
		return Colors{}, fmt.Errorf("embedded theme %q has unknown keys: %s", name, strings.Join(warns, ", "))
	}
	colors.Name = name
	return colors, nil
}

func parse(data []byte) (Colors, []string, error) {
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return Colors{}, nil, fmt.Errorf("invalid JSON: %w", err)
	}
	c := f.Colors
	c.Name = strings.TrimSpace(f.Name)
	var warns []string
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err == nil {
		for k := range top {
			if k != "name" && k != "colors" {
				warns = append(warns, fmt.Sprintf("unknown key %q ignored", k))
			}
		}
		if raw, ok := top["colors"]; ok {
			var ckeys map[string]json.RawMessage
			if err := json.Unmarshal(raw, &ckeys); err == nil {
				for k := range ckeys {
					if !knownRole(k) {
						warns = append(warns, fmt.Sprintf("unknown color role %q ignored", k))
					}
				}
			}
		}
	}
	// Normalize every role and collect unknown keys. A role left empty
	// inherits the dark default so partial themes stay usable.
	base := Dark0()
	roles := map[string]*string{
		"primary": &c.Primary, "secondary": &c.Secondary, "fg": &c.Fg,
		"muted": &c.Muted, "subtle": &c.Subtle, "success": &c.Success,
		"warn": &c.Warn, "danger": &c.Danger, "user": &c.User,
		"on_primary": &c.OnPrimary, "code_bg": &c.CodeBg,
		"string": &c.String, "link": &c.Link,
	}
	defaults := map[string]*string{
		"primary": &base.Primary, "secondary": &base.Secondary, "fg": &base.Fg,
		"muted": &base.Muted, "subtle": &base.Subtle, "success": &base.Success,
		"warn": &base.Warn, "danger": &base.Danger, "user": &base.User,
		"on_primary": &base.OnPrimary, "code_bg": &base.CodeBg,
		"string": &base.String, "link": &base.Link,
	}
	keys := make([]string, 0, len(roles))
	for k := range roles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := strings.TrimSpace(*roles[k])
		if v == "" {
			*roles[k] = *defaults[k]
			continue
		}
		norm, err := normalizeHex(v)
		if err != nil {
			return Colors{}, nil, fmt.Errorf("color %q: %w", k, err)
		}
		*roles[k] = norm
	}
	return c, warns, nil
}

// Dark0 is the unadorned dark role map used as the fill for partial user
// themes. It duplicates Dark()'s data without recursion into parse.
func Dark0() Colors {
	return Colors{
		Name: "dark", Primary: "#A78BFA", Secondary: "#2DD4BF", Fg: "#E4E4E7",
		Muted: "#71717A", Subtle: "#52525B", Success: "#4ADE80", Warn: "#FBBF24",
		Danger: "#FB7185", User: "#93C5FD", OnPrimary: "#1C1917",
		CodeBg: "#27272A", String: "#FDBA74", Link: "#A1A1AA",
	}
}

// normalizeHex accepts #rgb or #rrggbb and returns #rrggbb lowercase.
func normalizeHex(v string) (string, error) {
	if !strings.HasPrefix(v, "#") {
		return "", fmt.Errorf("must start with #: %q", v)
	}
	hexPart := v[1:]
	switch len(hexPart) {
	case 3:
		hexPart = string([]byte{hexPart[0], hexPart[0], hexPart[1], hexPart[1], hexPart[2], hexPart[2]})
	case 6:
	default:
		return "", fmt.Errorf("want #rgb or #rrggbb: %q", v)
	}
	if _, err := strconv.ParseUint(hexPart, 16, 32); err != nil {
		return "", fmt.Errorf("invalid hex: %q", v)
	}
	return "#" + strings.ToLower(hexPart), nil
}

// DetectLight reports whether the terminal background reads as light. It
// tries an OSC 11 query against the controlling terminal with a short
// deadline, falls back to the COLORFGBG convention (background index >= 8
// or == 7 is light… xterm sets "fg;bg" with 0-15 where light themes use
// 7 or 15), and defaults to dark when nothing answers.
func DetectLight() bool {
	if bg, ok := queryOSC11(150 * time.Millisecond); ok {
		return bg
	}
	if v := strings.TrimSpace(os.Getenv("COLORFGBG")); v != "" {
		parts := strings.Split(v, ";")
		if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			return n == 7 || n >= 8
		}
	}
	return false
}

// queryOSC11 asks the terminal for its background color (OSC 11) and
// reports whether the answer is a light color. The exchange goes through
// the controlling terminal so it also works when stdio is redirected.
func queryOSC11(deadline time.Duration) (light bool, ok bool) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false, false
	}
	defer tty.Close()
	if _, err := tty.WriteString("\x1b]11;?\x07"); err != nil {
		return false, false
	}
	type result struct {
		response string
		err      error
	}
	ch := make(chan result, 1)
	go func() {
		buf := make([]byte, 64)
		var b strings.Builder
		for b.Len() < len(buf) {
			n, err := tty.Read(buf[:1])
			if n == 1 {
				b.WriteByte(buf[0])
				if buf[0] == '\a' || strings.HasSuffix(b.String(), "\x1b\\") {
					break
				}
			}
			if err != nil {
				ch <- result{b.String(), err}
				return
			}
		}
		ch <- result{b.String(), nil}
	}()
	var resp string
	select {
	case r := <-ch:
		resp = r.response
	case <-time.After(deadline):
		return false, false
	}
	r, g, b, found := parseOSC11(resp)
	if !found {
		return false, false
	}
	// ITU-R BT.601 luma; the 16-bit components dominate beyond byte precision.
	luma := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	return luma > 0.5, true
}

// parseOSC11 extracts the rgb:RR/GG/BB triple from an OSC 11 reply such as
// "\x1b]11;rgb:ffff/ffff/ffff\a". Components are normalized to 0-1.
func parseOSC11(resp string) (r, g, b float64, ok bool) {
	i := strings.Index(resp, "rgb:")
	if i < 0 {
		return 0, 0, 0, false
	}
	spec := resp[i+4:]
	if j := strings.IndexAny(spec, "\a\x1b"); j >= 0 {
		spec = spec[:j]
	}
	parts := strings.Split(spec, "/")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	vals := make([]float64, 3)
	for i, p := range parts {
		if len(p) == 0 || len(p) > 4 {
			return 0, 0, 0, false
		}
		n, err := strconv.ParseUint(p, 16, 16)
		if err != nil {
			return 0, 0, 0, false
		}
		vals[i] = float64(n) / float64(int64(1)<<(4*len(p))-1)
	}
	return vals[0], vals[1], vals[2], true
}
