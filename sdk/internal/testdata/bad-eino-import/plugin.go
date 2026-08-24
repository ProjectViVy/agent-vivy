package badeino

import (
	"github.com/cloudwego/eino/adk"

	"agent-vivy/sdk/plugin"
)

func New() plugin.Plugin { return nil }

var _ = adk.NewRunner
