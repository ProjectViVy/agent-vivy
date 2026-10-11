package bundles

import (
	"agent-vivy/internal/bundlecontract"
	"bytes"
	"gopkg.in/yaml.v3"
	"io"
	"strings"
	"unicode/utf8"
)

type NativeSkill struct {
	Name        string
	Description string
	Body        string
}

func DecodeNativeSkill(raw []byte) (NativeSkill, error) {
	var out NativeSkill
	if !utf8.Valid(raw) || int64(len(raw)) > bundlecontract.DefaultLimits().SkillBytes {
		return out, bundlecontract.Invalid("skill_bytes")
	}
	s := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return out, bundlecontract.Invalid("skill_frontmatter")
	}
	end := strings.Index(s[4:], "\n---\n")
	if end < 0 {
		return out, bundlecontract.Invalid("skill_frontmatter")
	}
	header := s[4 : 4+end]
	out.Body = s[4+end+5:]
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(header), &doc); err != nil {
		return NativeSkill{}, &bundlecontract.Error{Code: "invalid_manifest", Field: "skill_frontmatter", Cause: err}
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return NativeSkill{}, bundlecontract.Invalid("skill_frontmatter")
	}
	seen := map[string]bool{}
	nodes := doc.Content[0].Content
	for i := 0; i < len(nodes); i += 2 {
		k, v := nodes[i], nodes[i+1]
		if k.Tag != "!!str" || v.Kind != yaml.ScalarNode || v.Tag != "!!str" || v.Anchor != "" || k.Anchor != "" || seen[k.Value] || (k.Value != "name" && k.Value != "description") {
			return NativeSkill{}, bundlecontract.Invalid("skill_frontmatter")
		}
		seen[k.Value] = true
	}
	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	d := yaml.NewDecoder(bytes.NewReader([]byte(header)))
	d.KnownFields(true)
	if err := d.Decode(&fm); err != nil {
		return NativeSkill{}, bundlecontract.Invalid("skill_frontmatter")
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return NativeSkill{}, bundlecontract.Invalid("skill_frontmatter")
	}
	if strings.TrimSpace(fm.Description) == "" {
		return NativeSkill{}, bundlecontract.Invalid("skill_description")
	}
	out.Name = fm.Name
	out.Description = fm.Description
	return out, nil
}
