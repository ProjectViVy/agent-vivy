// Package hellofs is the sample v1 ToolWorld Module.
package hellofs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/toolworld"
)

const statToolID = "hello_stat"

type vivyModule struct{}

func New() module.Module { return vivyModule{} }
func (vivyModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return moduleInstance{}, nil
}

type moduleInstance struct{}

func (moduleInstance) Start(context.Context) error { return nil }
func (moduleInstance) Ready(context.Context) error { return nil }
func (moduleInstance) Stop(context.Context) error  { return nil }
func (moduleInstance) Close(context.Context) error { return nil }

type Provider struct{}

func NewProvider() toolworld.Provider { return Provider{} }
func (Provider) Definition() toolworld.Definition {
	return toolworld.Definition{ID: "vivy.hello-fs", Description: "Workspace file metadata tools"}
}
func (Provider) Discover(context.Context, toolworld.Host) ([]toolworld.ToolDefinition, error) {
	return []toolworld.ToolDefinition{{ID: statToolID, Description: "Report the byte length of one workspace-relative file", Effect: toolworld.EffectRead, Schema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)}}, nil
}
func (Provider) Invoke(ctx context.Context, env toolworld.Host, id string, args json.RawMessage) (toolworld.Result, error) {
	if id != statToolID {
		return toolworld.Result{}, toolworld.ErrInvalidArgs
	}
	text, err := statTool{}.Run(ctx, env, args)
	return toolworld.Result{Text: text}, err
}
func (Provider) Close(context.Context) error { return nil }

type statTool struct{}

func (statTool) Name() string { return "hello_stat" }

func (statTool) Effect() toolworld.Effect { return toolworld.EffectRead }

func (statTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)
}

func (statTool) Run(_ context.Context, env toolworld.Host, args json.RawMessage) (string, error) {
	var in struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &in); err != nil || !workspaceRel(in.Path) {
		return "", toolworld.ErrInvalidArgs
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
