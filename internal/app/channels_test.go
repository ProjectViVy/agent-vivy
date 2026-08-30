package app

import (
	"context"
	"strings"
	"testing"

	"agent-vivy/internal/config"
	"agent-vivy/sdk/plugin"
)

// stubPlugin implements exactly plugin.Plugin — no Channel ABI.
type stubPlugin struct {
	name string
	seam plugin.Seam
}

func (s stubPlugin) Name() string           { return s.name }
func (s stubPlugin) Seam() plugin.Seam      { return s.seam }
func (s stubPlugin) Grants() []plugin.Grant { return nil }
func (s stubPlugin) Tools() []plugin.Tool   { return nil }

// stubChannel adds the plugin.Channel ABI on top of stubPlugin.
type stubChannel struct {
	stubPlugin
}

func (stubChannel) Start(_ context.Context, _ plugin.ChannelEnv) error { return nil }
func (stubChannel) Stop(_ context.Context) error                       { return nil }
func (stubChannel) Send(_ context.Context, _ plugin.OutboundMessage) ([]string, error) {
	return nil, nil
}

func TestPartitionChannelsHappyPath(t *testing.T) {
	tool := stubPlugin{name: "hello-fs", seam: plugin.SeamToolWorld}
	channel := stubChannel{stubPlugin{name: "fake", seam: plugin.SeamChannel}}
	out, err := partitionChannels([]plugin.Plugin{tool, channel}, config.Channels{
		"fake": {Enabled: true, AllowFrom: []string{"alice"}},
	})
	if err != nil {
		t.Fatalf("partition: %v", err)
	}
	if len(out) != 1 || out[0].Name() != "fake" {
		t.Fatalf("partitioned channels = %+v, want [fake]", out)
	}
}

func TestPartitionChannelsSkipsNilPlugins(t *testing.T) {
	channel := stubChannel{stubPlugin{name: "fake", seam: plugin.SeamChannel}}
	out, err := partitionChannels([]plugin.Plugin{nil, channel}, config.Channels{
		"fake": {Enabled: true, AllowFrom: []string{"alice"}},
	})
	if err != nil {
		t.Fatalf("partition: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("partitioned channels = %+v, want [fake]", out)
	}
}

func TestPartitionChannelsUnknownNameFailsStartup(t *testing.T) {
	channel := stubChannel{stubPlugin{name: "fake", seam: plugin.SeamChannel}}
	_, err := partitionChannels([]plugin.Plugin{channel}, config.Channels{
		"ghost": {Enabled: true, AllowFrom: []string{"alice"}},
	})
	if err == nil {
		t.Fatal("unknown channels.<name>: want error, got nil")
	}
	if !strings.Contains(err.Error(), "channels.ghost is not compiled into this generation") {
		t.Fatalf("error = %v, want the not-compiled-in message", err)
	}
}

func TestPartitionChannelsSeamChannelWithoutChannelABIFails(t *testing.T) {
	notChannel := stubPlugin{name: "broken", seam: plugin.SeamChannel}
	_, err := partitionChannels([]plugin.Plugin{notChannel}, nil)
	if err == nil {
		t.Fatal("channel-seam plugin without plugin.Channel: want error, got nil")
	}
	if !strings.Contains(err.Error(), `channel plugin "broken" does not implement plugin.Channel`) {
		t.Fatalf("error = %v, want the missing-ABI message", err)
	}
}
