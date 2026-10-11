package bundlecontract

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var envPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

func validText(s string, max int, required bool) bool {
	return utf8.ValidString(s) && len(s) <= max && (!required || strings.TrimSpace(s) != "")
}
func ValidateManifest(m Manifest, l Limits) error {
	if m.APIVersion != APIVersion || !bundleIDPattern.MatchString(m.ID) {
		return Invalid("identity")
	}
	if !validText(m.Version, l.VersionBytes, true) || !validText(m.Description, l.DescriptionBytes, false) || !validText(m.Name, 128, true) {
		return Invalid("metadata")
	}
	c := m.Components
	count := len(c.Skills) + len(c.Masks) + len(c.Rules) + len(c.Hooks) + len(c.MCP)
	if count == 0 || count > l.Components {
		return Invalid("components")
	}
	seen := map[ComponentKind]map[string]bool{}
	paths := map[string]string{}
	skills := map[string]bool{}
	add := func(k ComponentKind, id, p string) error {
		if !localIDPattern.MatchString(id) {
			return Invalid("component_id")
		}
		if seen[k] == nil {
			seen[k] = map[string]bool{}
		}
		if seen[k][id] {
			return Invalid("duplicate_component")
		}
		seen[k][id] = true
		if p != "" {
			if err := addPath(paths, p); err != nil {
				return err
			}
		}
		return nil
	}
	for _, s := range c.Skills {
		if s.Path == "" {
			return Invalid("skill_path")
		}
		if err := add(Skill, s.ID, s.Path); err != nil {
			return err
		}
		skills[s.ID] = true
	}
	for _, s := range c.Masks {
		if !validText(s.Name, int(l.MaskNameBytes), true) || !validText(s.Description, int(l.MaskDescriptionBytes), false) || s.Path == "" {
			return Invalid("mask")
		}
		if err := add(Mask, s.ID, s.Path); err != nil {
			return err
		}
		refs := map[string]bool{}
		for _, r := range s.RecommendedSkills {
			if !skills[r] || refs[r] {
				return Invalid("recommendedSkills")
			}
			refs[r] = true
		}
	}
	for _, s := range c.Rules {
		if s.Path == "" {
			return Invalid("rule_path")
		}
		if err := add(Rule, s.ID, s.Path); err != nil {
			return err
		}
		faces := map[string]bool{}
		for _, f := range s.Faces {
			if (f != "web" && f != "code" && f != "tui" && f != "headless") || faces[f] {
				return Invalid("faces")
			}
			faces[f] = true
		}
	}
	for _, s := range c.Hooks {
		if err := add(Hook, s.ID, ""); err != nil {
			return err
		}
		if s.Event != "PreToolUse" || !validText(s.Command, 16<<10, true) || s.TimeoutMs <= 0 || int64(s.TimeoutMs) > l.HookTimeoutMax.Milliseconds() {
			return Invalid("hook")
		}
		if s.Cwd != "" && s.Cwd != "workspace" {
			if err := ValidateRelativePath(s.Cwd); err != nil {
				return err
			}
		}
		if err := validateEnv(s.EnvFrom); err != nil {
			return err
		}
	}
	for _, s := range c.MCP {
		if err := add(MCP, s.ID, ""); err != nil {
			return err
		}
		if (s.Endpoint == "") == (s.Command == "") {
			return Invalid("mcp_transport")
		}
		if s.Endpoint != "" {
			u, err := url.Parse(s.Endpoint)
			if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
				return Invalid("mcp_endpoint")
			}
		}
		if s.AuthEnv != "" && !envPattern.MatchString(s.AuthEnv) {
			return Invalid("authEnv")
		}
		if s.Cwd != "" && s.Cwd != "workspace" {
			if err := ValidateRelativePath(s.Cwd); err != nil {
				return err
			}
		}
		if err := validateEnv(s.EnvFrom); err != nil {
			return err
		}
	}
	for _, p := range append(append([]string{}, m.Resources...), m.LicenseFiles...) {
		if err := addPath(paths, p); err != nil {
			return err
		}
	}
	if m.Provenance != "" {
		if err := addPath(paths, m.Provenance); err != nil {
			return err
		}
	}
	if len(paths)+1 > l.Files {
		return Invalid("files")
	}
	return nil
}
func addPath(paths map[string]string, p string) error {
	if err := ValidateRelativePath(p); err != nil {
		return err
	}
	if p == ManifestFile || p == "canonical-manifest.json" || p == "inventory.json" {
		return Invalid("reserved_path")
	}
	fold := strings.ToLower(p)
	for prev := range paths {
		if strings.EqualFold(prev, p) && prev != p {
			return Invalid("path_collision")
		}
	}
	// A file must not also be the directory of another declared file.
	for other := range paths {
		if strings.HasPrefix(other, fold+"/") || strings.HasPrefix(fold, other+"/") {
			return Invalid("path_collision")
		}
	}
	paths[fold] = p
	return nil
}
func validateEnv(env map[string]string) error {
	for k, v := range env {
		if !envPattern.MatchString(k) || !envPattern.MatchString(v) {
			return Invalid("envFrom")
		}
	}
	return nil
}

// DeclaredPaths is the explicit artifact file set, excluding the original
// manifest (whose canonical bytes have their own digest domain).
func DeclaredPaths(m Manifest) []string {
	set := map[string]bool{}
	for _, s := range m.Components.Skills {
		set[s.Path] = true
	}
	for _, s := range m.Components.Masks {
		set[s.Path] = true
	}
	for _, s := range m.Components.Rules {
		set[s.Path] = true
	}
	for _, p := range m.Resources {
		set[p] = true
	}
	for _, p := range m.LicenseFiles {
		set[p] = true
	}
	if m.Provenance != "" {
		set[m.Provenance] = true
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}
