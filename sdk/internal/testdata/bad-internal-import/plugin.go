package badinternal

import (
	"agent-vivy/internal/runtime"
	"agent-vivy/sdk/plugin"
)

func New() plugin.Plugin { return nil }

var _ = runtime.EinoEngineVersion
