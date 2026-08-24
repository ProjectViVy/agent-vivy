package sdk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"agent-vivy/sdk/plugin"
)

const apiVersionV0 = "vivy.plugin/v0"

var (
	namePattern    = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z]+)*$`)
)

type manifest struct {
	APIVersion string         `json:"apiVersion"`
	Name       string         `json:"name"`
	Version    string         `json:"version"`
	Seam       string         `json:"seam"`
	Module     string         `json:"module"`
	Grants     []string       `json:"grants"`
	Tools      []manifestTool `json:"tools"`
}

type manifestTool struct {
	Name        string          `json:"name"`
	Effect      string          `json:"effect"`
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

func loadManifest(dir string) (manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "vivy-plugin.json"))
	if err != nil {
		return manifest{}, fmt.Errorf("sdk: read vivy-plugin.json: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return manifest{}, fmt.Errorf("sdk: parse vivy-plugin.json: %w", err)
	}
	return m, nil
}

func checkManifest(dir string, m manifest) []string {
	var issues []string
	if m.APIVersion != apiVersionV0 {
		issues = append(issues, fmt.Sprintf("apiVersion %q is not %s", m.APIVersion, apiVersionV0))
	}
	base := filepath.Base(dir)
	if m.Name != base {
		issues = append(issues, fmt.Sprintf("name %q does not match directory %q", m.Name, base))
	}
	if !namePattern.MatchString(m.Name) {
		issues = append(issues, fmt.Sprintf("name %q must be lowercase digits and hyphens", m.Name))
	}
	if !versionPattern.MatchString(m.Version) {
		issues = append(issues, fmt.Sprintf("version %q is not semver", m.Version))
	}
	seam := plugin.Seam(m.Seam)
	if !seam.Valid() {
		issues = append(issues, fmt.Sprintf("seam %q is forbidden or unknown", m.Seam))
	}
	if strings.TrimSpace(m.Module) == "" {
		issues = append(issues, "module is required")
	}
	for _, grant := range m.Grants {
		if !plugin.Grant(grant).Valid() {
			issues = append(issues, fmt.Sprintf("grant %q is unknown", grant))
		}
	}
	if len(m.Tools) == 0 {
		issues = append(issues, "tools must list at least one tool")
	}
	seen := map[string]struct{}{}
	for i, tool := range m.Tools {
		if tool.Name == "" {
			issues = append(issues, fmt.Sprintf("tools[%d].name is required", i))
		} else if _, ok := seen[tool.Name]; ok {
			issues = append(issues, fmt.Sprintf("tools[%d].name %q is duplicated", i, tool.Name))
		}
		seen[tool.Name] = struct{}{}
		if !plugin.Effect(tool.Effect).Valid() {
			issues = append(issues, fmt.Sprintf("tools[%d].effect %q must be read or write", i, tool.Effect))
		}
		if !schemaObject(tool.Schema) {
			issues = append(issues, fmt.Sprintf("tools[%d].schema must be a JSON object", i))
		}
	}
	return issues
}

func schemaObject(raw json.RawMessage) bool {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return false
	}
	return true
}
