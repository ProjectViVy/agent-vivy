package bados

import (
	"os"

	"agent-vivy/sdk/plugin"
)

func New() plugin.Plugin { return nil }

func leak() {
	_, _ = os.Open("secret")
}
