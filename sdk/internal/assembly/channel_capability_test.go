package assembly_test

import (
	"testing"

	apphost "agent-vivy/internal/app"
	"agent-vivy/internal/channelhost"
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
// providerChannel) and reports the adapter's own method set. The v1
// text-only cut implements no optional interface, so every ear advertises
// the zero set today. When a batch lands a real capability on an adapter
// (tier1's CH-R-1 HealthChecker is first), this expectation flips with it —
// the advertised set must always be exactly the adapter's surface, never a
// wrapper's. The rune ceilings pin the outbound split bound each
// Definition declares (sources in the module_v1.go comments).
func TestChannelProvidersAdvertiseExactlyTheirAdapterSurface(t *testing.T) {
	ears := []struct {
		name     string
		provider channel.ChannelProvider
		want     channelhost.Capabilities
		runes    int
	}{
		{"dingtalk", dingtalk.NewProvider(), channelhost.Capabilities{}, 5000},
		{"discord", discord.NewProvider(), channelhost.Capabilities{}, 2000},
		{"feishu", feishu.NewProvider(), channelhost.Capabilities{}, 37500},
		{"qq", qq.NewProvider(), channelhost.Capabilities{}, 2000},
		{"telegram", telegram.NewProvider(), channelhost.Capabilities{}, 4096},
	}
	for _, ear := range ears {
		if got := ear.provider.Definition().MaxMessageRunes; got != ear.runes {
			t.Fatalf("%s: MaxMessageRunes = %d, want %d", ear.name, got, ear.runes)
		}
		bound, err := apphost.BindChannels([]channel.ChannelProvider{ear.provider}, nil, nil)
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
