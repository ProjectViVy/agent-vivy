package bundlecontract

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const APIVersion = "vivy.bundle/v1"
const ManifestFile = "vivy-bundle.json"

type Manifest struct {
	APIVersion   string     `json:"apiVersion"`
	ID           string     `json:"id"`
	Version      string     `json:"version"`
	Name         string     `json:"name"`
	Description  string     `json:"description,omitempty"`
	Components   Components `json:"components"`
	Resources    []string   `json:"resources"`
	Provenance   string     `json:"provenance,omitempty"`
	LicenseFiles []string   `json:"licenseFiles"`
}
type Components struct {
	Skills []SkillEntry `json:"skills"`
	Masks  []MaskEntry  `json:"masks"`
	Rules  []RuleEntry  `json:"rules"`
	Hooks  []HookEntry  `json:"hooks"`
	MCP    []MCPEntry   `json:"mcp"`
}
type SkillEntry struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}
type MaskEntry struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Description       string   `json:"description,omitempty"`
	Path              string   `json:"path"`
	RecommendedSkills []string `json:"recommendedSkills"`
}
type RuleEntry struct {
	ID    string   `json:"id"`
	Path  string   `json:"path"`
	Faces []string `json:"faces"`
}
type HookEntry struct {
	ID        string            `json:"id"`
	Event     string            `json:"event"`
	Command   string            `json:"command"`
	Matcher   string            `json:"matcher,omitempty"`
	Cwd       string            `json:"cwd,omitempty"`
	TimeoutMs int               `json:"timeoutMs"`
	EnvFrom   map[string]string `json:"envFrom,omitempty"`
}
type MCPEntry struct {
	ID             string            `json:"id"`
	Endpoint       string            `json:"endpoint,omitempty"`
	Command        string            `json:"command,omitempty"`
	Cwd            string            `json:"cwd,omitempty"`
	AuthEnv        string            `json:"authEnv,omitempty"`
	Args           []string          `json:"args"`
	EnvFrom        map[string]string `json:"envFrom,omitempty"`
	ResourceBridge bool              `json:"resourceBridge"`
}
type Limits struct {
	ManifestBytes, ProvenanceBytes, MaskBytes, MaskNameBytes, MaskDescriptionBytes, SkillBytes, RuleBytes, CombinedRuleBytes int64
	Files, Components                                                                                                        int
	TotalBytes, ResourceBytes                                                                                                int64
	VersionBytes, DescriptionBytes                                                                                           int
	HookTimeoutMax                                                                                                           time.Duration
}

func DefaultLimits() Limits {
	return Limits{ManifestBytes: 256 << 10, ProvenanceBytes: 256 << 10, MaskBytes: 16 << 10, MaskNameBytes: 128, MaskDescriptionBytes: 1024, SkillBytes: 512 << 10, RuleBytes: 16 << 10, CombinedRuleBytes: 64 << 10, Files: 4096, Components: 256, TotalBytes: 64 << 20, ResourceBytes: 8 << 20, VersionBytes: 128, DescriptionBytes: 4096, HookTimeoutMax: 30 * time.Second}
}

// strictJSON checks syntax, duplicate keys, nulls, depth and numeric encoding
// before decoding into the typed schema. A decoder's unknown-field check alone
// cannot reject duplicate authority-bearing keys.
func strictJSON(raw []byte) error {
	if !utf8.Valid(raw) {
		return Invalid("encoding")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 32 {
			return Invalid("depth")
		}
		t, err := d.Token()
		if err != nil {
			return Invalid("json")
		}
		switch v := t.(type) {
		case nil:
			return Invalid("null")
		case json.Number:
			if _, err := v.Int64(); err != nil {
				return Invalid("integer")
			}
		case json.Delim:
			switch v {
			case '{':
				keys := map[string]bool{}
				for d.More() {
					k, err := d.Token()
					if err != nil {
						return Invalid("json")
					}
					key, ok := k.(string)
					if !ok || keys[key] {
						return Invalid("duplicate_key")
					}
					keys[key] = true
					if err := value(depth + 1); err != nil {
						return err
					}
				}
				if end, err := d.Token(); err != nil || end != json.Delim('}') {
					return Invalid("json")
				}
			case '[':
				for d.More() {
					if err := value(depth + 1); err != nil {
						return err
					}
				}
				if end, err := d.Token(); err != nil || end != json.Delim(']') {
					return Invalid("json")
				}
			default:
				return Invalid("json")
			}
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return Invalid("trailing_document")
	}
	return nil
}
func DecodeManifest(raw []byte) (Manifest, error) {
	var m Manifest
	if int64(len(raw)) > DefaultLimits().ManifestBytes {
		return m, Invalid("manifest_bytes")
	}
	if err := strictJSON(raw); err != nil {
		return m, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return m, &Error{Code: "invalid_manifest", Field: "schema", Cause: err}
	}
	m = normalize(m)
	if err := ValidateManifest(m, DefaultLimits()); err != nil {
		return Manifest{}, err
	}
	return m, nil
}
func normalize(m Manifest) Manifest {
	if m.Resources == nil {
		m.Resources = []string{}
	}
	if m.LicenseFiles == nil {
		m.LicenseFiles = []string{}
	}
	if m.Components.Skills == nil {
		m.Components.Skills = []SkillEntry{}
	}
	if m.Components.Masks == nil {
		m.Components.Masks = []MaskEntry{}
	}
	if m.Components.Rules == nil {
		m.Components.Rules = []RuleEntry{}
	}
	if m.Components.Hooks == nil {
		m.Components.Hooks = []HookEntry{}
	}
	if m.Components.MCP == nil {
		m.Components.MCP = []MCPEntry{}
	}
	for i := range m.Components.Masks {
		if m.Components.Masks[i].RecommendedSkills == nil {
			m.Components.Masks[i].RecommendedSkills = []string{}
		}
	}
	for i := range m.Components.Rules {
		if m.Components.Rules[i].Faces == nil {
			m.Components.Rules[i].Faces = []string{}
		}
	}
	for i := range m.Components.Hooks {
		if m.Components.Hooks[i].Cwd == "" {
			m.Components.Hooks[i].Cwd = "workspace"
		}
	}
	for i := range m.Components.MCP {
		if m.Components.MCP[i].Cwd == "" {
			m.Components.MCP[i].Cwd = "workspace"
		}
		if m.Components.MCP[i].Args == nil {
			m.Components.MCP[i].Args = []string{}
		}
	}
	return m
}

func ValidateRelativePath(p string) error {
	if p == "" || !utf8.ValidString(p) || strings.ContainsAny(p, "\\:") || len(p) > 4096 {
		return Invalid("path")
	}
	for _, r := range p {
		if unicode.IsControl(r) {
			return Invalid("path")
		}
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return Invalid("path")
		}
		stem := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || stem == "CONIN$" || stem == "CONOUT$" || len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9' {
			return Invalid("path")
		}
		if strings.ContainsAny(part, `<>"|?*`) {
			return Invalid("path")
		}
	}
	return nil
}
