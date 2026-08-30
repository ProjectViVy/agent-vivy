package app

import (
	"fmt"
	"sort"

	"agent-vivy/internal/config"
	"agent-vivy/sdk/plugin"
)

// partitionChannels splits the compiled plugin set by seam: a channel-seam
// plugin must implement plugin.Channel and becomes a ChannelHost tenant;
// every other plugin stays in the tool table (pluginhost.Adapt skips the
// channel seam). Every channels.<name> envelope key must name a
// compiled-in channel plugin, mirroring tools.Resolve: config names must
// resolve at startup (FR-10).
func partitionChannels(plugins []plugin.Plugin, channels config.Channels) ([]plugin.Channel, error) {
	byName := make(map[string]plugin.Channel)
	var out []plugin.Channel
	for _, p := range plugins {
		if p == nil {
			continue
		}
		if p.Seam() != plugin.SeamChannel {
			continue
		}
		ch, ok := p.(plugin.Channel)
		if !ok {
			return nil, fmt.Errorf("app: channel plugin %q does not implement plugin.Channel", p.Name())
		}
		if _, dup := byName[ch.Name()]; dup {
			return nil, fmt.Errorf("app: duplicate channel plugin name %q", ch.Name())
		}
		byName[ch.Name()] = ch
		out = append(out, ch)
	}
	names := make([]string, 0, len(channels))
	for name := range channels {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, ok := byName[name]; !ok {
			return nil, fmt.Errorf("app: channels.%s is not compiled into this generation", name)
		}
	}
	return out, nil
}
