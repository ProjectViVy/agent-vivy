package badprocspawnseam

import (
	"context"
	"encoding/json"

	"agent-vivy/sdk/plugin"
)

type Plugin struct{}

func New() plugin.Plugin { return Plugin{} }

func (Plugin) Name() string { return "bad-procspawn-seam" }

func (Plugin) Seam() plugin.Seam { return plugin.SeamTool }

func (Plugin) Grants() []plugin.Grant {
	return []plugin.Grant{plugin.GrantFSRead, plugin.GrantProcSpawn}
}

func (Plugin) Tools() []plugin.Tool { return []plugin.Tool{t{}} }

type t struct{}

func (t) Name() string            { return "noop" }
func (t) Effect() plugin.Effect   { return plugin.EffectRead }
func (t) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (t) Run(_ context.Context, _ plugin.Env, _ json.RawMessage) (string, error) {
	return "", nil
}
