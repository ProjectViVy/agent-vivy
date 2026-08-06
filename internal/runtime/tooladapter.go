package runtime

import (
	"context"
	"encoding/json"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"agent-vivy/internal/tools"
)

// toolAdapter exposes a Vivy tools.Tool to Eino's ToolsNode. Readonly tools
// execute directly here; effectful tools get an approval gate layered on
// top in C6 (D-012) — InvokableRun is only ever reached for calls that
// already passed policy.
type toolAdapter struct {
	t tools.Tool
}

var _ einotool.InvokableTool = (*toolAdapter)(nil)

func newToolAdapter(t tools.Tool) *toolAdapter {
	return &toolAdapter{t: t}
}

func (a *toolAdapter) Info(_ context.Context) (*schema.ToolInfo, error) {
	spec := a.t.Spec()
	return &schema.ToolInfo{Name: spec.Name, Desc: spec.Description}, nil
}

func (a *toolAdapter) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...einotool.Option) (string, error) {
	return a.t.InvokableRun(ctx, json.RawMessage(argumentsInJSON))
}
