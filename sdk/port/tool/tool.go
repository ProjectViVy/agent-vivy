// Package tool defines the focused std/tool@v1 Provider contract.
package tool

import (
	"context"
	"encoding/json"

	"agent-vivy/sdk/module"
)

type Effect string

const (
	EffectRead  Effect = "read"
	EffectWrite Effect = "write"
)

type Definition struct {
	ID          string
	Description string
	Effect      Effect
	Schema      json.RawMessage
}

type ToolProvider interface {
	Definition() Definition
	Invoke(context.Context, Host, json.RawMessage) (Result, error)
}

type Host interface {
	module.Host
	InvokeTool(context.Context, string, json.RawMessage) (string, error)
}
type Result struct{ Text string }
