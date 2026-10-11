package bundlecontract

import (
	"errors"
	"strings"
	"testing"
)

const minimal = `{"apiVersion":"vivy.bundle/v1","id":"example/experts","version":"1","name":"Experts","components":{"skills":[{"id":"plan","path":"skills/plan/SKILL.md"}]}}`

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Code != code {
		t.Fatalf("got %v, want %s", err, code)
	}
}

func TestDecodeManifestRejectsAmbiguity(t *testing.T) {
	m, err := DecodeManifest([]byte(minimal))
	if err != nil {
		t.Fatal(err)
	}
	if m.Components.Masks == nil || m.Resources == nil {
		t.Fatal("omitted arrays must normalize")
	}
	cases := []string{
		strings.Replace(minimal, `"id":"example/experts"`, `"id":"example/experts","id":"evil/experts"`, 1),
		strings.Replace(minimal, `"name":"Experts"`, `"name":"Experts","approved":true`, 1),
		minimal + `{}`,
		strings.Replace(minimal, `vivy.bundle/v1`, `vivy.plugin/v0`, 1),
		strings.Replace(minimal, `"skills":`, `"unknown":`, 1),
		strings.Replace(minimal, `"skills":[`, `"rules":[{"id":"r","path":"r.md","faces":["unknown"]}],"skills":[`, 1),
		strings.Replace(minimal, `"version":"1"`, `"version":1.0`, 1),
		strings.Replace(minimal, `"version":"1"`, `"version":null`, 1),
		strings.Replace(minimal, `"skills":[`, `"hooks":[{"id":"h","event":"PostToolUse","command":"echo hi","timeoutMs":1}],"skills":[`, 1),
	}
	for i, raw := range cases {
		t.Run(strings.Repeat("x", i+1), func(t *testing.T) { _, err := DecodeManifest([]byte(raw)); requireCode(t, err, "invalid_manifest") })
	}
}

func TestBundleIDsAreUnambiguous(t *testing.T) {
	a := "01234567-89ab-4def-8123-456789abcdef"
	b := "01234567-89ab-4def-8123-456789abcdea"
	x, err := SkillID(a, "plan")
	if err != nil {
		t.Fatal(err)
	}
	y, _ := SkillID(b, "plan")
	if x == y || x != "b_0123456789ab4def8123456789abcdef_plan" {
		t.Fatal(x, y)
	}
	mcp, _ := MCPInstanceID(a, "plan")
	if mcp != x {
		t.Fatal(mcp)
	}
	mask, err := MaskID(a, strings.Repeat("a", 24))
	if err != nil || len(mask) > 128 {
		t.Fatal(mask, err)
	}
	for _, id := range []string{"Plan", "a/b", "a_b", strings.Repeat("a", 25)} {
		_, err := SkillID(a, id)
		requireCode(t, err, "invalid_manifest")
	}
	for _, id := range []string{"not-a-uuid", strings.ToUpper(a), "00000000-0000-0000-0000-000000000000"} {
		_, err := SkillID(id, "plan")
		requireCode(t, err, "invalid_manifest")
	}
	m, _ := DecodeManifest([]byte(minimal))
	m.Components.Masks = []MaskEntry{{ID: "plan", Name: "Planner", Path: "masks/plan.md", RecommendedSkills: []string{"plan"}}}
	if err := ValidateManifest(m, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	m.Components.Skills = append(m.Components.Skills, m.Components.Skills[0])
	requireCode(t, ValidateManifest(m, DefaultLimits()), "invalid_manifest")
}

func TestManifestPathsAndBounds(t *testing.T) {
	for _, p := range []string{"/a", "C:/a", "a\\b", "a/../b", "a//b", "a/./b", "CON", "aux.txt", "a.", "a ", "a\x00b", "a:b"} {
		requireCode(t, ValidateRelativePath(p), "invalid_manifest")
	}
	if err := ValidateRelativePath("skills/plan/SKILL.md"); err != nil {
		t.Fatal(err)
	}
	m, _ := DecodeManifest([]byte(minimal))
	m.Version = strings.Repeat("a", 128)
	if err := ValidateManifest(m, DefaultLimits()); err != nil {
		t.Fatal(err)
	}
	m.Version += "a"
	requireCode(t, ValidateManifest(m, DefaultLimits()), "invalid_manifest")
	m, _ = DecodeManifest([]byte(minimal))
	m.Resources = []string{"skills/PLAN/SKILL.md"}
	requireCode(t, ValidateManifest(m, DefaultLimits()), "invalid_manifest")
	m, _ = DecodeManifest([]byte(minimal))
	m.Components.Masks = []MaskEntry{{ID: "pm", Name: "PM", Path: "pm.md", RecommendedSkills: []string{"missing"}}}
	requireCode(t, ValidateManifest(m, DefaultLimits()), "invalid_manifest")
}
