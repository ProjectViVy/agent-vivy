// Package hellofs is the S5 sample user plugin. It only imports the SDK
// window and reads through Env.
package hellofs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"agent-vivy/sdk/plugin"
)

type Plugin struct{}

func New() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "hello-fs" }

func (Plugin) Seam() plugin.Seam { return plugin.SeamToolWorld }

func (Plugin) Grants() []plugin.Grant { return []plugin.Grant{plugin.GrantFSRead} }

func (Plugin) Tools() []plugin.Tool { return []plugin.Tool{statTool{}} }

type statTool struct{}

func (statTool) Name() string { return "hello_stat" }

func (statTool) Effect() plugin.Effect { return plugin.EffectRead }

func (statTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)
}

func (statTool) Run(_ context.Context, env plugin.Env, args json.RawMessage) (string, error) {
	var in struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &in); err != nil || !workspaceRel(in.Path) {
		return "", plugin.ErrInvalidArgs
	}
	rc, err := env.OpenRead(in.Path)
	if err != nil {
		return "", err
	}
	defer func() { _ = rc.Close() }()
	body, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %d", in.Path, len(body)), nil
}

func workspaceRel(p string) bool {
	if p == "" || path.IsAbs(p) || filepath.IsAbs(p) || strings.ContainsAny(p, `:\`) || strings.HasPrefix(p, "/") {
		return false
	}
	cleaned := path.Clean(p)
	return cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}
