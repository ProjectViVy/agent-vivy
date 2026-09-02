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
	APIVersion string           `json:"apiVersion"`
	Name       string           `json:"name"`
	Version    string           `json:"version"`
	Seam       string           `json:"seam"`
	Module     string           `json:"module"`
	Grants     []string         `json:"grants"`
	Tools      []manifestTool   `json:"tools"`
	Channel    *manifestChannel `json:"channel"`
	Face       *manifestFace    `json:"face"`
}

// manifestChannel is the channel envelope of a seam-channel manifest
// (VIVY-CHANNEL-PACK.md §9.2).
type manifestChannel struct {
	Transport       string `json:"transport"`
	MaxMessageRunes int    `json:"max_message_runes"`
}

// manifestFace is the face envelope of a seam-face manifest
// (VIVY-FACE-PACK.md §6). Kind names the presentation family and must
// match the organ's plugin.Face Kind(); Listen must stay false in this
// batch — HTTP listening is a face's own effect (e.g. faces/web), never
// the kernel's obligation.
type manifestFace struct {
	Kind   string `json:"kind"`
	Listen bool   `json:"listen"`
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
			continue
		}
		// Channel-family grants exist only for the channel seam; no other
		// seam may declare them. proc.spawn is the tool-world sibling
		// restriction (VC-3, D4): only a tool-world plugin may run child
		// processes, because only there a spawned language server's tools
		// are model-reachable.
		if isChannelGrant(grant) && seam != plugin.SeamChannel {
			issues = append(issues, fmt.Sprintf("grant %q is not available to seam %q", grant, m.Seam))
		}
		if grant == string(plugin.GrantProcSpawn) && seam != plugin.SeamToolWorld {
			issues = append(issues, fmt.Sprintf("grant %q is not available to seam %q", grant, m.Seam))
		}
	}
	switch seam {
	case plugin.SeamChannel:
		return append(issues, checkChannelManifest(m)...)
	case plugin.SeamFace:
		return append(issues, checkFaceManifest(m)...)
	default:
		return append(issues, checkToolManifest(m)...)
	}
}

// isChannelGrant reports whether a grant belongs to the channel family.
func isChannelGrant(grant string) bool {
	switch plugin.Grant(grant) {
	case plugin.GrantChannelPoll, plugin.GrantChannelWebhook, plugin.GrantChannelListen, plugin.GrantChannelA2A, plugin.GrantSecretRead:
		return true
	default:
		return false
	}
}

// isFaceGrant reports whether a grant belongs to the face family
// (VIVY-FACE-PACK.md §6): terminal, argv, and control-plane client.
func isFaceGrant(grant string) bool {
	switch plugin.Grant(grant) {
	case plugin.GrantTTY, plugin.GrantArgv, plugin.GrantRPCClient:
		return true
	default:
		return false
	}
}

// checkToolManifest holds the seam-tool/tool-world/provider rules: a
// non-channel seam is a model tool source and must list at least one tool.
func checkToolManifest(m manifest) []string {
	var issues []string
	if m.Face != nil {
		issues = append(issues, "face object requires seam face")
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

// checkChannelManifest holds the seam-channel rules (VIVY-CHANNEL-PACK.md
// §9.2): a channel is consumed by the kernel ChannelHost, not the model
// tool table, so it forbids tools, restricts grants, and requires a
// channel object with a transport this batch allows.
func checkChannelManifest(m manifest) []string {
	var issues []string
	if len(m.Tools) > 0 {
		issues = append(issues, "seam channel forbids tools (channel is not a model tool)")
	}
	seen := map[string]struct{}{}
	for _, grant := range m.Grants {
		if _, ok := seen[grant]; ok {
			issues = append(issues, fmt.Sprintf("grant %q is duplicated", grant))
		}
		seen[grant] = struct{}{}
		switch plugin.Grant(grant) {
		case plugin.GrantChannelPoll, plugin.GrantSecretRead:
		case plugin.GrantChannelWebhook, plugin.GrantChannelListen, plugin.GrantChannelA2A:
			issues = append(issues, fmt.Sprintf("grant %q is not allowed in this batch (only channel.poll and secret.read)", grant))
		}
	}
	if m.Channel == nil {
		issues = append(issues, `seam channel requires a "channel" object with transport "poll"`)
		return issues
	}
	switch m.Channel.Transport {
	case "poll":
	case "":
		issues = append(issues, `channel.transport must be "poll" in this batch`)
	default:
		issues = append(issues, fmt.Sprintf("channel.transport %q is not allowed in this batch (only %q)", m.Channel.Transport, "poll"))
	}
	if m.Channel.MaxMessageRunes < 0 {
		issues = append(issues, "channel.max_message_runes must not be negative")
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

// checkFaceManifest holds the seam-face rules (VIVY-FACE-PACK.md §6): a
// face is consumed by the kernel FaceHost as a control-plane client, so
// it forbids tools, admits only face-family grants, and requires a face
// object with a known kind and no listening effect.
func checkFaceManifest(m manifest) []string {
	var issues []string
	if len(m.Tools) > 0 {
		issues = append(issues, "seam face forbids tools (a face is a control-plane client, not a model tool)")
	}
	seen := map[string]struct{}{}
	for _, grant := range m.Grants {
		if _, ok := seen[grant]; ok {
			issues = append(issues, fmt.Sprintf("grant %q is duplicated", grant))
		}
		seen[grant] = struct{}{}
		if !isFaceGrant(grant) {
			issues = append(issues, fmt.Sprintf("grant %q is not available to seam %q", grant, m.Seam))
		}
	}
	if m.Face == nil {
		issues = append(issues, `seam face requires a "face" object with kind web|tui|headless`)
		return issues
	}
	switch m.Face.Kind {
	case "web", "tui", "headless":
	default:
		issues = append(issues, fmt.Sprintf("face.kind %q is not a face family (web|tui|headless)", m.Face.Kind))
	}
	if m.Face.Listen {
		issues = append(issues, "face.listen must be false in this batch (listening is the face's own effect)")
	}
	return issues
}
