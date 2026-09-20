package assembly_test

import (
	"testing"

	"agent-vivy/internal/channelcontract"
	"agent-vivy/internal/channelhost"
	channelmodule "agent-vivy/internal/modules/channel"
	"agent-vivy/sdk/port/channel"
	dingtalk "example.com/vivy/plugins/dingtalk"
	discord "example.com/vivy/plugins/discord"
	feishu "example.com/vivy/plugins/feishu"
	qq "example.com/vivy/plugins/qq"
	telegram "example.com/vivy/plugins/telegram"
)

// TestChannelProvidersAdvertiseExactlyTheirAdapterSurface pins the gate-0
// forwarding fix for the five compiled ears: capability discovery follows
// the CapabilitySource seam through the full bind chain (boundChannel ->
// providerChannel) and reports the adapter's own method set. CH-R-1 gave
// every adapter a HealthChecker; the tier-1 text loop added Typing where
// the platform has one (telegram, discord, qq — dingtalk and feishu have
// none), and the interact batch added Edit/Delete/Placeholder on the ears
// that can anchor them (telegram, discord, feishu — plus the Reaction pair
// on feishu; qq and dingtalk stay empty, §1). Each ear advertises exactly
// its adapter's surface, never a wrapper's. The rune ceilings pin the
// outbound split bound each Definition declares (sources in the
// module_v1.go comments).
func TestChannelProvidersAdvertiseExactlyTheirAdapterSurface(t *testing.T) {
	ears := []struct {
		name     string
		provider channel.ChannelProvider
		want     channelhost.Capabilities
		runes    int
	}{
		{"dingtalk", dingtalk.NewProvider(), channelhost.Capabilities{Health: true}, 5000},
		{"discord", discord.NewProvider(), channelhost.Capabilities{Health: true, Typing: true, Media: true, Edit: true, Delete: true, Placeholder: true}, 2000},
		{"feishu", feishu.NewProvider(), channelhost.Capabilities{Health: true, Media: true, Edit: true, Delete: true, Reaction: true, Placeholder: true}, 37500},
		{"qq", qq.NewProvider(), channelhost.Capabilities{Health: true, Typing: true, Media: true}, 2000},
		{"telegram", telegram.NewProvider(), channelhost.Capabilities{Health: true, Typing: true, Media: true, Edit: true, Delete: true, Placeholder: true}, 4096},
	}
	for _, ear := range ears {
		if got := ear.provider.Definition().MaxMessageRunes; got != ear.runes {
			t.Fatalf("%s: MaxMessageRunes = %d, want %d", ear.name, got, ear.runes)
		}
		bound, err := channelmodule.BindProviders([]channel.ChannelProvider{ear.provider}, nil, channelcontract.Config{})
		if err != nil || len(bound) != 1 {
			t.Fatalf("%s: bind = %v, err = %v", ear.name, bound, err)
		}
		if got := channelhost.Discover(bound[0]); got != ear.want {
			t.Fatalf("%s: capabilities = %+v, want %+v", ear.name, got, ear.want)
		}
		limited, ok := bound[0].(channel.RunesLimiter)
		if !ok || limited.MaxMessageRunes() != ear.runes {
			t.Fatalf("%s: bound MaxMessageRunes = %v, want %d", ear.name, limited, ear.runes)
		}
	}
}
