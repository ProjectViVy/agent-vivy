// Species body of the main line: every first-party channel plugin ships
// compiled in, so `just run`, the embedded-UI binary and Docker open with
// all ears present. vivy-sdk pack replaces this exact file at build time
// (--with) to assemble a narrower generation; the committed body stays the
// full set. Hand-maintained — keep names in sync with plugins/.

package plugins

import (
	"agent-vivy/sdk/plugin"

	"example.com/vivy/plugins/dingtalk"
	"example.com/vivy/plugins/discord"
	"example.com/vivy/plugins/feishu"
	"example.com/vivy/plugins/qq"
	"example.com/vivy/plugins/telegram"
)

// Register returns the plugins compiled into this generation.
func Register() []plugin.Plugin {
	return []plugin.Plugin{
		dingtalk.New(),
		discord.New(),
		feishu.New(),
		qq.New(),
		telegram.New(),
	}
}
